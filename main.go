package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

// ============================================================================
// main.go — ICEbot Server v7 (Go Edition) with Residential Proxy Support
// Entry point: HTTP server on port 8090 with WebSocket upgrade, status APIs,
// and Turnstile token management.
// ============================================================================

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true }, // Allow all origins
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if this is a WebSocket upgrade request
	if r.Header.Get("Upgrade") == "" {
		// Regular HTTP request (browser visit) — show the modern dashboard
		handleDashboard(w, r)
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		color.Red("WebSocket upgrade failed: %s", err)
		return
	}

	room := r.URL.Query().Get("room")
	session := GetOrCreateSession(ws, room)

	defer func() {
		session.CloseConn(ws)
		ws.Close()
	}()

	// Read loop
	for {
		_, raw, err := ws.ReadMessage()
		if err != nil {
			break
		}
		session.HandleMessage(raw)
	}
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

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
	tokenAvail, tokenLastIn := TurnstilePoolStatus()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions":       sessCount,
		"totalBots":      totalBots,
		"joinedBots":     joinedBots,
		"proxies":        proxyTotal,
		"proxiesAvail":   proxyAvail,
		"proxiesBlocked": proxyBlocked,
		"tokens":         tokenAvail,
		"tokenLastInAgo": fmt.Sprintf("%.1fs", time.Since(tokenLastIn).Seconds()),
	})
}

func handleProxyStatus(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")

	total, avail, blocked := ProxyStats()
	proxies := make([]map[string]interface{}, 0)
	blockedMu.RLock()
	for _, p := range residentialProxies {
		status := "available"
		blockedAt, isBlocked := blockedProxies[p.IP]
		if isBlocked && time.Since(blockedAt) <= blockDuration {
			status = "blocked"
		}
		proxies = append(proxies, map[string]interface{}{
			"ip":     p.IP,
			"port":   p.Port,
			"status": status,
		})
	}
	blockedMu.RUnlock()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":     total,
		"available": avail,
		"blocked":   blocked,
		"proxies":   proxies,
	})
}

func main() {
	// Set up file logging — write to both console and server.log
	logFile, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		multiWriter := io.MultiWriter(os.Stdout, logFile)
		log.SetOutput(multiWriter)
		color.Output = io.MultiWriter(color.Output, logFile)
	}

	cyan := color.New(color.FgHiCyan, color.Bold)
	green := color.New(color.FgHiGreen)

	fmt.Println()
	cyan.Println("  ┌────────────────────────────────────────────────────────┐")
	cyan.Println("  │     ⚡ ICEbot v7.2 — HIGH-SPEED RESIDENTIAL EDITION    │")
	cyan.Println("  └────────────────────────────────────────────────────────┘")
	fmt.Println()

	green.Printf("  ▸ Connection Mode  : RESIDENTIAL PROXY\n")
	green.Printf("  ▸ Proxy Pool Size  : %d Residential Proxies Active\n", len(residentialProxies))
	green.Printf("  ▸ Thread Workers   : %d Concurrent Deployers\n", connectWorkers)
	green.Printf("  ▸ Token Target     : %d Tokens (TTL %ds)\n", turnstilePullTarget, int(turnstileTokenTTL.Seconds()))
	green.Printf("  ▸ Server Port      : %s\n", "8090")
	fmt.Println()

	// Start the turnstile token puller
	StartTurnstilePuller()
	green.Println("  ▸ System Status    : ONLINE & ACTIVE")
	fmt.Println()

	// Routes
	http.HandleFunc("/", handleWebSocket)
	http.HandleFunc("/api/status", handleStatus)
	http.HandleFunc("/api/turnstile-token", handleTurnstileToken)
	http.HandleFunc("/api/turnstile-status", handleTurnstileStatus)
	http.HandleFunc("/api/proxy-status", handleProxyStatus)
	http.HandleFunc("/api/proxies", handleProxiesAPI)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	cyan.Println("  ⚡ Waiting for extension connection on port " + port + "...")
	fmt.Println()

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		color.Red("Server failed: %s", err)
	}
}
