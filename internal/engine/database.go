package engine

import (
	"errors"
	"sync"

	"api.brutecore/libs/lib_db"
)

type Database struct {
	id        int64
	data_type string
	db        *lib_db.DB
	buffer    []ComboListRecord
	mul       sync.Mutex
	sWorkerC  int
}

func NewDatabase(d *lib_db.DB, database_id int64) (*Database, error) {
	if database_id == -1 {
		return nil, errors.New("База данных не выбрана")
	}

	res, err := d.QueryRow(lib_db.TxRead, "SELECT DATA_TYPE FROM DATABASE WHERE ID = $1", database_id)
	if err != nil {
		return nil, err
	}

	return &Database{
		id:        database_id,
		db:        d,
		data_type: (*res)[0]["data_type"].(string),
	}, nil
}

func NewDatabaseProtocol(d *lib_db.DB, i map[string]interface{}) (*Database, error) {
	database_id := i["DATABASE_ID"].(int64)
	username_id := i["USERNAME_ID"].(int64)
	password_id := i["PASSWORD_ID"].(int64)
	session_id := i["ID"].(int64)

	if database_id == -1 {
		return nil, errors.New("Хосты не выбраны")
	}

	if username_id == -1 {
		return nil, errors.New("Логины не выбраны")
	}

	if password_id == -1 {
		return nil, errors.New("Пароли не выбраны")
	}

	res, err := d.QueryRow(lib_db.TxRead, QCehckAlreadyCreated, session_id)
	if err != nil {
		return nil, err
	}

	stat := (*res)[0]["status"].(int64)
	if stat == 0 {
		_, err = d.Exec(lib_db.TxWrite, QInsertProtocolLines, database_id, username_id, password_id, session_id)
		if err != nil {
			return nil, err
		}
	}

	return &Database{
		id:        session_id,
		db:        d,
		data_type: "MT2",
	}, nil
}

func (d *Database) GetStartBatch(session_id int64) error {
	var (
		res *lib_db.DBResult
		err error
	)

	if d.data_type != "MT2" {
		res, err = d.db.QueryRow(lib_db.TxRead, QGetMaxConId, session_id)
		if err != nil {
			return err
		}
		max_con_id := (*res)[0]["M"].(string)

		res, err = d.db.QueryRow(lib_db.TxRead, QGetComboListBatchStart, d.id, max_con_id, session_id, d.id, max_con_id, d.sWorkerC*15)
		if err != nil {
			return err
		}
	} else {
		res, err = d.db.QueryRow(lib_db.TxRead, QGetComboListProtocolBatchStart, session_id, d.sWorkerC*15)
		if err != nil {
			return err
		}
	}

	for _, item := range *res {
		d.buffer = append(d.buffer, ComboListRecord{
			id:            item["id"].(int64),
			database_link: item["database_link"].(int64),
			database_id:   d.id,
			data:          item["data"].(string),
			login:         item["login"].(string),
			password:      item["password"].(string),
			con_id:        item["con_id"].(string),
		})
	}

	return nil
}

func (d *Database) GetComboLine() *ComboListRecord {
	d.mul.Lock()
	defer d.mul.Unlock()

	var cl ComboListRecord
	switch len(d.buffer) {
	case 0:
		cl.data = "-1"
	case 1:
		var (
			res *lib_db.DBResult
			err error
			q   string
		)

		if d.data_type != "MT2" {
			q = QGetComboListBatch
		} else {
			q = QGetComboListProtocolBatch
		}

		res, err = d.db.QueryRow(lib_db.TxRead, q, d.id, d.buffer[0].con_id, d.sWorkerC*15)
		if err != nil {
			cl.data = "~"
		} else {
			for _, item := range *res {
				d.buffer = append(d.buffer, ComboListRecord{
					id:            item["id"].(int64),
					database_link: item["database_link"].(int64),
					database_id:   d.id,
					data:          item["data"].(string),
					login:         item["login"].(string),
					password:      item["password"].(string),
					con_id:        item["con_id"].(string),
				})
			}

			cl = d.buffer[0]
			d.buffer = d.buffer[1:]
		}
	default:
		cl = d.buffer[0]
		d.buffer = d.buffer[1:]
	}

	return &cl
}
