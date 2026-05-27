package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type LogLevel string

const (
	LevelInfo    LogLevel = "info"
	LevelWarn    LogLevel = "warn"
	LevelError   LogLevel = "error"
	LevelSuccess LogLevel = "success"
)

type LogEntry struct {
	Timestamp string   `json:"ts"`
	Level     LogLevel `json:"level"`
	Source    string   `json:"source"`
	Message   string   `json:"msg"`
}

type RingLogger struct {
	mu      sync.Mutex
	entries []LogEntry
	capacity int
	next     int
	count    int
}

var activityLog = NewRingLogger(500)

func NewRingLogger(capacity int) *RingLogger {
	return &RingLogger{
		entries:  make([]LogEntry, capacity),
		capacity: capacity,
	}
}

func (l *RingLogger) Log(level LogLevel, source, msg string) {
	entry := LogEntry{
		Timestamp: time.Now().Format("15:04:05.000"),
		Level:     level,
		Source:    source,
		Message:   msg,
	}

	l.mu.Lock()
	l.entries[l.next] = entry
	l.next = (l.next + 1) % l.capacity
	if l.count < l.capacity {
		l.count++
	}
	l.mu.Unlock()
}

func (l *RingLogger) Recent(n int) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n <= 0 || n > l.count {
		n = l.count
	}

	res := make([]LogEntry, n)
	if l.count < l.capacity {
		start := l.count - n
		if start < 0 {
			start = 0
		}
		copy(res, l.entries[start:l.count])
		return res
	}

	start := (l.next - n + l.capacity) % l.capacity
	for i := 0; i < n; i++ {
		res[i] = l.entries[(start+i)%l.capacity]
	}
	return res
}

func LogInfo(source, msg string) {
	activityLog.Log(LevelInfo, source, msg)
}

func LogWarn(source, msg string) {
	activityLog.Log(LevelWarn, source, msg)
}

func LogError(source, msg string) {
	activityLog.Log(LevelError, source, msg)
}

func LogSuccess(source, msg string) {
	activityLog.Log(LevelSuccess, source, msg)
}

func handleAdminLogs(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	n := 200
	entries := activityLog.Recent(n)
	if entries == nil {
		entries = []LogEntry{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"count":   len(entries),
	})
}

// Integration helpers

func logBotJoin(botNumId int, nick, room string, success bool) {
	if success {
		LogSuccess("Bot", fmt.Sprintf("Bot #%d (%s) joined room %s", botNumId, nick, room))
	} else {
		LogError("Bot", fmt.Sprintf("Bot #%d (%s) FAILED to join room %s", botNumId, nick, room))
	}
}

func logProxyFailure(ip, port string, err error) {
	LogWarn("Proxy", fmt.Sprintf("Proxy %s:%s failed: %v", ip, port, err))
}

func logSessionAction(sessionID, action string) {
	LogInfo("Session", fmt.Sprintf("[%s] %s", sessionID, action))
}
