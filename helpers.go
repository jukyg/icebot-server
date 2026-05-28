package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// ============================================================================
// helpers.go — Shared utility functions + global HTTP client
// ============================================================================

// httpClient is the shared HTTP client used by bot fetches (via CroxyProxy IPs).
// Uses a 15s timeout and skips TLS verification (CroxyProxy uses self-signed certs).
var httpClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     60 * time.Second,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
	},
}

// getCookieForProxy returns any cookies to send for the given proxy IP.
// Currently returns empty string — extend here if per-proxy cookies are needed.
func getCookieForProxy(ip string) string {
	return ""
}

// FetchTurnstileToken pops a token from the local pool, waiting up to 45 seconds
// if none is immediately available.
func FetchTurnstileToken() string {
	return PopTurnstileTokenWait(45 * time.Second)
}

// ReleaseTurnstileToken is a no-op — tokens are consumed immediately by gartic
// siteverify. The function exists to match callers that previously had a
// release/lease model. Kept for API compatibility.
func ReleaseTurnstileToken(tok string) {
	// no-op — token is consumed on first use by gartic
}

// jsonString returns a JSON-encoded string value (with surrounding quotes).
// Used to safely embed user text into socket.io protocol messages.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// parseRoom extracts the room code from a gartic.io URL or plain code.
// Examples:
//
//	"https://gartic.io/abcde"  → "abcde"
//	"gartic.io/abcde?foo=bar"  → "abcde"
//	"abcde"                    → "abcde"
func parseRoom(input string) string {
	input = strings.TrimSpace(input)
	if strings.Contains(input, "gartic.io/") {
		parts := strings.Split(input, "gartic.io/")
		if len(parts) > 1 {
			code := strings.Split(parts[1], "?")[0]
			code = strings.TrimRight(code, "/")
			return code
		}
	}
	return input
}

// JoinMessage represents a message to send automatically after a bot joins a room.
// Type can be "broadcast" (chat + answer), "message" (chat only), or "answer" (answer only).
type JoinMessage struct {
	Type string `json:"type"` // "broadcast", "message", or "answer"
	Msg  string `json:"msg"`
}

// CustomNick represents a custom nickname with an optional avatar override.
type CustomNick struct {
	Nick   string `json:"nick"`
	Avatar *int   `json:"avatar,omitempty"`
}

// safePrintf wraps fmt.Printf to prevent panics from bad format strings.
func safePrintf(format string, args ...interface{}) {
	defer func() { recover() }()
	fmt.Printf(format, args...)
}

// getFloat extracts a float64 from a map, with a default fallback.
// Handles both float64 and string values.
func getFloat(m map[string]interface{}, key string, def float64) float64 {
	v, ok := m[key]
	if !ok {
		return def
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		if f == 0 {
			return def
		}
		return f
	default:
		return def
	}
}

// getString extracts a string from a map, with a default fallback.
func getString(m map[string]interface{}, key, def string) string {
	v, ok := m[key]
	if !ok {
		return def
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return def
	}
	return s
}