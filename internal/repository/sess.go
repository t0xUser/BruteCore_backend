package repository

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"

	eg "api.brutecore/internal/engine"
	"api.brutecore/internal/utility"

	"api.brutecore/libs/lib_db"
	"github.com/google/uuid"
	"github.com/tealeg/xlsx"
)

type SESSRepositoryLayer struct {
	db *lib_db.DB
	p  *eg.Pulling
}

type AlterSess struct {
	ID       *int64      `json:"id"`
	Field    string      `json:"field"`
	NewValue interface{} `json:"new_value"`
}

type MUploadProxy struct {
	SessionID int64  `json:"session_id"`
	ProxyType string `json:"proxy_type"`
	TimeOut   int    `json:"timeout"`
	Data      string `json:"data"`
}

type AlterInputFields struct {
	Inputs []struct {
		Description string      `json:"description"`
		ID          int         `json:"id"`
		Name        string      `json:"name"`
		Value       interface{} `json:"value"`
		Type        string      `json:"type"`
	} `json:"inputs"`
}

const (
	GoodStatus     = "Хорошие"
	WithBalance    = "С балансом"
	WithoutBalance = "Пустышки"
	LogStatus      = "Хорошие с логом"
	Line           = "\n-------------\n"
)

func NewSESSRepositoryLayer(d *lib_db.DB, pl *eg.Pulling) *SESSRepositoryLayer {
	return &SESSRepositoryLayer{
		db: d,
		p:  pl,
	}
}

func (r *SESSRepositoryLayer) GetSessions() (*lib_db.DBResult, error) {
	return r.db.QueryRow(lib_db.TxRead, QGetSessions)
}

func (r *SESSRepositoryLayer) ApplyInputFields(sID string, dat AlterInputFields) error {
	session_id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return err
	}

	res, err := r.db.QueryRow(lib_db.TxRead, "SELECT * FROM SESSION_DTL WHERE SESSION_ID = $1 AND KEY IN ('IT1','IT2','IT3','IT4')", session_id)
	if err != nil {
		return err
	}

	tx, err := r.db.StartTx(lib_db.TxWrite)
	if err != nil {
		return err
	}
	trueTx := tx.(*sql.Tx)

	for _, v := range dat.Inputs {
		var (
			exists bool
			query  string
			iInt   int64
		)
		switch v.Type {
		case "IT1", "IT3":
			if len(v.Value.(string)) > 59 {
				err = errors.New(`Поле "` + v.Description + `" превышает максимальную длину в 60 символов`)
			}
		case "IT2":
			if iFloat, ok := v.Value.(float64); ok {
				v.Value = int64(iFloat)
			} else if iInt, ok = v.Value.(int64); ok {
				v.Value = iInt
			} else {
				val, ok := v.Value.(string)
				if (!ok) || (ok && val != "") {
					err = errors.New(`Значение поля "` + v.Description + `" не является числом`)
				}
			}
		}

		if err != nil {
			trueTx.Rollback()
			return err
		}

		for _, i := range *res {
			if i["key"].(string) == v.Type {
				exists = true
				break
			}
		}
		if exists {
			query = QSessionInputFieldsUpdate
		} else {
			query = QSessionInputFieldsInsert
		}

		_, err := trueTx.Exec(query, v.Value, session_id, v.Type, v.Name)
		if err != nil {
			trueTx.Rollback()
			return err
		}
	}

	err = trueTx.Commit()
	if err != nil {
		trueTx.Rollback()
	}

	return err
}

func (r *SESSRepositoryLayer) CreateSession(sID string) (*lib_db.DBResult, error) {
	module_id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return nil, err
	}

	uuid := uuid.New()
	_, err = r.db.Exec(lib_db.TxWrite, QInsertSession, uuid, module_id)
	if err != nil {
		return nil, err
	}

	return r.db.QueryRow(lib_db.TxRead, "SELECT ID, CREATE_TIME, NAME FROM SESSION WHERE NAME = $1", uuid)
}

func (r *SESSRepositoryLayer) DeleteSession(sID string) error {
	id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(lib_db.TxWrite, QDeleteSession, id, id, id, id, id, id)
	_, _ = r.db.Exec(lib_db.TxWrite, QVacuum)
	return err
}

