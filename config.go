package main

import (
	"encoding/json"
	"os"
	"sync"
)

type AppConfig struct {
	GeminiAPIKey string `json:"gemini_api_key"`
	GeminiModel  string `json:"gemini_model"`
}

var (
	appConfig     AppConfig
	appConfigMu   sync.RWMutex
	configPath    = "config.json"
)

func LoadConfig() {
	appConfigMu.Lock()
	defer appConfigMu.Unlock()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			appConfig = AppConfig{}
			saveConfigLocked()
			return
		}
		return
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return
	}
	appConfig = cfg

	if cfg.GeminiAPIKey != "" {
		geminiAPIKey = cfg.GeminiAPIKey
	}
	if cfg.GeminiModel != "" {
		geminiModel = cfg.GeminiModel
	}
}

func saveConfigLocked() {
	data, err := json.MarshalIndent(appConfig, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(configPath, data, 0644)
}

func GetConfig() AppConfig {
	appConfigMu.RLock()
	defer appConfigMu.RUnlock()
	return appConfig
}

func SetGeminiAPIKey(key string) {
	appConfigMu.Lock()
	defer appConfigMu.Unlock()
	appConfig.GeminiAPIKey = key
	geminiAPIKey = key
	saveConfigLocked()
}

func SetGeminiModel(model string) {
	appConfigMu.Lock()
	defer appConfigMu.Unlock()
	appConfig.GeminiModel = model
	geminiModel = model
	saveConfigLocked()
}
