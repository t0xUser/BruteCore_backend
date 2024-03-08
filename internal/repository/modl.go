package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"api.brutecore/libs/lib_db"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type MODLRepositoryLayer struct {
	db *lib_db.DB
}

const (
	dirName = "modules"
)

func NewMODLRepositoryLayer(d *lib_db.DB) *MODLRepositoryLayer {
	return &MODLRepositoryLayer{
		db: d,
	}
}

func (r *MODLRepositoryLayer) GetModules() (*lib_db.DBResult, error) {
	return r.db.QueryRow(lib_db.TxRead, QGetMdules)
}

func (r *MODLRepositoryLayer) DeleteModule(sID string) error {
	id, err := strconv.ParseInt(sID, 10, 64)
	if err != nil {
		return err
	}

	res, err := r.db.QueryRow(lib_db.TxRead, "SELECT * FROM MODULE WHERE ID = $1", id)
	if err != nil {
		return err
	}

	if err := os.Remove((*res)[0]["path"].(string)); err != nil {
		return err
	}

	_, err = r.db.Exec(lib_db.TxWrite, "DELETE FROM MODULE WHERE ID = $1; DELETE FROM MODULE_INPUT WHERE ID = $2;", id, id)
	return err
}

func (r *MODLRepositoryLayer) UploadModule(c *fiber.Ctx) error {
	file, err := c.FormFile("file")
	if err != nil {
		return errors.New("Файл модуля не загружен")
	}

	if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
		return errors.New("Не удалось создать директорию")
	}

	uuid := uuid.New().String()
	fileName := filepath.Join(dirName, uuid+"___"+file.Filename)
	if _, err := os.Stat(fileName); err == nil {
		return errors.New("Модуль с таким именем уже существует")
	}

	if err := c.SaveFile(file, fileName); err != nil {
		return errors.New("Не удалось сохранить файл")
	}

	if strings.ToUpper(runtime.GOOS) == "LINUX" {
		_, _ = exec.Command("sudo", "chmod", "+x", fileName).Output()
	}
	cmd, err := exec.Command(fileName, "-getinfo").Output()
	if err != nil {
		_ = os.Remove(fileName)
		return err
	}

	mapa := make(map[string]interface{})
	mapa["name"] = "none"
	mapa["version"] = "none"
	mapa["author"] = "none"
	mapa["data_type"] = "0"
	mapa["type"] = "MT1"

	if err := json.Unmarshal(cmd, &mapa); err != nil {
		_ = os.Remove(fileName)
		return err
	}

	tx, err := r.db.StartTx(lib_db.TxWrite)
	if err != nil {
		_ = os.Remove(fileName)
		return err
	}

	trueTx := tx.(*sql.Tx)
	ras, err := trueTx.Exec(QInsertModule, mapa["name"].(string), mapa["version"].(string), mapa["author"].(string), mapa["data_type"].(string), fileName, mapa["type"].(string))
	if err != nil {
		trueTx.Rollback()
		_ = os.Remove(fileName)
		return err
	}

	lID, _ := ras.LastInsertId()
	if mapa["input"] != nil {
		for _, i := range mapa["input"].([]interface{}) {
			inputData := i.(map[string]interface{})
			_, err = trueTx.Exec(QInsertModuleFields, lID, inputData["type"].(string), inputData["name"].(string), inputData["description"].(string))
			if err != nil {
				trueTx.Rollback()
				_ = os.Remove(fileName)
				return err
			}
		}
	}
	err = trueTx.Commit()
	if err != nil {
		_ = os.Remove(fileName)
	}
	return err

}
