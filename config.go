package main

import (
	"encoding/base64"
	"encoding/json"
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
	Listen string      `json:"listen"`
	Key    string      `json:"key"`
	Minio  ConfigMinio `json:"minio"`

	key []byte

	// new command only
	URL string `json:"url"`
}

func parse_config(path string) Config {
	config_file, err := os.Open(path)
	if err != nil {
		panic(err)
	}

	var config Config
	err = json.NewDecoder(config_file).Decode(&config)
	if err != nil {
		panic(err)
	}

	config.key, err = base64.StdEncoding.DecodeString(config.Key)
	if err != nil {
		panic(err)
	}

	if config.Listen == "" {
		config.Listen = ":8080"
	}

	return config
}
