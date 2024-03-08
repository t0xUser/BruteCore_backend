package repository

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"io/ioutil"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"api.brutecore/libs/lib_db"
)

type ComboListLink struct {
	LinkID     int64  `json:"id"`
	LinkStatus string `json:"status"`
	LinkType   string `json:"type"`
	LinkCount  int64  `json:"count"`
	LinkPath   string `json:"path"`
	LinkError  string `json:"err_txt,omitempty"`
}

type MComboListData struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	P_ID       *int64          `json:"p_id"`
	CreateTime time.Time       `json:"create_time"`
	Type       string          `json:"type"`
	DataType   *string         `json:"data_type"`
	Status     string          `json:"status"`
	LinesCount int64           `json:"lines_count"`
	Links      []ComboListLink `json:"links"`
}

type Tree []map[string]interface{}
type Statistic map[string]interface{}

type LISTRepositoryLayer struct {
	db *lib_db.DB
}

func NewLISTRepositoryLayer(d *lib_db.DB) *LISTRepositoryLayer {
	return &LISTRepositoryLayer{
		db: d,
	}
}

func (r LISTRepositoryLayer) Serialize(res *lib_db.DBResult, id int64) Tree {
	var tr Tree
	for _, item := range *res {
		if item["p_id"] == id {
			item["child"] = r.Serialize(res, item["id"].(int64))
			tr = append(tr, item)
		}
	}
	return tr
}

func (r LISTRepositoryLayer) GetComboList() (*Tree, error) {
	res, err := r.db.QueryRow(lib_db.TxRead, QTreeComboList)
	if err != nil {
		return nil, err
	}

	var tree Tree
	for _, item := range *res {
		if item["p_id"] == nil {
			item["child"] = r.Serialize(res, item["id"].(int64))
			tree = append(tree, item)
		}
	}

	return &tree, nil
}

func (r LISTRepositoryLayer) GetInfoComboList(sID string) (interface{}, error) {
	id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return nil, err
	}

	res, err := r.db.QueryRow(lib_db.TxRead, QComboListInfo, id)
	if err != nil {
		return nil, err
	}
	if res.Count() == 0 {
		return nil, errors.New("No data found")
	}
	info := (*res)[0]

	res, err = r.db.QueryRow(lib_db.TxRead, QComboListLinksInfo, id)
	if err != nil {
		return nil, err
	}
	if res.Count() != 0 {
		info["links"] = res
	}
	return &info, nil
}

func (r LISTRepositoryLayer) DeleteComboList(sID string) error {
	id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return err
	}

	res, err := r.db.QueryRow(lib_db.TxRead, QisAbleToDelete, id, id)
	if err != nil {
		return err
	}

	switch (*res)[0]["isAble"].(int64) {
	case 1:
		return errors.New("Сначало удалите дочерние элементы")
	case 2:
		return errors.New("Нельзя удалить лист по которому есть сессии")
	}

	_, err = r.db.Exec(lib_db.TxWrite, QDeleteComboList, id, id, id)
	_, _ = r.db.Exec(lib_db.TxWrite, QVacuum)
	return err
}

func leftPad(s string, length int, padChar rune) string {
	return strings.Repeat(string(padChar), length-len(s)) + s
}

func (r LISTRepositoryLayer) threadUpload(DatabaseId int64) {
	res, _ := r.db.QueryRow(lib_db.TxRead, QGetDatabaseLinks, DatabaseId)
	for _, item := range *res {
		id := item["id"].(int64)
		link := item["link"].(string)
		link_type := item["link_type"].(string)

		r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST2", nil, DatabaseId, id)

		var (
			body []byte
			err  error
		)

		switch link_type {
		case "LT2":
			body, err = ioutil.ReadFile(link)
			if err != nil {
				r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST3", "Ошибка чтения файла", DatabaseId, id)
				continue
			}

		case "LT3":
			response, err := http.Get(link)
			if err != nil {
				r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST3", "Ошибка соединения", DatabaseId, id)
				continue
			}
			defer response.Body.Close()

			body, err = ioutil.ReadAll(response.Body)
			if err != nil {
				r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST3", "Ошибка чтения", DatabaseId, id)
				continue
			}

		default:
			r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST3", "Неизвестный тип", DatabaseId, id)
			continue
		}

		bodyReader := bytes.NewReader(body)
		scanner := bufio.NewScanner(bodyReader)

		tx, err := r.db.StartTx(lib_db.TxWrite)
		if err != nil {
			r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST3", "Ошибка транзакции", DatabaseId, id)
			continue
		}

		trueTx := tx.(*sql.Tx)
		I := int64(0)
		for scanner.Scan() {
			I++
			paddedStr := leftPad(strconv.FormatInt(id, 10), 3, '0') + leftPad(strconv.FormatInt(I, 10), 15, '0')
			trueTx.Exec(QInsertDatabaseLinkData, DatabaseId, id, I, scanner.Text(), paddedStr)
		}

		if err := scanner.Err(); err != nil {
			trueTx.Rollback()
			r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST3", "Ошибка сканирования", DatabaseId, id)
			continue
		}

		err = trueTx.Commit()
		if err != nil {
			r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST3", "Ошибка коммита", DatabaseId, id)
			continue
		}

		r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST4", nil, DatabaseId, id)
	}
	r.db.Exec(lib_db.TxWrite, QUpdateDatabase, DatabaseId, DatabaseId, DatabaseId)
}

func (r LISTRepositoryLayer) UploadComboList(resp *MComboListData) error {
	if resp.Name == "" {
		return errors.New("Укажите наименование файла/папки")
	}

	if resp.Type != "CL1" && resp.Type != "CL2" {
		return errors.New("Укажите тип элемента, который хотите записать")
	}

	if resp.P_ID != nil {
		res, err := r.db.QueryRow(lib_db.TxRead, QGetDatabaseType, resp.P_ID)
		if err != nil {
			return err
		}

		i := (*res)[0]["type"].(string)
		if i != "CL1" {
			return errors.New("Родителем может быть только папка")
		}
	}

	if resp.Type == "CL1" {
		resp.DataType = nil
	}

	if resp.Type == "CL2" {
		if resp.DataType == nil || !slices.Contains([]string{"DT1", "DT2", "DT3", "DT4", "DT5", "DT6", "DT7", "DT8"}, *resp.DataType) {
			return errors.New("Укажите тип данных файла")
		}

		if len(resp.Links) < 1 {
			return errors.New("Укажите ссылки для загрузки")
		}

		for _, item := range resp.Links {
			if !slices.Contains([]string{"LT1", "LT2", "LT3"}, item.LinkType) || len(item.LinkPath) < 3 {
				return errors.New("Перепроверьте указанные источники")
			}
		}
	}

	tx, err := r.db.StartTx(lib_db.TxWrite)
	if err != nil {
		return err
	}

	trueTx := tx.(*sql.Tx)
	res, err := trueTx.Exec(QInsertDatabase, resp.Name, resp.P_ID, resp.Type, resp.DataType, "ST1")
	if err != nil {
		trueTx.Rollback()
		return err
	}
	resp.ID, _ = res.LastInsertId()
	resp.CreateTime = time.Now()

	for i, item := range resp.Links {
		_, err = trueTx.Exec(QInsertDatabaseLink, resp.ID, i+1, item.LinkPath, "ST1", item.LinkType)
		if err != nil {
			trueTx.Rollback()
			return err
		}
	}

	err = trueTx.Commit()
	if err != nil {
		return err
	}

	go r.threadUpload(resp.ID)
	return nil
}
