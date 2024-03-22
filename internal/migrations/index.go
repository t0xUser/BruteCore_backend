package migrations

import (
	"api.brutecore/libs/lib_db"
)

func getVersion(db *lib_db.DB) (string, error) {
	sets, err := db.QueryRow(lib_db.TxRead, qSearchSettings)
	if err != nil {
		return "", err
	}

	if sets.Count() == 0 {
		if _, err := db.Exec(lib_db.TxWrite, qCreateSettings); err != nil {
			return "", err
		} else {
			return "0", nil
		}
	} else {
		res, err := db.QueryRow(lib_db.TxRead, qGetDBVersion)
		if err != nil {
			return "", err
		}
		return (*res)[0]["value"].(string), nil
	}
}

func MigrateDataBase(db *lib_db.DB, query string) error {
	_, err := db.Exec(lib_db.TxWrite, query)
	return err
}

const (
	actualVersion = "4"
)

func CheckMigrations(db *lib_db.DB) error {
	ver, err := getVersion(db)
	if err != nil {
		return err
	}

	if ver != actualVersion {
		if err := MigrateDataBase(db, updateArr[ver]); err != nil {
			return err
		}
		return CheckMigrations(db)
	} else {
		return nil
	}
}
