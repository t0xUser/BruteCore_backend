package repository

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"api.brutecore/libs/lib_db"
	"github.com/gofiber/fiber/v2"
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

func (r LISTRepositoryLayer) threadUpload(DatabaseId int64, files map[int]*multipart.FileHeader) {
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
		case "LT1":
			s, err := files[int(id)].Open()
			if err != nil {
				r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST3", "Ошибка чтения мультипарта", DatabaseId, id)
				continue
			}
			body, err = ioutil.ReadAll(s)
			if err != nil {
				r.db.Exec(lib_db.TxWrite, QUpdateDatabaseLinkStatus, "ST3", "Ошибка чтения мультипарта", DatabaseId, id)
				continue
			}
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

func (r LISTRepositoryLayer) UploadComboListFormData(c *fiber.Ctx) (*string, error) {
	name := c.FormValue("name")
	p_id := c.FormValue("p_id")
	ctype := c.FormValue("type")
	data_type := c.FormValue("data_type")
	id_types := c.FormValue("id_types")

	if name == "" {
		return nil, errors.New("Укажите наименование файла/папки")
	}

	if p_id != "" && p_id != "undefined" {
		res, err := r.db.QueryRow(lib_db.TxRead, QGetDatabaseType, p_id)
		if err != nil {
			return nil, err
		}

		i := (*res)[0]["type"].(string)
		if i != "CL1" {
			return nil, errors.New("Родителем может быть только папка")
		}
	}

	if ctype != "CL1" && ctype != "CL2" {
		return nil, errors.New("Укажите тип элемента, который хотите записать")
	}

	var rid *string
	if p_id != "" && p_id != "undefined" {
		rid = &p_id
	}

	if ctype == "CL1" {
		data_type = ""
	}

	m := make(map[string]interface{})
	if ctype == "CL2" {
		if data_type == "" || !slices.Contains([]string{"DT1", "DT2", "DT3", "DT4", "DT5", "DT6", "DT7", "DT8"}, data_type) {
			return nil, errors.New("Укажите тип данных файла")
		}

		if id_types == "" {
			return nil, errors.New("Укажите источники")
		}

		pairs := strings.Split(id_types, ";")
		for _, val := range pairs {
			k := strings.Split(val, ":")
			if len(k) != 2 {
				continue
			}

			if !slices.Contains([]string{"LT1", "LT2", "LT3"}, k[1]) {
				return nil, errors.New("Перепроверьте указанные источники")
			}

			if k[1] == "LT1" {
				file, err := c.FormFile("sid:" + k[0] + ";type:" + k[1] + ";")
				if err != nil {
					return nil, errors.New("Файл источник не загружен")
				}
				m[val] = file
			} else {
				link := c.FormValue("sid:" + k[0] + ";type:" + k[1] + ";")
				if len(link) < 3 {
					return nil, errors.New("Перепроверьте указанные источники")
				}
				m[val] = link
			}
		}
	}

	tx, err := r.db.StartTx(lib_db.TxWrite)
	if err != nil {
		return nil, err
	}

	var rdata_type *string
	if data_type != "" {
		rdata_type = &data_type
	}

	trueTx := tx.(*sql.Tx)
	res, err := trueTx.Exec(QInsertDatabase, name, rid, ctype, rdata_type, "ST1")
	if err != nil {
		trueTx.Rollback()
		return nil, err
	}

	id, _ := res.LastInsertId()
	i := 0
	m2 := make(map[int]*multipart.FileHeader)
	for key, value := range m {
		i++
		path := ""

		f := strings.Split(key, ":")
		if len(f) != 2 {
			continue
		}

		if f[1] == "LT1" {
			path = value.(*multipart.FileHeader).Filename
			m2[i] = value.(*multipart.FileHeader)
		} else {
			path = value.(string)
		}

		_, err = trueTx.Exec(QInsertDatabaseLink, id, i, path, "ST1", f[1])
		if err != nil {
			trueTx.Rollback()
			return nil, err
		}
	}

	err = trueTx.Commit()
	if err != nil {
		return nil, err
	}

	strId := strconv.FormatInt(id, 10)
	go r.threadUpload(id, m2)
	return &strId, nil
}
