package main

type FileInfo struct {
	ID         string `json:"id"`
	Version    int    `json:"version"`
	Identifier string `json:"identifier"`
	Sha1       string `json:"sha1"`
	Md5        string `json:"md5"`
}

type FileStorage struct {
	Files         []FileInfo `json:"files"`
	Configuration Configuration
}

type Configuration struct {
	JSONFile   string `json:"json_file"`
	ConfPath   string `json:"conf_path"`
	ListenAddr string `json:"listen_addr"`
	Port       string `json:"port"`
	CertFile   string `json:"cert_file"`
	KeyFile    string `json:"key_file"`
}

const (
	defaultJSONFile   = "file_status.json"
	defaultListenAddr = "127.0.0.1"
	defaultPort       = "8080"
	defaultConfPath   = "storedconfs"
)
