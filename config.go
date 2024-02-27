package main

import (
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"os"
)

type ConfigMinio struct {
	Endpoint string `json:"endpoint"`
	ID       string `json:"id"`
	Secret   string `json:"secret"`
	Bucket   string `json:"bucket"`
	UseSSL   bool   `json:"use_ssl"`
}

type Config struct {
	Listen string            `json:"listen"`
	Users  map[string]string `json:"users"`
	Minio  ConfigMinio       `json:"minio"`

	users map[string][]byte

	// "new" command only
	URL string `json:"url"`
}

func parse_config(path string) Config {
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

	config.users = map[string][]byte{}
	// decode keys
	for username, key := range config.Users {
		config.users[username], err = base64.StdEncoding.DecodeString(key)
		if err != nil {
			slog.Error("parse config: decode key", "username", username, "key", key)
			panic(err)
		}
	}
	config.Users = nil

	if config.Listen == "" {
		slog.Info(`no listen set, default to ":8080"`)
		config.Listen = ":8080"
	}

	return config
}
