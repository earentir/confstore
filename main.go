package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func main() {
	configuration, err := loadConfiguration()
	if err != nil {
		log.Fatal("Error loading configuration: ", err)
	}

	err = os.MkdirAll(configuration.ConfPath, 0755)
	if err != nil {
		log.Fatal("Error creating storedconfs directory: ", err)
	}

	storage := &FileStorage{
		Files: make([]FileInfo, 0),
	}
	storage.loadFileStatus(configuration)

	if len(os.Args) > 1 {
		if os.Args[1] == "rebuild" {
			fmt.Println("Removing ", configuration.JSONFile)
			err := os.Remove(configuration.JSONFile)
			if err != nil {
				log.Fatal("Error removing JSON file: ", err)
			}

			fmt.Println("Rebuilding file status from storedconfs directory")
			err = storage.rebuildFileStatusFromStoredFiles()
			if err != nil {
				log.Fatal("Error rebuilding file status: ", err)
			}
			storage.saveFileStatus()
			return
		}
	}

	r := mux.NewRouter()
	r.HandleFunc("/upload", storage.uploadFile).Methods("POST")
	r.HandleFunc("/files", storage.listFiles).Methods("GET")
	r.HandleFunc("/files/{identifier}", storage.downloadFile).Methods("GET")
	r.HandleFunc("/files/{identifier}/diff/{version}", storage.showDiff).Methods("GET")
	r.HandleFunc("/hash/{hash}", storage.getFileByHash).Methods("GET")
	r.HandleFunc("/files/{identifier}", storage.deleteFile).Methods("DELETE")

	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt)

	shutdownChannel := make(chan struct{})
	go func() {
		<-signalChannel
		close(shutdownChannel)
	}()

	srv := &http.Server{
		Handler:      r,
		Addr:         fmt.Sprintf("%s:%s", configuration.ListenAddr, configuration.Port),
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func() {
		log.Println("Starting server on", srv.Addr)
		if configuration.CertFile != "" && configuration.KeyFile != "" {
			if err := srv.ListenAndServeTLS(configuration.CertFile, configuration.KeyFile); err != nil {
				log.Fatal(err)
			}
		} else if err := srv.ListenAndServe(); err != nil {
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

func (s *FileStorage) rebuildFileStatusFromStoredFiles() error {
	files, err := os.ReadDir(s.Configuration.ConfPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".zip" {
			filename, data, err := ExtractFileFromZipToBytes(filepath.Join(s.Configuration.ConfPath, file.Name()))
			fmt.Println("filename", filename)
			if err != nil {
				return err
			}

			_, err = s.processFile(data, filename, "", false)
			if err != nil {
				return err
			}

		}
	}
	return nil
}

func (s *FileStorage) processFile(input interface{}, fileName, identifier string, savefile bool) (*FileInfo, error) {
	var (
		data []byte
		err  error
	)

	switch v := input.(type) {
	case string:
		savefile = true
		data, err = os.ReadFile(v)
		if err != nil {
			return nil, fmt.Errorf("Error reading file: %v", err)
		}
	case []byte:
		data = v
	default:
		return nil, fmt.Errorf("Invalid input type")
	}

	if identifier == "" {
		identifier = strings.ReplaceAll(fileName, ".", "")
	}

	sha1, md5 := hashFile(data)
	for _, fileInfo := range s.Files {
		if fileInfo.Identifier == identifier && (fileInfo.Sha1 == sha1 || fileInfo.Md5 == md5) {
			return nil, fmt.Errorf("File with the same hash is already stored")
		}
	}

	compressedData, err := compressFile(data, fileName)
	if err != nil {
		return nil, fmt.Errorf("Error compressing file: %v", err)
	}

	fmt.Println("storing file")
	fileInfo := storeFile(s, identifier, compressedData, sha1, md5, savefile)
	s.saveFileStatus()

	return &fileInfo, nil
}

func storeFile(s *FileStorage, identifier string, compressedData *bytes.Buffer, sha1, md5 string, savefile bool) FileInfo {
	version := 1
	for _, fileInfo := range s.Files {
		if fileInfo.Identifier == identifier {
			if fileInfo.Version >= version {
				version = fileInfo.Version + 1
			}
		}
	}

	fileInfo := FileInfo{
		ID:         fmt.Sprintf("%s_v%d", identifier, version),
		Version:    version,
		Identifier: identifier,
		Sha1:       sha1,
		Md5:        md5,
	}

	if savefile {
		fmt.Println("saving file")
		err := os.WriteFile(filepath.Join("storedconfs", fileInfo.ID+".zip"), compressedData.Bytes(), 0644)
		if err != nil {
			log.Fatal("Error storing file: ", err)
		}
	}
	s.Files = append(s.Files, fileInfo)
	return fileInfo
}

func (s *FileStorage) listFiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(s.Files)
	if err != nil {
		http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
		return
	}
}

func (s *FileStorage) getFileByVersion(r *http.Request) (*FileInfo, error) {
	vars := mux.Vars(r)
	identifier := vars["identifier"]
	versionStr := vars["version"]

	if versionStr == "" {
		versionStr = "1"
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid version")
	}

	for _, fileInfo := range s.Files {
		if fileInfo.Identifier == identifier && fileInfo.Version == version {
			return &fileInfo, nil
		}
	}

	return nil, fmt.Errorf("file not found")
}

func (s *FileStorage) deleteFile(w http.ResponseWriter, r *http.Request) {
	fileInfo, err := s.getFileByVersion(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	err = os.Remove(filepath.Join(s.Configuration.ConfPath, fileInfo.ID+".zip"))
	if err != nil {
		fmt.Println("Error removing file: ", err)
	}

	s.removeFileInfo(fileInfo.ID)

	w.WriteHeader(http.StatusOK)
}
