package server

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
)

var fileNamePattern = regexp.MustCompile(`^[a-zA-Z0-9\-_\.]+$`)

// CheckFileName проверяет, что имя файла содержит только допустимые символы.
func CheckFileName(name string) error {
	if !fileNamePattern.MatchString(name) {
		return errors.New("имя файла содержит недопустимые символы")
	}
	return nil
}

// CreateNewFile открывает или создаёт новый файл в указанной директории.
func CreateNewFile(directory, name string) (*os.File, error) {
	if err := CheckFileName(name); err != nil {
		return nil, err
	}
	fullPath := filepath.Join(directory, name)
	return os.Create(fullPath)
}

// WriteFileData записывает полученный кусок данных в файл.
func WriteFileData(f *os.File, data []byte) error {
	_, err := f.Write(data)
	return err
}
