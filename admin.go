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
	b.SendRaw(fmt.Sprintf(`42[13,%d,%s]`, int(gid), jsonString(msg)))
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
		BotID     int    `json:"botId"`
		NumericID int    `json:"numericId"`
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

	// Resolve target botId (support both field names)
	botId := req.BotID
	if botId == 0 {
		botId = req.NumericID
	}

	sent := 0

	if botId > 0 {
		// Targeted single bot — find it by numericId across all sessions
		sessionsMu.RLock()
		for _, s := range sessions {
			s.mu.RLock()
			if b, ok := s.bots[botId]; ok && b.IsAlive() && b.garticId.Load() != 0 {
				sendChatMessage(b, req.Message)
				sent++
			}
			s.mu.RUnlock()
			if sent > 0 {
				break
			}
		}
		sessionsMu.RUnlock()
		if sent == 0 {
			json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "bot not found or not alive"})
			return
		}
	} else if req.SessionID == "*all*" {
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
// Admin HTML
// ──────────────────────────────────────────────────────────────────────

var adminLoginHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>ICEbot · Admin Login</title>
<style>
*,*::before,*::after{margin:0;padding:0;box-sizing:border-box}
html{font-size:16px}
body{
  font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Oxygen,Ubuntu,sans-serif;
  background:linear-gradient(135deg,#0a0e1a 0%,#141b2d 50%,#0a0e1a 100%);
  color:#e0e6f0;min-height:100vh;display:flex;align-items:center;justify-content:center;
  padding:24px;position:relative;overflow:hidden;
}
body::before{content:'';position:fixed;inset:0;background:radial-gradient(ellipse at 50% 0%, rgba(99,102,241,0.08) 0%,transparent 60%);pointer-events:none}
.login-card{
  background:rgba(20,27,45,0.85);backdrop-filter:blur(20px);
  border:1px solid rgba(99,102,241,0.15);border-radius:16px;
  padding:48px 40px 40px;width:400px;max-width:100%;
  text-align:center;position:relative;
  box-shadow:0 25px 60px rgba(0,0,0,0.5),inset 0 1px 0 rgba(255,255,255,0.03);
}
.login-card::before{content:'';position:absolute;top:0;left:50%;transform:translateX(-50%);width:60%;height:1px;background:linear-gradient(90deg,transparent,rgba(99,102,241,0.4),transparent)}
.logo{font-size:1.6rem;font-weight:700;letter-spacing:-0.02em;margin-bottom:4px}
.logo span{color:#818cf8}
.logo .accent{color:#fbbf24}
.sub{font-size:0.75rem;color:#4a5a7a;margin-bottom:32px;letter-spacing:0.04em}
.pw-input{width:100%;padding:14px 18px;border-radius:10px;border:1px solid rgba(99,102,241,0.15);background:rgba(10,14,26,0.6);color:#e0e6f0;font-size:1rem;outline:none;transition:all 0.2s;font-family:inherit;text-align:center}
.pw-input:focus{border-color:#818cf8;box-shadow:0 0 0 3px rgba(99,102,241,0.1)}
.pw-input::placeholder{color:#2a3a5a}
.submit-btn{width:100%;padding:14px;border-radius:10px;border:none;background:linear-gradient(135deg,#6366f1,#4f46e5);color:#fff;font-size:0.9rem;font-weight:600;cursor:pointer;transition:all 0.2s;margin-top:12px;font-family:inherit}
.submit-btn:hover{transform:translateY(-1px);box-shadow:0 8px 25px rgba(99,102,241,0.25)}
.submit-btn:active{transform:translateY(0)}
.err{color:#ef4444;font-size:0.8rem;min-height:1.4em;margin-top:12px}
.footer{font-size:0.65rem;color:#1e2a42;margin-top:24px;letter-spacing:0.03em}
</style>
</head>
<body>
<div class="login-card">
  <div class="logo"><span>ICEbot</span><span class="accent">Admin</span></div>
  <div class="sub">Owner Control Panel v8.5</div>
  <input type="password" class="pw-input" id="pw" placeholder="Enter access code" autofocus spellcheck="false" autocomplete="off" />
  <button class="submit-btn" id="loginBtn">Authenticate</button>
  <div class="err" id="err"></div>
  <div class="footer">All operations are logged</div>
</div>
<script>
(function(){
  var pw=document.getElementById('pw'),btn=document.getElementById('loginBtn'),err=document.getElementById('err');
  function login(){
    var v=pw.value.trim();
    if(!v){err.textContent='Please enter the access code';return}
    window.location.href='/admin?password='+encodeURIComponent(v);
  }
  btn.addEventListener('click',login);
  pw.addEventListener('keydown',function(e){if(e.key==='Enter'){e.preventDefault();login()}});
  pw.focus();
})();
</script>
</body>
</html>`

var adminPanelHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>ICEbot · Admin Panel</title>
<style>
*,*::before,*::after{margin:0;padding:0;box-sizing:border-box}
html{font-size:15px}
body{
  font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Oxygen,Ubuntu,sans-serif;
  background:#0a0e1a;color:#c8d0dc;min-height:100vh;overflow-x:hidden;
  -webkit-font-smoothing:antialiased;
}
::selection{background:rgba(99,102,241,0.25);color:#fff}
::-webkit-scrollbar{width:5px;height:5px}
::-webkit-scrollbar-track{background:#0a0e1a}
::-webkit-scrollbar-thumb{background:#1e2a42;border-radius:3px}
::-webkit-scrollbar-thumb:hover{background:#2a3a5a}

/* ─── HEADER ─── */
.header{
  background:rgba(20,27,45,0.85);backdrop-filter:blur(16px);
  border-bottom:1px solid rgba(99,102,241,0.08);
  padding:16px 24px;
  display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:12px;
  position:sticky;top:0;z-index:100;
}
.brand{display:flex;align-items:center;gap:12px}
.brand-icon{width:36px;height:36px;border-radius:10px;background:linear-gradient(135deg,#6366f1,#4f46e5);display:flex;align-items:center;justify-content:center;color:#fff;font-weight:700;font-size:1rem;flex-shrink:0}
.brand-text{font-size:1.1rem;font-weight:700;color:#e0e6f0;letter-spacing:-0.01em}
.brand-text span{color:#818cf8}
.brand-sub{font-size:0.65rem;color:#4a5a7a;margin-top:1px;letter-spacing:0.04em}
.header-stats{display:flex;align-items:center;gap:14px;flex-wrap:wrap;font-size:0.78rem}
.hstat{display:flex;align-items:center;gap:5px;color:#5a6a8a}
.hstat .val{color:#c8d0dc;font-weight:600}
.dot{width:7px;height:7px;border-radius:50%;display:inline-block}
.dot.on{background:#22c55e;box-shadow:0 0 8px rgba(34,197,94,0.4)}
.dot.off{background:#4a5a7a}
.dot.warn{background:#f59e0b;box-shadow:0 0 8px rgba(245,158,11,0.4)}
.header-stats a{color:#4a5a7a;text-decoration:none;font-size:0.7rem;padding:4px 10px;border:1px solid rgba(99,102,241,0.1);border-radius:6px;transition:all 0.15s}
.header-stats a:hover{color:#818cf8;border-color:rgba(99,102,241,0.2)}

/* ─── TABS ─── */
.tab-bar{
  display:flex;background:rgba(20,27,45,0.5);
  border-bottom:1px solid rgba(99,102,241,0.06);
  padding:0 12px;overflow-x:auto;gap:2px;
}
.tab-btn{
  background:transparent;border:none;color:#5a6a8a;
  font-family:inherit;font-size:0.8rem;font-weight:500;
  padding:12px 18px;cursor:pointer;
  border-bottom:2px solid transparent;transition:all 0.15s;
  white-space:nowrap;
}
.tab-btn:hover{color:#c8d0dc;background:rgba(99,102,241,0.03)}
.tab-btn.active{color:#818cf8;border-bottom-color:#818cf8;background:rgba(99,102,241,0.05)}

/* ─── MAIN ─── */
.main{padding:20px 24px;max-width:1400px}
.tab-panel{display:none;animation:fadeIn 0.2s ease}
.tab-panel.active{display:block}
@keyframes fadeIn{from{opacity:0;transform:translateY(4px)}to{opacity:1;transform:translateY(0)}}

.section-label{
  font-size:0.65rem;font-weight:600;text-transform:uppercase;
  letter-spacing:0.08em;color:#4a5a7a;
  margin:20px 0 10px;padding-bottom:6px;
  border-bottom:1px solid rgba(99,102,241,0.06);
}
.section-label:first-child{margin-top:0}

/* ─── CARDS ─── */
.card{
  background:rgba(20,27,45,0.6);border:1px solid rgba(99,102,241,0.08);
  border-radius:12px;padding:16px 20px;margin-bottom:14px;
}
.card-title{
  font-size:0.8rem;font-weight:600;color:#a0aaba;
  margin-bottom:12px;display:flex;align-items:center;gap:8px;
}

/* ─── FORM ─── */
input,select,textarea{
  background:rgba(10,14,26,0.7);border:1px solid rgba(99,102,241,0.12);
  border-radius:8px;padding:9px 12px;color:#c8d0dc;
  font-family:inherit;font-size:0.82rem;outline:none;transition:all 0.15s;
}
input:focus,select:focus,textarea:focus{border-color:rgba(99,102,241,0.3);box-shadow:0 0 0 3px rgba(99,102,241,0.05)}
input::placeholder,textarea::placeholder{color:#2a3a5a}
select option{background:#141b2d;color:#c8d0dc}

/* ─── BUTTONS ─── */
.btn{
  background:transparent;border:1px solid rgba(99,102,241,0.15);
  color:#a0aaba;font-family:inherit;font-size:0.75rem;font-weight:500;
  padding:9px 16px;border-radius:8px;cursor:pointer;
  transition:all 0.15s;
}
.btn:hover{border-color:rgba(99,102,241,0.3);color:#c8d0dc}
.btn-primary{border-color:#6366f1;color:#818cf8}
.btn-primary:hover{background:rgba(99,102,241,0.08);box-shadow:0 0 12px rgba(99,102,241,0.08)}
.btn-success{border-color:#22c55e;color:#4ade80}
.btn-success:hover{background:rgba(34,197,94,0.06)}
.btn-danger{border-color:#ef4444;color:#f87171}
.btn-danger:hover{background:rgba(239,68,68,0.06)}
.btn-warning{border-color:#f59e0b;color:#fbbf24}
.btn-warning:hover{background:rgba(245,158,11,0.06)}
.btn-sm{padding:5px 10px;font-size:0.65rem}
.btn:disabled{opacity:0.3;cursor:default;pointer-events:none}

/* ─── LAYOUT ─── */
.row{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:6px}
.col{flex:1;min-width:180px}
.col-2{flex:2;min-width:260px}
.col-3{flex:3;min-width:300px}
.w-full{width:100%}
.mt-4{margin-top:4px}
.mt-8{margin-top:8px}

/* ─── TABLES ─── */
.table-wrap{overflow-x:auto}
table.data-table{width:100%;border-collapse:collapse;font-size:0.82rem}
table.data-table th{
  padding:8px 12px;text-align:left;
  color:#4a5a7a;font-weight:500;font-size:0.65rem;text-transform:uppercase;
  border-bottom:1px solid rgba(99,102,241,0.06);letter-spacing:0.04em;
}
table.data-table td{
  padding:7px 12px;border-bottom:1px solid rgba(99,102,241,0.04);
  color:#8a92a2;
}
table.data-table tr:hover td{background:rgba(99,102,241,0.02)}
table.data-table td.mono{font-weight:500;color:#c8d0dc;font-size:0.78rem}
table.data-table td.sub{color:#4a5a7a;font-size:0.7rem}
table.data-table td.green{color:#4ade80}
table.data-table td.amber{color:#fbbf24}

/* ─── PROXY LIST ─── */
.proxy-scroll{max-height:240px;overflow-y:auto;font-size:0.78rem}
.proxy-scroll .p-entry{
  padding:4px 8px;border-bottom:1px solid rgba(99,102,241,0.04);
  display:flex;justify-content:space-between;color:#5a6a8a;
}
.proxy-scroll .p-entry .ok{color:#4ade80}
.proxy-scroll .p-entry .blk{color:#f87171}

/* ─── LOG VIEWER ─── */
.log-viewer{
  background:rgba(10,14,26,0.8);border:1px solid rgba(99,102,241,0.06);
  border-radius:8px;padding:10px 12px;height:380px;overflow-y:auto;
  font-size:0.75rem;line-height:1.6;font-family:'SF Mono','Cascadia Code','Fira Code','Consolas',monospace;
}
.log-viewer .l-entry{padding:2px 0;border-bottom:1px solid rgba(99,102,241,0.03);display:flex;gap:8px}
.log-viewer .l-ts{color:#4a5a7a;flex-shrink:0;width:72px}
.log-viewer .l-lv{flex-shrink:0;width:36px;text-transform:uppercase;font-size:0.65rem;font-weight:600;font-family:inherit}
.log-viewer .l-info .l-lv{color:#60a5fa}
.log-viewer .l-warn .l-lv{color:#fbbf24}
.log-viewer .l-error .l-lv{color:#f87171}
.log-viewer .l-success .l-lv{color:#4ade80}
.log-viewer .l-src{color:#818cf8;flex-shrink:0;width:60px}
.log-viewer .l-msg{color:#8a92a2;word-break:break-all}

/* ─── HEALTH GRID ─── */
.health-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:8px}
.health-item{
  background:rgba(20,27,45,0.4);border:1px solid rgba(99,102,241,0.06);
  border-radius:10px;padding:14px 16px;text-align:center;
}
.health-item .h-num{font-size:1.4rem;font-weight:700;color:#c8d0dc}
.health-item .h-label{font-size:0.65rem;color:#4a5a7a;text-transform:uppercase;letter-spacing:0.05em;margin-top:4px;font-weight:500}
.health-item .h-sub{font-size:0.6rem;color:#3a4a6a;margin-top:2px}
.health-item.gold .h-num{color:#fbbf24}
.health-item.green .h-num{color:#4ade80}
.health-item.cyan .h-num{color:#60a5fa}
.health-item.amber .h-num{color:#f59e0b}
.health-item.red .h-num{color:#f87171}

/* ─── CHAT SPAM ─── */
.chat-message-area{
  width:100%;min-height:90px;resize:vertical;
  font-family:inherit;font-size:0.82rem;
  background:rgba(10,14,26,0.7);border:1px solid rgba(99,102,241,0.12);
  border-radius:8px;color:#c8d0dc;padding:10px 12px;outline:none;
}
.chat-history{
  max-height:170px;overflow-y:auto;font-size:0.75rem;
  background:rgba(10,14,26,0.6);border:1px solid rgba(99,102,241,0.06);
  border-radius:8px;padding:8px 10px;
}
.chat-history .ch-entry{
  padding:3px 0;border-bottom:1px solid rgba(99,102,241,0.03);
  color:#8a92a2;display:flex;gap:8px;
}
.chat-history .ch-room{color:#818cf8;flex-shrink:0;min-width:80px;font-weight:500}
.chat-history .ch-bots{color:#4a5a7a;flex-shrink:0;min-width:60px}
.chat-history .ch-msg{color:#c8d0dc;word-break:break-all}

/* ─── TOGGLE ─── */
.toggle-row{display:flex;align-items:center;gap:8px;cursor:pointer;font-size:0.78rem;color:#8a92a2}
.toggle-row input[type=checkbox]{width:16px;height:16px;accent-color:#818cf8;cursor:pointer}

/* ─── SPAM STATUS ─── */
.spam-stat{display:inline-flex;align-items:center;gap:5px;font-size:0.78rem;color:#5a6a8a}
.spam-stat .num{font-weight:600;color:#c8d0dc}
.spam-stat.green .num{color:#4ade80}
.spam-stat.amber .num{color:#fbbf24}

/* ─── STAT MINI ─── */
.stat-mini{
  display:inline-flex;align-items:center;gap:5px;
  font-size:0.75rem;color:#5a6a8a;
  padding:5px 12px;background:rgba(10,14,26,0.5);
  border-radius:8px;border:1px solid rgba(99,102,241,0.06);
}
.stat-mini .num{color:#c8d0dc;font-weight:600}
.stat-mini.green .num{color:#4ade80}
.stat-mini.red .num{color:#f87171}
.stat-mini.amber .num{color:#fbbf24}

/* ─── TOAST ─── */
#toastRoot{
  position:fixed;bottom:20px;right:20px;z-index:9999;
  display:flex;flex-direction:column;gap:8px;pointer-events:none;
}
.toast{
  padding:10px 18px;border-radius:8px;font-size:0.78rem;
  border:1px solid;pointer-events:auto;
  animation:slideIn 0.2s ease-out;max-width:320px;
  backdrop-filter:blur(8px);
}
.toast.ok{background:rgba(34,197,94,0.12);border-color:rgba(34,197,94,0.25);color:#4ade80}
.toast.err{background:rgba(239,68,68,0.12);border-color:rgba(239,68,68,0.25);color:#f87171}
.toast.info{background:rgba(96,165,250,0.1);border-color:rgba(96,165,250,0.2);color:#60a5fa}
.toast.warn{background:rgba(245,158,11,0.1);border-color:rgba(245,158,11,0.2);color:#fbbf24}
@keyframes slideIn{from{opacity:0;transform:translateX(10px)}to{opacity:1;transform:translateX(0)}}

/* ─── BADGES ─── */
.badge{
  display:inline-block;padding:2px 8px;border-radius:4px;
  font-size:0.65rem;font-weight:600;
}
.badge-on{background:rgba(34,197,94,0.1);color:#4ade80;border:1px solid rgba(34,197,94,0.15)}
.badge-off{background:rgba(239,68,68,0.08);color:#f87171;border:1px solid rgba(239,68,68,0.12)}
.badge-warn{background:rgba(245,158,11,0.08);color:#fbbf24;border:1px solid rgba(245,158,11,0.12)}
.badge-idle{background:rgba(96,165,250,0.05);color:#5a6a8a;border:1px solid rgba(99,102,241,0.06)}

/* ─── RESPONSIVE ─── */
@media(max-width:768px){
  .main{padding:14px 16px}
  .header{padding:12px 16px}
  .tab-btn{font-size:0.7rem;padding:10px 12px}
  .health-grid{grid-template-columns:repeat(auto-fill,minmax(120px,1fr))}
}
</style>
</head>
<body>

<div class="header">
  <div class="brand">
    <div class="brand-icon">A</div>
    <div>
      <div class="brand-text">ICEbot<span>Admin</span></div>
      <div class="brand-sub">Owner Control Panel v8.5</div>
    </div>
  </div>
  <div class="header-stats">
    <span class="hstat"><span class="dot" id="hdDot"></span> <span id="hdStatus">---</span></span>
    <span class="hstat">Bots <span class="val" id="hdBots">0</span></span>
    <span class="hstat">Rooms <span class="val" id="hdRooms">0</span></span>
    <span class="hstat">Memory <span class="val" id="hdMem">---</span></span>
    <a href="/">Dashboard</a>
  </div>
</div>

<div class="tab-bar" id="tabBar">
  <button class="tab-btn active" data-tab="tabBots">Bot Control</button>
  <button class="tab-btn" data-tab="tabSpam">Chat Spam</button>
  <button class="tab-btn" data-tab="tabProxy">Proxy Manager</button>
  <button class="tab-btn" data-tab="tabRooms">Room Tracker</button>
  <button class="tab-btn" data-tab="tabLogs">Activity Log</button>
  <button class="tab-btn" data-tab="tabHealth">System Health</button>
</div>

<div class="main">

<!-- BOT CONTROL -->
<div class="tab-panel active" id="tabBots">
  <div class="section-label">Deploy Bots</div>
  <div class="card">
    <div class="card-title">Quick Deploy</div>
    <div class="row">
      <div class="col"><input type="text" id="admBotRoom" placeholder="Room code" class="w-full" spellcheck="false" /></div>
      <div class="col" style="max-width:90px"><input type="number" id="admBotQty" class="w-full" value="5" min="1" /></div>
      <div class="col"><input type="text" id="admBotName" placeholder="Bot name" class="w-full" spellcheck="false" value="Botnik" /></div>
      <div class="col" style="max-width:110px">
        <select id="admBurstMode" class="w-full">
          <option value="fast">Burst</option>
          <option value="normal" selected>Normal</option>
          <option value="stealth">Stealth</option>
        </select>
      </div>
      <button class="btn btn-success" id="admStartBots">Deploy</button>
    </div>
    <div class="row mt-4"><span style="font-size:0.7rem;color:#4a5a7a" id="admBotResult"></span></div>
  </div>

  <div class="section-label">Active Sessions</div>
  <div class="card">
    <div class="table-wrap">
      <table class="data-table" id="admSessionTable">
        <thead><tr><th>Session</th><th>Room</th><th>Bots</th><th>Joined</th><th>Action</th></tr></thead>
        <tbody id="admSessionBody"></tbody>
      </table>
    </div>
    <div class="row mt-4" style="justify-content:space-between">
      <span style="font-size:0.7rem;color:#4a5a7a" id="admSessionInfo"></span>
      <button class="btn btn-danger btn-sm" id="admStopAll">Stop All</button>
    </div>
  </div>
</div>

<!-- CHAT SPAM -->
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
      <div class="col" style="max-width:90px">
        <select id="admSpamCount" class="w-full">
          <option value="1">1</option><option value="3">3</option><option value="5" selected>5</option>
          <option value="10">10</option><option value="20">20</option><option value="0">ALL</option>
        </select>
      </div>
    </div>
    <div class="row">
      <div class="col-3"><textarea id="admSpamMessage" class="chat-message-area" placeholder="Type your message..."></textarea></div>
    </div>
    <div class="row">
      <button class="btn btn-primary" id="admSpamSend">Send Message</button>
      <button class="btn btn-success" id="admSpamStart" style="display:none">Start Spam</button>
      <button class="btn btn-danger" id="admSpamStop" style="display:none">Stop Spam</button>
      <label class="toggle-row"><input type="checkbox" id="admSpamToggle" /> Spam Mode</label>
      <span style="font-size:0.7rem;color:#4a5a7a">Interval:</span>
      <select id="admSpamInterval" style="width:auto;font-size:0.75rem;padding:6px 8px">
        <option value="500">0.5s</option><option value="1000" selected>1s</option>
        <option value="2000">2s</option><option value="3000">3s</option><option value="5000">5s</option>
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
      <div style="color:#4a5a7a;text-align:center;padding:20px 0">No messages sent yet.</div>
    </div>
    <div class="row mt-4">
      <span style="font-size:0.7rem;color:#4a5a7a" id="admChatHistoryCount">0 messages</span>
      <button class="btn btn-sm" id="admChatHistoryClear">Clear</button>
    </div>
  </div>
</div>

<!-- PROXY MANAGER -->
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
    <div class="card-title">Upload &amp; Swap</div>
    <div class="row">
      <div class="col-3">
        <textarea id="admProxyText" placeholder="IP:PORT or IP:PORT:USER:PASS (one per line)" spellcheck="false"
          style="width:100%;height:140px;resize:vertical;font-size:0.78rem;background:rgba(10,14,26,0.7);border:1px solid rgba(99,102,241,0.12);border-radius:8px;color:#c8d0dc;padding:10px;outline:none;font-family:inherit"></textarea>
      </div>
      <div style="display:flex;flex-direction:column;gap:6px">
        <button class="btn btn-primary" id="admUploadProxy">Upload &amp; Swap</button>
        <button class="btn btn-sm" id="admProxyReset">Reset Defaults</button>
      </div>
    </div>
    <div style="font-size:0.7rem;color:#4a5a7a;margin-top:4px" id="admProxyResult"></div>
  </div>

  <div class="card">
    <div class="card-title">All Proxies <span style="font-size:0.7rem;color:#4a5a7a;font-weight:400">(scroll)</span></div>
    <div class="proxy-scroll" id="admProxyList">Loading...</div>
  </div>
</div>

<!-- ROOM TRACKER -->
<div class="tab-panel" id="tabRooms">
  <div class="section-label">Live Room Monitor</div>
  <div class="card">
    <div class="table-wrap">
      <table class="data-table" id="admRoomTable">
        <thead><tr><th>Room</th><th>Session</th><th>Bots</th><th>Joined</th><th>Uptime</th><th>Action</th></tr></thead>
        <tbody id="admRoomBody"></tbody>
      </table>
    </div>
    <div style="font-size:0.7rem;color:#4a5a7a;margin-top:6px" id="admRoomInfo"></div>
  </div>
</div>

<!-- ACTIVITY LOG -->
<div class="tab-panel" id="tabLogs">
  <div class="section-label">Activity Log <span style="font-weight:400;color:#4a5a7a">(last 200)</span></div>
  <div class="card" style="padding:10px 12px">
    <div class="log-viewer" id="admLogViewer">
      <div style="color:#4a5a7a;text-align:center;padding:40px 0">Waiting for log data...</div>
    </div>
    <div class="row mt-4" style="justify-content:space-between">
      <span style="font-size:0.7rem;color:#4a5a7a" id="admLogCount">0 entries</span>
      <button class="btn btn-sm" id="admLogClear">Clear</button>
      <label style="display:flex;align-items:center;gap:6px;font-size:0.7rem;color:#4a5a7a;cursor:pointer">
        <input type="checkbox" id="admLogAutoScroll" checked style="width:16px;height:16px" /> Auto-scroll
      </label>
    </div>
  </div>
</div>

<!-- SYSTEM HEALTH -->
<div class="tab-panel" id="tabHealth">
  <div class="section-label">System Resources</div>
  <div class="card">
    <div class="health-grid" id="admHealthGrid">
      <div class="health-item gold"><div class="h-num" id="hUptime">---</div><div class="h-label">Uptime</div></div>
      <div class="health-item cyan"><div class="h-num" id="hGoroutines">---</div><div class="h-label">Goroutines</div><div class="h-sub">active threads</div></div>
      <div class="health-item green"><div class="h-num" id="hMemory">---</div><div class="h-label">Memory</div><div class="h-sub" id="hMemDetail">alloc / total</div></div>
      <div class="health-item amber"><div class="h-num" id="hSessions">---</div><div class="h-label">Sessions</div></div>
      <div class="health-item cyan"><div class="h-num" id="hBotsTotal">---</div><div class="h-label">Total Bots</div></div>
      <div class="health-item green"><div class="h-num" id="hBotsJoined">---</div><div class="h-label">Joined</div></div>
      <div class="health-item amber"><div class="h-num" id="hProxyTotal">---</div><div class="h-label">Proxies Total</div></div>
      <div class="health-item green"><div class="h-num" id="hProxyAvail">---</div><div class="h-label">Available</div></div>
      <div class="health-item red"><div class="h-num" id="hProxyBlocked">---</div><div class="h-label">Blocked</div></div>
      <div class="health-item"><div class="h-num" id="hGoVersion">---</div><div class="h-label">Go Version</div></div>
      <div class="health-item"><div class="h-num" id="hCPUCount">---</div><div class="h-label">CPU Cores</div></div>
    </div>
  </div>
</div>

</div>

<div id="toastRoot"></div>

<script>
(function(){
var D={},PW='',POLL_MS=4000,LOG_POLL_MS=2000,lastLogCount=0,spamTimer=null,spamActive=false,spamSentTotal=0;

function init(){
  PW=new URLSearchParams(window.location.search).get('password')||'';
  var ids=['hdDot','hdStatus','hdBots','hdRooms','hdMem','tabBar',
    'admBotRoom','admBotQty','admBotName','admBurstMode','admStartBots','admBotResult',
    'admSessionBody','admSessionInfo','admStopAll',
    'admSpamRoom','admSpamCount','admSpamMessage','admSpamSend','admSpamStart','admSpamStop',
    'admSpamToggle','admSpamInterval','admSpamSentCount','admSpamActiveBots','admChatHistory','admChatHistoryCount','admChatHistoryClear',
    'admProxyTotal','admProxyAvail','admProxyBlocked',
    'admProxyText','admUploadProxy','admProxyReset','admProxyResult','admProxyList',
    'admRoomBody','admRoomInfo',
    'admLogViewer','admLogCount','admLogClear','admLogAutoScroll',
    'hUptime','hGoroutines','hMemory','hMemDetail','hSessions','hBotsTotal','hBotsJoined',
    'hProxyTotal','hProxyAvail','hProxyBlocked','hGoVersion','hCPUCount','toastRoot'];
  ids.forEach(function(id){D[id]=document.getElementById(id)});

  D.tabBar.addEventListener('click',function(e){
    var btn=e.target.closest('.tab-btn');
    if(!btn)return;
    document.querySelectorAll('.tab-btn').forEach(function(b){b.classList.remove('active')});
    btn.classList.add('active');
    document.querySelectorAll('.tab-panel').forEach(function(p){p.classList.remove('active')});
    var pnl=document.getElementById(btn.getAttribute('data-tab'));
    if(pnl)pnl.classList.add('active');
  });

  D.admStartBots.addEventListener('click',function(){
    var room=D.admBotRoom.value.trim(),qty=parseInt(D.admBotQty.value)||5,name=D.admBotName.value.trim()||'Botnik';
    if(!room){toast('Enter a room code','err');return}
    if(qty<1||qty>200){toast('Quantity 1-200','err');return}
    apiPost('/api/admin/bot/start',{room:room,qty:qty,name:name},function(d){
      if(d&&d.ok){toast('Deploying '+qty+' bots to '+room,'ok');D.admBotResult.textContent='Deploying '+qty+' bots to "'+room+'" ('+name+')';setTimeout(pollAll,3000)}
      else{toast('Deploy failed','err');D.admBotResult.textContent='ERROR: deploy failed'}
    });
  });

  D.admStopAll.addEventListener('click',function(){
    if(!confirm('Stop ALL bots in ALL sessions?'))return;
    apiPost('/api/admin/bot/stop',{sessionId:'*all*'},function(d){
      if(d&&d.ok){toast('All bots stopped','ok');setTimeout(pollAll,1000)}else toast('Stop failed','err');
    });
  });

  D.admSpamMessage.addEventListener('keydown',function(e){if(e.key==='Enter'&&e.ctrlKey)doSpamSend()});
  D.admSpamSend.addEventListener('click',doSpamSend);
  D.admSpamToggle.addEventListener('change',function(){
    if(D.admSpamToggle.checked){D.admSpamStart.style.display='';D.admSpamStop.style.display='';D.admSpamSend.style.display='none'}
    else{D.admSpamStart.style.display='none';D.admSpamStop.style.display='none';D.admSpamSend.style.display='';stopSpam()}
  });
  D.admSpamStart.addEventListener('click',function(){
    if(spamActive){toast('Already spamming','warn');return}
    var msg=D.admSpamMessage.value.trim();
    if(!msg){toast('Enter a message first','err');return}
    spamActive=true;spamSentTotal=0;D.admSpamStart.disabled=true;D.admSpamSend.disabled=true;D.admSpamMessage.disabled=true;
    toast('Spam started','ok');spamLoop();
  });
  D.admSpamStop.addEventListener('click',function(){stopSpam();toast('Spam stopped','info')});
  D.admChatHistoryClear.addEventListener('click',function(){
    D.admChatHistory.innerHTML='<div style="color:#4a5a7a;text-align:center;padding:20px 0">No messages sent yet.</div>';
    D.admChatHistoryCount.textContent='0 messages';
  });

  D.admUploadProxy.addEventListener('click',function(){
    var text=D.admProxyText.value.trim();
    if(!text){toast('Enter proxies','err');return}
    var lines=text.split('\n').filter(function(l){return l.trim()});
    if(lines.length<1){toast('No valid proxies','err');return}
    apiPost('/api/admin/proxy/upload',{proxies:text},function(d){
      if(d&&d.ok){toast('Uploaded '+d.count+' proxies','ok');D.admProxyText.value='';D.admProxyResult.textContent=d.count+' proxies loaded';setTimeout(pollAll,500)}
      else toast('Upload failed','err');
    });
  });
  D.admProxyReset.addEventListener('click',function(){
    if(!confirm('Reset proxy list to defaults?'))return;
    apiPost('/api/admin/proxy/reset',{},function(d){
      if(d&&d.ok){toast('Proxy list reset','ok');setTimeout(pollAll,500)}
    });
  });
  D.admLogClear.addEventListener('click',function(){
    D.admLogViewer.innerHTML='<div style="color:#4a5a7a;text-align:center;padding:40px 0">Cleared. Waiting for new data...</div>';
    lastLogCount=0;
  });
  pollAll();setInterval(pollAll,POLL_MS);setInterval(pollLogs,LOG_POLL_MS);
}

function doSpamSend(){
  var msg=D.admSpamMessage.value.trim();
  if(!msg){toast('Enter a message','err');return}
  sendChatMessageToRoom(D.admSpamRoom.value,msg,parseInt(D.admSpamCount.value)||5);
}
function sendChatMessageToRoom(room,msg,count){
  apiPost('/api/admin/bot/chat',{sessionId:room,message:msg,count:count},function(d){
    if(d&&d.ok){var lbl=room==='*all*'?'ALL ROOMS':room;addChatHistory(lbl,msg,d.sent||0);D.admSpamSentCount.textContent=d.sent||0;toast('Sent to '+lbl+' ('+(d.sent||0)+' bots)','ok')}
    else toast('Send failed','err');
  });
}
function spamLoop(){
  if(!spamActive)return;
  var msg=D.admSpamMessage.value.trim();
  if(!msg){stopSpam();return}
  var room=D.admSpamRoom.value,count=parseInt(D.admSpamCount.value)||5,intv=parseInt(D.admSpamInterval.value)||1000;
  apiPost('/api/admin/bot/chat',{sessionId:room,message:msg,count:count},function(d){
    if(d&&d.ok){spamSentTotal+=d.sent||0;addChatHistory(room==='*all*'?'ALL ROOMS':room,msg,d.sent||0);D.admSpamSentCount.textContent=spamSentTotal;D.admSpamActiveBots.textContent=count===0?'ALL':count}
  });
  spamTimer=setTimeout(spamLoop,intv);
}
function stopSpam(){
  spamActive=false;if(spamTimer){clearTimeout(spamTimer);spamTimer=null}
  D.admSpamStart.disabled=false;D.admSpamSend.disabled=false;D.admSpamMessage.disabled=false;
}
function addChatHistory(room,msg,count){
  var empty=D.admChatHistory.querySelector('div[style*="padding:20px"]');
  if(empty)D.admChatHistory.innerHTML='';
  var entry=document.createElement('div');
  entry.className='ch-entry';
  entry.innerHTML='<span class="ch-room">['+esc(room)+']</span><span class="ch-bots">('+count+' bots)</span><span class="ch-msg">'+esc(msg)+'</span>';
  D.admChatHistory.appendChild(entry);
  D.admChatHistory.scrollTop=D.admChatHistory.scrollHeight;
  D.admChatHistoryCount.textContent=D.admChatHistory.querySelectorAll('.ch-entry').length+' messages';
}

function url(p){return p+'?password='+encodeURIComponent(PW)}
function apiGet(p,cb){
  var x=new XMLHttpRequest();
  x.open('GET',url(p),true);x.timeout=10000;
  x.onload=function(){if(x.status===200){try{cb(JSON.parse(x.responseText))}catch(e){cb(null)}}else cb(null)};
  x.onerror=function(){cb(null)};x.ontimeout=function(){cb(null)};x.send();
}
function apiPost(p,d,cb){
  var x=new XMLHttpRequest();
  x.open('POST',url(p),true);x.setRequestHeader('Content-Type','application/json');x.timeout=15000;
  x.onload=function(){if(x.status===200){try{cb(JSON.parse(x.responseText))}catch(e){cb({ok:false})}}else cb({ok:false})};
  x.onerror=function(){cb({ok:false})};x.ontimeout=function(){cb({ok:false})};x.send(JSON.stringify(d));
}
function toast(m,t){
  t=t||'info';var e=document.createElement('div');
  e.className='toast '+t;e.textContent=m;
  D.toastRoot.appendChild(e);
  setTimeout(function(){if(e.parentNode)e.parentNode.removeChild(e)},3500);
}
function esc(s){if(!s)return '';return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')}

function pollAll(){
  apiGet('/api/status',function(d){
    if(d){D.hdDot.className='dot on';D.hdStatus.textContent='ONLINE';D.hdBots.textContent=d.totalBots||0}
    else{D.hdDot.className='dot warn';D.hdStatus.textContent='DEGRADED'}
  });
  apiGet('/api/admin/rooms',function(d){
    if(!d||!d.sessions)return;
    D.hdRooms.textContent=d.sessions.length;
    var sh='',rh='',so='',sc=0,bc=0,jc=0;
    d.sessions.forEach(function(s){
      sc++;bc+=s.bots||0;jc+=s.joined||0;
      sh+='<tr><td class="sub">'+esc(s.id)+'</td><td>'+esc(s.room||'---')+'</td><td class="mono">'+(s.bots||0)+'</td><td class="mono green">'+(s.joined||0)+'</td><td><button class="btn btn-danger btn-sm" onclick="stopSess(\''+esc(s.id)+'\')">Stop</button></td></tr>';
      rh+='<tr><td>'+esc(s.room||'---')+'</td><td class="sub">'+esc(s.id)+'</td><td class="mono">'+(s.bots||0)+'</td><td class="mono green">'+(s.joined||0)+'</td><td style="font-size:0.7rem;color:#4a5a7a">'+(s.uptime||'---')+'</td><td><button class="btn btn-warning btn-sm" onclick="sendToRoom(\''+esc(s.id)+'\')">Send</button></td></tr>';
      if(s.room)so+='<option value="'+esc(s.id)+'">'+esc(s.room)+' ('+(s.bots||0)+' bots)</option>';
    });
    D.admSessionBody.innerHTML=sh;
    D.admSessionInfo.textContent=sc+' session(s), '+bc+' bots, '+jc+' joined';
    D.admRoomBody.innerHTML=rh;
    D.admRoomInfo.textContent=sc+' active room(s)';
    var cv=D.admSpamRoom.value;
    D.admSpamRoom.innerHTML='<option value="*all*">ALL ROOMS (Broadcast)</option>'+so;
    D.admSpamRoom.value=cv||'*all*';
  });
  apiGet('/api/proxy-status',function(d){
    if(!d)return;
    D.admProxyTotal.textContent=d.total||0;
    D.admProxyAvail.textContent=d.available||0;
    D.admProxyBlocked.textContent=d.blocked||0;
    if(d.proxies){
      var h='';
      d.proxies.forEach(function(p){h+='<div class="p-entry"><span><span class="'+(p.status==='available'?'ok':'blk')+'">\u25a0</span> '+esc(p.ip)+':'+esc(p.port)+'</span><span style="color:#4a5a7a">['+p.status+']</span></div>'});
      D.admProxyList.innerHTML=h;
    }
  });
  apiGet('/api/admin/health',function(d){
    if(!d)return;
    D.hUptime.textContent=d.uptime||'---';
    D.hGoroutines.textContent=d.goroutines||0;
    D.hMemory.textContent=d.allocMB?d.allocMB.toFixed(1)+' MB':'---';
    D.hMemDetail.textContent=(d.memoryMB?d.memoryMB.toFixed(0):'0')+'M / '+(d.allocMB?d.allocMB.toFixed(0):'0')+'M';
    D.hSessions.textContent=d.sessions||0;
    D.hBotsTotal.textContent=d.totalBots||0;
    D.hBotsJoined.textContent=d.joinedBots||0;
    D.hProxyTotal.textContent=d.proxyTotal||0;
    D.hProxyAvail.textContent=d.proxyAvail||0;
    D.hProxyBlocked.textContent=d.proxyBlocked||0;
    D.hGoVersion.textContent=d.goVersion||'---';
    D.hCPUCount.textContent=d.cpuCount||'---';
    D.hdMem.textContent=d.allocMB?d.allocMB.toFixed(1)+'M':'---';
  });
}
function pollLogs(){
  apiGet('/api/admin/logs',function(d){
    if(!d||!d.entries)return;
    if(d.entries.length===lastLogCount&&lastLogCount>0)return;
    lastLogCount=d.entries.length;
    D.admLogCount.textContent=d.entries.length+' entries';
    var h='';
    for(var i=d.entries.length-1;i>=0;i--){
      var e=d.entries[i];
      h+='<div class="l-entry l-'+e.level+'"><span class="l-ts">'+esc(e.ts)+'</span><span class="l-lv">'+e.level.toUpperCase().slice(0,4)+'</span><span class="l-src">'+esc(e.source)+'</span><span class="l-msg">'+esc(e.msg)+'</span></div>';
    }
    D.admLogViewer.innerHTML=h;
    if(D.admLogAutoScroll.checked)D.admLogViewer.scrollTop=0;
  });
}

window.stopSess=function(sid){
  if(!confirm('Stop session '+sid+'?'))return;
  apiPost('/api/admin/bot/stop',{sessionId:sid},function(d){if(d&&d.ok){toast('Session stopped','ok');setTimeout(pollAll,1000)}else toast('Stop failed','err')});
};
window.sendToRoom=function(sid){
  var msg=prompt('Enter message to send to session '+sid+':');
  if(!msg||!msg.trim())return;
  apiPost('/api/admin/bot/chat',{sessionId:sid,message:msg.trim(),count:5},function(d){if(d&&d.ok)toast('Sent to '+sid,'ok');else toast('Send failed','err')});
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
			ID:   s.id,
			Room: s.room,
			Bots: len(s.bots),
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
				privateMode: true,
				createdAt:   time.Now(),
			}
			session.autoRejoin.Store(true)
			session.autoRejoinConfig = &AutoJoinConfig{
				Room:     req.Room,
				Name:     req.Name,
				Nick:     req.Name,
				NickMode: "0",
				Avatar:   "0",
				Target:   req.Qty,
			}
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
			s.autoRejoin.Store(false)
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

	s.autoRejoin.Store(false)
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
