package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	geminiAPIKey    string
	geminiModel     string
	geminiMu        sync.RWMutex
	geminiHTTP      = &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{IdleConnTimeout: 90 * time.Second}}
	defaultModel    = "gemini-2.0-flash-lite"
)

func GeminiSetKey(key string) {
	geminiMu.Lock()
	defer geminiMu.Unlock()
	geminiAPIKey = key
}

func GeminiSetModel(model string) {
	geminiMu.Lock()
	defer geminiMu.Unlock()
	if strings.TrimSpace(model) == "" {
		geminiModel = defaultModel
	} else {
		geminiModel = model
	}
}

func GeminiKey() string {
	geminiMu.RLock()
	defer geminiMu.RUnlock()
	return geminiAPIKey
}

func GeminiModel() string {
	geminiMu.RLock()
	defer geminiMu.RUnlock()
	if geminiModel == "" {
		return defaultModel
	}
	return geminiModel
}

func GeminiReady() bool {
	return GeminiKey() != ""
}

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	SystemInstruction *geminiSystemInstruction `json:"system_instruction,omitempty"`
	GenerationConfig geminiGenConfig        `json:"generation_config"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiSystemInstruction struct {
	Parts []geminiPart `json:"parts"`
}

type geminiGenConfig struct {
	Temperature     float64 `json:"temperature"`
	TopP            float64 `json:"top_p"`
	MaxOutputTokens int     `json:"max_output_tokens"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
	Error      *geminiError      `json:"error,omitempty"`
}

type geminiCandidate struct {
	Content      geminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type geminiError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func geminiGenerate(persona, userMsg, targetName string) (string, error) {
	key := GeminiKey()
	if key == "" {
		return "", fmt.Errorf("gemini API key not set")
	}
	model := GeminiModel()

	systemPrompt := fmt.Sprintf(`You are a Gartic.io player chatting in the game's public chat.

Personality: %s

RULES:
- Respond in FLUENT ARABIC only.
- Keep responses short (1-2 sentences max, 3-5 words preferred).
- Be natural, casual, and match the energy of the conversation.
- Do not use formal Arabic. Use spoken/colloquial Arabic (a mix of fusha and common dialect is fine).
- Never mention you are an AI or bot.
- Never use emojis or excessive punctuation.
- If the message is directed at you (mentions your name "%s"), reply directly.
- Reply as a real Gartic player would — quick, playful, sometimes sarcastic.
- Do not repeat the user's message back to them.
- Do not use greetings like "مرحبا" unless the conversation is just starting.`, persona, targetName)

	req := geminiRequest{
		Contents: []geminiContent{
			{
				Role: "user",
				Parts: []geminiPart{
					{Text: userMsg},
				},
			},
		},
		SystemInstruction: &geminiSystemInstruction{
			Parts: []geminiPart{{Text: systemPrompt}},
		},
		GenerationConfig: geminiGenConfig{
			Temperature:     0.9,
			TopP:            0.95,
			MaxOutputTokens: 60,
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, key)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := geminiHTTP.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("gemini request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		var errResp geminiResponse
		if json.Unmarshal(raw, &errResp) == nil && errResp.Error != nil {
			return "", fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", fmt.Errorf("gemini API error (HTTP %d): %s", resp.StatusCode, string(raw))
	}

	var result geminiResponse
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini returned empty response")
	}

	text := strings.TrimSpace(result.Candidates[0].Content.Parts[0].Text)
	if text == "" {
		return "", fmt.Errorf("gemini returned empty text")
	}

	return text, nil
}
