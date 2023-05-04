package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sergi/go-diff/diffmatchpatch"
)

type FileInfo struct {
	ID         string `json:"id"`
	Version    int    `json:"version"`
	Identifier string `json:"identifier"`
	Sha1       string `json:"sha1"`
	Md5        string `json:"md5"`
}

type FileStorage struct {
	Files    map[string][]FileInfo `json:"files"`
	JSONFile string
}

var (
	storedConfPath = "storedconfs"
)

func main() {
	err := os.MkdirAll(storedConfPath, 0755)
	if err != nil {
		log.Fatal("Error creating storedconfs directory: ", err)
	}

	storage := &FileStorage{
		Files:    make(map[string][]FileInfo),
		JSONFile: "file_status.json",
	}
	storage.loadFileStatus()

	r := mux.NewRouter()
	r.HandleFunc("/upload", storage.uploadFile).Methods("POST")
	r.HandleFunc("/files", storage.listFiles).Methods("GET")
	r.HandleFunc("/files/{identifier}", storage.downloadFile).Methods("GET")
	r.HandleFunc("/files/{identifier}/diff/{version}", storage.showDiff).Methods("GET")
	r.HandleFunc("/hash/{hash}", storage.getFileByHash).Methods("GET")

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)

	shutdownChannel := make(chan struct{})
	go func() {
		<-signalChannel
		close(shutdownChannel)
	}()

	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func() {
		log.Println("Starting server on", srv.Addr)
		if err := srv.ListenAndServe(); err != nil {
			log.Fatal(err)
		}
	}()

	defer storage.saveFileStatus()
	<-shutdownChannel

	// Gracefully shut down the server.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server shutdown failed:", err)
	}

	log.Println("Server stopped")
}

func (s *FileStorage) saveFileStatus() {
	file, err := os.Create(s.JSONFile)
	if err != nil {
		log.Fatal("Error creating JSON file: ", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(s)
	if err != nil {
		log.Fatal("Error saving JSON file: ", err)
	}
}

func (s *FileStorage) loadFileStatus() {
	file, err := os.Open(s.JSONFile)
	if err != nil {
		if os.IsNotExist(err) {
			return // If file does not exist, we don't need to load it.
		}
		log.Fatal("Error opening JSON file: ", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(s)
	if err != nil {
		log.Fatal("Error loading JSON file: ", err)
	}
}

func (s *FileStorage) getFileByHash(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hash := vars["hash"]

	for _, versions := range s.Files {
		for _, fileInfo := range versions {
			if fileInfo.Sha1 == hash || fileInfo.Md5 == hash {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(fileInfo)
				return
			}
		}
	}

	http.Error(w, "File not found", http.StatusNotFound)
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

	sha1, md5 := hashFile(data)
	for _, fileInfo := range s.Files[identifier] {
		if fileInfo.Sha1 == sha1 && fileInfo.Md5 == md5 {
			http.Error(w, "File with the same hash is already stored", http.StatusBadRequest)
			return
		}
	}

	compressedData, err := compressFile(data, header.Filename)
	if err != nil {
		http.Error(w, "Error compressing file", http.StatusInternalServerError)
		return
	}

	fileInfo := storeFile(s, identifier, compressedData, sha1, md5)
	s.saveFileStatus()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(fileInfo)
}

func compressFile(data []byte, filename string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	fileWriter, err := zipWriter.Create(filename)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(fileWriter, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	err = zipWriter.Close()
	if err != nil {
		return nil, err
	}

	return &buf, nil
}

func storeFile(s *FileStorage, identifier string, compressedData *bytes.Buffer, sha1, md5 string) FileInfo {
	version := 1
	versions, exists := s.Files[identifier]
	if exists {
		version = len(versions) + 1
	}

	fileInfo := FileInfo{
		ID:         fmt.Sprintf("%s_v%d", identifier, version),
		Version:    version,
		Identifier: identifier,
		Sha1:       sha1,
		Md5:        md5,
	}

	err := os.WriteFile(filepath.Join("storedconfs", fileInfo.ID+".zip"), compressedData.Bytes(), 0644)
	if err != nil {
		log.Fatal("Error storing file: ", err)
	}

	s.Files[identifier] = append(s.Files[identifier], fileInfo)
	return fileInfo
}

func (s *FileStorage) listFiles(w http.ResponseWriter, r *http.Request) {
	var fileList []FileInfo
	for _, versions := range s.Files {
		fileList = append(fileList, versions...)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileList)
}

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
	io.WriteString(w, fileData)
}

func (s *FileStorage) showDiff(w http.ResponseWriter, r *http.Request) {
	fileInfo1, err := s.getFileByVersion(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	version := fileInfo1.Version + 1
	versions, exists := s.Files[fileInfo1.Identifier]
	if !exists || version > len(versions) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	fileInfo2 := versions[version-1]

	fileData1, err := readFileContent(fileInfo1.ID + ".zip")
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	fileData2, err := readFileContent(fileInfo2.ID + ".zip")
	if err != nil {
		http.Error(w, "Error reading file", http.StatusInternalServerError)
		return
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(fileData1, fileData2, false)
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, dmp.DiffPrettyText(diffs))

}

func (s *FileStorage) getFileByVersion(r *http.Request) (*FileInfo, error) {
	vars := mux.Vars(r)
	// fmt.Println(r)
	identifier := vars["identifier"]
	versionStr := vars["version"]

	if versionStr == "" {
		versionStr = "1"
	}
	// fmt.Println("ident", identifier, "ver", versionStr)
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version")
	}

	versions, exists := s.Files[identifier]
	if !exists || version > len(versions) {
		return nil, fmt.Errorf("file not found")
	}

	return &versions[version-1], nil

}

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

func hashFile(data []byte) (string, string) {
	sha1Hash := sha1.New()
	sha1Hash.Write(data)
	sha1Result := sha1Hash.Sum(nil)

	md5Hash := md5.New()
	md5Hash.Write(data)
	md5Result := md5Hash.Sum(nil)

	return hex.EncodeToString(sha1Result), hex.EncodeToString(md5Result)
}
