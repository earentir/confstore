package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
)

func (s *FileStorage) downloadFile(w http.ResponseWriter, r *http.Request) {
	fileInfo, err := s.getFileByVersion(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	fileData, err := readFileContent(filepath.Join("storedconfs", fileInfo.ID+".zip"))
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(fileInfo.ID))
	w.Header().Set("Content-Type", "application/octet-stream")
	_, err = io.WriteString(w, fileData)
	if err != nil {
		http.Error(w, "Error writing file", http.StatusInternalServerError)
		return
	}
}

func (s *FileStorage) uploadFile(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error uploading file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	identifier := r.FormValue("identifier")
	if identifier == "" {
		http.Error(w, "Identifier is required", http.StatusBadRequest)
		return
	}

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	fileInfo, err := s.processFile(data, header.Filename, identifier, true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(fileInfo)
	if err != nil {
		fmt.Println(err)
	}
}
