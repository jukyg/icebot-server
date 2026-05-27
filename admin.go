package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/fatih/color"
)

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

	serveAdminPanel(w)
}

func serveAdminLogin(w http.ResponseWriter) {
	w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>ICEbot · ADMIN</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
html{font-size:16px;background:#020408;color:#e8f4ff;font-family:'Courier New',monospace}
body{min-height:100vh;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:24px;padding:20px}
h1{font-size:1.1rem;letter-spacing:0.15em;text-transform:uppercase;color:#ffd740;text-shadow:0 0 20px rgba(255,215,64,0.2)}
h1 span{color:#7aa8d2;font-weight:400}
.lock-icon{font-size:2rem;color:#ffd740;margin-bottom:8px;opacity:0.6}
form{display:flex;flex-direction:column;gap:12px;width:280px}
input{background:#0c1628;border:1px solid rgba(0,229,255,0.12);border-radius:3px;padding:10px 14px;color:#e8f4ff;font-family:inherit;font-size:0.75rem;outline:none;text-align:center;letter-spacing:0.08em}
input:focus{border-color:#ffd740;box-shadow:0 0 12px rgba(255,215,64,0.1)}
button{background:transparent;border:1px solid #ffd740;color:#ffd740;border-radius:3px;padding:10px;font-family:inherit;font-size:0.7rem;cursor:pointer;letter-spacing:0.12em;transition:all 0.15s}
button:hover{background:rgba(255,215,64,0.08);box-shadow:0 0 16px rgba(255,215,64,0.12)}
.err{color:#ff1744;font-size:0.65rem;text-align:center;min-height:1em}
.footer{font-size:0.55rem;color:#1e3550;letter-spacing:0.05em;margin-top:16px}
</style>
</head>
<body>
<div class="lock-icon">&#9673;</div>
<h1>OWNER <span>TERMINAL</span></h1>
<form method="get" action="/admin" onsubmit="var p=document.getElementById('pw').value;if(!p){document.getElementById('err').textContent='Enter password';return false;}window.location.href='/admin?password='+encodeURIComponent(p);return false;">
<input type="password" id="pw" placeholder="ENTER ACCESS CODE" autofocus spellcheck="false" />
<button type="submit">AUTHENTICATE</button>
<div class="err" id="err"></div>
</form>
<div class="footer">ICEbot v8.0 PHANTOM · Owner Terminal</div>
</body>
</html>`))
}

func serveAdminPanel(w http.ResponseWriter) {
	w.Write([]byte(adminPanelHTML))
}

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
  --text-1:#e8f4ff;--text-2:#7aa8d2;--text-3:#3d6080;--text-4:#1e3550;
  --border-1:rgba(0,229,255,0.12);--border-2:rgba(0,229,255,0.22);--border-3:rgba(0,229,255,0.40);
  --font-display:'Courier New',Courier,monospace;
  --font-body:'Courier New',Courier,monospace;
  --font-mono:'Courier New',Courier,monospace;
  --r-xs:3px;--r-sm:6px;--r-md:10px;--r-lg:16px;
  --shadow-cyan:0 0 24px rgba(0,229,255,0.18),0 0 2px rgba(0,229,255,0.6);
  --shadow-gold:0 0 24px rgba(255,215,64,0.18),0 0 2px rgba(255,215,64,0.6);
  --shadow-card:0 4px 40px rgba(0,0,0,0.6);
  --t-fast:0.15s cubic-bezier(0.4,0,0.2,1);--t-med:0.3s cubic-bezier(0.4,0,0.2,1);--t-slow:0.6s cubic-bezier(0.4,0,0.2,1);
}
body{font-family:var(--font-body);background:var(--void);color:var(--text-1);min-height:100vh;overflow-x:hidden}
::selection{background:rgba(0,229,255,0.25);color:#fff}
::-webkit-scrollbar{width:4px;height:4px}
::-webkit-scrollbar-track{background:var(--void)}
::-webkit-scrollbar-thumb{background:var(--border-2);border-radius:2px}
::-webkit-scrollbar-thumb:hover{background:var(--border-3)}

/* ADMIN HEADER */
.admin-header{
  background:linear-gradient(180deg,var(--surface),var(--void));
  border-bottom:1px solid var(--gold-glow);
  padding:20px 24px 16px;
  display:flex;
  align-items:center;
  justify-content:space-between;
  flex-wrap:wrap;
  gap:12px;
}
.admin-header h1{
  font-size:1rem;
  letter-spacing:0.2em;
  text-transform:uppercase;
  font-weight:400;
}
.admin-header h1 .gold{color:var(--gold);text-shadow:0 0 20px rgba(255,215,64,0.3)}
.admin-header h1 .dim{color:var(--text-3)}
.admin-header .sub{font-size:0.55rem;color:var(--text-4);letter-spacing:0.08em}
.admin-header .status-row{display:flex;align-items:center;gap:16px;font-size:0.6rem;letter-spacing:0.05em}
.admin-header .status-row .dot{width:6px;height:6px;border-radius:50%;display:inline-block}
.admin-header .status-row .dot.on{background:var(--green);box-shadow:0 0 8px rgba(0,230,118,0.4)}
.admin-header .status-row .dot.off{background:var(--text-4)}
.admin-header .status-row .dot.warn{background:var(--amber);box-shadow:0 0 8px rgba(255,171,0,0.4)}
.admin-header a{color:var(--text-3);text-decoration:none;font-size:0.55rem;border:1px solid var(--border-1);padding:4px 10px;border-radius:var(--r-xs);transition:var(--t-fast)}
.admin-header a:hover{color:var(--cyan);border-color:var(--border-3)}

/* TABS */
.tab-bar{
  display:flex;
  background:var(--surface);
  border-bottom:1px solid var(--border-1);
  padding:0 16px;
  overflow-x:auto;
  gap:0;
}
.tab-btn{
  background:transparent;
  border:none;
  color:var(--text-3);
  font-family:var(--font-body);
  font-size:0.6rem;
  letter-spacing:0.08em;
  text-transform:uppercase;
  padding:12px 18px;
  cursor:pointer;
  border-bottom:2px solid transparent;
  transition:var(--t-fast);
  white-space:nowrap;
}
.tab-btn:hover{color:var(--text-2);background:rgba(0,229,255,0.03)}
.tab-btn.active{color:var(--gold);border-bottom-color:var(--gold);background:rgba(255,215,64,0.04)}

/* MAIN CONTENT */
.main{padding:20px 24px;max-width:1200px}
.tab-panel{display:none}
.tab-panel.active{display:block}

.section-label{
  font-size:0.55rem;
  letter-spacing:0.15em;
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

/* CARDS */
.card{
  background:var(--surface);
  border:1px solid var(--border-1);
  border-radius:var(--r-sm);
  padding:16px 18px;
  margin-bottom:14px;
}
.card-title{
  font-size:0.65rem;
  letter-spacing:0.08em;
  text-transform:uppercase;
  color:var(--text-2);
  margin-bottom:10px;
  display:flex;
  align-items:center;
  gap:8px;
}

/* FORM ELEMENTS */
input,select,textarea{
  background:var(--raised);
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  padding:7px 10px;
  color:var(--text-1);
  font-family:var(--font-mono);
  font-size:0.68rem;
  outline:none;
  transition:var(--t-fast);
}
input:focus,select:focus,textarea:focus{border-color:var(--gold);box-shadow:0 0 10px rgba(255,215,64,0.08)}
input::placeholder{color:var(--text-4)}
select option{background:var(--raised);color:var(--text-1)}

.btn{
  background:transparent;
  border:1px solid var(--border-2);
  color:var(--text-2);
  font-family:var(--font-body);
  font-size:0.6rem;
  letter-spacing:0.08em;
  text-transform:uppercase;
  padding:7px 14px;
  border-radius:var(--r-xs);
  cursor:pointer;
  transition:var(--t-fast);
}
.btn:hover{border-color:var(--border-3);color:var(--text-1)}
.btn-gold{border-color:var(--gold-dim);color:var(--gold)}
.btn-gold:hover{background:rgba(255,215,64,0.08);box-shadow:0 0 12px rgba(255,215,64,0.1)}
.btn-green{border-color:var(--green-dim);color:var(--green)}
.btn-green:hover{background:rgba(0,230,118,0.08)}
.btn-red{border-color:var(--red-dim);color:var(--red)}
.btn-red:hover{background:rgba(255,23,68,0.08)}
.btn-sm{padding:4px 10px;font-size:0.55rem}

/* GRID LAYOUTS */
.row{display:flex;gap:10px;align-items:center;flex-wrap:wrap;margin-bottom:8px}
.row-gap{display:flex;gap:10px;flex-wrap:wrap}
.col{flex:1;min-width:200px}
.col-2{flex:2;min-width:280px}
.w-full{width:100%}

/* STAT MINI */
.stat-mini{display:inline-flex;align-items:center;gap:6px;font-size:0.6rem;color:var(--text-3);padding:4px 10px;background:var(--raised);border-radius:var(--r-xs)}
.stat-mini .num{color:var(--text-1);font-family:var(--font-mono)}

/* PROXY LIST */
.proxy-list{max-height:200px;overflow-y:auto;font-size:0.6rem;font-family:var(--font-mono)}
.proxy-list .entry{padding:3px 6px;color:var(--text-3);border-bottom:1px solid var(--border-1)}
.proxy-list .entry .ok{color:var(--green)}
.proxy-list .entry .blk{color:var(--red)}

/* ROOM TABLE */
.room-table{width:100%;border-collapse:collapse;font-size:0.62rem}
.room-table th{padding:6px 8px;text-align:left;color:var(--text-4);font-weight:400;letter-spacing:0.08em;text-transform:uppercase;border-bottom:1px solid var(--border-1)}
.room-table td{padding:5px 8px;border-bottom:1px solid var(--border-1);color:var(--text-2)}
.room-table td .gid{color:var(--text-3);font-size:0.55rem}
.room-table .nick{color:var(--text-1)}

/* TOAST */
.toast-container{position:fixed;bottom:20px;right:20px;z-index:999;display:flex;flex-direction:column;gap:8px}
.toast{padding:8px 16px;border-radius:var(--r-xs);font-size:0.6rem;font-family:var(--font-mono);letter-spacing:0.05em;border:1px solid;animation:toastIn 0.25s ease-out}
.toast.ok{background:rgba(0,230,118,0.1);border-color:var(--green-dim);color:var(--green)}
.toast.err{background:rgba(255,23,68,0.1);border-color:var(--red-dim);color:var(--red)}
.toast.info{background:rgba(0,229,255,0.08);border-color:var(--cyan-dim);color:var(--cyan)}
@keyframes toastIn{from{opacity:0;transform:translateY(10px)}to{opacity:1;transform:translateY(0)}}

/* STATUS BADGE */
.badge{display:inline-block;padding:2px 8px;border-radius:2px;font-size:0.55rem;letter-spacing:0.05em}
.badge-on{background:rgba(0,230,118,0.15);color:var(--green);border:1px solid rgba(0,230,118,0.2)}
.badge-off{background:rgba(255,23,68,0.1);color:var(--red);border:1px solid rgba(255,23,68,0.15)}
.badge-warn{background:rgba(255,171,0,0.1);color:var(--amber);border:1px solid rgba(255,171,0,0.15)}

/* PERSONA LIST */
.persona-item{padding:8px 0;border-bottom:1px solid var(--border-1);font-size:0.65rem}
.persona-item:last-child{border-bottom:none}
.persona-item .p-name{color:var(--text-1)}
.persona-item .p-persona{color:var(--text-3);font-size:0.6rem;margin-top:2px}

/* ANIMATIONS */
.fade-up{animation:fadeUp 0.4s ease-out both}
@keyframes fadeUp{from{opacity:0;transform:translateY(12px)}to{opacity:1;transform:translateY(0)}}
</style>
</head>
<body>

<div class="admin-header">
  <div>
    <h1><span class="gold">●</span> OWNER <span class="gold">TERMINAL</span> <span class="dim">|</span> <span class="gold">SAIF</span></h1>
    <div class="sub">ICEbot v8.0 PHANTOM · Private Admin Console</div>
  </div>
  <div class="status-row">
    <span><span class="dot" id="hdStatusDot"></span> Server <span id="hdStatusLabel">—</span></span>
    <span>Bots <span id="hdBotCount" style="color:var(--text-1);font-family:var(--font-mono)">0</span></span>
    <span>Rooms <span id="hdRoomCount" style="color:var(--text-1);font-family:var(--font-mono)">0</span></span>
    <a href="/">Public Dashboard</a>
  </div>
</div>

<div class="tab-bar" id="tabBar">
  <button class="tab-btn active" data-tab="tabAI">AI Hub</button>
  <button class="tab-btn" data-tab="tabBots">Bot Control</button>
  <button class="tab-btn" data-tab="tabProxy">Proxy Manager</button>
  <button class="tab-btn" data-tab="tabRooms">Room Tracker</button>
</div>

<div class="main">

<!-- TAB 1: AI HUB -->
<div class="tab-panel active" id="tabAI">
  <div class="section-label">AI Engine Configuration</div>

  <div class="card fade-up">
    <div class="card-title">Gemini API</div>
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
    <div style="font-size:0.55rem;color:var(--text-4);margin-top:6px">
      Status: <span id="admGeminiStatus">—</span>
    </div>
  </div>

  <div class="card fade-up">
    <div class="card-title">AI Chat Mode</div>
    <div class="row">
      <label style="display:flex;align-items:center;gap:10px;cursor:pointer;font-size:0.65rem;color:var(--text-2)">
        <input type="checkbox" id="admAIChatToggle" style="width:auto" />
        Enable AI Chat (bots auto-reply to tracked players)
      </label>
      <span style="font-size:0.6rem;color:var(--text-4)" id="admAiStatus">Inactive</span>
    </div>
  </div>

  <div class="section-label">Tracked Players (AI-enabled)</div>
  <div class="card fade-up">
    <div id="admPersonaList" style="font-size:0.65rem;color:var(--text-3)">Loading...</div>
  </div>
</div>

<!-- TAB 2: BOT CONTROL -->
<div class="tab-panel" id="tabBots">
  <div class="section-label">Bot Deployment</div>

  <div class="card fade-up">
    <div class="card-title">Quick Start Bots</div>
    <div class="row">
      <div class="col">
        <input type="text" id="admBotRoom" placeholder="Room code (e.g. room-abc)" class="w-full" spellcheck="false" />
      </div>
      <div class="col">
        <input type="number" id="admBotQty" placeholder="Quantity" class="w-full" value="5" min="1" />
      </div>
      <div class="col">
        <input type="text" id="admBotName" placeholder="Bot name (default: Botnik)" class="w-full" spellcheck="false" />
      </div>
      <button class="btn btn-green" id="admStartBots">DEPLOY</button>
    </div>
    <div style="font-size:0.55rem;color:var(--text-4);margin-top:6px" id="admBotResult"></div>
  </div>

  <div class="section-label">Active Sessions</div>
  <div class="card fade-up">
    <table class="room-table" id="admSessionTable">
      <thead><tr><th>Session</th><th>Room</th><th>Bots</th><th>Joined</th><th>AI Chat</th><th>Actions</th></tr></thead>
      <tbody id="admSessionBody"></tbody>
    </table>
    <div style="font-size:0.55rem;color:var(--text-4);margin-top:8px;text-align:right" id="admSessionInfo"></div>
  </div>
</div>

<!-- TAB 3: PROXY MANAGER -->
<div class="tab-panel" id="tabProxy">
  <div class="section-label">Proxy Pool</div>

  <div class="card fade-up">
    <div class="card-title">Pool Status</div>
    <div class="row" style="gap:16px">
      <span class="stat-mini">Total <span class="num" id="admProxyTotal">0</span></span>
      <span class="stat-mini" style="color:var(--green)">Available <span class="num" id="admProxyAvail">0</span></span>
      <span class="stat-mini" style="color:var(--red)">Blocked <span class="num" id="admProxyBlocked">0</span></span>
    </div>
  </div>

  <div class="card fade-up">
    <div class="card-title">Upload / Swap Proxy List</div>
    <div class="row">
      <div class="col-2">
        <textarea id="admProxyText" placeholder="One proxy per line: IP:PORT:USER:PASS" spellcheck="false"
          style="width:100%;height:140px;resize:vertical;font-size:0.6rem;font-family:var(--font-mono);background:var(--raised);border:1px solid var(--border-1);border-radius:var(--r-xs);color:var(--text-1);padding:8px;outline:none"></textarea>
      </div>
      <button class="btn btn-gold" id="admUploadProxy">UPLOAD & SWAP</button>
    </div>
    <div style="font-size:0.55rem;color:var(--text-4);margin-top:4px" id="admProxyResult"></div>
  </div>

  <div class="card fade-up">
    <div class="card-title">All Proxies</div>
    <div class="proxy-list" id="admProxyList">Loading...</div>
  </div>
</div>

<!-- TAB 4: ROOM TRACKER -->
<div class="tab-panel" id="tabRooms">
  <div class="section-label">Live Room Monitor</div>

  <div class="card fade-up">
    <div class="card-title">Active Rooms</div>
    <table class="room-table" id="admRoomTable">
      <thead><tr><th>Room</th><th>Session</th><th>Bots</th><th>Joined</th><th>AI Chat</th><th>Uptime</th></tr></thead>
      <tbody id="admRoomBody"></tbody>
    </table>
    <div style="font-size:0.55rem;color:var(--text-4);margin-top:8px" id="admRoomInfo"></div>
  </div>

  <div class="card fade-up">
    <div class="card-title">Bot Details (last 50)</div>
    <div style="font-size:0.58rem;font-family:var(--font-mono);max-height:240px;overflow-y:auto" id="admBotDetails">
      Waiting for data...
    </div>
  </div>
</div>

</div><!-- /main -->

<div class="toast-container" id="toastRoot"></div>

<script>
(function(){
var D = {};
var PW = '';
var POLL_INTERVAL = 5000;

function init(){
  var params = new URLSearchParams(window.location.search);
  PW = params.get('password') || '';

  // DOM cache
  var ids = ['hdStatusDot','hdStatusLabel','hdBotCount','hdRoomCount',
    'tabBar','tabAI','tabBots','tabProxy','tabRooms',
    'admGeminiKey','admGeminiModel','admSaveGemini','admGeminiStatus',
    'admAIChatToggle','admAiStatus','admPersonaList',
    'admBotRoom','admBotQty','admBotName','admStartBots','admBotResult',
    'admSessionBody','admSessionTable','admSessionInfo',
    'admProxyTotal','admProxyAvail','admProxyBlocked',
    'admProxyText','admUploadProxy','admProxyResult','admProxyList',
    'admRoomBody','admRoomTable','admRoomInfo','admBotDetails',
    'toastRoot'];
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

  // AI Hub: Save Gemini config
  D.admSaveGemini.addEventListener('click', function(){
    var key = D.admGeminiKey.value.trim();
    var model = D.admGeminiModel.value;
    if(!key){ toast('Enter Gemini API Key','err'); return; }
    apiPost('/api/gemini-config', {apiKey:key, model:model}, function(d){
      if(d.ok){
        D.admGeminiKey.value = '';
        toast('Gemini config saved','ok');
        poll();
      } else {
        toast('Save failed','err');
      }
    });
  });

  // AI Hub: AI Chat toggle
  D.admAIChatToggle.addEventListener('change', function(){
    var enabled = D.admAIChatToggle.checked;
    apiPost('/api/ai-chat', {sessionId:'', enabled:enabled}, function(d){
      if(d.ok){
        toast('AI Chat '+(enabled?'engaged':'disengaged'), enabled?'ok':'info');
        poll();
      } else {
        D.admAIChatToggle.checked = !enabled;
        toast('Toggle failed','err');
      }
    });
  });

  // Bot Control: Deploy bots
  D.admStartBots.addEventListener('click', function(){
    var room = D.admBotRoom.value.trim();
    var qty = parseInt(D.admBotQty.value) || 5;
    var name = D.admBotName.value.trim() || 'Botnik';
    if(!room){ toast('Enter a room code','err'); return; }
    if(qty < 1){ toast('Quantity must be >= 1','err'); return; }
    apiPost('/api/admin/bot/start', {room:room, qty:qty, name:name}, function(d){
      if(d.ok){
        toast('Deploying '+qty+' bots to '+room,'ok');
        D.admBotResult.textContent = 'Deployed '+qty+' bots to room '+room;
        setTimeout(poll, 2000);
      } else {
        toast('Deploy failed: '+(d.error||'unknown'),'err');
      }
    });
  });

  // Proxy Manager: Upload
  D.admUploadProxy.addEventListener('click', function(){
    var text = D.admProxyText.value.trim();
    if(!text){ toast('Enter proxies','err'); return; }
    var lines = text.split('\n').filter(function(l){return l.trim()});
    if(lines.length < 1){ toast('No valid proxies','err'); return; }
    apiPost('/api/admin/proxy/upload', {proxies:text}, function(d){
      if(d.ok){
        toast('Uploaded '+d.count+' proxies','ok');
        D.admProxyText.value = '';
        D.admProxyResult.textContent = d.count+' proxies loaded';
        setTimeout(poll, 500);
      } else {
        toast('Upload failed: '+(d.error||'unknown'),'err');
      }
    });
  });

  // Start polling
  poll();
  setInterval(poll, POLL_INTERVAL);
}

function getURL(path){
  return path + '?password=' + encodeURIComponent(PW);
}

function apiGet(path, cb){
  var xhr = new XMLHttpRequest();
  xhr.open('GET', getURL(path), true);
  xhr.timeout = 10000;
  xhr.onload = function(){
    if(xhr.status === 200){
      try{ cb(JSON.parse(xhr.responseText)); }catch(e){ cb(null); }
    } else {
      cb(null);
    }
  };
  xhr.onerror = function(){ cb(null); };
  xhr.ontimeout = function(){ cb(null); };
  xhr.send();
}

function apiPost(path, data, cb){
  var xhr = new XMLHttpRequest();
  xhr.open('POST', getURL(path), true);
  xhr.setRequestHeader('Content-Type','application/json');
  xhr.timeout = 15000;
  xhr.onload = function(){
    if(xhr.status === 200){
      try{ cb(JSON.parse(xhr.responseText)); }catch(e){ cb({ok:false}); }
    } else {
      cb({ok:false, error:'HTTP '+xhr.status});
    }
  };
  xhr.onerror = function(){ cb({ok:false, error:'network'}); };
  xhr.ontimeout = function(){ cb({ok:false, error:'timeout'}); };
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

function poll(){
  // Status
  apiGet('/api/status', function(d){
    if(d){
      D.hdStatusDot.className = 'dot on';
      D.hdStatusLabel.textContent = 'ONLINE';
      D.hdBotCount.textContent = d.totalBots || 0;
    } else {
      D.hdStatusDot.className = 'dot warn';
      D.hdStatusLabel.textContent = 'DEGRADED';
    }
  });

  // AI Chat state + Gemini
  apiGet('/api/ai-chat', function(d){
    if(!d) return;
    var anyOn = false;
    if(d.sessions){ d.sessions.forEach(function(s){ if(s.enabled) anyOn=true; }); }
    D.admAIChatToggle.checked = anyOn;
    D.admAiStatus.textContent = anyOn ? 'Active' : 'Inactive';
    D.admAiStatus.style.color = anyOn ? 'var(--green)' : 'var(--text-4)';
    D.admGeminiStatus.textContent = d.geminiReady ? 'Ready ('+(d.geminiModel||'gemini-2.0-flash-lite')+')' : (d.geminiKeySet ? 'Key set, not ready' : 'Not configured');
    D.admGeminiStatus.style.color = d.geminiReady ? 'var(--green)' : (d.geminiKeySet ? 'var(--amber)' : 'var(--text-4)');
    // Model
    if(d.geminiModel){
      for(var i=0;i<D.admGeminiModel.options.length;i++){
        if(D.admGeminiModel.options[i].value === d.geminiModel){
          D.admGeminiModel.selectedIndex = i; break;
        }
      }
    }
  });

  // Tracked players (AI-enabled)
  apiGet('/api/ai-chat', function(d){
    if(!d || !d.tracked) return;
    var html = '';
    if(d.tracked.length === 0){
      html = '<div style="color:var(--text-4)">No tracked players with AI chat enabled. Use Bird script to track players.</div>';
    } else {
      d.tracked.forEach(function(t){
        html += '<div class="persona-item">' +
          '<div class="p-name">&#9654; ' + esc(t.name||t.trackedID) + '</div>' +
          '<div class="p-persona">Persona: ' + esc(t.persona||'none') + ' &middot; ID: ' + esc(t.trackedID) + '</div>' +
          '</div>';
      });
    }
    D.admPersonaList.innerHTML = html;
  });

  // Sessions
  apiGet('/api/admin/rooms', function(d){
    if(!d || !d.sessions) return;
    D.hdRoomCount.textContent = d.sessions.length;
    var shtml = '', rhtml = '';
    var sCount = 0, bCount = 0, jCount = 0;
    d.sessions.forEach(function(s){
      sCount++;
      bCount += s.bots || 0;
      jCount += s.joined || 0;
      var aiBadge = s.aiChat ? '<span class="badge badge-on">ON</span>' : '<span class="badge badge-off">OFF</span>';
      shtml += '<tr><td><span style="color:var(--text-4)">' + esc(s.id) + '</span></td>' +
        '<td>' + esc(s.room||'—') + '</td>' +
        '<td style="font-family:var(--font-mono);color:var(--text-1)">' + (s.bots||0) + '</td>' +
        '<td style="font-family:var(--font-mono);color:var(--green)">' + (s.joined||0) + '</td>' +
        '<td>' + aiBadge + '</td>' +
        '<td><button class="btn btn-red btn-sm" onclick="stopSession(\''+esc(s.id)+'\')">STOP</button></td></tr>';
      rhtml += '<tr><td>' + esc(s.room||'—') + '</td>' +
        '<td><span style="color:var(--text-4);font-size:0.55rem">' + esc(s.id) + '</span></td>' +
        '<td style="font-family:var(--font-mono)">' + (s.bots||0) + '</td>' +
        '<td style="font-family:var(--font-mono);color:var(--green)">' + (s.joined||0) + '</td>' +
        '<td>' + aiBadge + '</td>' +
        '<td style="font-size:0.55rem;color:var(--text-4)">' + (s.uptime||'—') + '</td></tr>';
    });
    D.admSessionBody.innerHTML = shtml;
    D.admSessionInfo.textContent = sCount + ' session(s), ' + bCount + ' bots total, ' + jCount + ' joined';
    D.admRoomBody.innerHTML = rhtml;
    D.admRoomInfo.textContent = d.sessions.length + ' active room(s)';
  });

  // Proxy stats
  apiGet('/api/proxy-status', function(d){
    if(!d) return;
    D.admProxyTotal.textContent = d.total || 0;
    D.admProxyAvail.textContent = d.available || 0;
    D.admProxyBlocked.textContent = d.blocked || 0;
    if(d.proxies){
      var html = '';
      d.proxies.forEach(function(p){
        var cls = p.status === 'available' ? 'ok' : 'blk';
        html += '<div class="entry"><span class="' + cls + '">&#9632;</span> ' + esc(p.ip) + ':' + esc(p.port) + ' <span style="color:var(--text-4)">[' + p.status + ']</span></div>';
      });
      D.admProxyList.innerHTML = html;
    }
  });
}

window.stopSession = function(sid){
  if(!confirm('Stop all bots in session ' + sid + '?')) return;
  apiPost('/api/admin/bot/stop', {sessionId:sid}, function(d){
    if(d && d.ok){
      toast('Session stopped','ok');
      setTimeout(poll, 1000);
    } else {
      toast('Stop failed','err');
    }
  });
};

function esc(s){
  if(!s) return '';
  return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');
}

init();
})();
</script>
</body>
</html>`

// Admin API endpoints

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
	if req.Qty > 100 {
		req.Qty = 100
	}
	if req.Name == "" {
		req.Name = "Botnik"
	}

	go func() {
		// Find existing session for this room, or create headless
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
		}
		sessionsMu.Unlock()

		for i := 0; i < req.Qty; i++ {
			createBot(session, req.Room, req.Name, "sequential", "", nil, nil, "", "")
			time.Sleep(200 * time.Millisecond)
		}
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

	sessionsMu.RLock()
	s, ok := sessions[req.SessionID]
	sessionsMu.RUnlock()
	if !ok {
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": false, "error": "session not found"})
		return
	}

	s.mu.Lock()
	for _, b := range s.bots {
		b.Destroy()
	}
	s.bots = make(map[int]*Bot)
	s.mu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
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
	residentialProxies = parsed
	blockedProxies = make(map[string]time.Time)
	proxyFailureCount = make(map[string]int)
	blockedMu.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "count": len(parsed)})
}
