package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/sergi/go-diff/diffmatchpatch"
)

func readFileContent(zipPath string) (string, error) {
	zipData, err := os.ReadFile(zipPath)
	if err != nil {
		return "", err
	}
	zipReader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return "", err
	}

	if len(zipReader.File) != 1 {
		return "", fmt.Errorf("unexpected number of files in zip")
	}

	file := zipReader.File[0]
	fileData, err := file.Open()
	if err != nil {
		return "", err
	}
	defer fileData.Close()

	data, err := io.ReadAll(fileData)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func (s *FileStorage) showDiff(w http.ResponseWriter, r *http.Request) {
	fileInfo1, err := s.getFileByVersion(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	version := fileInfo1.Version + 1
	var fileInfo2 *FileInfo
	for _, fileInfo := range s.Files {
		if fileInfo.Identifier == fileInfo1.Identifier && fileInfo.Version == version {
			fileInfo2 = &fileInfo
			break
		}
	}

	if fileInfo2 == nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	fileData1, err := readFileContent(filepath.Join("storedconfs", fileInfo1.ID+".zip"))
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	fileData2, err := readFileContent(filepath.Join("storedconfs", fileInfo2.ID+".zip"))
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(fileData1, fileData2, false)
	w.Header().Set("Content-Type", "text/plain")
	_, err = io.WriteString(w, dmp.DiffPrettyText(diffs))
	if err != nil {
		http.Error(w, "Error writing file", http.StatusInternalServerError)
		return
	}
}
