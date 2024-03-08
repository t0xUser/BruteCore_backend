package lib_db

import (
	"fmt"
	"log"
	"os"
	"time"
)

type DBResult []map[string]interface{}

type DBInterface interface {
	Close()
	Open() error
	Exec(txType int, query string, args ...interface{}) (*string, error)
	ExecWithTimeout(txType int, timeOut time.Duration, query string, args ...interface{}) (*string, error)
	QueryRow(txType int, query string, args ...interface{}) (*DBResult, error)
	QueryRowWithTimeout(txType int, timeOut time.Duration, query string, args ...interface{}) (*DBResult, error)

	StartTx(txType int) (interface{}, error)
}

type DB struct {
	log    *log.Logger
	DBIntr DBInterface
}

const (
	MSSQL = iota
	PostgreSQL
	SQLite
	Mock

	TxRead
	TxWrite
)

func New(driver uint, connectionString string, logfile *os.File) (*DB, error) {
	var (
		dbIntr DBInterface
		ilog   *log.Logger
	)

	if logfile != nil {
		ilog = log.New(logfile, "[DATABASE] ", log.Ldate|log.Lmicroseconds)
	}

	switch driver {
	case MSSQL:
		dbIntr = NewMSSQL(connectionString)
	case PostgreSQL:
		dbIntr = NewPostgreSQL(connectionString)
	case SQLite:
		dbIntr = NewSQLite(connectionString)
	case Mock:
		dbIntr = NewMock()
	default:
		return nil, fmt.Errorf("Укажите driver, тип БД")
	}

	if err := dbIntr.Open(); err != nil {
		return nil, err
	}

	return &DB{
		DBIntr: dbIntr,
		log:    ilog,
	}, nil
}

func (d *DB) Open() error {
	return d.DBIntr.Open()
}

func (d *DB) Close() {
	d.DBIntr.Close()
}

func (d *DB) StartTx(txType int) (interface{}, error) {
	return d.DBIntr.StartTx(txType)
}

func (d *DB) Exec(txType int, query string, args ...interface{}) (*string, error) {
	if d.log != nil {
		d.log.Printf("Exec: \"%s\", Args: \"%v\", TxType: %d", query, args, txType)
	}
	return d.DBIntr.Exec(txType, query, args...)
}

func (d *DB) ExecWithTimeout(txType int, timeOut time.Duration, query string, args ...interface{}) (*string, error) {
	if d.log != nil {
		d.log.Printf("ExecWithTimeout: \"%s\", Args: \"%v\", TxType: %d", query, args, txType)
	}
	return d.DBIntr.ExecWithTimeout(txType, timeOut, query, args...)
}

func (d *DB) QueryRow(txType int, query string, args ...interface{}) (*DBResult, error) {
	if d.log != nil {
		d.log.Printf("QueryRow: \"%s\", Args: \"%v\", TxType: %d", query, args, txType)
	}
	return d.DBIntr.QueryRow(txType, query, args...)
}

func (d *DB) QueryRowWithTimeout(txType int, timeOut time.Duration, query string, args ...interface{}) (*DBResult, error) {
	if d.log != nil {
		d.log.Printf("QueryRowWithTimeout: \"%s\", Args: \"%v\", TxType: %d", query, args, txType)
	}
	return d.DBIntr.QueryRowWithTimeout(txType, timeOut, query, args...)
}

func (r *DBResult) Count() int {
	if r == nil {
		return 0
	}
	return len(*r)
}
