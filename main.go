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
		// Regular HTTP request (browser visit) — show status page
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		proxyTotal, proxyAvail, proxyBlocked := ProxyStats()
		tokenAvail, _ := TurnstilePoolStatus()
		fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>ICEbot Server v7</title>
<style>
  *{margin:0;padding:0;box-sizing:border-box}
  body{background:#0a0a1a;color:#e2e8f0;font-family:'Segoe UI',system-ui,sans-serif;padding:40px;min-height:100vh}
  .container{max-width:700px;margin:0 auto}
  h1{font-size:2.2rem;background:linear-gradient(135deg,#38bdf8,#818cf8,#c084fc);-webkit-background-clip:text;-webkit-text-fill-color:transparent;margin-bottom:8px}
  .subtitle{color:#64748b;font-size:0.95rem;margin-bottom:32px}
  .card{background:rgba(30,41,59,0.7);border:1px solid rgba(56,189,248,0.15);border-radius:16px;padding:24px;margin-bottom:16px;backdrop-filter:blur(10px)}
  .card h2{color:#38bdf8;font-size:1.1rem;margin-bottom:16px;display:flex;align-items:center;gap:8px}
  .stat-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px}
  .stat{background:rgba(15,23,42,0.6);border-radius:10px;padding:14px;text-align:center}
  .stat .value{font-size:1.8rem;font-weight:700;color:#38bdf8}
  .stat .label{font-size:0.8rem;color:#64748b;margin-top:4px;text-transform:uppercase;letter-spacing:0.5px}
  .status-badge{display:inline-flex;align-items:center;gap:6px;background:rgba(34,197,94,0.15);color:#22c55e;padding:4px 12px;border-radius:20px;font-size:0.85rem;font-weight:600}
  .status-badge::before{content:'';width:8px;height:8px;background:#22c55e;border-radius:50%%;animation:pulse 2s infinite}
  @keyframes pulse{0%%,100%%{opacity:1}50%%{opacity:0.4}}
  .links{display:flex;gap:12px;margin-top:8px}
  .links a{color:#818cf8;text-decoration:none;font-size:0.9rem;padding:6px 14px;border:1px solid rgba(129,140,248,0.3);border-radius:8px;transition:all 0.2s}
  .links a:hover{background:rgba(129,140,248,0.1);border-color:#818cf8}
  .proxy-list{font-size:0.8rem;color:#64748b;margin-top:12px;line-height:1.8}
  .proxy-list span{background:rgba(56,189,248,0.1);color:#38bdf8;padding:2px 8px;border-radius:4px;margin:2px;display:inline-block}
</style></head>
<body><div class="container">
  <h1>⚡ ICEbot Server v7</h1>
  <p class="subtitle">Residential Proxy Edition — Go Backend</p>
  <div class="card">
    <h2><span class="status-badge">RUNNING</span></h2>
    <div class="stat-grid">
      <div class="stat"><div class="value">%d</div><div class="label">Proxies Total</div></div>
      <div class="stat"><div class="value">%d</div><div class="label">Available</div></div>
      <div class="stat"><div class="value">%d</div><div class="label">Blocked</div></div>
      <div class="stat"><div class="value">%d</div><div class="label">Tokens</div></div>
    </div>
  </div>
  <div class="card">
    <h2>🔗 API Endpoints</h2>
    <div class="links">
      <a href="/api/status">📊 Status</a>
      <a href="/api/turnstile-status">🔑 Tokens</a>
      <a href="/api/proxy-status">🌐 Proxies</a>
    </div>
  </div>
  <div class="card">
    <h2>🌐 Proxy Pool</h2>
    <div class="proxy-list">%s</div>
  </div>
</div></body></html>`, proxyTotal, proxyAvail, proxyBlocked, tokenAvail, renderProxyList())
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

// renderProxyList generates HTML spans for each proxy in the pool.
func renderProxyList() string {
	result := ""
	for _, p := range residentialProxies {
		result += fmt.Sprintf(`<span>%s:%s</span> `, p.IP, p.Port)
	}
	return result
}

func handleStatus(w http.ResponseWriter, r *http.Request) {
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
