package main
// ============================================================================
// health.go — System health API
// ============================================================================
import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

var startTime = time.Now()

type SystemHealth struct {
	Uptime       string  `json:"uptime"`
	GoVersion    string  `json:"goVersion"`
	Goroutines   int     `json:"goroutines"`
	MemoryMB     float64 `json:"memoryMB"`
	AllocMB      float64 `json:"allocMB"`
	HeapInuseMB  float64 `json:"heapInuseMB"`
	CPUCount     int     `json:"cpuCount"`
	Sessions     int     `json:"sessions"`
	TotalBots    int     `json:"totalBots"`
	JoinedBots   int     `json:"joinedBots"`
	ProxyTotal   int     `json:"proxyTotal"`
	ProxyAvail   int     `json:"proxyAvail"`
	ProxyBlocked int     `json:"proxyBlocked"`
}

func collectHealth() SystemHealth {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	sessionsMu.RLock()
	sessCount := len(sessions)
	totalBots := 0
	joinedBots := 0
	for _, s := range sessions {
		s.mu.RLock()
		totalBots += len(s.bots)
		for _, b := range s.bots {
			if b.joinConfirmed.Load() {
				joinedBots++
			}
		}
		s.mu.RUnlock()
	}
	sessionsMu.RUnlock()

	proxyTotal, proxyAvail, proxyBlocked := ProxyStats()

	return SystemHealth{
		Uptime:       time.Since(startTime).Round(time.Second).String(),
		GoVersion:    runtime.Version(),
		Goroutines:   runtime.NumGoroutine(),
		MemoryMB:     float64(m.TotalAlloc) / 1024 / 1024,
		AllocMB:      float64(m.Alloc) / 1024 / 1024,
		HeapInuseMB:  float64(m.HeapInuse) / 1024 / 1024,
		CPUCount:     runtime.NumCPU(),
		Sessions:     sessCount,
		TotalBots:    totalBots,
		JoinedBots:   joinedBots,
		ProxyTotal:   proxyTotal,
		ProxyAvail:   proxyAvail,
		ProxyBlocked: proxyBlocked,
	}
}

func handleAdminHealth(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	health := collectHealth()
	json.NewEncoder(w).Encode(health)
}