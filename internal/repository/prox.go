package repository

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"io/ioutil"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"time"

	"api.brutecore/internal/utility"
	"api.brutecore/libs/lib_db"
)

type ProxyLink struct {
	LinkID         int64  `json:"id"`
	LinkStatus     int64  `json:"status"`
	LinkSourceType string `json:"source_type"`
	LinkProxyType  string `json:"proxy_type"`
	LinkCount      int64  `json:"count"`
	LinkPath       string `json:"path"`
	LinkError      string `json:"err_txt,omitempty"`
	LinkData       string `json:"data,omitempty"`
}

type MProxyPreset struct {
	ID         int64        `json:"id"`
	Name       string       `json:"name"`
	CreateTime time.Time    `json:"create_time"`
	AutoUpdate bool         `json:"use_update"`
	Interval   int64        `json:"interval"`
	TimeOut    int64        `json:"timeout"`
	LinesCount int64        `json:"lines_count"`
	Links      *[]ProxyLink `json:"links"`
}

type PROXRepositoryLayer struct {
	db *lib_db.DB
}

func NewPROXRepositoryLayer(d *lib_db.DB) *PROXRepositoryLayer {
	return &PROXRepositoryLayer{
		db: d,
	}
}

func (r *PROXRepositoryLayer) GetProxyPresets() (*lib_db.DBResult, error) {
	return r.db.QueryRow(lib_db.TxRead, QProxyPresets)
}

func (r *PROXRepositoryLayer) GetInfoProxyPreset(sID string) (interface{}, error) {
	id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return nil, err
	}

	res, err := r.db.QueryRow(lib_db.TxRead, QProxyPresetInfo, id)
	if err != nil {
		return nil, err
	}

	if res.Count() == 0 {
		return nil, errors.New("No data found")
	}

	info := (*res)[0]
	info["use_update"] = ((*res)[0]["use_update"].(int64) == 1)

	res, err = r.db.QueryRow(lib_db.TxRead, QProxyPresetLinksInfo, id)
	if err != nil {
		return nil, err
	}
	if res.Count() != 0 {
		info["links"] = res
	}

	return &info, nil
}

func (r *PROXRepositoryLayer) DeleteProxyPreset(sID string) error {
	id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(lib_db.TxWrite, QDeleteProxyPreset, id, id, id)
	_, _ = r.db.Exec(lib_db.TxWrite, QVacuum)
	return err
}

func (r *PROXRepositoryLayer) threadUpload(resp *MProxyPreset) {
	for _, item := range *resp.Links {
		var (
			body []byte
			err  error
			msg  string
			tx   interface{}
		)

		switch item.LinkSourceType {
		case "LT1":
			body, err = base64.StdEncoding.DecodeString(item.LinkData)
			msg = "Ошибка расшифровки файла"
		case "LT2":
			body, err = ioutil.ReadFile(item.LinkPath)
			msg = "Ошибка чтения файла"
		case "LT3":
			response, err := http.Get(item.LinkPath)
			if err != nil {
				r.db.Exec(lib_db.TxWrite, QUpdateProxyPresetLinkStatus, "ST3", "Ошибка соединения", resp.ID, item.LinkID)
				continue
			}
			defer response.Body.Close()

			body, err = ioutil.ReadAll(response.Body)
			msg = "Ошибка чтения"
		default:
			err = errors.New("default")
			msg = "Неизвестный тип"
		}

		if err != nil {
			r.db.Exec(lib_db.TxWrite, QUpdateProxyPresetLinkStatus, "ST3", msg, resp.ID, item.LinkID)
			continue
		}

		if tx, err = r.db.StartTx(lib_db.TxWrite); err != nil {
			r.db.Exec(lib_db.TxWrite, QUpdateProxyPresetLinkStatus, "ST3", "Ошибка транзакции", resp.ID, item.LinkID)
			continue
		}

		trueTx := tx.(*sql.Tx)
		I := int64(0)
		re := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:\d{1,5}`)
		matches := re.FindAllString(string(body), -1)
		for _, match := range matches {
			if host, port, err := utility.SplitProxy(match); err == nil {
				I++
				trueTx.Exec(QInsertProxyPresetLinkData, resp.ID, item.LinkID, I, host, port)
			}
		}

		if err = trueTx.Commit(); err != nil {
			r.db.Exec(lib_db.TxWrite, QUpdateProxyPresetLinkStatus, "ST3", "Ошибка коммита", resp.ID, item.LinkID)
			continue
		}

		r.db.Exec(lib_db.TxWrite, QUpdateProxyPresetLinkStatus, "ST4", nil, resp.ID, item.LinkID)
	}
}

func (r *PROXRepositoryLayer) WriteProxyAndLinks(resp *MProxyPreset) error {
	tx, err := r.db.StartTx(lib_db.TxWrite)
	if err != nil {
		return err
	}

	trueTx := tx.(*sql.Tx)
	res, err := trueTx.Exec(QInsertProxy, resp.Name, resp.Interval, resp.TimeOut, utility.BoolToInt(resp.AutoUpdate))
	if err != nil {
		trueTx.Rollback()
		return err
	}
	resp.ID, _ = res.LastInsertId()

	for _, item := range *resp.Links {
		if _, err = trueTx.Exec(QInsertProxyLink, resp.ID, item.LinkID, item.LinkSourceType, item.LinkProxyType, item.LinkPath); err != nil {
			trueTx.Rollback()
			return err
		}
	}

	err = trueTx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (r *PROXRepositoryLayer) UploadProxyPreset(resp *MProxyPreset) (int64, error) {
	if resp.Name == "" {
		return 0, errors.New("Укажите наименование пресета")
	}

	if resp.TimeOut == 0 {
		return 0, errors.New("Укажите таймаут")
	}

	if resp.AutoUpdate == true && resp.Interval < 10 {
		return 0, errors.New("Укажите интервал автообновления прокси")
	}

	if resp.Links == nil {
		return 0, errors.New("Укажите хотя бы один источник")
	}

	for lid, item := range *resp.Links {
		if !slices.Contains([]string{"LT1", "LT2", "LT3"}, item.LinkSourceType) || len(item.LinkPath) < 3 ||
			!slices.Contains([]string{"PT1", "PT2", "PT3"}, item.LinkProxyType) {
			return 0, errors.New("Перепроверьте указанные источники")
		}

		if item.LinkSourceType == "LT1" && item.LinkData == "" {
			return 0, errors.New("Должен быть загружен файл")
		}

		(*resp.Links)[lid].LinkID = int64(lid + 1)
	}

	if err := r.WriteProxyAndLinks(resp); err != nil {
		return 0, err
	}

	go r.threadUpload(resp)
	return resp.ID, nil
}
