package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func (s *FileStorage) saveFileStatus() {
	fmt.Println("config from s", s.Configuration.JSONFile)
	file, err := os.Create(s.Configuration.JSONFile)
	if err != nil {
		log.Fatal("Error creating JSON file: ", err)
	}
	defer file.Close()

	// Clear out the files array before saving.
	s.Configuration = Configuration{}
	fmt.Println("jssssss", s.Configuration.JSONFile)

	encoder := json.NewEncoder(file)
	err = encoder.Encode(s)
	if err != nil {
		log.Fatal("Error saving JSON file: ", err)
	}
}

func (s *FileStorage) loadFileStatus(configuration *Configuration) {
	fmt.Println("JSON file", configuration.JSONFile)
	s.Configuration = *configuration
	file, err := os.Open(configuration.JSONFile)
	if err != nil {
		if os.IsNotExist(err) {
			// If file does not exist, rebuild it from storedconfs directory.
			fmt.Println(configuration.JSONFile, " file does not exist, rebuilding from storedconfs directory")
			err := s.rebuildFileStatusFromStoredFiles()
			if err != nil {
				log.Fatal("Error rebuilding file status: ", err)
			}
			return
		}
		log.Fatal("Error opening JSON file: ", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(s)
	if err != nil {
		// If file exists but is empty or malformed, rebuild it from storedconfs directory.
		err = s.rebuildFileStatusFromStoredFiles()
		if err != nil {
			log.Fatal("Error rebuilding file status: ", err)
		}
		return
	}
}

func (s *FileStorage) removeFileInfo(id string) {
	for i, fileInfo := range s.Files {
		if fileInfo.ID == id {
			s.Files = append(s.Files[:i], s.Files[i+1:]...)
			break
		}
	}
	s.saveFileStatus()
}
