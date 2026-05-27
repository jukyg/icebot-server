package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func handleAIChatAPI(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		entries := autoDeploy.GetAll()
		aiEntries := make([]map[string]interface{}, 0)
		for _, e := range entries {
			if e.AIChat {
				aiEntries = append(aiEntries, map[string]interface{}{
					"trackedID": e.TrackedID,
					"name":      e.Name,
					"persona":   e.AIPersona,
					"message":   e.Message,
				})
			}
		}

		sessionsMu.RLock()
		sessionsList := make([]map[string]interface{}, 0)
		for _, s := range sessions {
			s.mu.RLock()
			enabled := s.aiChatEnabled
			room := s.room
			botCount := len(s.bots)
			s.mu.RUnlock()
			sessionsList = append(sessionsList, map[string]interface{}{
				"id":      s.id,
				"room":    room,
				"enabled": enabled,
				"bots":    botCount,
			})
		}
		sessionsMu.RUnlock()

		cfg := GetConfig()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"masterEnabled":     autoDeploy.Master(),
			"tracked":           aiEntries,
			"sessions":          sessionsList,
			"geminiKeySet":      cfg.GeminiAPIKey != "",
			"geminiModel":       GeminiModel(),
			"geminiReady":       GeminiReady(),
		})

	case "POST":
		var req struct {
			SessionID string `json:"sessionId"`
			Enabled   bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		sessionsMu.RLock()
		s, ok := sessions[req.SessionID]
		sessionsMu.RUnlock()
		if !ok {
			http.Error(w, fmt.Sprintf("session %s not found", req.SessionID), http.StatusNotFound)
			return
		}
		s.mu.Lock()
		s.aiChatEnabled = req.Enabled
		s.mu.Unlock()
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "enabled": req.Enabled})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleGeminiConfig(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case "GET":
		cfg := GetConfig()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"geminiKeySet": cfg.GeminiAPIKey != "",
			"geminiModel":  GeminiModel(),
			"geminiReady":  GeminiReady(),
		})

	case "POST":
		var req struct {
			APIKey string `json:"apiKey"`
			Model  string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.APIKey != "" {
			SetGeminiAPIKey(req.APIKey)
		}
		if req.Model != "" {
			SetGeminiModel(req.Model)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":           true,
			"geminiKeySet": GeminiKey() != "",
			"geminiModel":  GeminiModel(),
			"geminiReady":  GeminiReady(),
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
