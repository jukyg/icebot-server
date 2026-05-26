package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

// ============================================================================
// dashboard.go — Modern Web Dashboard for ICEbot Server v7
//
// Serves a single-page HTML dashboard at the root URL ("/") with real-time
// server stats, a password-protected proxy pool viewer, and API references.
//
// Password: saif1234 (used to unlock the proxy list)
// ============================================================================

// dashboardPassword is the password required to view the proxy list.
const dashboardPassword = "saif1234"

// ============================================================================
// HTTP Handlers
// ============================================================================

// handleDashboard renders the full dashboard HTML page.
// This replaces the old inline HTML that was previously in handleWebSocket.
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(dashboardHTML))
}

// handleProxiesAPI returns the proxy list as JSON, protected by a password.
// Usage: GET /api/proxies?password=saif1234
//
// Returns 401 if the password is missing or incorrect.
// Returns the proxy list JSON on success.
func handleProxiesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	password := r.URL.Query().Get("password")
	if password != dashboardPassword {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "unauthorized — invalid or missing password",
		})
		return
	}

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

	// Sort: blocked proxies last (for cleaner display)
	sort.Slice(proxies, func(i, j int) bool {
		if proxies[i]["status"] != proxies[j]["status"] {
			return proxies[i]["status"].(string) == "available"
		}
		return proxies[i]["ip"].(string) < proxies[j]["ip"].(string)
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":     total,
		"available": avail,
		"blocked":   blocked,
		"proxies":   proxies,
	})
}

// ============================================================================
// Dashboard HTML — Full Single-Page Application
//
// This is a self-contained HTML document with embedded CSS and JavaScript.
// It fetches live data from the API endpoints and renders a modern,
// responsive dark-themed dashboard.
// ============================================================================

var dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>ICEbot Server — Dashboard</title>
<style>
  /* ==========================================================================
     RESET & BASE
     ========================================================================== */
  *, *::before, *::after { margin: 0; padding: 0; box-sizing: border-box; }
  html { font-size: 16px; scroll-behavior: smooth; }
  body {
    font-family: 'Segoe UI', system-ui, -apple-system, sans-serif;
    background: #0b0d1a;
    color: #e2e8f0;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    line-height: 1.6;
    overflow-x: hidden;
  }
  a { color: inherit; text-decoration: none; }

  /* ==========================================================================
     SCROLLBAR
     ========================================================================== */
  ::-webkit-scrollbar { width: 6px; }
  ::-webkit-scrollbar-track { background: #0b0d1a; }
  ::-webkit-scrollbar-thumb { background: #2d3748; border-radius: 3px; }
  ::-webkit-scrollbar-thumb:hover { background: #4a5568; }

  /* ==========================================================================
     HEADER
     ========================================================================== */
  .header {
    background: linear-gradient(135deg, #0f1128 0%, #1a1040 50%, #0f1128 100%);
    border-bottom: 1px solid rgba(99, 102, 241, 0.2);
    padding: 20px 24px;
    position: sticky;
    top: 0;
    z-index: 100;
    backdrop-filter: blur(12px);
  }
  .header-inner {
    max-width: 1200px;
    margin: 0 auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 12px;
  }
  .header-brand {
    display: flex;
    align-items: center;
    gap: 16px;
  }
  .header-logo {
    width: 44px;
    height: 44px;
    border-radius: 12px;
    background: linear-gradient(135deg, #6366f1, #a855f7);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.3rem;
    font-weight: 800;
    color: #fff;
    box-shadow: 0 0 20px rgba(99, 102, 241, 0.3);
  }
  .header-title h1 {
    font-size: 1.4rem;
    font-weight: 700;
    background: linear-gradient(135deg, #e2e8f0, #a5b4fc, #c084fc);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    letter-spacing: -0.3px;
  }
  .header-title .sub {
    font-size: 0.75rem;
    color: #64748b;
    -webkit-text-fill-color: #64748b;
    font-weight: 400;
    letter-spacing: 0.3px;
  }
  .header-badge {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(34, 197, 94, 0.12);
    border: 1px solid rgba(34, 197, 94, 0.25);
    padding: 6px 16px;
    border-radius: 20px;
    font-size: 0.8rem;
    font-weight: 600;
    color: #22c55e;
  }
  .header-badge .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: #22c55e;
    animation: pulse-dot 1.8s ease-in-out infinite;
  }
  @keyframes pulse-dot {
    0%, 100% { opacity: 1; transform: scale(1); box-shadow: 0 0 0 0 rgba(34,197,94,0.4); }
    50% { opacity: 0.6; transform: scale(1.1); box-shadow: 0 0 12px 4px rgba(34,197,94,0.2); }
  }

  /* ==========================================================================
     MAIN CONTAINER
     ========================================================================== */
  .main {
    max-width: 1200px;
    margin: 0 auto;
    padding: 28px 24px;
    flex: 1;
    width: 100%;
  }

  /* ==========================================================================
     STATS ROW
     ========================================================================== */
  .stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
    gap: 14px;
    margin-bottom: 28px;
  }
  .stat-card {
    background: linear-gradient(145deg, rgba(30, 41, 59, 0.6), rgba(15, 23, 42, 0.6));
    border: 1px solid rgba(99, 102, 241, 0.1);
    border-radius: 14px;
    padding: 18px 16px;
    text-align: center;
    transition: all 0.25s ease;
    position: relative;
    overflow: hidden;
  }
  .stat-card::before {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 3px;
    background: linear-gradient(90deg, transparent, #6366f1, transparent);
    opacity: 0.4;
  }
  .stat-card:hover {
    border-color: rgba(99, 102, 241, 0.3);
    transform: translateY(-2px);
    box-shadow: 0 8px 24px rgba(0,0,0,0.3);
  }
  .stat-card .icon {
    font-size: 1.6rem;
    margin-bottom: 6px;
  }
  .stat-card .value {
    font-size: 2rem;
    font-weight: 800;
    background: linear-gradient(135deg, #e2e8f0, #a5b4fc);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    line-height: 1.2;
  }
  .stat-card .value.warning {
    background: linear-gradient(135deg, #fbbf24, #f59e0b);
    -webkit-background-clip: text;
    background-clip: text;
  }
  .stat-card .value.danger {
    background: linear-gradient(135deg, #f87171, #ef4444);
    -webkit-background-clip: text;
    background-clip: text;
  }
  .stat-card .value.success {
    background: linear-gradient(135deg, #34d399, #10b981);
    -webkit-background-clip: text;
    background-clip: text;
  }
  .stat-card .label {
    font-size: 0.75rem;
    color: #64748b;
    text-transform: uppercase;
    letter-spacing: 0.8px;
    font-weight: 600;
    margin-top: 4px;
  }
  .stat-card .sub-label {
    font-size: 0.7rem;
    color: #475569;
    margin-top: 2px;
  }

  /* ==========================================================================
     SECTIONS
     ========================================================================== */
  .section {
    background: linear-gradient(145deg, rgba(30, 41, 59, 0.5), rgba(15, 23, 42, 0.5));
    border: 1px solid rgba(99, 102, 241, 0.1);
    border-radius: 14px;
    margin-bottom: 20px;
    overflow: hidden;
    transition: border-color 0.25s ease;
  }
  .section:hover {
    border-color: rgba(99, 102, 241, 0.2);
  }
  .section-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid rgba(99, 102, 241, 0.08);
    flex-wrap: wrap;
    gap: 10px;
  }
  .section-title {
    font-size: 1rem;
    font-weight: 600;
    color: #cbd5e1;
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .section-title .badge {
    font-size: 0.7rem;
    font-weight: 600;
    padding: 2px 10px;
    border-radius: 10px;
    background: rgba(99, 102, 241, 0.15);
    color: #818cf8;
  }
  .section-body {
    padding: 20px;
  }

  /* ==========================================================================
     PROXY LIST
     ========================================================================== */
  .proxy-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: 10px;
  }
  .proxy-item {
    background: rgba(15, 23, 42, 0.6);
    border: 1px solid rgba(99, 102, 241, 0.08);
    border-radius: 10px;
    padding: 12px 14px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.82rem;
    font-family: 'Cascadia Code', 'Fira Code', 'Consolas', monospace;
    transition: all 0.2s ease;
  }
  .proxy-item:hover {
    border-color: rgba(99, 102, 241, 0.25);
    background: rgba(15, 23, 42, 0.8);
  }
  .proxy-item .addr {
    color: #94a3b8;
  }
  .proxy-item .status-tag {
    font-size: 0.65rem;
    font-weight: 600;
    padding: 3px 10px;
    border-radius: 8px;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    font-family: 'Segoe UI', system-ui, sans-serif;
  }
  .status-tag.available {
    background: rgba(34, 197, 94, 0.15);
    color: #22c55e;
  }
  .status-tag.blocked {
    background: rgba(239, 68, 68, 0.15);
    color: #f87171;
  }

  /* ==========================================================================
     PASSWORD LOCK
     ========================================================================== */
  .lock-overlay {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 40px 20px;
    text-align: center;
    gap: 12px;
  }
  .lock-overlay .lock-icon {
    font-size: 2.6rem;
    opacity: 0.6;
  }
  .lock-overlay p {
    color: #64748b;
    font-size: 0.9rem;
    max-width: 360px;
  }
  .lock-overlay .input-group {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
    justify-content: center;
    margin-top: 4px;
  }
  .lock-overlay input {
    background: rgba(15, 23, 42, 0.8);
    border: 1px solid rgba(99, 102, 241, 0.2);
    border-radius: 10px;
    padding: 10px 16px;
    color: #e2e8f0;
    font-size: 0.9rem;
    outline: none;
    width: 200px;
    transition: border-color 0.2s;
  }
  .lock-overlay input:focus {
    border-color: #6366f1;
    box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.15);
  }
  .lock-overlay input::placeholder { color: #475569; }
  .lock-overlay button {
    background: linear-gradient(135deg, #6366f1, #8b5cf6);
    border: none;
    border-radius: 10px;
    padding: 10px 24px;
    color: #fff;
    font-weight: 600;
    font-size: 0.85rem;
    cursor: pointer;
    transition: all 0.2s;
  }
  .lock-overlay button:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 16px rgba(99, 102, 241, 0.3);
  }
  .lock-overlay .error-msg {
    color: #f87171;
    font-size: 0.8rem;
    min-height: 1.2em;
  }
  .lock-overlay .loading-msg {
    color: #818cf8;
    font-size: 0.8rem;
  }

  /* ==========================================================================
     API LINKS
     ========================================================================== */
  .api-links {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }
  .api-link {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    background: rgba(99, 102, 241, 0.08);
    border: 1px solid rgba(99, 102, 241, 0.15);
    border-radius: 10px;
    padding: 10px 16px;
    font-size: 0.82rem;
    color: #a5b4fc;
    transition: all 0.2s;
    font-family: 'Cascadia Code', 'Fira Code', 'Consolas', monospace;
  }
  .api-link:hover {
    background: rgba(99, 102, 241, 0.15);
    border-color: #6366f1;
    color: #c7d2fe;
    transform: translateY(-1px);
  }
  .api-link .method {
    font-size: 0.65rem;
    font-weight: 700;
    padding: 2px 6px;
    border-radius: 4px;
    background: rgba(34, 197, 94, 0.2);
    color: #22c55e;
  }

  /* ==========================================================================
     FOOTER
     ========================================================================== */
  .footer {
    text-align: center;
    padding: 20px 24px;
    font-size: 0.78rem;
    color: #334155;
    border-top: 1px solid rgba(99, 102, 241, 0.06);
    margin-top: 20px;
  }
  .footer strong {
    color: #64748b;
    font-weight: 600;
  }

  /* ==========================================================================
     TOAST NOTIFICATIONS
     ========================================================================== */
  .toast-container {
    position: fixed;
    bottom: 24px;
    right: 24px;
    z-index: 9999;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .toast {
    background: rgba(30, 41, 59, 0.95);
    border: 1px solid rgba(99, 102, 241, 0.2);
    border-radius: 10px;
    padding: 12px 18px;
    font-size: 0.82rem;
    color: #e2e8f0;
    backdrop-filter: blur(12px);
    animation: slide-in 0.3s ease;
    max-width: 340px;
    box-shadow: 0 8px 24px rgba(0,0,0,0.4);
  }
  .toast.success { border-color: rgba(34, 197, 94, 0.3); }
  .toast.error { border-color: rgba(239, 68, 68, 0.3); }
  @keyframes slide-in {
    from { opacity: 0; transform: translateX(40px); }
    to { opacity: 1; transform: translateX(0); }
  }

  /* ==========================================================================
     RESPONSIVE
     ========================================================================== */
  @media (max-width: 640px) {
    .header { padding: 14px 16px; }
    .main { padding: 16px; }
    .stats-grid { grid-template-columns: repeat(2, 1fr); gap: 10px; }
    .stat-card .value { font-size: 1.5rem; }
    .proxy-grid { grid-template-columns: 1fr; }
    .header-title h1 { font-size: 1.1rem; }
    .lock-overlay input { width: 160px; }
  }

  /* ==========================================================================
     ANIMATED BG
     ========================================================================== */
  .bg-glow {
    position: fixed;
    top: -30%;
    left: -20%;
    width: 60%;
    height: 60%;
    background: radial-gradient(ellipse, rgba(99, 102, 241, 0.04), transparent 70%);
    pointer-events: none;
    z-index: -1;
  }
  .bg-glow-2 {
    position: fixed;
    bottom: -20%;
    right: -20%;
    width: 50%;
    height: 50%;
    background: radial-gradient(ellipse, rgba(168, 85, 247, 0.03), transparent 70%);
    pointer-events: none;
    z-index: -1;
  }
  .last-updated {
    font-size: 0.72rem;
    color: #475569;
    text-align: right;
    margin-top: 12px;
  }
</style>
</head>
<body>

<!-- =========================================================================
     AMBIENT GLOW
     ===================================================================== -->
<div class="bg-glow"></div>
<div class="bg-glow-2"></div>

<!-- =========================================================================
     HEADER
     ===================================================================== -->
<header class="header">
  <div class="header-inner">
    <div class="header-brand">
      <div class="header-logo">I</div>
      <div class="header-title">
        <h1>ICEbot Server <span style="font-weight:300;background:linear-gradient(135deg,#a5b4fc,#c084fc);-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text;">by Saif</span></h1>
        <div class="sub">Residential Proxy Edition — Go Backend v7</div>
      </div>
    </div>
    <div class="header-badge">
      <span class="dot"></span>
      <span id="statusLabel">RUNNING</span>
    </div>
  </div>
</header>

<!-- =========================================================================
     MAIN
     ===================================================================== -->
<main class="main">

  <!-- ======================================================================
       STATS GRID
       =================================================================== -->
  <div class="stats-grid" id="statsGrid">
    <div class="stat-card">
      <div class="icon">🖥️</div>
      <div class="value" id="statSessions">0</div>
      <div class="label">Sessions</div>
    </div>
    <div class="stat-card">
      <div class="icon">🤖</div>
      <div class="value" id="statBots">0</div>
      <div class="label">Bots Total</div>
    </div>
    <div class="stat-card">
      <div class="icon">✅</div>
      <div class="value success" id="statJoined">0</div>
      <div class="label">Bots Joined</div>
    </div>
    <div class="stat-card">
      <div class="icon">🔌</div>
      <div class="value" id="statProxies">0</div>
      <div class="label">Proxies</div>
      <div class="sub-label"><span id="statAvail">0</span> avail · <span id="statBlocked">0</span> blocked</div>
    </div>
    <div class="stat-card">
      <div class="icon">🎫</div>
      <div class="value" id="statTokens">0</div>
      <div class="label">Turnstile Tokens</div>
      <div class="sub-label" id="statTokenTime">—</div>
    </div>
  </div>

  <!-- ======================================================================
       PROXY POOL SECTION (Password Protected)
       =================================================================== -->
  <div class="section">
    <div class="section-header">
      <div class="section-title">
        🌐 Proxy Pool
        <span class="badge" id="proxyCount">0 proxies</span>
      </div>
    </div>
    <div class="section-body" id="proxySectionBody">
      <div class="lock-overlay" id="proxyLock">
        <div class="lock-icon">🔒</div>
        <p>Enter the dashboard password to view the proxy pool.</p>
        <div class="input-group">
          <input type="password" id="proxyPassword" placeholder="Enter password..." autocomplete="off" />
          <button id="proxyUnlockBtn">Unlock</button>
        </div>
        <div class="error-msg" id="proxyError"></div>
      </div>
      <div id="proxyListContainer" style="display:none;">
        <div class="proxy-grid" id="proxyGrid"></div>
        <div class="loading-msg" id="proxyLoading" style="text-align:center;padding:20px;">Loading proxies...</div>
      </div>
    </div>
  </div>

  <!-- ======================================================================
       API ENDPOINTS
       =================================================================== -->
  <div class="section">
    <div class="section-header">
      <div class="section-title">
        🔗 API Endpoints
        <span class="badge">REST</span>
      </div>
    </div>
    <div class="section-body">
      <div class="api-links">
        <a class="api-link" href="/api/status" target="_blank">
          <span class="method">GET</span> /api/status
        </a>
        <a class="api-link" href="/api/turnstile-status" target="_blank">
          <span class="method">GET</span> /api/turnstile-status
        </a>
        <a class="api-link" href="/api/proxy-status" target="_blank">
          <span class="method">GET</span> /api/proxy-status
        </a>
        <span class="api-link" style="cursor:help;" title="Requires ?password=saif1234">
          <span class="method">GET</span> /api/proxies
        </span>
      </div>
    </div>
  </div>

  <!-- ======================================================================
       LAST UPDATED
       =================================================================== -->
  <div class="last-updated" id="lastUpdated">Loading...</div>
</main>

<!-- =========================================================================
     FOOTER
     ===================================================================== -->
<footer class="footer">
  &copy; 2026 — <strong>ICEbot Server by Saif</strong> — All systems operational
</footer>

<!-- =========================================================================
     TOAST CONTAINER
     ===================================================================== -->
<div class="toast-container" id="toastContainer"></div>

<!-- =========================================================================
     JAVASCRIPT
     ===================================================================== -->
<script>
(function() {
  'use strict';

  // =========================================================================
  // STATE
  // =========================================================================
  let proxyUnlocked = false;
  let statsInterval = null;

  // =========================================================================
  // UTILITY HELPERS
  // =========================================================================
  function $(id) { return document.getElementById(id); }

  function showToast(message, type) {
    type = type || 'info';
    var container = $('toastContainer');
    var toast = document.createElement('div');
    toast.className = 'toast ' + type;
    toast.textContent = message;
    container.appendChild(toast);
    setTimeout(function() {
      if (toast.parentNode) { toast.parentNode.removeChild(toast); }
    }, 3500);
  }

  // =========================================================================
  // STATS FETCHER — polls /api/status every 5 seconds
  // =========================================================================
  function fetchStats() {
    var xhr = new XMLHttpRequest();
    xhr.open('GET', '/api/status', true);
    xhr.onload = function() {
      if (xhr.status !== 200) return;
      try {
        var d = JSON.parse(xhr.responseText);
        $('statSessions').textContent = d.sessions || 0;
        $('statBots').textContent = d.totalBots || 0;
        $('statJoined').textContent = d.joinedBots || 0;
        $('statProxies').textContent = d.proxies || 0;
        $('statAvail').textContent = d.proxiesAvail || 0;
        $('statBlocked').textContent = d.proxiesBlocked || 0;
        $('statTokens').textContent = d.tokens || 0;
        $('statTokenTime').textContent = d.tokenLastInAgo ? 'Last token: ' + d.tokenLastInAgo : '—';
        $('lastUpdated').textContent = 'Last updated: ' + new Date().toLocaleTimeString();
      } catch(e) { /* ignore parse errors */ }
    };
    xhr.onerror = function() {
      $('statusLabel').textContent = 'OFFLINE';
      $('statusLabel').style.color = '#f87171';
    };
    xhr.send();
  }

  // =========================================================================
  // PROXY LIST — password-protected
  // =========================================================================
  function loadProxies(password) {
    var grid = $('proxyGrid');
    var loading = $('proxyLoading');
    var errorEl = $('proxyError');
    loading.style.display = 'block';
    grid.innerHTML = '';
    errorEl.textContent = '';

    var xhr = new XMLHttpRequest();
    xhr.open('GET', '/api/proxies?password=' + encodeURIComponent(password), true);
    xhr.onload = function() {
      loading.style.display = 'none';
      if (xhr.status === 401) {
        errorEl.textContent = '❌ Incorrect password. Try again.';
        showToast('Incorrect password', 'error');
        return;
      }
      if (xhr.status !== 200) {
        errorEl.textContent = 'Server error (' + xhr.status + ').';
        return;
      }
      try {
        var d = JSON.parse(xhr.responseText);
        $('proxyCount').textContent = d.proxies.length + ' proxies';
        if (!d.proxies || d.proxies.length === 0) {
          grid.innerHTML = '<div style="color:#64748b;text-align:center;padding:20px;">No proxies loaded.</div>';
          return;
        }
        var html = '';
        for (var i = 0; i < d.proxies.length; i++) {
          var p = d.proxies[i];
          var statusClass = (p.status === 'available') ? 'available' : 'blocked';
          var statusLabel = (p.status === 'available') ? '✓ Available' : '✗ Blocked';
          html += '<div class="proxy-item">' +
            '<span class="addr">' + p.ip + ':' + p.port + '</span>' +
            '<span class="status-tag ' + statusClass + '">' + statusLabel + '</span>' +
            '</div>';
        }
        grid.innerHTML = html;
        proxyUnlocked = true;
        showToast('Proxy pool unlocked — ' + d.available + ' available', 'success');
      } catch(e) {
        errorEl.textContent = 'Failed to parse proxy data.';
      }
    };
    xhr.onerror = function() {
      loading.style.display = 'none';
      errorEl.textContent = 'Network error — server unreachable.';
    };
    xhr.send();
  }

  // =========================================================================
  // PASSWORD INPUT: Enter key support
  // =========================================================================
  $('proxyPassword').addEventListener('keydown', function(e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      loadProxies(this.value);
    }
  });

  // =========================================================================
  // UNLOCK BUTTON
  // =========================================================================
  $('proxyUnlockBtn').addEventListener('click', function() {
    var pw = $('proxyPassword').value;
    if (!pw) {
      $('proxyError').textContent = 'Please enter a password.';
      return;
    }
    loadProxies(pw);
  });

  // =========================================================================
  // INIT
  // =========================================================================
  fetchStats();
  statsInterval = setInterval(fetchStats, 5000);

})();
</script>
</body>
</html>`
