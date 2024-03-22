package repository

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"io/ioutil"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	eg "api.brutecore/internal/engine"
	"api.brutecore/internal/utility"

	"api.brutecore/libs/lib_db"
	"github.com/gofiber/fiber/v2"
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

		if v.Value != nil {
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

	if res.Count() == 0 {
		return nil, errors.New("Session not found")
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

func (r *SESSRepositoryLayer) UploadProxyFormData(c *fiber.Ctx) error {
	proxy_type := c.FormValue("proxy_type")
	if !slices.Contains([]string{"PT1", "PT2", "PT3"}, proxy_type) {
		return errors.New("Тип проси указан некорректно")
	}

	session_id := c.FormValue("session_id")
	if session_id == "" {
		return errors.New("session_id is undefined")
	}

	file, err := c.FormFile("file")
	if err != nil {
		return err
	}

	s, err := file.Open()
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(s)
	if err != nil {
		return err
	}

	tx, err := r.db.StartTx(lib_db.TxWrite)
	if err != nil {
		return err
	}

	trueTx := tx.(*sql.Tx)
	trueTx.Exec("DELETE FROM SESSION_PROXY WHERE SESSION_ID = $1", session_id)
	bodyReader := bytes.NewReader(body)
	scanner := bufio.NewScanner(bodyReader)
	for scanner.Scan() {
		var port int
		dline := strings.Split(scanner.Text(), ":")
		if dline[0] == "" {
			continue
		}

		if port, err = strconv.Atoi(strings.ReplaceAll(dline[1], "\r", "")); err != nil {
			continue
		}

		trueTx.Exec(QInsertSessionProxy, session_id, dline[0], port, proxy_type)
	}

	trueTx.Exec("UPDATE SESSION SET PROXY_ID = -2 WHERE ID = $1", session_id)

	err = trueTx.Commit()
	if err != nil {
		trueTx.Rollback()
		return err
	}

	return nil
}

func (r *SESSRepositoryLayer) UploadComboListFormData(c *fiber.Ctx) error {
	session_id := c.FormValue("session_id")
	if session_id == "" {
		return errors.New("session_id не указан")
	}

	combo_type := c.FormValue("combo_type")
	if combo_type == "" {
		return errors.New("combo_type не указан")
	}

	data_type := c.FormValue("data_type")
	if data_type == "" {
		return errors.New("data_type не указан")
	}

	if data_type == "" || !slices.Contains([]string{"DT1", "DT2", "DT3", "DT4", "DT5", "DT6", "DT7", "DT8"}, data_type) {
		return errors.New("Укажите тип данных файла")
	}

	res, err := r.db.QueryRow(lib_db.TxRead, QGetSession, session_id)
	if err != nil {
		return err
	}

	module_type := (*res)[0]["type"].(string)

	file, err := c.FormFile("file")
	if err != nil {
		return err
	}

	s, err := file.Open()
	if err != nil {
		return err
	}

	body, err := ioutil.ReadAll(s)
	if err != nil {
		return err
	}

	tx, err := r.db.StartTx(lib_db.TxWrite)
	if err != nil {
		return err
	}

	trueTx := tx.(*sql.Tx)
	bodyReader := bytes.NewReader(body)
	scanner := bufio.NewScanner(bodyReader)

	if module_type == "MT1" {
		trueTx.Exec("DELETE FROM SESSION_DATA_WEBAPI WHERE SESSION_ID = $1;", session_id)
		I := 0
		for scanner.Scan() {
			I++
			trueTx.Exec(`
				INSERT INTO SESSION_DATA_WEBAPI
				VALUES($1, $2, $3, NULL);
			`, session_id, I, scanner.Text())
		}
	} else {
		trueTx.Exec(QDeleteTempCombolist, session_id, combo_type)

		var vtype string
		switch combo_type {
		case "0":
			vtype = "DATABASE_ID"
		case "1":
			vtype = "USERNAME_ID"
		case "2":
			vtype = "PASSWORD_ID"
		}

		for scanner.Scan() {
			trueTx.Exec(QInsertTempCombolist, session_id, vtype, scanner.Text())
		}

		trueTx.Exec(`
		DELETE FROM SESSION_DTL WHERE SESSION_ID = $1 AND KEY = $2;
		INSERT INTO SESSION_DTL VALUES($3, $4, (SELECT COUNT(1) FROM SESSION_COMBOLIST_TMP WHERE SESSION_ID = $5 AND "TYPE" = $6), '-')
		`, session_id, vtype, session_id, vtype, session_id, vtype)

	}

	if err := scanner.Err(); err != nil {
		trueTx.Rollback()
		return err
	}

	switch combo_type {
	case "0":
		trueTx.Exec("UPDATE SESSION SET database_id = -2 WHERE ID = $1", session_id)
	case "1":
		trueTx.Exec("UPDATE SESSION SET username_ID = -2 WHERE ID = $1", session_id)
	case "2":
		trueTx.Exec("UPDATE SESSION SET password_ID = -2 WHERE ID = $1", session_id)
	}

	return trueTx.Commit()
}

func (r *SESSRepositoryLayer) AlterSession(s *AlterSess) error {
	if s.ID == nil {
		return errors.New("Укажите ID сессии")
	}

	if s.Field == "" {
		return errors.New("Укажите поле для изменения")
	}
	s.Field = strings.ToUpper(s.Field)

	if s.NewValue == nil {
		return errors.New("Укажите новое значение")
	}

	res, err := r.db.QueryRow(lib_db.TxRead, QGetSession, *s.ID)
	if err != nil {
		return err
	}

	module_type := (*res)[0]["type"].(string)
	session_status := (*res)[0]["status"].(string)

	if session_status == "ST6" {
		return errors.New("Нельзя проводить никаких изменений с завершенной сессией")
	}

	if s.Field == "STATUS" && s.NewValue == "ST7" && session_status != "ST5" {
		return errors.New("Нельзя остановить не запущенную сессию")
	}

	if s.Field == "STATUS" && s.NewValue == "ST5" {
		if err := r.StartSession(*s.ID); err != nil {
			return err
		}
	}

	if session_status == "ST5" && slices.Contains([]string{
		"DATABASE_ID", "USERNAME_ID", "PASSWORD_ID",
		"PROXY_ID",
	}, s.Field) {
		return errors.New("Нельзя менять данное поле при активной сессии")
	}

	if s.Field == "WORKER_COUNT" && (s.NewValue.(float64) > 2000 || s.NewValue.(float64) < 1) {
		return errors.New("Недопустимое значение")
	}

	if s.Field == "TIMEOUT" && (s.NewValue.(float64) > 180000 || s.NewValue.(float64) < 1000) {
		return errors.New("Недопустимое значение")
	}

	if s.Field == "NAME" && len(s.NewValue.(string)) > 60 {
		return errors.New("Длина поля превышает максимальную длину")
	}

	if slices.Contains([]string{"DATABASE_ID", "USERNAME_ID", "PASSWORD_ID"}, s.Field) {
		res, err := r.db.QueryRow(lib_db.TxRead, QGetDatabaseType, s.NewValue)
		if err != nil {
			return err
		}

		if (*res)[0]["type"].(string) != "CL2" {
			return errors.New("Можно выбирать только комболист")
		}
	}

	if module_type == "MT1" && s.Field == "DATABASE_ID" {

		_, err = r.db.Exec(lib_db.TxWrite, `
			DELETE FROM SESSION_DATA_WEBAPI WHERE SESSION_ID = $1;
			INSERT INTO SESSION_DATA_WEBAPI
			SELECT 
				$2, ROW_NUMBER() OVER(PARTITION BY 1 ORDER BY 1) AS id, data, null
			FROM DATABASE_LINK_DATA
			WHERE DATABASE_ID = $3;
			UPDATE SESSION SET DATABASE_ID = $4 WHERE ID = $5;
		`, *s.ID, *s.ID, s.NewValue.(float64), s.NewValue.(float64), *s.ID)

		return err
	}

	if module_type == "MT2" && slices.Contains([]string{"DATABASE_ID", "USERNAME_ID", "PASSWORD_ID"}, s.Field) {

		_, err = r.db.Exec(lib_db.TxWrite, `
			DELETE FROM SESSION_COMBOLIST_TMP WHERE SESSION_ID = $1 AND "TYPE" = $2;
			INSERT INTO SESSION_COMBOLIST_TMP
			SELECT $3, $4, data
			FROM DATABASE_LINK_DATA
			WHERE DATABASE_ID = $5;
			UPDATE SESSION SET `+s.Field+` = $6 WHERE ID = $7;
			DELETE FROM SESSION_DTL WHERE SESSION_ID = $8 AND KEY = $9;
			INSERT INTO SESSION_DTL VALUES($10, $11, (SELECT COUNT(1) FROM SESSION_COMBOLIST_TMP WHERE SESSION_ID = $12 AND "TYPE" = $13), '-')
		`,
			*s.ID, s.Field, *s.ID, s.Field, s.NewValue.(float64), s.NewValue.(float64),
			*s.ID, *s.ID, s.Field, *s.ID, s.Field, *s.ID, s.Field)

		return err
	}

	_, err = r.db.Exec(lib_db.TxWrite, `UPDATE "session" SET `+s.Field+` = $1 WHERE ID = $2`, s.NewValue, *s.ID)
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

	if (*res)[0]["module_type"].(string) == "MT2" {
		res2, err := r.db.QueryRow(lib_db.TxRead, QCheckExists, sessionId)
		if err != nil {
			return err
		}

		if res2.Count() == 0 {
			return errors.New("Произошла внетрення ошибка")
		}

		H_COUNT := (*res2)[0]["H_COUNT"].(int64)
		if H_COUNT == 0 {
			return errors.New("Хосты не были загружены")
		}
		U_COUNT := (*res2)[0]["U_COUNT"].(int64)
		if U_COUNT == 0 {
			return errors.New("Логины не были загружены")
		}
		P_COUNT := (*res2)[0]["P_COUNT"].(int64)
		if P_COUNT == 0 {
			return errors.New("Пароли не были загружены")
		}
	}

	return nil
}

func (r *SESSRepositoryLayer) GetSessionStatistic(sID string) (*lib_db.DBResult, *lib_db.DBResult, *string, *string, error) {
	id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	res, err := r.db.QueryRow(lib_db.TxRead, QGetModuleType, id)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	mtype := (*res)[0]["type"].(string)

	var result *lib_db.DBResult
	if mtype == "MT1" {
		result, err = r.db.QueryRow(lib_db.TxRead, QGetSessionStatistic, id, id)
	} else {
		result, err = r.db.QueryRow(lib_db.TxRead, QGetSessionStatisticProtocol, id, id)
	}
	if err != nil {
		return nil, nil, nil, nil, err
	}

	var statistic *lib_db.DBResult
	if mtype == "MT1" {
		statistic, err = r.db.QueryRow(lib_db.TxRead, QGetPercent, id, id)
	} else {
		statistic, err = r.db.QueryRow(lib_db.TxRead, QGetPercentProtocol, id, id)
	}

	if err != nil {
		return nil, nil, nil, nil, err
	}

	(*statistic)[0]["proxycount"] = r.p.GetProxyCount(id)

	status := (*statistic)[0]["STATUS_NAME"].(string)
	delete((*statistic)[0], "STATUS_NAME")

	finish_time := "Не закончено"
	if (*statistic)[0]["finish_time"] != nil {
		finish_time = (*statistic)[0]["finish_time"].(time.Time).Format("2006-01-02 15:04:05")
	}
	delete((*statistic)[0], "finish_time")

	return result, statistic, &status, &finish_time, nil
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

		res, err = r.db.QueryRow(lib_db.TxRead, strings.Replace(QGetSessionResults, "-_-", utility.ChangeParam(params), -1), session_id, nine, entries_count, (page_id-1)*entries_count)
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
