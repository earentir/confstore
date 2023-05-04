package main

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

type ServerConfig struct {
	Address      string `json:"address"`
	Port         int    `json:"port"`
	ReadTimeout  string `json:"read_timeout"`
	WriteTimeout string `json:"write_timeout"`
	ConfigPath   string `json:"config_path"`
}
