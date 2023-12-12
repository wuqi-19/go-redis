package main

import (
	"encoding/json"
	"io"
	"os"
)

type Config struct {
	Port int `josn:"port"`
}

func LoadConfig(path string) (config *Config, err error) {
	config = &Config{}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	jsonStr, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(jsonStr, config); err != nil {
		return nil, err
	}
	return config, nil
}
