package lib_db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type DB_SQLite struct {
	db      map[int]*sql.DB
	connStr string
}

func NewSQLite(cStr string) *DB_SQLite {
	return &DB_SQLite{
		connStr: cStr,
		db: map[int]*sql.DB{
			TxRead:  nil,
			TxWrite: nil,
		},
	}
}

func (d *DB_SQLite) Open() error {
	if d.db[TxRead] == nil {
		rdb, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", d.connStr))
		if err != nil {
			return err
		}
		d.db[TxRead] = rdb
	}

	if d.db[TxWrite] == nil {
		wdb, err := sql.Open("sqlite3", d.connStr)
		if err != nil {
			return err
		}
		d.db[TxWrite] = wdb
	}

	return nil
}

func (d *DB_SQLite) Close() {
	if d.db[TxRead] != nil {
		d.db[TxRead].Close()
	}

	if d.db[TxWrite] != nil {
		d.db[TxWrite].Close()
	}
}

func (d *DB_SQLite) StartTx(txType int) (interface{}, error) {
	return d.db[txType].Begin()
}

func (d *DB_SQLite) Exec(txType int, query string, args ...interface{}) (*string, error) {
	if d.db[txType] == nil {
		if err := d.Open(); err != nil {
			return nil, err
		}
	}

	tx, err := d.db[txType].Begin()
	if err != nil {
		return nil, err
	}

	res, err := tx.Exec(query, args...)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	a, _ := res.LastInsertId()
	b, _ := res.RowsAffected()

	result := fmt.Sprintf("LastInsertId: %d; RowsAffected: %d;", a, b)
	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (d *DB_SQLite) ExecWithTimeout(txType int, timeOut time.Duration, query string, args ...interface{}) (*string, error) {
	if d.db[txType] == nil {
		if err := d.Open(); err != nil {
			return nil, err
		}
	}

	ctx := context.Background()
	ctxTime, cancel := context.WithTimeout(ctx, timeOut)
	defer cancel()

	tx, err := d.db[txType].BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	var res sql.Result
	done := make(chan bool)

	go func() {
		res, err = tx.ExecContext(ctxTime, query, args...)
		done <- true
	}()

	select {
	case <-ctxTime.Done():
		defer func() {
			tx.Rollback()
		}()
		return nil, ctxTime.Err()
	case <-done:
		if err != nil {
			defer func() {
				tx.Rollback()
			}()
			return nil, err
		} else {
			defer func() {
				tx.Commit()
			}()
			a, _ := res.LastInsertId()
			b, _ := res.RowsAffected()
			c := fmt.Sprintf("LastInsertId: %d; RowsAffected: %d;", a, b)
			return &c, nil
		}
	}
}

func (d *DB_SQLite) QueryRow(txType int, query string, args ...interface{}) (*DBResult, error) {
	if d.db[txType] == nil {
		if err := d.Open(); err != nil {
			return nil, err
		}
	}

	tx, err := d.db[txType].Begin()
	if err != nil {
		return nil, err
	}

	var rows *sql.Rows
	rows, err = tx.Query(query, args...)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	defer rows.Close()

	result, err := d.rowsToMaps(rows)
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (d *DB_SQLite) QueryRowWithTimeout(txType int, timeOut time.Duration, query string, args ...interface{}) (*DBResult, error) {
	if d.db[txType] == nil {
		if err := d.Open(); err != nil {
			return nil, err
		}
	}

	ctx := context.Background()
	ctxTime, cancel := context.WithTimeout(ctx, timeOut)
	defer cancel()

	tx, err := d.db[txType].BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	var rows *sql.Rows
	done := make(chan bool)

	go func() {
		rows, err = tx.QueryContext(ctxTime, query, args...)
		done <- true
	}()

	select {
	case <-ctxTime.Done():
		defer func() {
			rows.Close()
			tx.Rollback()
		}()
		return nil, ctxTime.Err()
	case <-done:
		if err != nil {
			defer func() {
				rows.Close()
				tx.Rollback()
			}()
			return nil, err
		} else {
			defer func() {
				rows.Close()
				tx.Commit()
			}()
			return d.rowsToMaps(rows)
		}
	}
}

func (d *DB_SQLite) rowsToMaps(rows *sql.Rows) (*DBResult, error) {

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result DBResult

	values := make([]interface{}, len(columns))
	for i := range values {
		values[i] = new(interface{})
	}

	for rows.Next() {
		err := rows.Scan(values...)
		if err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, column := range columns {
			rowMap[column] = *(values[i].(*interface{}))
		}

		result = append(result, rowMap)
	}

	return &result, nil
}
