package lib_db

import (
	"context"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
)

type DB_PostgreSQL struct {
	db      map[int]*pgx.Conn
	connStr string
}

func NewPostgreSQL(cStr string) *DB_PostgreSQL {
	return &DB_PostgreSQL{
		connStr: cStr,
		db: map[int]*pgx.Conn{
			TxRead:  nil,
			TxWrite: nil,
		},
	}
}

func (d *DB_PostgreSQL) Open() error {
	if d.db[TxRead] == nil {
		rdb, err := pgx.Connect(context.Background(), d.connStr)
		if err != nil {
			return err
		}
		d.db[TxRead] = rdb
	}

	if d.db[TxWrite] == nil {
		wdb, err := pgx.Connect(context.Background(), d.connStr)
		if err != nil {
			return err
		}
		d.db[TxWrite] = wdb
	}

	return nil
}

func (d *DB_PostgreSQL) Close() {
	if d.db[TxRead] == nil {
		d.db[TxRead].Close(context.Background())
	}

	if d.db[TxWrite] == nil {
		d.db[TxWrite].Close(context.Background())
	}
}

func (d *DB_PostgreSQL) StartTx(txType int) (interface{}, error) {
	return d.db[txType].Begin(context.Background())
}

func (d *DB_PostgreSQL) Exec(txType int, query string, args ...interface{}) (*string, error) {
	if d.db[txType] == nil {
		if err := d.Open(); err != nil {
			return nil, err
		}
	}

	coma, err := d.db[txType].Exec(context.Background(), query, args...)
	str := coma.String()
	return &str, err
}

func (d *DB_PostgreSQL) ExecWithTimeout(txType int, timeOut time.Duration, query string, args ...interface{}) (*string, error) {
	if d.db[txType] == nil {
		if err := d.Open(); err != nil {
			return nil, err
		}
	}

	ctx := context.Background()
	ctxTime, cancel := context.WithTimeout(ctx, timeOut)
	defer cancel()

	tx, err := d.db[txType].Begin(ctx)
	if err != nil {
		return nil, err
	}

	var rows pgconn.CommandTag
	done := make(chan bool)

	go func() {
		rows, err = tx.Exec(ctxTime, query, args...)
		done <- true
	}()

	select {
	case <-ctxTime.Done():
		tx.Rollback(ctx)
		return nil, ctxTime.Err()
	case <-done:
		if err != nil {
			tx.Rollback(ctx)
			return nil, err
		} else {
			res := rows.String()
			if err != nil {
				tx.Rollback(ctx)
				return nil, err
			}
			tx.Commit(ctx)
			return &res, nil
		}
	}
}

func (d *DB_PostgreSQL) QueryRow(txType int, query string, args ...interface{}) (*DBResult, error) {
	if d.db[txType] == nil {
		if err := d.Open(); err != nil {
			return nil, err
		}
	}

	rows, err := d.db[txType].Query(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	return d.rowsToMap(rows)
}

func (d *DB_PostgreSQL) QueryRowWithTimeout(txType int, timeOut time.Duration, query string, args ...interface{}) (*DBResult, error) {
	if d.db[txType] == nil {
		if err := d.Open(); err != nil {
			return nil, err
		}
	}

	ctx := context.Background()
	ctxTime, cancel := context.WithTimeout(ctx, timeOut)
	defer cancel()

	tx, err := d.db[txType].Begin(ctx)
	if err != nil {
		return nil, err
	}

	var rows pgx.Rows
	done := make(chan bool)

	go func() {
		rows, err = tx.Query(ctxTime, query, args...)
		done <- true
	}()

	select {
	case <-ctxTime.Done():
		rows.Close()
		tx.Rollback(ctx)
		return nil, ctxTime.Err()
	case <-done:
		if err != nil {
			rows.Close()
			tx.Rollback(ctx)
			return nil, err
		} else {
			res, err := d.rowsToMap(rows)
			defer rows.Close()
			if err != nil {
				tx.Rollback(ctx)
				return nil, err
			}
			tx.Commit(ctx)
			return res, nil
		}
	}
}

func (d *DB_PostgreSQL) rowsToMap(rows pgx.Rows) (*DBResult, error) {
	columns := rows.FieldDescriptions()
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
			rowMap[string(column.Name)] = *(values[i].(*interface{}))
		}

		result = append(result, rowMap)
	}

	return &result, nil
}
