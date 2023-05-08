package main

import (
	"encoding/json"
	"os"
)

func loadConfiguration() (*Configuration, error) {
	file, err := os.Open("confstore.json")
	if err != nil {
		if os.IsNotExist(err) {
			configuration := *defaultConfiguration()
			err = saveConfiguration(&configuration)
			if err != nil {
				return nil, err
			}
			return &configuration, nil
		}
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	conf := &Configuration{}
	err = decoder.Decode(conf)
	if err != nil {
		return nil, err
	}

	return conf, nil
}

func saveConfiguration(conf *Configuration) error {
	file, err := os.Create("confstore.json")
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(conf)
	if err != nil {
		return err
	}

	return nil
}

func defaultConfiguration() *Configuration {
	return &Configuration{
		JSONFile:   defaultJSONFile,
		ConfPath:   defaultConfPath,
		ListenAddr: defaultListenAddr,
		Port:       defaultPort,
	}
}
