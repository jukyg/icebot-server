package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
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
	ReadBufferSize:  8192,
	WriteBufferSize: 8192,
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Check if this is a WebSocket upgrade request
	if r.Header.Get("Upgrade") == "" {
		// Regular HTTP request (browser visit) — show the modern dashboard
		handleDashboard(w, r)
		return
	}

	// Require password for WebSocket connections
	pw := r.URL.Query().Get("password")
	if pw == "" || !timingSafeEqual(pw, wssPassword) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized — invalid or missing password"))
		return
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		color.Red("WebSocket upgrade failed: %s", err)
		return
	}

	room := r.URL.Query().Get("room")
	isOwner := r.URL.Query().Get("owner") == "true"
	session := GetOrCreateSession(ws, room)
	session.mu.Lock()
	session.isOwner = isOwner
	session.mu.Unlock()
	NotifyOwnersOfClientChange()


	// Handle pong responses to keep the connection alive through proxies.
	ws.SetPongHandler(func(string) error {
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	boldRed := color.New(color.FgRed, color.Bold)

	// Stop channel for the ping goroutine
	pingDone := make(chan struct{})

	defer func() {
		close(pingDone) // Stop the ping goroutine first
		session.CloseConn(ws)
		ws.Close()
		NotifyOwnersOfClientChange()
	}()

	// WebSocket keepalive: send ping every 20s to prevent proxy/NAT timeouts.
	// Many cloud reverse proxies (HostingGuru, etc.) drop idle WebSocket
	// connections after 30-60s of inactivity.
	go func() {
		pingTicker := time.NewTicker(20 * time.Second)
		defer pingTicker.Stop()
		for {
			select {
			case <-pingDone:
				return
			case <-pingTicker.C:
				ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := ws.WriteControl(websocket.PingMessage, []byte("keepalive"), time.Now().Add(5*time.Second)); err != nil {
					ws.SetWriteDeadline(time.Time{})
					return
				}
				ws.SetWriteDeadline(time.Time{})
			}
		}
	}()

	// Read loop
	for {
		_, raw, err := ws.ReadMessage()
		if err != nil {
			// Suppress noisy "use of closed network connection" errors
			// These happen naturally during WebSocket reconnection and are not bugs
			errStr := err.Error()
			if !strings.Contains(errStr, "use of closed network connection") &&
				!strings.Contains(errStr, "connection reset by peer") {
				color.New(color.FgYellow).Printf("[WS] Session %s connection ended: %s\n", session.id, errStr)
			}
			break
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					boldRed.Printf("[PANIC] session=%s msg=%s: %v\n", session.id, string(raw), r)
				}
			}()
			session.HandleMessage(raw)
		}()
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
	// Admin API
	http.HandleFunc("/admin", handleAdmin)
	http.HandleFunc("/api/admin/rooms", handleAdminRooms)
	http.HandleFunc("/api/admin/bot/start", handleAdminBotStart)
	http.HandleFunc("/api/admin/bot/stop", handleAdminBotStop)
	http.HandleFunc("/api/admin/proxy/upload", handleAdminProxyUpload)
	http.HandleFunc("/api/admin/proxy/reset", handleAdminProxyReset)
	http.HandleFunc("/api/admin/logs", handleAdminLogs)
	http.HandleFunc("/api/admin/health", handleAdminHealth)
	http.HandleFunc("/api/admin/bot/chat", handleAdminBotChat)

	// Bird auto-deploy API
	http.HandleFunc("/bird/api/auto-deploy", handleAutoDeployList)
	http.HandleFunc("/bird/api/auto-deploy/upsert", handleAutoDeployUpsert)
	http.HandleFunc("/bird/api/auto-deploy/delete", handleAutoDeployDelete)
	http.HandleFunc("/bird/api/auto-deploy/master", handleAutoDeployMaster)
	http.HandleFunc("/bird/api/auto-deploy/immune-rooms", handleAutoDeployImmuneRooms)
	http.HandleFunc("/bird/api/auto-deploy/forced-rooms", handleAutoDeployForcedRooms)

	// Start session reaper to clean up orphaned sessions
	go sessionReaper()
	// Start proxy reset loop to unblock proxies every 60s
	go proxyResetLoop()

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