func (r *SESSRepositoryLayer) GetInfoSession(sID string) (*lib_db.DBResult, error) {
	id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return nil, err
	}

	res, err := r.db.QueryRow(lib_db.TxRead, QSessionInfo, id)
	if err != nil {
		return nil, err
	}

	res2, err := r.db.QueryRow(lib_db.TxRead, QSessionInfoInput, id, (*res)[0]["module_id"].(int64))
	if err != nil {
		return nil, err
	}

	for i, v := range *res2 {
		v["id"] = i + 1
	}

	(*res)[0]["inputs"] = res2

	return res, nil
}

func (r *SESSRepositoryLayer) UploadProxy(resp *MUploadProxy) error {
	if resp.TimeOut == 0 {
		return errors.New("Укажите таймаут")
	}

	if !slices.Contains([]string{"PT1", "PT2", "PT3"}, resp.ProxyType) {
		return errors.New("Тип проси указан некорректно")
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(resp.Data)
	if err != nil {
		return errors.New("Ошибка декодирования данных")
	}
	decodedString := string(decodedBytes)
	lines := strings.Split(decodedString, "\n")
	tx, err := r.db.StartTx(lib_db.TxWrite)
	if err != nil {
		return err
	}
	trueTx := tx.(*sql.Tx)
	trueTx.Exec("DELETE FROM SESSION_PROXY WHERE SESSION_ID = $1", resp.SessionID)
	trueTx.Exec(QInsertSessionDTLTimeOut, resp.SessionID, "P1T", resp.TimeOut, "Таймаут прокси")
	for _, line := range lines {
		var port int
		dline := strings.Split(line, ":")
		if dline[0] == "" {
			continue
		}

		if port, err = strconv.Atoi(strings.ReplaceAll(dline[1], "\r", "")); err != nil {
			continue
		}
		trueTx.Exec(QInsertSessionProxy, resp.SessionID, dline[0], port, resp.ProxyType)
	}

	trueTx.Exec("UPDATE SESSION SET PROXY_ID = -2 WHERE ID = $1", resp.SessionID)

	err = trueTx.Commit()
	if err != nil {
		trueTx.Rollback()
		return err
	}

	return nil
}

func (r *SESSRepositoryLayer) AlterSession(s *AlterSess) error {
	if s.ID == nil {
		return errors.New("Укажите ID сессии")
	}

	if s.Field == "" {
		return errors.New("Укажите поле для изменения")
	}

	if s.NewValue == nil {
		return errors.New("Укажите новое значение")
	}

	switch strings.ToUpper(s.Field) {
	case "DATABASE_ID", "USERNAME_ID", "PASSWORD_ID":
		res, err := r.db.QueryRow(lib_db.TxRead, QGetDatabaseType, s.NewValue)
		if err != nil {
			return err
		}

		if (*res)[0]["type"].(string) != "CL2" {
			return errors.New("Можно выбирать только комболист")
		}

	case "PROXY_ID":
	case "MODULE_ID":
	case "STATUS":
		if s.NewValue == "ST5" {
			if err := r.StartSession(*s.ID); err != nil {
				return err
			}
		}

	case "NAME":
		if len(s.NewValue.(string)) > 60 {
			return errors.New("Длина поля превышает максимальную длину")
		}

	case "WORKER_COUNT":
		if s.NewValue.(float64) > 2000 || s.NewValue.(float64) < 1 {
			return errors.New("Недопустимое значение")
		}
	default:
		return errors.New("Неопознанное поле для изменения")
	}

	_, err := r.db.Exec(lib_db.TxWrite, `UPDATE "session" SET `+s.Field+` = $1 WHERE ID = $2`, s.NewValue, *s.ID)
	if strings.ToUpper(s.Field) == "STATUS" {
		r.p.Iteration()
	}
	return err
}

func (r *SESSRepositoryLayer) StartSession(sessionId int64) error {
	res, err := r.db.QueryRow(lib_db.TxRead, QGetSessionStartInfo, sessionId)
	if err != nil {
		return err
	}

	if (*res)[0]["database_id"] == nil {
		return errors.New("Укажите Комболист для перебора")
	}

	if (*res)[0]["proxy_id"] != nil && (*res)[0]["proxy_id"].(int64) == -2 &&
		(*res)[0]["PROX_COUNT"].(int64) < 5 {
		return errors.New("Прокси выбраны из файла но не загружены")
	}

	if (*res)[0]["module_id"] == nil {
		return errors.New("Укажите модуль для перебора")
	}

	return nil
}

func (r *SESSRepositoryLayer) GetSessionStatistic(sID string) (*lib_db.DBResult, *lib_db.DBResult, error) {
	id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return nil, nil, err
	}

	res, err := r.db.QueryRow(lib_db.TxRead, QGetModuleType, id)
	if err != nil {
		return nil, nil, err
	}
	mtype := (*res)[0]["type"].(string)

	var res1 *lib_db.DBResult
	if mtype == "MT1" {
		res1, err = r.db.QueryRow(lib_db.TxRead, QGetSessionStatistic, id, id)
	} else {
		res1, err = r.db.QueryRow(lib_db.TxRead, QGetSessionStatisticProtocol, id, id)
	}
	if err != nil {
		return nil, nil, err
	}

	var res2 *lib_db.DBResult
	if mtype == "MT1" {
		res2, err = r.db.QueryRow(lib_db.TxRead, QGetPercent, id)
	} else {
		res2, err = r.db.QueryRow(lib_db.TxRead, QGetPercentProtocol, id, id)
	}

	if err != nil {
		return nil, nil, err
	}

	(*res2)[0]["proxycount"] = r.p.GetProxyCount(id)
	return res1, res2, nil
}

