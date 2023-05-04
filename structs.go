type ServerConfig struct {
	Address      string `json:"address"`
	Port         int    `json:"port"`
	ReadTimeout  string `json:"read_timeout"`
	WriteTimeout string `json:"write_timeout"`
	ConfigPath   string `json:"config_path"`
}
