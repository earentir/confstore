package main

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

func (s *FileStorage) getFileByHash(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	hash := vars["hash"]

	for _, fileInfo := range s.Files {
		if fileInfo.Sha1 == hash || fileInfo.Md5 == hash {
			w.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(w).Encode(fileInfo)
			if err != nil {
				http.Error(w, "Error encoding JSON", http.StatusInternalServerError)
			}
			return
		}
	}

	http.Error(w, "File not found", http.StatusNotFound)
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