func (r *SESSRepositoryLayer) GetResults(sId string, pId string, eCount string, params string) (error, int, *lib_db.DBResult) {
	session_id, err := strconv.ParseInt(sId, 10, 64)
	if err != nil {
		return err, 0, nil
	}
	page_id, err := strconv.ParseInt(pId, 10, 64)
	if err != nil {
		return err, 0, nil
	}
	entries_count, err := strconv.ParseInt(eCount, 10, 64)

	res, err := r.db.QueryRow(lib_db.TxRead, QGetModuleType, session_id)
	if err != nil {
		return err, 0, nil
	}
	mtype := (*res)[0]["type"].(string)

	match, err := regexp.MatchString(`^[a-zA-Z0-9]+(,[a-zA-Z0-9]+)*$`, params)
	if err != nil {
		return err, 0, nil
	}
	if !match {
		return errors.New("Перепроверьте запрашиваемые типы"), 0, nil
	}

	match, err = regexp.MatchString(`\b999\b`, params)
	if err != nil {
		return err, 0, nil
	}
	nine := 0
	if match {
		nine = 999
	}

	if mtype == "MT1" {
		res, err = r.db.QueryRow(lib_db.TxRead, strings.Replace(QGetSessionPagesCount, "-_-", utility.ChangeParam(params), -1), session_id, nine, session_id)
		if err != nil {
			return err, 0, nil
		}

		dataCount := (*res)[0]["C1"].(int64)
		pagesCount := int(dataCount / entries_count)
		if dataCount%entries_count > 0 {
			pagesCount++
		}
		if pagesCount == 0 {
			pagesCount = 1
		}

		res, err = r.db.QueryRow(lib_db.TxRead, strings.Replace(QGetSessionResults, "-_-", utility.ChangeParam(params), -1), session_id, nine, session_id, entries_count, (page_id-1)*entries_count)
		if err != nil {
			return err, 0, nil
		}

		return nil, pagesCount, res
	} else {
		q := strings.Replace(QGetSessionPagesCountProtocol, "-_-", utility.ChangeParam(params), -1)
		if nine == 999 {
			q = strings.Replace(q, "XXX", "OR DATA_STATUS IS NULL", -1)
		} else {
			q = strings.Replace(q, "XXX", "", -1)
		}
		res, err = r.db.QueryRow(lib_db.TxRead, q, session_id)

		dataCount := (*res)[0]["C1"].(int64)
		pagesCount := int(dataCount / entries_count)
		if dataCount%entries_count > 0 {
			pagesCount++
		}
		if pagesCount == 0 {
			pagesCount = 1
		}

		q = strings.Replace(QGetSessionResultsProtocol, "-_-", utility.ChangeParam(params), -1)
		if nine == 999 {
			q = strings.Replace(q, "XXX", "OR DATA_STATUS IS NULL", -1)
		} else {
			q = strings.Replace(q, "XXX", "", -1)
		}

		res, err = r.db.QueryRow(lib_db.TxRead, q, session_id, entries_count, (page_id-1)*entries_count)
		if err != nil {
			return err, 0, nil
		}

		return nil, pagesCount, res
	}
}

