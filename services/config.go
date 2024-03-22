package services

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"os"
)

const (
	STORAGE_TYPE_MINIO = "minio"
	STORAGE_TYPE_LOCAL = "local"
)

type ConfigStorageMinio struct {
	Endpoint string `json:"endpoint"`
	ID       string `json:"id"`
	Secret   string `json:"secret"`
	Bucket   string `json:"bucket"`
	UseSSL   bool   `json:"use_ssl"`
}

type ConfigStorageLocal struct {
	Path string `json:"path"`
}

type ConfigStorage struct {
	Type        string             `json:"type"`
	ConfigLocal ConfigStorageLocal `json:"localSettings"`
	ConfigMinio ConfigStorageMinio `json:"minioSettings"`
}

type Config struct {
	Listen       string            `json:"listen"`
	UsersStr     map[string]string `json:"users"`
	DatabasePath string            `json:"db"`
	Storage      ConfigStorage     `json:"storage"`

	Users map[string][]byte

	// "new" command only
	URL string `json:"url"`
}

func ParseConfig(path string) Config {
	config_file, err := os.Open(path)
	if err != nil {
		slog.Error("parse config: open config", "path", path)
		panic(err)
	}

	var config Config
	err = json.NewDecoder(config_file).Decode(&config)
	if err != nil {
		slog.Error("parse config: decode config", "path", path)
		panic(err)
	}

	config.Users = map[string][]byte{}
	// decode keys
	for username, key := range config.UsersStr {
		config.Users[username], err = base64.StdEncoding.DecodeString(key)
		if err != nil {
			slog.Error("parse config: decode key", "username", username, "key", key)
			panic(err)
		}
	}
	config.UsersStr = nil

	if config.Listen == "" {
		slog.Info(`no listen set, default to ":8080"`)
		config.Listen = ":8080"
	}

	return config
}
