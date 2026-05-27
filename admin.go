package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

var (
	joinStaggerMu   sync.Mutex
	joinStaggerBase = 150
)

func setJoinStagger(ms int) {
	joinStaggerMu.Lock()
	defer joinStaggerMu.Unlock()
	joinStaggerBase = ms
	if joinStaggerBase < 50 {
		joinStaggerBase = 50
	}
	if joinStaggerBase > 2000 {
		joinStaggerBase = 2000
	}
}

func getJoinDelay() time.Duration {
	joinStaggerMu.Lock()
	base := joinStaggerBase
	joinStaggerMu.Unlock()
	jitter := rand.Intn(base)
	return time.Duration(base+jitter) * time.Millisecond
}

var adminPWCache = ""
var adminPWCacheMu sync.RWMutex

func setAdminPW(pw string) {
	adminPWCacheMu.Lock()
	defer adminPWCacheMu.Unlock()
	adminPWCache = pw
}

func getAdminPW() string {
	adminPWCacheMu.RLock()
	defer adminPWCacheMu.RUnlock()
	return adminPWCache
}

func handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: 'self'; connect-src 'self'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	pw := r.URL.Query().Get("password")
	if pw == "" || !timingSafeEqual(pw, dashboardPassword) {
		serveAdminLogin(w)
		return
	}

	setAdminPW(pw)
	serveAdminPanel(w)
}

func serveAdminLogin(w http.ResponseWriter) {
	w.Write([]byte(adminLoginHTML))
}

func serveAdminPanel(w http.ResponseWriter) {
	w.Write([]byte(adminPanelHTML))
}

// ──────────────────────────────────────────────────────────────────────
// Chat spam helper
// ──────────────────────────────────────────────────────────────────────

func sendChatMessage(b *Bot, msg string) {
	gid := b.garticId.Load()
	if gid == 0 {
		return
	}
	b.SendRaw(fmt.Sprintf(`42[11,%d,%s]`, int(gid), jsonString(msg)))
}

func handleAdminBotChat(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"sessionId"`
		Message   string `json:"message"`
		Count     int    `json:"count"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "bad request"})
		return
	}
	if req.Message == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "message required"})
		return
	}
	if req.Count < 1 {
		req.Count = 1
	}

	sent := 0

	if req.SessionID == "*all*" {
		sessionsMu.RLock()
		for _, s := range sessions {
			s.mu.RLock()
			for _, b := range s.bots {
				if b.IsAlive() && b.garticId.Load() != 0 {
					sendChatMessage(b, req.Message)
					sent++
				}
			}
			s.mu.RUnlock()
		}
		sessionsMu.RUnlock()
	} else {
		sessionsMu.RLock()
		s, ok := sessions[req.SessionID]
		sessionsMu.RUnlock()
		if !ok {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "session not found"})
			return
		}
		s.mu.RLock()
		for _, b := range s.bots {
			if b.IsAlive() && b.garticId.Load() != 0 {
				if sent >= req.Count {
					break
				}
				sendChatMessage(b, req.Message)
				sent++
			}
		}
		s.mu.RUnlock()
	}

	LogSuccess("Chat", fmt.Sprintf("Sent \"%s\" via %d bot(s) to %s", req.Message, sent, req.SessionID))
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "sent": sent})
}

// ──────────────────────────────────────────────────────────────────────
// AI Chat + Gemini config handlers (from aiapi.go)
// ──────────────────────────────────────────────────────────────────────

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

// ──────────────────────────────────────────────────────────────────────
// Admin HTML
// ──────────────────────────────────────────────────────────────────────

var adminLoginHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>ICEbot · ADMIN</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
html{font-size:16px;background:#020408;color:#e8f4ff;font-family:'Courier New',monospace}
body{min-height:100vh;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:32px;padding:30px}
.brand{text-align:center}
.brand .logo{font-size:1.0rem;color:#ffd740;letter-spacing:0.25em;text-transform:uppercase;text-shadow:0 0 30px rgba(255,215,64,0.15)}
.brand .logo span{color:#7aa8d2}
.brand .sub{font-size:0.7rem;color:#1e3550;letter-spacing:0.1em;margin-top:6px}
.lock-icon{font-size:2.5rem;color:#ffd740;opacity:0.5}
form{display:flex;flex-direction:column;gap:16px;width:360px}
input{background:#0c1628;border:1px solid rgba(255,215,64,0.15);border-radius:6px;padding:14px 18px;color:#e8f4ff;font-family:inherit;font-size:1.0rem;outline:none;text-align:center;letter-spacing:0.1em;transition:all 0.2s}
input:focus{border-color:#ffd740;box-shadow:0 0 20px rgba(255,215,64,0.1)}
button{background:transparent;border:1px solid #ffd740;color:#ffd740;border-radius:6px;padding:14px;font-family:inherit;font-size:0.9rem;cursor:pointer;letter-spacing:0.15em;text-transform:uppercase;transition:all 0.2s}
button:hover{background:rgba(255,215,64,0.06);box-shadow:0 0 20px rgba(255,215,64,0.1)}
.err{color:#ff1744;font-size:0.8rem;text-align:center;min-height:1.2em}
.footer{font-size:0.65rem;color:#1e3550;letter-spacing:0.05em;margin-top:8px}
</style>
</head>
<body>
<div class="brand">
  <div class="lock-icon">&#9673;</div>
  <div class="logo"><span>ICEbot</span> · OWNER TERMINAL</div>
  <div class="sub">v8.5 PHANTOM · Ultimate Control Suite</div>
</div>
<form method="get" action="/admin" onsubmit="var p=document.getElementById('pw').value;if(!p){document.getElementById('err').textContent='ACCESS CODE REQUIRED';return false;}window.location.href='/admin?password='+encodeURIComponent(p);return false;">
<input type="password" id="pw" placeholder="ENTER ACCESS CODE" autofocus spellcheck="false" />
<button type="submit">AUTHENTICATE</button>
<div class="err" id="err"></div>
</form>
<div class="footer">ICEbot v8.5 PHANTOM · All operations logged</div>
</body>
</html>`

var adminPanelHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>ICEbot · ADMIN · Saif</title>
<style>
*,*::before,*::after{margin:0;padding:0;box-sizing:border-box}
html{font-size:16px;scroll-behavior:smooth;-webkit-font-smoothing:antialiased}
:root{
  --void:#020408;--base:#050a14;--surface:#080f1e;--raised:#0c1628;--elevated:#111d35;
  --cyan:#00e5ff;--cyan-dim:#0099bb;--cyan-glow:rgba(0,229,255,0.15);
  --teal:#1de9b6;--teal-dim:#0dba90;
  --gold:#ffd740;--gold-dim:#c9a400;--gold-glow:rgba(255,215,64,0.12);
  --red:#ff1744;--red-dim:#b71c1c;--red-glow:rgba(255,23,68,0.12);
  --green:#00e676;--green-dim:#00a84e;--green-glow:rgba(0,230,118,0.10);
  --amber:#ffab00;--purple:#d500f9;
  --text-1:#e8f4ff;--text-2:#a0c4e8;--text-3:#4a7a9a;--text-4:#1e3550;
  --border-1:rgba(0,229,255,0.08);--border-2:rgba(0,229,255,0.18);--border-3:rgba(255,215,64,0.2);
  --font-display:'Courier New',Courier,monospace;
  --font-body:'Courier New',Courier,monospace;
  --font-mono:'Courier New',Courier,monospace;
  --r-xs:6px;--r-sm:10px;--r-md:14px;--r-lg:20px;
  --shadow-gold:0 0 30px rgba(255,215,64,0.1),0 0 2px rgba(255,215,64,0.3);
  --shadow-card:0 4px 40px rgba(0,0,0,0.5);
  --t-fast:0.12s ease;--t-med:0.25s ease;
}
body{font-family:var(--font-body);background:var(--void);color:var(--text-1);min-height:100vh;overflow-x:hidden;font-size:0.85rem;line-height:1.5}
::selection{background:rgba(255,215,64,0.2);color:#fff}
::-webkit-scrollbar{width:6px;height:6px}
::-webkit-scrollbar-track{background:var(--void)}
::-webkit-scrollbar-thumb{background:var(--text-4);border-radius:3px}
::-webkit-scrollbar-thumb:hover{background:var(--text-3)}

/* ===== ADMIN HEADER ===== */
.admin-header{
  background:linear-gradient(180deg,var(--surface),var(--void));
  border-bottom:1px solid var(--border-3);
  padding:20px 28px 16px;
  display:flex;
  align-items:center;
  justify-content:space-between;
  flex-wrap:wrap;
  gap:12px;
}
.admin-header .brand h1{
  font-size:1.2rem;
  letter-spacing:0.2em;
  text-transform:uppercase;
  font-weight:400;
}
.admin-header .brand h1 .gold{color:var(--gold);text-shadow:0 0 25px rgba(255,215,64,0.2)}
.admin-header .brand h1 .dim{color:var(--text-3)}
.admin-header .brand .sub{font-size:0.7rem;color:var(--text-4);letter-spacing:0.1em;margin-top:3px}
.admin-header .stats-row{display:flex;align-items:center;gap:18px;font-size:0.78rem;letter-spacing:0.05em;flex-wrap:wrap}
.admin-header .stats-row .stat{display:flex;align-items:center;gap:6px;color:var(--text-3)}
.admin-header .stats-row .stat .val{color:var(--text-1);font-family:var(--font-mono);font-size:0.82rem}
.admin-header .stats-row .dot{width:8px;height:8px;border-radius:50%;display:inline-block}
.admin-header .stats-row .dot.on{background:var(--green);box-shadow:0 0 8px rgba(0,230,118,0.5)}
.admin-header .stats-row .dot.off{background:var(--text-4)}
.admin-header .stats-row .dot.warn{background:var(--amber);box-shadow:0 0 8px rgba(255,171,0,0.4)}
.admin-header .stats-row a{color:var(--text-4);text-decoration:none;border:1px solid var(--border-1);padding:4px 12px;border-radius:var(--r-xs);font-size:0.7rem;transition:var(--t-fast)}
.admin-header .stats-row a:hover{color:var(--cyan);border-color:var(--border-2)}

/* ===== TAB BAR ===== */
.tab-bar{
  display:flex;
  background:var(--surface);
  border-bottom:1px solid var(--border-1);
  padding:0 14px;
  overflow-x:auto;
  gap:2px;
}
.tab-btn{
  background:transparent;
  border:none;
  color:var(--text-3);
  font-family:var(--font-body);
  font-size:0.8rem;
  letter-spacing:0.1em;
  text-transform:uppercase;
  padding:14px 20px;
  cursor:pointer;
  border-bottom:3px solid transparent;
  transition:var(--t-fast);
  white-space:nowrap;
}
.tab-btn:hover{color:var(--text-2);background:rgba(255,215,64,0.02)}
.tab-btn.active{color:var(--gold);border-bottom-color:var(--gold);background:rgba(255,215,64,0.04)}

/* ===== MAIN ===== */
.main{padding:22px 28px;max-width:1400px}
.tab-panel{display:none;animation:fadeIn 0.25s ease}
.tab-panel.active{display:block}
@keyframes fadeIn{from{opacity:0;transform:translateY(6px)}to{opacity:1;transform:translateY(0)}}

.section-label{
  font-size:0.7rem;
  letter-spacing:0.18em;
  text-transform:uppercase;
  color:var(--text-4);
  margin:20px 0 10px;
  padding-bottom:6px;
  border-bottom:1px solid var(--border-1);
  display:flex;
  align-items:center;
  gap:10px;
}
.section-label:first-child{margin-top:0}

/* ===== CARDS ===== */
.card{
  background:var(--surface);
  border:1px solid var(--border-1);
  border-radius:var(--r-sm);
  padding:16px 20px;
  margin-bottom:14px;
}
.card-title{
  font-size:0.85rem;
  letter-spacing:0.1em;
  text-transform:uppercase;
  color:var(--text-2);
  margin-bottom:12px;
  display:flex;
  align-items:center;
  gap:8px;
}
.card-title .badge{margin-left:auto}

/* ===== FORM ELEMENTS ===== */
input,select,textarea{
  background:var(--raised);
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  padding:9px 12px;
  color:var(--text-1);
  font-family:var(--font-mono);
  font-size:0.82rem;
  outline:none;
  transition:var(--t-fast);
}
input:focus,select:focus,textarea:focus{border-color:var(--border-3)}
input::placeholder{color:var(--text-4)}
select option{background:var(--raised);color:var(--text-1)}

/* ===== BUTTONS ===== */
.btn{
  background:transparent;
  border:1px solid var(--border-2);
  color:var(--text-2);
  font-family:var(--font-body);
  font-size:0.75rem;
  letter-spacing:0.1em;
  text-transform:uppercase;
  padding:9px 16px;
  border-radius:var(--r-xs);
  cursor:pointer;
  transition:var(--t-fast);
}
.btn:hover{border-color:var(--text-2);color:var(--text-1)}
.btn-gold{border-color:var(--gold-dim);color:var(--gold)}
.btn-gold:hover{background:rgba(255,215,64,0.06);box-shadow:0 0 15px rgba(255,215,64,0.08)}
.btn-green{border-color:var(--green-dim);color:var(--green)}
.btn-green:hover{background:rgba(0,230,118,0.06)}
.btn-red{border-color:var(--red-dim);color:var(--red)}
.btn-red:hover{background:rgba(255,23,68,0.06)}
.btn-amber{border-color:var(--amber);color:var(--amber)}
.btn-amber:hover{background:rgba(255,171,0,0.06)}
.btn-sm{padding:4px 10px;font-size:0.65rem}
.btn:disabled{opacity:0.3;cursor:default;pointer-events:none}

/* ===== LAYOUT ===== */
.row{display:flex;gap:12px;align-items:center;flex-wrap:wrap;margin-bottom:8px}
.col{flex:1;min-width:200px}
.col-2{flex:2;min-width:280px}
.col-3{flex:3;min-width:320px}
.w-full{width:100%}
.gap-4{gap:4px}
.gap-8{gap:8px}
.mt-4{margin-top:4px}
.mt-8{margin-top:8px}
.mb-4{margin-bottom:4px}

/* ===== BADGES ===== */
.badge{
  display:inline-block;
  padding:2px 10px;
  border-radius:3px;
  font-size:0.7rem;
  letter-spacing:0.05em;
  font-family:var(--font-mono);
  font-weight:600;
}
.badge-on{background:rgba(0,230,118,0.12);color:var(--green);border:1px solid rgba(0,230,118,0.2)}
.badge-off{background:rgba(255,23,68,0.08);color:var(--red);border:1px solid rgba(255,23,68,0.12)}
.badge-warn{background:rgba(255,171,0,0.08);color:var(--amber);border:1px solid rgba(255,171,0,0.12)}
.badge-idle{background:rgba(0,229,255,0.06);color:var(--text-3);border:1px solid var(--border-1)}

/* ===== STAT MINI ===== */
.stat-mini{
  display:inline-flex;align-items:center;gap:6px;
  font-size:0.75rem;color:var(--text-3);
  padding:4px 12px;background:var(--raised);
  border-radius:var(--r-xs);
  border:1px solid var(--border-1);
}
.stat-mini .num{color:var(--text-1);font-family:var(--font-mono);font-size:0.85rem}
.stat-mini.green .num{color:var(--green)}
.stat-mini.red .num{color:var(--red)}
.stat-mini.amber .num{color:var(--amber)}

/* ===== TABLES ===== */
.table-wrap{overflow-x:auto}
table.admin-table{width:100%;border-collapse:collapse;font-size:0.82rem}
table.admin-table th{
  padding:8px 12px;text-align:left;
  color:var(--text-4);font-weight:400;letter-spacing:0.08em;text-transform:uppercase;
  border-bottom:1px solid var(--border-1);
  font-size:0.7rem;
}
table.admin-table td{
  padding:6px 12px;
  border-bottom:1px solid var(--border-1);
  color:var(--text-2);
}
table.admin-table tr:hover td{background:rgba(255,215,64,0.02)}
table.admin-table td.mono{font-family:var(--font-mono);color:var(--text-1)}
table.admin-table td.gid{color:var(--text-4);font-size:0.7rem}

/* ===== PROXY LIST ===== */
.proxy-scroll{max-height:260px;overflow-y:auto;font-size:0.75rem;font-family:var(--font-mono)}
.proxy-scroll .p-entry{padding:3px 8px;color:var(--text-3);border-bottom:1px solid var(--border-1);display:flex;justify-content:space-between}
.proxy-scroll .p-entry .ok{color:var(--green)}
.proxy-scroll .p-entry .blk{color:var(--red)}

/* ===== LOG VIEWER ===== */
.log-viewer{
  background:var(--void);
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  padding:10px 12px;
  height:400px;
  overflow-y:auto;
  font-family:var(--font-mono);
  font-size:0.75rem;
  line-height:1.7;
}
.log-viewer .l-entry{padding:2px 0;border-bottom:1px solid rgba(0,229,255,0.03);display:flex;gap:10px}
.log-viewer .l-ts{color:var(--text-4);flex-shrink:0;width:80px}
.log-viewer .l-lv{flex-shrink:0;width:45px;text-transform:uppercase;letter-spacing:0.05em}
.log-viewer .l-info .l-lv{color:var(--cyan)}
.log-viewer .l-warn .l-lv{color:var(--amber)}
.log-viewer .l-error .l-lv{color:var(--red)}
.log-viewer .l-success .l-lv{color:var(--green)}
.log-viewer .l-src{color:var(--gold-dim);flex-shrink:0;width:65px}
.log-viewer .l-msg{color:var(--text-2);word-break:break-all}

/* ===== HEALTH GRID ===== */
.health-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:10px}
.health-item{
  background:var(--raised);
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  padding:14px 16px;
  text-align:center;
}
.health-item .h-num{font-size:1.4rem;font-family:var(--font-mono);color:var(--text-1);font-weight:700}
.health-item .h-label{font-size:0.7rem;color:var(--text-4);text-transform:uppercase;letter-spacing:0.08em;margin-top:4px}
.health-item .h-sub{font-size:0.6rem;color:var(--text-4);margin-top:2px}
.health-item.gold .h-num{color:var(--gold)}
.health-item.green .h-num{color:var(--green)}
.health-item.cyan .h-num{color:var(--cyan)}
.health-item.amber .h-num{color:var(--amber)}
.health-item.red .h-num{color:var(--red)}

/* ===== PERSONA LIST ===== */
.p-persona{padding:8px 0;border-bottom:1px solid var(--border-1);font-size:0.8rem}
.p-persona:last-child{border-bottom:none}
.p-persona .p-name{color:var(--text-1)}
.p-persona .p-desc{color:var(--text-3);font-size:0.7rem;margin-top:2px}

/* ===== CHAT SPAM ===== */
.chat-message-area{
  width:100%;
  min-height:100px;
  resize:vertical;
  font-family:var(--font-mono);
  font-size:0.82rem;
  background:var(--raised);
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  color:var(--text-1);
  padding:10px 12px;
  outline:none;
}
.chat-history{
  max-height:180px;
  overflow-y:auto;
  font-size:0.75rem;
  font-family:var(--font-mono);
  background:var(--void);
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  padding:8px 10px;
}
.chat-history .ch-entry{
  padding:2px 0;
  border-bottom:1px solid rgba(0,229,255,0.03);
  color:var(--text-2);
  display:flex;
  gap:8px;
}
.chat-history .ch-room{color:var(--gold-dim);flex-shrink:0;min-width:80px}
.chat-history .ch-bots{color:var(--text-4);flex-shrink:0;min-width:60px}
.chat-history .ch-msg{color:var(--text-1);word-break:break-all}

/* ===== TOAST ===== */
.toast-container{position:fixed;bottom:20px;right:20px;z-index:9999;display:flex;flex-direction:column;gap:8px;pointer-events:none}
.toast{
  padding:10px 18px;
  border-radius:var(--r-xs);
  font-size:0.75rem;
  font-family:var(--font-mono);
  letter-spacing:0.03em;
  border:1px solid;
  animation:slideIn 0.2s ease-out;
  pointer-events:auto;
  backdrop-filter:blur(4px);
}
.toast.ok{background:rgba(0,230,118,0.12);border-color:var(--green-dim);color:var(--green)}
.toast.err{background:rgba(255,23,68,0.12);border-color:var(--red-dim);color:var(--red)}
.toast.info{background:rgba(0,229,255,0.08);border-color:var(--cyan-dim);color:var(--cyan)}
.toast.warn{background:rgba(255,171,0,0.1);border-color:var(--amber);color:var(--amber)}
@keyframes slideIn{from{opacity:0;transform:translateX(10px)}to{opacity:1;transform:translateX(0)}}

/* ===== TOGGLE ===== */
.toggle-row{display:flex;align-items:center;gap:10px;cursor:pointer;font-size:0.8rem;color:var(--text-2)}
.toggle-row input[type=checkbox]{width:18px;height:18px;accent-color:var(--gold);cursor:pointer}

/* ===== SPAM STATUS ===== */
.spam-stat{display:inline-flex;align-items:center;gap:6px;font-size:0.75rem;color:var(--text-3)}
.spam-stat .num{font-family:var(--font-mono);color:var(--text-1);font-size:0.85rem}
.spam-stat.green .num{color:var(--green)}
.spam-stat.amber .num{color:var(--amber)}
.spam-stat.red .num{color:var(--red)}

/* ===== RESPONSIVE ===== */
@media(max-width:768px){
  .main{padding:14px 16px}
  .admin-header{padding:14px 16px}
  .admin-header .stats-row{gap:10px;font-size:0.7rem}
  .tab-btn{font-size:0.7rem;padding:10px 12px}
  .health-grid{grid-template-columns:repeat(auto-fill,minmax(120px,1fr))}
}
</style>
</head>
<body>

<div class="admin-header">
  <div class="brand">
    <h1><span class="gold">&#9679;</span> OWNER <span class="gold">TERMINAL</span> <span class="dim">|</span> <span class="gold">SAIF</span></h1>
    <div class="sub">ICEbot v8.5 PHANTOM · Ultimate Control Suite</div>
  </div>
  <div class="stats-row">
    <span class="stat"><span class="dot" id="hdDot"></span> <span id="hdStatus">—</span></span>
    <span class="stat">Bots <span class="val" id="hdBots">0</span></span>
    <span class="stat">Rooms <span class="val" id="hdRooms">0</span></span>
    <span class="stat">Memory <span class="val" id="hdMem">—</span></span>
    <a href="/">Public Dashboard</a>
  </div>
</div>

<div class="tab-bar" id="tabBar">
  <button class="tab-btn active" data-tab="tabAI">AI Hub</button>
  <button class="tab-btn" data-tab="tabBots">Bot Control</button>
  <button class="tab-btn" data-tab="tabSpam">Chat Spam</button>
  <button class="tab-btn" data-tab="tabProxy">Proxy Manager</button>
  <button class="tab-btn" data-tab="tabRooms">Room Tracker</button>
  <button class="tab-btn" data-tab="tabLogs">Activity Log</button>
  <button class="tab-btn" data-tab="tabHealth">System Health</button>
</div>

<div class="main">

<!-- TAB 1: AI HUB -->
<div class="tab-panel active" id="tabAI">
  <div class="section-label">AI Engine</div>
  <div class="card">
    <div class="card-title">Gemini API Configuration</div>
    <div class="row">
      <div class="col-2">
        <input type="password" id="admGeminiKey" placeholder="Gemini API Key" class="w-full" spellcheck="false" />
      </div>
      <div class="col">
        <select id="admGeminiModel" class="w-full">
          <option value="gemini-2.0-flash-lite">Flash Lite (fastest)</option>
          <option value="gemini-2.0-flash">Flash</option>
          <option value="gemini-1.5-flash">1.5 Flash</option>
          <option value="gemini-2.5-pro-exp-03-25">2.5 Pro (best)</option>
        </select>
      </div>
      <button class="btn btn-gold" id="admSaveGemini">SAVE</button>
    </div>
    <div class="row mt-4">
      <span class="stat-mini">Status <span class="num" id="admGeminiStatus">Not configured</span></span>
      <span class="stat-mini">Model <span class="num" id="admGeminiModelStatus">—</span></span>
    </div>
  </div>

  <div class="section-label">AI Chat Mode</div>
  <div class="card">
    <div class="row">
      <label class="toggle-row">
        <input type="checkbox" id="admAIChatToggle" />
        Enable AI Chat
      </label>
      <span class="badge badge-idle" id="admAiBadge">INACTIVE</span>
      <span style="font-size:0.7rem;color:var(--text-4);margin-left:auto">Bots auto-reply to tracked players via Gemini</span>
    </div>
  </div>

  <div class="section-label">Tracked Players (AI Enabled)</div>
  <div class="card">
    <div id="admPersonaList" style="font-size:0.8rem;color:var(--text-3);min-height:50px">Loading...</div>
  </div>
</div>

<!-- TAB 2: BOT CONTROL -->
<div class="tab-panel" id="tabBots">
  <div class="section-label">Deploy Bots</div>
  <div class="card">
    <div class="card-title">Quick Deploy</div>
    <div class="row">
      <div class="col">
        <input type="text" id="admBotRoom" placeholder="Room code" class="w-full" spellcheck="false" />
      </div>
      <div class="col" style="max-width:100px">
        <input type="number" id="admBotQty" placeholder="Qty" class="w-full" value="5" min="1" />
      </div>
      <div class="col">
        <input type="text" id="admBotName" placeholder="Bot name" class="w-full" spellcheck="false" value="Botnik" />
      </div>
      <div class="col" style="max-width:100px">
        <select id="admBurstMode" class="w-full">
          <option value="fast">Burst</option>
          <option value="normal" selected>Normal</option>
          <option value="stealth">Stealth</option>
        </select>
      </div>
      <button class="btn btn-green" id="admStartBots">DEPLOY</button>
    </div>
    <div class="row mt-4">
      <span style="font-size:0.7rem;color:var(--text-4)" id="admBotResult"></span>
    </div>
  </div>

  <div class="section-label">Active Sessions</div>
  <div class="card">
    <div class="table-wrap">
      <table class="admin-table" id="admSessionTable">
        <thead><tr><th>Session</th><th>Room</th><th>Bots</th><th>Joined</th><th>AI</th><th>Actions</th></tr></thead>
        <tbody id="admSessionBody"></tbody>
      </table>
    </div>
    <div class="row mt-4" style="justify-content:space-between">
      <span style="font-size:0.7rem;color:var(--text-4)" id="admSessionInfo"></span>
      <button class="btn btn-red btn-sm" id="admStopAll">STOP ALL</button>
    </div>
  </div>
</div>

<!-- TAB 3: CHAT SPAM -->
<div class="tab-panel" id="tabSpam">
  <div class="section-label">Message Broadcast</div>
  <div class="card">
    <div class="card-title">Send Chat Message</div>
    <div class="row">
      <div class="col-2">
        <select id="admSpamRoom" class="w-full">
          <option value="*all*">ALL ROOMS (Broadcast)</option>
        </select>
      </div>
      <div class="col" style="max-width:100px">
        <select id="admSpamCount" class="w-full">
          <option value="1">1 bot</option>
          <option value="3">3 bots</option>
          <option value="5" selected>5 bots</option>
          <option value="10">10 bots</option>
          <option value="20">20 bots</option>
          <option value="0">ALL bots</option>
        </select>
      </div>
    </div>
    <div class="row">
      <div class="col-3">
        <textarea id="admSpamMessage" class="chat-message-area" placeholder="Type your message here..."></textarea>
      </div>
    </div>
    <div class="row">
      <button class="btn btn-gold" id="admSpamSend">SEND MESSAGE</button>
      <button class="btn btn-green" id="admSpamStart" style="display:none">START SPAM</button>
      <button class="btn btn-red" id="admSpamStop" style="display:none">STOP SPAM</button>
      <label class="toggle-row" style="font-size:0.75rem">
        <input type="checkbox" id="admSpamToggle" /> Spam Mode
      </label>
      <span style="font-size:0.7rem;color:var(--text-4)">Interval:</span>
      <select id="admSpamInterval" style="width:auto;font-size:0.75rem;padding:6px 8px">
        <option value="500">0.5s</option>
        <option value="1000" selected>1s</option>
        <option value="2000">2s</option>
        <option value="3000">3s</option>
        <option value="5000">5s</option>
      </select>
    </div>
    <div class="row mt-4" id="admSpamStatus">
      <span class="spam-stat">Sent: <span class="num" id="admSpamSentCount">0</span></span>
      <span class="spam-stat amber">Active bots: <span class="num" id="admSpamActiveBots">0</span></span>
    </div>
  </div>

  <div class="section-label">Send History</div>
  <div class="card">
    <div class="chat-history" id="admChatHistory">
      <div style="color:var(--text-4);text-align:center;padding:20px 0">No messages sent yet.</div>
    </div>
    <div class="row mt-4">
      <span style="font-size:0.7rem;color:var(--text-4)" id="admChatHistoryCount">0 messages</span>
      <button class="btn btn-sm" id="admChatHistoryClear">CLEAR</button>
    </div>
  </div>
</div>

<!-- TAB 4: PROXY MANAGER -->
<div class="tab-panel" id="tabProxy">
  <div class="section-label">Proxy Pool</div>
  <div class="card">
    <div class="card-title">Pool Status</div>
    <div class="row gap-8">
      <span class="stat-mini">Total <span class="num" id="admProxyTotal">0</span></span>
      <span class="stat-mini green">Available <span class="num" id="admProxyAvail">0</span></span>
      <span class="stat-mini red">Blocked <span class="num" id="admProxyBlocked">0</span></span>
    </div>
  </div>

  <div class="card">
    <div class="card-title">Upload &amp; Swap Proxy List</div>
    <div class="row">
      <div class="col-3">
        <textarea id="admProxyText" placeholder="IP:PORT or IP:PORT:USER:PASS (one per line)" spellcheck="false"
          style="width:100%;height:150px;resize:vertical;font-size:0.78rem;font-family:var(--font-mono);background:var(--raised);border:1px solid var(--border-1);border-radius:var(--r-xs);color:var(--text-1);padding:10px;outline:none"></textarea>
      </div>
      <div style="display:flex;flex-direction:column;gap:6px">
        <button class="btn btn-gold" id="admUploadProxy">UPLOAD &amp; SWAP</button>
        <button class="btn btn-sm" id="admProxyReset">RESET TO DEFAULTS</button>
      </div>
    </div>
    <div style="font-size:0.7rem;color:var(--text-4);margin-top:4px" id="admProxyResult"></div>
  </div>

  <div class="card">
    <div class="card-title">All Proxies <span style="font-size:0.7rem;color:var(--text-4);font-weight:400">(scroll)</span></div>
    <div class="proxy-scroll" id="admProxyList">Loading...</div>
  </div>
</div>

<!-- TAB 5: ROOM TRACKER -->
<div class="tab-panel" id="tabRooms">
  <div class="section-label">Live Room Monitor</div>
  <div class="card">
    <div class="table-wrap">
      <table class="admin-table" id="admRoomTable">
        <thead><tr><th>Room</th><th>Session</th><th>Bots</th><th>Joined</th><th>AI Chat</th><th>Uptime</th><th>Action</th></tr></thead>
        <tbody id="admRoomBody"></tbody>
      </table>
    </div>
    <div style="font-size:0.7rem;color:var(--text-4);margin-top:6px" id="admRoomInfo"></div>
  </div>
</div>

<!-- TAB 6: ACTIVITY LOG -->
<div class="tab-panel" id="tabLogs">
  <div class="section-label">Real-Time Activity Log <span style="font-weight:400;color:var(--text-4)">(last 200 entries)</span></div>
  <div class="card" style="padding:10px 12px">
    <div class="log-viewer" id="admLogViewer">
      <div style="color:var(--text-4);text-align:center;padding:40px 0">Waiting for log data...</div>
    </div>
    <div class="row mt-4" style="justify-content:space-between">
      <span style="font-size:0.7rem;color:var(--text-4)" id="admLogCount">0 entries</span>
      <button class="btn btn-sm" id="admLogClear">CLEAR VIEW</button>
      <label style="display:flex;align-items:center;gap:6px;font-size:0.7rem;color:var(--text-4);cursor:pointer">
        <input type="checkbox" id="admLogAutoScroll" checked style="width:auto;width:16px;height:16px" /> Auto-scroll
      </label>
    </div>
  </div>
</div>

<!-- TAB 7: SYSTEM HEALTH -->
<div class="tab-panel" id="tabHealth">
  <div class="section-label">System Resources</div>
  <div class="card">
    <div class="health-grid" id="admHealthGrid">
      <div class="health-item gold"><div class="h-num" id="hUptime">—</div><div class="h-label">Uptime</div></div>
      <div class="health-item cyan"><div class="h-num" id="hGoroutines">—</div><div class="h-label">Goroutines</div><div class="h-sub">active threads</div></div>
      <div class="health-item green"><div class="h-num" id="hMemory">—</div><div class="h-label">Memory</div><div class="h-sub" id="hMemDetail">alloc / total</div></div>
      <div class="health-item amber"><div class="h-num" id="hSessions">—</div><div class="h-label">Sessions</div></div>
      <div class="health-item cyan"><div class="h-num" id="hBotsTotal">—</div><div class="h-label">Total Bots</div></div>
      <div class="health-item green"><div class="h-num" id="hBotsJoined">—</div><div class="h-label">Joined</div></div>
      <div class="health-item amber"><div class="h-num" id="hProxyTotal">—</div><div class="h-label">Proxies Total</div></div>
      <div class="health-item green"><div class="h-num" id="hProxyAvail">—</div><div class="h-label">Available</div></div>
      <div class="health-item red"><div class="h-num" id="hProxyBlocked">—</div><div class="h-label">Blocked</div></div>
      <div class="health-item"><div class="h-num" id="hGoVersion">—</div><div class="h-label">Go Version</div></div>
      <div class="health-item"><div class="h-num" id="hCPUCount">—</div><div class="h-label">CPU Cores</div></div>
    </div>
  </div>
</div>

</div>

<div class="toast-container" id="toastRoot"></div>

<script>
(function(){
var D = {};
var PW = '';
var POLL_MS = 4000;
var LOG_POLL_MS = 2000;
var lastLogCount = 0;
var spamTimer = null;
var spamActive = false;
var spamSentTotal = 0;

function init(){
  var params = new URLSearchParams(window.location.search);
  PW = params.get('password') || '';

  var ids = [
    'hdDot','hdStatus','hdBots','hdRooms','hdMem',
    'tabBar',
    'admGeminiKey','admGeminiModel','admSaveGemini','admGeminiStatus','admGeminiModelStatus',
    'admAIChatToggle','admAiBadge','admPersonaList',
    'admBotRoom','admBotQty','admBotName','admBurstMode','admStartBots','admBotResult',
    'admSessionBody','admSessionTable','admSessionInfo','admStopAll',
    'admSpamRoom','admSpamCount','admSpamMessage','admSpamSend','admSpamStart','admSpamStop',
    'admSpamToggle','admSpamInterval','admSpamSentCount','admSpamActiveBots','admChatHistory','admChatHistoryCount','admChatHistoryClear',
    'admProxyTotal','admProxyAvail','admProxyBlocked',
    'admProxyText','admUploadProxy','admProxyReset','admProxyResult','admProxyList',
    'admRoomBody','admRoomTable','admRoomInfo',
    'admLogViewer','admLogCount','admLogClear','admLogAutoScroll',
    'hUptime','hGoroutines','hMemory','hMemDetail','hSessions','hBotsTotal','hBotsJoined',
    'hProxyTotal','hProxyAvail','hProxyBlocked','hGoVersion','hCPUCount',
    'toastRoot'
  ];
  ids.forEach(function(id){ D[id] = document.getElementById(id); });

  // Tabs
  D.tabBar.addEventListener('click', function(e){
    var btn = e.target.closest('.tab-btn');
    if(!btn) return;
    document.querySelectorAll('.tab-btn').forEach(function(b){b.classList.remove('active')});
    btn.classList.add('active');
    document.querySelectorAll('.tab-panel').forEach(function(p){p.classList.remove('active')});
    var panel = document.getElementById(btn.getAttribute('data-tab'));
    if(panel) panel.classList.add('active');
  });

  // Save Gemini
  D.admSaveGemini.addEventListener('click', function(){
    var key = D.admGeminiKey.value.trim();
    var model = D.admGeminiModel.value;
    if(!key){ toast('Enter Gemini API Key','err'); return; }
    apiPost('/api/gemini-config', {apiKey:key, model:model}, function(d){
      if(d && d.ok){
        D.admGeminiKey.value = '';
        toast('Gemini config saved','ok');
        pollAll();
      } else {
        toast('Failed to save Gemini config','err');
      }
    });
  });

  // AI Chat toggle
  D.admAIChatToggle.addEventListener('change', function(){
    var enabled = D.admAIChatToggle.checked;
    if(!PW){ toast('Not authenticated','err'); D.admAIChatToggle.checked = !enabled; return; }
    apiPost('/api/ai-chat', {sessionId:'', enabled:enabled}, function(d){
      if(d && d.ok){
        toast('AI Chat '+(enabled?'engaged':'disengaged'), enabled?'ok':'info');
        pollAll();
      } else {
        D.admAIChatToggle.checked = !enabled;
        toast('Toggle failed','err');
      }
    });
  });

  // Start bots
  D.admStartBots.addEventListener('click', function(){
    var room = D.admBotRoom.value.trim();
    var qty = parseInt(D.admBotQty.value) || 5;
    var name = D.admBotName.value.trim() || 'Botnik';
    if(!room){ toast('Enter a room code','err'); return; }
    if(qty<1||qty>200){ toast('Quantity 1-200','err'); return; }
    apiPost('/api/admin/bot/start', {room:room, qty:qty, name:name}, function(d){
      if(d && d.ok){
        toast('Deploying '+qty+' bots to '+room,'ok');
        D.admBotResult.textContent = '>> Deploying '+qty+' bots to "'+room+'" ('+name+')';
        setTimeout(pollAll, 3000);
      } else {
        toast('Deploy failed','err');
        D.admBotResult.textContent = '>> ERROR: deploy failed';
      }
    });
  });

  // Stop all sessions
  D.admStopAll.addEventListener('click', function(){
    if(!confirm('Stop ALL bots in ALL sessions?')) return;
    apiPost('/api/admin/bot/stop', {sessionId:'*all*'}, function(d){
      if(d && d.ok){
        toast('All bots stopped','ok');
        setTimeout(pollAll, 1000);
      } else {
        toast('Stop failed','err');
      }
    });
  });

  // SPAM: Send message (one-shot)
  D.admSpamMessage.addEventListener('keydown', function(e){
    if(e.key === 'Enter' && e.ctrlKey){ doSpamSend(); }
  });
  D.admSpamSend.addEventListener('click', doSpamSend);

  // SPAM: Toggle spam mode
  D.admSpamToggle.addEventListener('change', function(){
    if(D.admSpamToggle.checked){
      D.admSpamStart.style.display = '';
      D.admSpamStop.style.display = '';
      D.admSpamSend.style.display = 'none';
    } else {
      D.admSpamStart.style.display = 'none';
      D.admSpamStop.style.display = 'none';
      D.admSpamSend.style.display = '';
      stopSpam();
    }
  });

  // SPAM: Start
  D.admSpamStart.addEventListener('click', function(){
    if(spamActive){ toast('Already spamming','warn'); return; }
    var msg = D.admSpamMessage.value.trim();
    if(!msg){ toast('Enter a message first','err'); return; }
    spamActive = true;
    spamSentTotal = 0;
    D.admSpamStart.disabled = true;
    D.admSpamSend.disabled = true;
    D.admSpamMessage.disabled = true;
    toast('Spam started','ok');
    spamLoop();
  });

  // SPAM: Stop
  D.admSpamStop.addEventListener('click', function(){
    stopSpam();
    toast('Spam stopped','info');
  });

  // SPAM: Clear history
  D.admChatHistoryClear.addEventListener('click', function(){
    D.admChatHistory.innerHTML = '<div style="color:var(--text-4);text-align:center;padding:20px 0">No messages sent yet.</div>';
    D.admChatHistoryCount.textContent = '0 messages';
  });

  // Upload proxies
  D.admUploadProxy.addEventListener('click', function(){
    var text = D.admProxyText.value.trim();
    if(!text){ toast('Enter proxies','err'); return; }
    var lines = text.split('\n').filter(function(l){return l.trim()});
    if(lines.length<1){ toast('No valid proxies','err'); return; }
    apiPost('/api/admin/proxy/upload', {proxies:text}, function(d){
      if(d && d.ok){
        toast('Uploaded '+d.count+' proxies','ok');
        D.admProxyText.value = '';
        D.admProxyResult.textContent = d.count+' proxies loaded successfully';
        setTimeout(pollAll, 500);
      } else {
        toast('Upload failed','err');
      }
    });
  });

  // Reset proxies
  D.admProxyReset.addEventListener('click', function(){
    if(!confirm('Reset proxy list to defaults?')) return;
    apiPost('/api/admin/proxy/reset', {}, function(d){
      if(d && d.ok){
        toast('Proxy list reset','ok');
        setTimeout(pollAll, 500);
      }
    });
  });

  // Log clear
  D.admLogClear.addEventListener('click', function(){
    D.admLogViewer.innerHTML = '<div style="color:var(--text-4);text-align:center;padding:40px 0">Cleared. Waiting for new data...</div>';
    lastLogCount = 0;
  });

  // Start polling
  pollAll();
  setInterval(pollAll, POLL_MS);
  setInterval(pollLogs, LOG_POLL_MS);
}

// ─── SPAM FUNCTIONS ───

function doSpamSend(){
  var msg = D.admSpamMessage.value.trim();
  if(!msg){ toast('Enter a message','err'); return; }
  var room = D.admSpamRoom.value;
  var count = parseInt(D.admSpamCount.value) || 5;
  sendChatMessageToRoom(room, msg, count);
}

function sendChatMessageToRoom(room, msg, count){
  var payload = {sessionId:room, message:msg, count:count};
  apiPost('/api/admin/bot/chat', payload, function(d){
    if(d && d.ok){
      var label = room === '*all*' ? 'ALL ROOMS' : room;
      addChatHistory(label, msg, d.sent||0);
      D.admSpamSentCount.textContent = d.sent || 0;
      toast('Sent to '+label+' ('+(d.sent||0)+' bots)','ok');
    } else {
      toast('Send failed','err');
    }
  });
}

function spamLoop(){
  if(!spamActive) return;
  var msg = D.admSpamMessage.value.trim();
  if(!msg){ stopSpam(); return; }
  var room = D.admSpamRoom.value;
  var count = parseInt(D.admSpamCount.value) || 5;
  var interval = parseInt(D.admSpamInterval.value) || 1000;

  apiPost('/api/admin/bot/chat', {sessionId:room, message:msg, count:count}, function(d){
    if(d && d.ok){
      spamSentTotal += d.sent || 0;
      var label = room === '*all*' ? 'ALL ROOMS' : room;
      addChatHistory(label, msg, d.sent||0);
      D.admSpamSentCount.textContent = spamSentTotal;
      D.admSpamActiveBots.textContent = count === 0 ? 'ALL' : count;
    }
  });

  spamTimer = setTimeout(spamLoop, interval);
}

function stopSpam(){
  spamActive = false;
  if(spamTimer){ clearTimeout(spamTimer); spamTimer = null; }
  D.admSpamStart.disabled = false;
  D.admSpamSend.disabled = false;
  D.admSpamMessage.disabled = false;
}

function addChatHistory(room, msg, count){
  var empty = D.admChatHistory.querySelector('div[style*="padding:20px"]');
  if(empty) D.admChatHistory.innerHTML = '';
  var entry = document.createElement('div');
  entry.className = 'ch-entry';
  var ts = new Date().toLocaleTimeString();
  entry.innerHTML = '<span class="ch-room">['+esc(room)+']</span><span class="ch-bots">('+count+' bots)</span><span class="ch-msg">'+esc(msg)+'</span>';
  D.admChatHistory.appendChild(entry);
  D.admChatHistory.scrollTop = D.admChatHistory.scrollHeight;
  var total = D.admChatHistory.querySelectorAll('.ch-entry').length;
  D.admChatHistoryCount.textContent = total + ' messages';
}

// ─── POLLING ───

function url(path){ return path + '?password=' + encodeURIComponent(PW); }

function apiGet(path, cb){
  var xhr = new XMLHttpRequest();
  xhr.open('GET', url(path), true);
  xhr.timeout = 10000;
  xhr.onload = function(){
    if(xhr.status===200){ try{ cb(JSON.parse(xhr.responseText)); }catch(e){ cb(null); } }
    else { cb(null); }
  };
  xhr.onerror = function(){ cb(null); };
  xhr.ontimeout = function(){ cb(null); };
  xhr.send();
}

function apiPost(path, data, cb){
  var xhr = new XMLHttpRequest();
  xhr.open('POST', url(path), true);
  xhr.setRequestHeader('Content-Type','application/json');
  xhr.timeout = 15000;
  xhr.onload = function(){
    if(xhr.status===200){ try{ cb(JSON.parse(xhr.responseText)); }catch(e){ cb({ok:false}); } }
    else { cb({ok:false}); }
  };
  xhr.onerror = function(){ cb({ok:false}); };
  xhr.ontimeout = function(){ cb({ok:false}); };
  xhr.send(JSON.stringify(data));
}

function toast(msg, type){
  type = type || 'info';
  var t = document.createElement('div');
  t.className = 'toast ' + type;
  t.textContent = msg;
  D.toastRoot.appendChild(t);
  setTimeout(function(){ if(t.parentNode) t.parentNode.removeChild(t); }, 3500);
}

function esc(s){
  if(!s) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

function pollAll(){
  // Status header
  apiGet('/api/status', function(d){
    if(d){
      D.hdDot.className = 'dot on';
      D.hdStatus.textContent = 'ONLINE';
      D.hdBots.textContent = d.totalBots || 0;
    } else {
      D.hdDot.className = 'dot warn';
      D.hdStatus.textContent = 'DEGRADED';
    }
  });

  // AI Chat + Gemini
  apiGet('/api/ai-chat', function(d){
    if(!d) return;
    var anyOn = false;
    if(d.sessions){ d.sessions.forEach(function(s){ if(s.enabled) anyOn=true; }); }
    D.admAIChatToggle.checked = anyOn;
    D.admAiBadge.textContent = anyOn ? 'ACTIVE' : 'INACTIVE';
    D.admAiBadge.className = 'badge ' + (anyOn ? 'badge-on' : 'badge-idle');
    D.admGeminiStatus.textContent = d.geminiReady ? 'READY' : (d.geminiKeySet ? 'KEY SET' : 'NOT CONFIGURED');
    D.admGeminiStatus.style.color = d.geminiReady ? 'var(--green)' : (d.geminiKeySet ? 'var(--amber)' : 'var(--text-4)');
    D.admGeminiModelStatus.textContent = d.geminiModel || '—';
    if(d.geminiModel){
      for(var i=0;i<D.admGeminiModel.options.length;i++){
        if(D.admGeminiModel.options[i].value === d.geminiModel){
          D.admGeminiModel.selectedIndex = i; break;
        }
      }
    }
  });

  // Tracked players
  apiGet('/api/ai-chat', function(d){
    if(!d||!d.tracked) return;
    if(d.tracked.length===0){
      D.admPersonaList.innerHTML = '<div style="color:var(--text-4)">No tracked players with AI chat enabled. Use Bird to track players.</div>';
    } else {
      var html = '';
      d.tracked.forEach(function(t){
        html += '<div class="p-persona"><span class="p-name">&#9654; ' + esc(t.name||t.trackedID) + '</span>' +
          '<div class="p-desc">Persona: ' + esc(t.persona||'none') + ' &middot; ID: ' + esc(t.trackedID) + '</div></div>';
      });
      D.admPersonaList.innerHTML = html;
    }
  });

  // Sessions + Rooms
  apiGet('/api/admin/rooms', function(d){
    if(!d||!d.sessions) return;
    D.hdRooms.textContent = d.sessions.length;
    var shtml = '', rhtml = '', sopt = '';
    var sCnt=0, bCnt=0, jCnt=0;
    d.sessions.forEach(function(s){
      sCnt++; bCnt+=s.bots||0; jCnt+=s.joined||0;
      var ai = s.aiChat ? '<span class="badge badge-on">ON</span>' : '<span class="badge badge-off">OFF</span>';
      shtml += '<tr><td class="gid">' + esc(s.id) + '</td><td>' + esc(s.room||'—') + '</td>' +
        '<td class="mono">'+(s.bots||0)+'</td><td class="mono" style="color:var(--green)">'+(s.joined||0)+'</td><td>'+ai+'</td>' +
        '<td><button class="btn btn-red btn-sm" onclick="stopSess(\''+esc(s.id)+'\')">STOP</button></td></tr>';
      rhtml += '<tr><td>' + esc(s.room||'—') + '</td><td class="gid">' + esc(s.id) + '</td>' +
        '<td class="mono">'+(s.bots||0)+'</td><td class="mono" style="color:var(--green)">'+(s.joined||0)+'</td><td>'+ai+'</td>' +
        '<td style="font-size:0.7rem;color:var(--text-4)">'+(s.uptime||'—')+'</td>' +
        '<td><button class="btn btn-amber btn-sm" onclick="sendToRoom(\''+esc(s.id)+'\')">SEND</button></td></tr>';
      // Build room select options for spam tab
      if(s.room){
        sopt += '<option value="'+esc(s.id)+'">'+esc(s.room)+' ('+(s.bots||0)+' bots)</option>';
      }
    });
    D.admSessionBody.innerHTML = shtml;
    D.admSessionInfo.textContent = sCnt+' session(s), '+bCnt+' bots, '+jCnt+' joined';
    D.admRoomBody.innerHTML = rhtml;
    D.admRoomInfo.textContent = d.sessions.length+' active room(s)';
    // Update room dropdown
    var curVal = D.admSpamRoom.value;
    D.admSpamRoom.innerHTML = '<option value="*all*">ALL ROOMS (Broadcast)</option>' + sopt;
    D.admSpamRoom.value = curVal || '*all*';
  });

  // Proxies
  apiGet('/api/proxy-status', function(d){
    if(!d) return;
    D.admProxyTotal.textContent = d.total||0;
    D.admProxyAvail.textContent = d.available||0;
    D.admProxyBlocked.textContent = d.blocked||0;
    if(d.proxies){
      var html = '';
      d.proxies.forEach(function(p){
        var cls = p.status==='available'?'ok':'blk';
        html += '<div class="p-entry"><span><span class="'+cls+'">&#9632;</span> ' + esc(p.ip)+':'+esc(p.port) + '</span><span style="color:var(--text-4)">['+p.status+']</span></div>';
      });
      D.admProxyList.innerHTML = html;
    }
  });

  // Health
  apiGet('/api/admin/health', function(d){
    if(!d) return;
    D.hUptime.textContent = d.uptime || '—';
    D.hGoroutines.textContent = d.goroutines || 0;
    D.hMemory.textContent = d.allocMB ? d.allocMB.toFixed(1)+' MB' : '—';
    D.hMemDetail.textContent = (d.memoryMB ? d.memoryMB.toFixed(0) : '0') + 'M / ' + (d.allocMB ? d.allocMB.toFixed(0) : '0') + 'M';
    D.hSessions.textContent = d.sessions || 0;
    D.hBotsTotal.textContent = d.totalBots || 0;
    D.hBotsJoined.textContent = d.joinedBots || 0;
    D.hProxyTotal.textContent = d.proxyTotal || 0;
    D.hProxyAvail.textContent = d.proxyAvail || 0;
    D.hProxyBlocked.textContent = d.proxyBlocked || 0;
    D.hGoVersion.textContent = d.goVersion || '—';
    D.hCPUCount.textContent = d.cpuCount || '—';
    D.hdMem.textContent = d.allocMB ? d.allocMB.toFixed(1)+'M' : '—';
  });
}

function pollLogs(){
  apiGet('/api/admin/logs', function(d){
    if(!d||!d.entries) return;
    var entries = d.entries;
    if(entries.length === lastLogCount && lastLogCount > 0) return;
    lastLogCount = entries.length;
    D.admLogCount.textContent = entries.length + ' entries';

    var html = '';
    for(var i=entries.length-1; i>=0; i--){
      var e = entries[i];
      var cls = 'l-'+e.level;
      var lv = e.level.toUpperCase().slice(0,4);
      html += '<div class="l-entry '+cls+'"><span class="l-ts">'+esc(e.ts)+'</span>' +
        '<span class="l-lv">'+lv+'</span>' +
        '<span class="l-src">'+esc(e.source)+'</span>' +
        '<span class="l-msg">'+esc(e.msg)+'</span></div>';
    }
    D.admLogViewer.innerHTML = html;

    if(D.admLogAutoScroll.checked){
      D.admLogViewer.scrollTop = 0;
    }
  });
}

window.stopSess = function(sid){
  if(!confirm('Stop session '+sid+'?')) return;
  apiPost('/api/admin/bot/stop', {sessionId:sid}, function(d){
    if(d&&d.ok){ toast('Session stopped','ok'); setTimeout(pollAll,1000); }
    else { toast('Stop failed','err'); }
  });
};

window.sendToRoom = function(sid){
  var msg = prompt('Enter message to send to session ' + sid + ':');
  if(!msg || !msg.trim()) return;
  apiPost('/api/admin/bot/chat', {sessionId:sid, message:msg.trim(), count:5}, function(d){
    if(d&&d.ok){ toast('Sent to '+sid,'ok'); }
    else { toast('Send failed','err'); }
  });
};

init();
})();
</script>
</body>
</html>`

// ──────────────────────────────────────────────────────────────────────
// Admin Session Info struct
// ──────────────────────────────────────────────────────────────────────

type adminSessionInfo struct {
	ID     string `json:"id"`
	Room   string `json:"room"`
	Bots   int    `json:"bots"`
	Joined int    `json:"joined"`
	AIChat bool   `json:"aiChat"`
	Uptime string `json:"uptime"`
}

func handleAdminRooms(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sessionsMu.RLock()
	var list []adminSessionInfo
	for _, s := range sessions {
		s.mu.RLock()
		info := adminSessionInfo{
			ID:     s.id,
			Room:   s.room,
			Bots:   len(s.bots),
			AIChat: s.aiChatEnabled,
		}
		joined := 0
		for _, b := range s.bots {
			if b.joinConfirmed.Load() {
				joined++
			}
		}
		info.Joined = joined
		if !s.orphaned {
			info.Uptime = time.Since(s.createdAt).Round(time.Second).String()
		} else {
			info.Uptime = "orphaned"
		}
		s.mu.RUnlock()
		list = append(list, info)
	}
	sessionsMu.RUnlock()

	if list == nil {
		list = []adminSessionInfo{}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sessions": list,
	})
}

func handleAdminBotStart(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Room string `json:"room"`
		Qty  int    `json:"qty"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "bad request"})
		return
	}
	if req.Room == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "room required"})
		return
	}
	if req.Qty < 1 {
		req.Qty = 1
	}
	if req.Qty > 200 {
		req.Qty = 200
	}
	if req.Name == "" {
		req.Name = "Botnik"
	}

	go func() {
		sessionsMu.Lock()
		var session *Session
		for _, s := range sessions {
			s.mu.RLock()
			if s.room == req.Room {
				session = s
				s.mu.RUnlock()
				break
			}
			s.mu.RUnlock()
		}
		if session == nil {
			id := fmt.Sprintf("adm_%d_%d", time.Now().UnixMilli(), rand.Intn(10000))
			session = &Session{
				id:          id,
				bots:        make(map[int]*Bot),
				room:        req.Room,
				srvCache:    &ServerInfoCache{},
				connectPool: make(chan ConnectJob, 500),
				connectDone: make(chan struct{}),
				autofarm:    true,
				createdAt:   time.Now(),
			}
			session.autoRejoin.Store(true)
			for i := 0; i < connectWorkers; i++ {
				go session.connectWorker()
			}
			sessions[id] = session
			color.New(color.FgHiCyan).Printf("[Admin] Created headless session %s for room %s\n", id, req.Room)
			LogSuccess("Admin", fmt.Sprintf("Created headless session %s for room %s", id, req.Room))
		}
		sessionsMu.Unlock()

		LogInfo("Admin", fmt.Sprintf("Deploying %d bots to room %s (name: %s)", req.Qty, req.Room, req.Name))

		for i := 0; i < req.Qty; i++ {
			botNumId := createBot(session, req.Room, req.Name, "sequential", "", nil, nil, "", "")
			if botNumId > 0 {
				LogInfo("Bot", fmt.Sprintf("Bot #%d created for room %s", botNumId, req.Room))
			} else {
				LogError("Bot", fmt.Sprintf("Failed to create bot #%d for room %s", i+1, req.Room))
			}
			time.Sleep(getJoinDelay())
		}

		LogSuccess("Admin", fmt.Sprintf("Deploy complete: %d bots to %s", req.Qty, req.Room))
	}()

	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "qty": req.Qty, "room": req.Room})
}

func handleAdminBotStop(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "bad request"})
		return
	}

	if req.SessionID == "*all*" {
		sessionsMu.RLock()
		for _, s := range sessions {
			s.mu.Lock()
			for _, b := range s.bots {
				b.Destroy()
			}
			s.bots = make(map[int]*Bot)
			s.mu.Unlock()
		}
		sessionsMu.RUnlock()
		LogInfo("Admin", "All bots stopped across all sessions")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		return
	}

	sessionsMu.RLock()
	s, ok := sessions[req.SessionID]
	sessionsMu.RUnlock()
	if !ok {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "session not found"})
		return
	}

	s.mu.Lock()
	count := len(s.bots)
	for _, b := range s.bots {
		b.Destroy()
	}
	s.bots = make(map[int]*Bot)
	s.mu.Unlock()

	LogInfo("Admin", fmt.Sprintf("Stopped %d bots in session %s", count, req.SessionID))
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "stopped": count})
}

func handleAdminProxyUpload(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Proxies string `json:"proxies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "bad request"})
		return
	}

	lines := strings.Split(strings.TrimSpace(req.Proxies), "\n")
	var parsed []ResidentialProxy
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		var p ResidentialProxy
		switch len(parts) {
		case 2:
			p = ResidentialProxy{IP: parts[0], Port: parts[1]}
		case 4:
			p = ResidentialProxy{IP: parts[0], Port: parts[1], Username: parts[2], Password: parts[3]}
		default:
			continue
		}
		parsed = append(parsed, p)
	}

	if len(parsed) == 0 {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "no valid proxies"})
		return
	}

	blockedMu.Lock()
	oldCount := len(residentialProxies)
	residentialProxies = parsed
	blockedProxies = make(map[string]time.Time)
	proxyFailureCount = make(map[string]int)
	blockedMu.Unlock()

	LogSuccess("Admin", fmt.Sprintf("Proxy list swapped: %d → %d proxies", oldCount, len(parsed)))
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "count": len(parsed)})
}

func handleAdminProxyReset(w http.ResponseWriter, r *http.Request) {
	if !requirePassword(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	blockedMu.Lock()
	residentialProxies = make([]ResidentialProxy, len(proxySeeds))
	copy(residentialProxies, proxySeeds)
	blockedProxies = make(map[string]time.Time)
	proxyFailureCount = make(map[string]int)
	blockedMu.Unlock()

	LogSuccess("Admin", fmt.Sprintf("Proxy list reset to %d defaults", len(proxySeeds)))
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "count": len(proxySeeds)})
}