func (r *SESSRepositoryLayer) DownloadUniversal(sID string, sFormat string, params string, sType bool) ([]byte, error) {
	var (
		nine  int64
		err   error
		res   *lib_db.DBResult
		files []utility.File
	)
	session_id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return nil, err
	}

	res, err = r.db.QueryRow(lib_db.TxRead, QGetModuleType, session_id)
	if err != nil {
		return nil, err
	}
	mtype := (*res)[0]["type"].(string)

	format := strings.ToUpper(sFormat)
	if format != "TXT" && format != "XLSX" {
		return nil, errors.New("Неизвестный формат")
	}

	if sType {
		match, err := regexp.MatchString(`^[a-zA-Z0-9]+(,[a-zA-Z0-9]+)*$`, params)
		if err != nil {
			return nil, err
		}
		if !match {
			return nil, errors.New("Перепроверьте запрашиваемые типы")
		}

		match, err = regexp.MatchString(`\b999\b`, params)
		if err != nil {
			return nil, err
		}
		nine = 0
		if match {
			nine = 999
		}
	}

	if mtype == "MT1" {
		if !sType {
			res, err = r.db.QueryRow(lib_db.TxRead, QueryALL, session_id, session_id)
		} else {
			res, err = r.db.QueryRow(lib_db.TxRead, strings.Replace(QuerySelected, "-_-", utility.ChangeParam(params), -1), session_id, nine, session_id)
		}
		if err != nil {
			return nil, err
		}

		switch format {
		case "TXT":
			buf := map[string]string{}
			for _, item := range *res {
				status := item["st"].(string)
				data := item["da"].(string)
				log := item["lo"].(string)

				buf[status] = buf[status] + data + "\n"
				if (status == GoodStatus || status == WithBalance || status == WithoutBalance) && log != "" {
					buf[LogStatus] = buf[LogStatus] + data + "\n" + log + Line
				}
			}
			for key, val := range buf {
				files = append(files, utility.File{
					Name: key + ".txt",
					Body: []byte(val),
				})
			}
		case "XLSX":
			excel := xlsx.NewFile()
			sheet, err := excel.AddSheet("Results")
			if err != nil {
				return nil, err
			}

			headRow := sheet.AddRow()
			for _, value := range []string{"Статус", "Данные", "Лог"} {
				headRow.AddCell().Value = value
			}

			for _, item := range *res {
				rowObj := sheet.AddRow()
				rowObj.AddCell().Value = item["st"].(string)
				rowObj.AddCell().Value = item["da"].(string)
				rowObj.AddCell().Value = item["lo"].(string)
			}

			var buffer bytes.Buffer
			err = excel.Write(&buffer)
			files = append(files, utility.File{
				Name: "Результаты.xlsx",
				Body: buffer.Bytes(),
			})
		}
	} else {
		if !sType {
			res, err = r.db.QueryRow(lib_db.TxRead, QueryAllProtocol, session_id)
		} else {
			q := strings.Replace(QuerySelectedProtocol, "-_-", utility.ChangeParam(params), -1)
			if nine == 999 {
				q = strings.Replace(q, "XXX", "OR DATA_STATUS IS NULL", -1)
			} else {
				q = strings.Replace(q, "XXX", "", -1)
			}
			res, err = r.db.QueryRow(lib_db.TxRead, q, session_id)
		}
		if err != nil {
			return nil, err
		}

		switch format {
		case "TXT":
			buf := map[string]string{}
			for _, item := range *res {
				status := item["status"].(string)
				data := item["host"].(string) + "@" + item["login"].(string) + ":" + item["password"].(string)
				log := item["log"].(string)

				buf[status] = buf[status] + data + "\n"
				if (status == GoodStatus || status == WithBalance || status == WithoutBalance) && log != "" {
					buf[LogStatus] = buf[LogStatus] + data + "\n" + log + Line
				}
			}
			for key, val := range buf {
				files = append(files, utility.File{
					Name: key + ".txt",
					Body: []byte(val),
				})
			}
		case "XLSX":
			excel := xlsx.NewFile()
			sheet, err := excel.AddSheet("Results")
			if err != nil {
				return nil, err
			}

			headRow := sheet.AddRow()
			for _, value := range []string{"Статус", "Хост", "Логин", "Пароль", "Лог"} {
				headRow.AddCell().Value = value
			}

			for _, item := range *res {
				rowObj := sheet.AddRow()
				rowObj.AddCell().Value = item["status"].(string)
				rowObj.AddCell().Value = item["host"].(string)
				rowObj.AddCell().Value = item["login"].(string)
				rowObj.AddCell().Value = item["password"].(string)
				rowObj.AddCell().Value = item["log"].(string)
			}
			
			var buffer bytes.Buffer
			err = excel.Write(&buffer)
			files = append(files, utility.File{
				Name: "Результаты.xlsx",
				Body: buffer.Bytes(),
			})
		}
	}

	return utility.MakeZip(&files)
}
