package utility

import (
	"archive/zip"
	"bytes"
)

type File struct {
	Name string
	Body []byte
}

func MakeZip(files *[]File) ([]byte, error) {
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)

	for _, file := range *files {
		fileWriter, err := zipWriter.Create(file.Name)
		if err != nil {
			return nil, err
		}
		_, err = fileWriter.Write(file.Body)
		if err != nil {
			return nil, err
		}
	}

	err := zipWriter.Close()
	return archive.Bytes(), err
}
