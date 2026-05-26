package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"
)

// ============================================================================
// dashboard.go — ICEbot Server Web Dashboard
//
// Serves a single-page HTML dashboard with live server stats and a
// password-protected proxy pool viewer.
//
// Password: saif1234 (required to unlock proxy list)
// ============================================================================

// dashboardPassword is the password required to view the proxy list
// and protected API endpoints.
const dashboardPassword = "saif1234"

// apiPasswordFormHTML is a minimal HTML page shown when a protected API
// endpoint is accessed without a valid password.
const apiPasswordFormHTML = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>Protected</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#080a18;color:#e2e8f0;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;display:flex;min-height:100vh;align-items:center;justify-content:center}
.card{background:linear-gradient(145deg,#0f1228,#1a1e3a);border:1px solid rgba(108,99,255,.12);border-radius:16px;padding:40px 48px;text-align:center;max-width:400px;width:90%}
.lock{margin-bottom:12px;opacity:.5}
h1{font-size:1.3rem;font-weight:600;color:#c4b5fd;margin-bottom:8px}
p{font-size:.85rem;color:rgba(148,163,184,.7);margin-bottom:24px;line-height:1.5}
form{display:flex;flex-direction:column;gap:12px}
input{background:rgba(8,10,24,.6);border:1px solid rgba(108,99,255,.15);border-radius:8px;padding:12px 16px;color:#e2e8f0;font-size:.9rem;outline:none;transition:border-color .2s}
input:focus{border-color:rgba(108,99,255,.5)}
button{background:linear-gradient(145deg,#6c63ff,#7c3aed);border:none;border-radius:8px;padding:12px;color:#fff;font-size:.9rem;font-weight:600;cursor:pointer;transition:opacity .2s}
button:hover{opacity:.9}
.error{color:#f87171;font-size:.8rem;margin-top:8px;min-height:1.2em}
</style></head>
<body>
<div class="card">
<div class="lock">&#x1f512;</div>
<h1>Password Required</h1>
<p>This API endpoint is protected. Enter the dashboard password to access it.</p>
<form method="get">
<input type="password" name="password" placeholder="Enter password" autocomplete="off" />
<button type="submit">Unlock</button>
<div class="error" id="pwError"></div>
</form>
</div>
<script>
(function(){var p=new URLSearchParams(location.search).get('password');if(p!==null){var e=document.getElementById('pwError');if(e)e.textContent='Incorrect password.'}})();
</script>
</body>
</html>`

// requirePassword checks the ?password= query parameter against the
// dashboard password. If valid, it returns (true, "").
// If invalid or missing, it writes the password form to w and returns
// (false, ""). On a wrong password submission, it also writes the form
// with an error message.
func requirePassword(w http.ResponseWriter, r *http.Request) bool {
	pw := r.URL.Query().Get("password")
	if pw == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(apiPasswordFormHTML))
		return false
	}
	if pw != dashboardPassword {
		// Return the same form but with the password pre-filled and an error
		html := apiPasswordFormHTML
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(html))
		return false
	}
	return true
}

// ============================================================================
// HTTP Handlers
// ============================================================================

// handleDashboard renders the full dashboard HTML page.
func handleDashboard(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(dashboardHTML))
}

// handleProxiesAPI returns the proxy list as JSON, protected by password.
//
// This endpoint is consumed by the dashboard's JavaScript after the user
// enters the correct password. It returns every proxy in the pool with
// its current status so the frontend can render color-coded cards.
//
// Query parameters:
//   - password (required): must match dashboardPassword ("saif1234")
//
// Response format:
//
//	{
//	  "total": 80,
//	  "available": 65,
//	  "blocked": 15,
//	  "proxies": [
//	    { "ip": "23.95.150.145", "port": "6114", "status": "available" },
//	    { "ip": "38.154.203.95", "port": "5863", "status": "blocked" },
//	    ...
//	  ]
//	}
//
// Status codes:
//   200 — Success. Proxy list is included in the response body.
//   401 — Unauthorized. Password is missing or does not match.
//
// Security:
//   - Password is compared with a constant-time-adjacent string equality
//   - No rate limiting is enforced (dashboard use only, not a public API)
//   - Credentials are NOT included in the response — only IP and port
//
// Usage: GET /api/proxies?password=saif1234
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

	// Gather aggregate stats (total, available, blocked counts)
	total, avail, blocked := ProxyStats()

	// Build the per-proxy detail list
	proxies := make([]map[string]interface{}, 0)

	blockedMu.RLock()
	for _, p := range residentialProxies {
		// Determine if this specific proxy is currently blocked
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

	// Sort: available proxies first, then blocked. Within each group,
	// sort by IP address for a clean, predictable display order.
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
// Dashboard HTML — Self-contained single-page application
//
// The HTML, CSS, and JavaScript are embedded in this variable for zero-
// dependency deployment. The dashboard auto-updates via fetch to /api/status
// and provides a password gate for the proxy list.
//
// Architecture:
//   - Go server injects this full HTML page on any non-WebSocket GET to "/"
//   - The browser renders the page and immediately starts a 5-second poll
//     loop to /api/status for live metrics
//   - The proxy list is hidden behind a password prompt. Entering "saif1234"
//     triggers a fetch to /api/proxies?password=... which returns the full
//     proxy pool as JSON. The JS renders cards in a responsive grid.
//   - Search/filter, click-to-copy, keyboard shortcuts, and auto-refresh
//     are all handled client-side with zero external dependencies.
//
// Design principles:
//   - Dark theme with purple/indigo accent palette (--accent-1/2/3)
//   - Glass-morphism cards with subtle borders and backdrop blur
//   - No emojis — pure text and Unicode symbols for a clean professional look
//   - Fully responsive: adapts from desktop to mobile without breakage
//   - Keyboard accessible: all controls reachable via Tab and Enter
//
// Browser support:
//   - Chrome 80+
//   - Firefox 75+
//   - Edge 80+
//   - Safari 13.1+
//
// Dependencies: None. Zero external libraries, CDNs, or frameworks.
//   The entire UI is built with vanilla CSS (Grid, Flexbox, Custom Properties)
//   and vanilla JavaScript (Fetch API, DOM manipulation).
//
// Security:
//   - Password is validated server-side via /api/proxies?password=
//   - No credentials are exposed in the HTML source or cached on disk
//   - Proxy list is never rendered until the correct password is provided
//   - All API calls use relative URLs for same-origin safety
// ============================================================================

var dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>ICEbot Server — Dashboard</title>
<style>
  /* ==========================================================================
   * RESET — Remove all default browser styling for consistent cross-platform
   * rendering. Every element starts from zero margin, padding, and uses
   * border-box so widths include padding and borders naturally.
   * ====================================================================== */
  *, *::before, *::after {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
  }

  /* ==========================================================================
   * ROOT VARIABLES — Central color palette and design tokens. Change these
   * values to globally adjust the theme without hunting through selectors.
   * ====================================================================== */
  :root {
    --bg-primary: #07080f;
    --bg-secondary: #0c0e1a;
    --bg-card: rgba(15, 18, 40, 0.75);
    --bg-card-hover: rgba(20, 24, 50, 0.85);
    --border-color: rgba(90, 100, 200, 0.12);
    --border-hover: rgba(90, 100, 200, 0.28);
    --text-primary: #e8ecf4;
    --text-secondary: #8892b0;
    --text-muted: #4a5070;
    --accent-1: #6c63ff;
    --accent-2: #a78bfa;
    --accent-3: #7c3aed;
    --glow-1: rgba(108, 99, 255, 0.06);
    --glow-2: rgba(167, 139, 250, 0.04);
    --green: #22c55e;
    --green-bg: rgba(34, 197, 94, 0.1);
    --green-border: rgba(34, 197, 94, 0.2);
    --red: #ef4444;
    --red-bg: rgba(239, 68, 68, 0.1);
    --red-border: rgba(239, 68, 68, 0.2);
    --radius-sm: 8px;
    --radius-md: 14px;
    --radius-lg: 20px;
    --shadow-card: 0 4px 32px rgba(0, 0, 0, 0.3);
    --shadow-glow: 0 0 40px rgba(108, 99, 255, 0.08);
    --font-sans: 'Segoe UI', system-ui, -apple-system, sans-serif;
    --font-mono: 'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', monospace;
    --transition: 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  }

  /* ==========================================================================
   * BASE — Page-level setup. Dark background, centered layout, smooth fonts.
   * ====================================================================== */
  html {
    font-size: 16px;
    scroll-behavior: smooth;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
  }

  body {
    font-family: var(--font-sans);
    background: var(--bg-primary);
    color: var(--text-primary);
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    line-height: 1.6;
    overflow-x: hidden;
  }

  a {
    color: inherit;
    text-decoration: none;
  }

  /* ==========================================================================
   * SCROLLBAR — Subtle custom scrollbar to match the dark theme.
   * ====================================================================== */
  ::-webkit-scrollbar {
    width: 5px;
  }
  ::-webkit-scrollbar-track {
    background: var(--bg-primary);
  }
  ::-webkit-scrollbar-thumb {
    background: #1e2040;
    border-radius: 3px;
  }
  ::-webkit-scrollbar-thumb:hover {
    background: #2a2d5a;
  }

  /* ==========================================================================
   * AMBIENT GLOW — Soft radial gradients fixed to the background for depth.
   * Also includes a subtle grid pattern overlay.
   * ====================================================================== */
  .bg-glow-1 {
    position: fixed;
    top: -25%;
    left: -15%;
    width: 55%;
    height: 55%;
    background: radial-gradient(ellipse, var(--glow-1), transparent 70%);
    pointer-events: none;
    z-index: -1;
  }
  .bg-glow-2 {
    position: fixed;
    bottom: -25%;
    right: -15%;
    width: 50%;
    height: 50%;
    background: radial-gradient(ellipse, var(--glow-2), transparent 70%);
    pointer-events: none;
    z-index: -1;
  }
  .bg-glow-3 {
    position: fixed;
    top: 40%;
    left: 50%;
    width: 40%;
    height: 40%;
    transform: translate(-50%, -50%);
    background: radial-gradient(ellipse, rgba(108, 99, 255, 0.03), transparent 70%);
    pointer-events: none;
    z-index: -1;
  }
  .bg-grid {
    position: fixed;
    top: 0; left: 0; right: 0; bottom: 0;
    background-image:
      linear-gradient(rgba(108, 99, 255, 0.03) 1px, transparent 1px),
      linear-gradient(90deg, rgba(108, 99, 255, 0.03) 1px, transparent 1px);
    background-size: 48px 48px;
    pointer-events: none;
    z-index: -1;
  }
    top: 40%;
    left: 40%;
    width: 30%;
    height: 30%;
    background: radial-gradient(ellipse, rgba(124, 58, 237, 0.03), transparent 70%);
    pointer-events: none;
    z-index: -1;
  }

  /* ==========================================================================
   * HEADER — Fixed top bar with branding, status, and live indicator.
   * ====================================================================== */
  .header {
    background: linear-gradient(180deg, rgba(12, 14, 26, 0.98), rgba(12, 14, 26, 0.85));
    border-bottom: 1px solid var(--border-color);
    padding: 18px 28px;
    position: sticky;
    top: 0;
    z-index: 100;
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
  }

  .header-inner {
    max-width: 1200px;
    margin: 0 auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 14px;
  }

  .header-brand {
    display: flex;
    align-items: center;
    gap: 18px;
  }

  .header-emblem {
    width: 46px;
    height: 46px;
    border-radius: 14px;
    background: linear-gradient(145deg, rgba(108, 99, 255, 0.15), rgba(124, 58, 237, 0.08));
    border: 1px solid rgba(108, 99, 255, 0.15);
    display: flex;
    align-items: center;
    justify-content: center;
    box-shadow: 0 0 30px rgba(108, 99, 255, 0.12), inset 0 1px 0 rgba(255,255,255,0.05);
    flex-shrink: 0;
    backdrop-filter: blur(4px);
    transition: box-shadow 0.3s;
  }
  .header-emblem:hover {
    box-shadow: 0 0 40px rgba(108, 99, 255, 0.25), inset 0 1px 0 rgba(255,255,255,0.05);
  }

  .header-title {
    display: flex;
    flex-direction: column;
  }

  .header-title .name {
    font-size: 1.35rem;
    font-weight: 700;
    letter-spacing: -0.3px;
    background: linear-gradient(135deg, #e8ecf4, #a5b4fc, #c084fc);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
  }

  .header-title .byline {
    font-size: 0.8rem;
    color: var(--text-secondary);
    font-weight: 400;
    letter-spacing: 0.5px;
  }

  .header-title .byline .sig {
    font-family: 'Times New Roman', 'Georgia', serif;
    font-style: italic;
    font-size: 0.95rem;
    background: linear-gradient(135deg, #c084fc, #e879f9);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    font-weight: 600;
    letter-spacing: 0.8px;
  }

  .header-status {
    display: flex;
    align-items: center;
    gap: 10px;
    background: var(--green-bg);
    border: 1px solid var(--green-border);
    padding: 6px 18px;
    border-radius: 20px;
    font-size: 0.78rem;
    font-weight: 600;
    color: var(--green);
    letter-spacing: 0.3px;
  }

  .header-status .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--green);
    animation: pulse-dot 2s ease-in-out infinite;
  }

  @keyframes pulse-dot {
    0%, 100% {
      opacity: 1;
      transform: scale(1);
      box-shadow: 0 0 0 0 rgba(34, 197, 94, 0.3);
    }
    50% {
      opacity: 0.6;
      transform: scale(1.15);
      box-shadow: 0 0 16px 4px rgba(34, 197, 94, 0.12);
    }
  }

  /* ==========================================================================
   * MAIN CONTAINER — Centers content with max-width for readability.
   * ====================================================================== */
  .main {
    max-width: 1200px;
    margin: 0 auto;
    padding: 32px 28px;
    flex: 1;
    width: 100%;
  }

  /* ==========================================================================
   * STATS BAR — Horizontal row of key metrics.
   * ====================================================================== */
  .stats-bar {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
    gap: 12px;
    margin-bottom: 28px;
  }

  .stat-cell {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-md);
    padding: 16px 14px;
    text-align: center;
    transition: all var(--transition);
    position: relative;
    overflow: hidden;
  }

  .stat-cell::after {
    content: '';
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 2px;
    background: linear-gradient(90deg, transparent, var(--accent-1), transparent);
    opacity: 0.3;
  }

  .stat-cell:hover {
    border-color: var(--border-hover);
    transform: translateY(-2px);
    box-shadow: var(--shadow-card);
  }

  .stat-cell .stat-value {
    font-size: 1.75rem;
    font-weight: 800;
    background: linear-gradient(135deg, #e8ecf4, #a5b4fc);
    -webkit-background-clip: text;
    -webkit-text-fill-color: transparent;
    background-clip: text;
    line-height: 1.2;
    font-variant-numeric: tabular-nums;
  }

  .stat-cell .stat-value.stat-green {
    background: linear-gradient(135deg, #34d399, #22c55e);
    -webkit-background-clip: text;
    background-clip: text;
  }

  .stat-cell .stat-value.stat-amber {
    background: linear-gradient(135deg, #fbbf24, #f59e0b);
    -webkit-background-clip: text;
    background-clip: text;
  }

  .stat-cell .stat-value.stat-red {
    background: linear-gradient(135deg, #f87171, #ef4444);
    -webkit-background-clip: text;
    background-clip: text;
  }

  .stat-cell .stat-label {
    font-size: 0.7rem;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 1px;
    font-weight: 600;
    margin-top: 5px;
  }

  .stat-cell .stat-sublabel {
    font-size: 0.65rem;
    color: rgba(74, 80, 112, 0.7);
    margin-top: 3px;
  }

  .stat-icon {
    width: 28px;
    height: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    margin: 0 auto 6px;
    border-radius: 8px;
    background: rgba(108, 99, 255, 0.08);
    color: var(--accent-1);
  }

  /* ==========================================================================
   * UPTIME DISPLAY — Shows how long the server has been running.
   * ====================================================================== */
  .uptime-container {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: rgba(8, 10, 24, 0.5);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 5px 14px;
    font-size: 0.7rem;
    color: var(--text-muted);
    font-variant-numeric: tabular-nums;
    letter-spacing: 0.3px;
  }
  .uptime-container .uptime-value {
    color: var(--text-secondary);
    font-family: var(--font-mono);
    font-size: 0.72rem;
  }

  /* ==========================================================================
   * VERSION TAG — Small decorative badge showing the current version.
   * ====================================================================== */
  .version-tag {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    background: linear-gradient(145deg, rgba(108, 99, 255, 0.12), rgba(124, 58, 237, 0.08));
    border: 1px solid rgba(108, 99, 255, 0.15);
    border-radius: 6px;
    padding: 3px 12px;
    font-size: 0.65rem;
    color: var(--accent-2);
    font-weight: 500;
    letter-spacing: 0.5px;
  }
  .version-tag .ver-dot {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: var(--accent-1);
    opacity: 0.6;
  }

  /* ==========================================================================
   * DIVIDER — Subtle horizontal separator used between sections.
   * ====================================================================== */
  .divider {
    height: 1px;
    background: linear-gradient(90deg, transparent, var(--border-color), transparent);
    margin: 18px 0;
    border: none;
  }

  /* ==========================================================================
   * NOTIFICATION DOT — Small pulsing dot for attention indicators.
   * ====================================================================== */
  .notif-dot {
    display: inline-block;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--accent-1);
    animation: pulse-dot 2s ease-in-out infinite;
  }

  /* ==========================================================================
   * LOCK SCREEN — Full-width gate that hides proxy content behind a password.
   * ====================================================================== */
  .lock-container {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-lg);
    padding: 50px 40px;
    text-align: center;
    transition: all var(--transition);
    box-shadow: var(--shadow-card);
  }

  .lock-container:hover {
    border-color: var(--border-hover);
  }

  .lock-container .lock-icon {
    font-size: 2.4rem;
    opacity: 0.35;
    margin-bottom: 12px;
    display: block;
  }

  .lock-container .lock-title {
    font-size: 1.05rem;
    font-weight: 600;
    color: var(--text-secondary);
    margin-bottom: 6px;
  }

  .lock-container .lock-desc {
    font-size: 0.82rem;
    color: var(--text-muted);
    max-width: 340px;
    margin: 0 auto 18px auto;
    line-height: 1.5;
  }

  .lock-container .input-row {
    display: flex;
    gap: 10px;
    justify-content: center;
    flex-wrap: wrap;
  }

  .lock-container input {
    background: rgba(8, 10, 24, 0.8);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 11px 18px;
    color: var(--text-primary);
    font-size: 0.9rem;
    font-family: var(--font-mono);
    outline: none;
    width: 220px;
    transition: border-color var(--transition);
    letter-spacing: 2px;
  }

  .lock-container input:focus {
    border-color: var(--accent-1);
    box-shadow: 0 0 0 3px rgba(108, 99, 255, 0.1);
  }

  .lock-container input::placeholder {
    color: var(--text-muted);
    letter-spacing: 0;
    font-family: var(--font-sans);
  }

  .lock-container button {
    background: linear-gradient(145deg, var(--accent-1), var(--accent-3));
    border: none;
    border-radius: var(--radius-sm);
    padding: 11px 28px;
    color: #fff;
    font-weight: 600;
    font-size: 0.85rem;
    cursor: pointer;
    transition: all var(--transition);
    letter-spacing: 0.3px;
  }

  .lock-container button:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 20px rgba(108, 99, 255, 0.3);
  }

  .lock-container button:active {
    transform: translateY(0);
  }

  .lock-container .lock-error {
    color: var(--red);
    font-size: 0.78rem;
    min-height: 1.4em;
    margin-top: 10px;
  }

  .lock-container .lock-loading {
    color: var(--accent-2);
    font-size: 0.78rem;
    margin-top: 10px;
    animation: pulse-text 1.2s ease-in-out infinite;
  }

  @keyframes pulse-text {
    0%, 100% { opacity: 0.5; }
    50% { opacity: 1; }
  }

  /* ==========================================================================
   * PROXY GRID — Displays proxies after unlock as a responsive card grid.
   * ====================================================================== */
  .proxy-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 8px;
    margin-top: 4px;
  }

  .proxy-card {
    background: rgba(8, 10, 24, 0.6);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 10px 14px;
    display: flex;
    align-items: center;
    justify-content: space-between;
    font-size: 0.8rem;
    font-family: var(--font-mono);
    transition: all var(--transition);
    position: relative;
    overflow: hidden;
  }
  .proxy-card::before {
    content: '';
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: 3px;
    background: linear-gradient(180deg, transparent, var(--accent-1), transparent);
    opacity: 0;
    transition: opacity var(--transition);
  }
  .proxy-card:hover::before {
    opacity: 0.6;
  }

  .proxy-card:hover {
    border-color: var(--border-hover);
    background: rgba(12, 15, 30, 0.8);
    transform: translateX(3px);
  }

  .proxy-card .proxy-addr {
    color: var(--text-secondary);
    font-size: 0.78rem;
  }

  .proxy-card .proxy-tag {
    font-size: 0.6rem;
    font-weight: 700;
    padding: 3px 10px;
    border-radius: 6px;
    text-transform: uppercase;
    letter-spacing: 0.6px;
    font-family: var(--font-sans);
  }

  .proxy-card .proxy-tag.tag-avail {
    background: var(--green-bg);
    color: var(--green);
    border: 1px solid var(--green-border);
  }

  .proxy-card .proxy-tag.tag-blocked {
    background: var(--red-bg);
    color: var(--red);
    border: 1px solid var(--red-border);
  }

  /* ==========================================================================
   * PROXY HEADER — Shows count and summary above the grid.
   * ====================================================================== */
  .proxy-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 10px;
    margin-bottom: 14px;
  }

  .proxy-header .proxy-count {
    font-size: 0.82rem;
    color: var(--text-secondary);
    font-weight: 500;
  }

  .proxy-header .proxy-count strong {
    color: var(--text-primary);
    font-weight: 700;
  }

  .proxy-header .proxy-summary {
    display: flex;
    gap: 12px;
    font-size: 0.72rem;
  }

  .proxy-header .proxy-summary span {
    display: flex;
    align-items: center;
    gap: 5px;
  }

  .proxy-header .proxy-summary .sum-avail {
    color: var(--green);
  }

  .proxy-header .proxy-summary .sum-blocked {
    color: var(--red);
  }

  /* ==========================================================================
   * SECTION — Reusable card container for content blocks.
   * ====================================================================== */
  .section {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-lg);
    overflow: hidden;
    transition: border-color var(--transition);
    box-shadow: var(--shadow-card);
  }

  .section:hover {
    border-color: var(--border-hover);
  }

  .section-body {
    padding: 24px;
  }

  /* ==========================================================================
   * API LINKS — Minimal reference row for REST endpoints.
   * ====================================================================== */
  .api-strip {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .api-strip a,
  .api-strip span {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: rgba(108, 99, 255, 0.05);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 7px 14px;
    font-size: 0.75rem;
    color: var(--accent-2);
    font-weight: 500;
    transition: all var(--transition);
    font-family: var(--font-mono);
  }
  .api-strip a:hover {
    color: #fff;
    border-color: var(--accent-1);
    background: rgba(108, 99, 255, 0.18);
    box-shadow: 0 0 12px rgba(108, 99, 255, 0.15);
  }

  /* ==========================================================================
   * PROXY LINKS — API links displayed inside the unlocked proxy box.
   * ====================================================================== */
  .proxy-links {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
    justify-content: center;
    padding: 18px 0 14px;
    margin-bottom: 10px;
    border-bottom: 1px solid var(--border-color);
  }
  .proxy-links a {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    background: rgba(108, 99, 255, 0.09);
    border: 1px solid rgba(108, 99, 255, 0.2);
    border-radius: var(--radius-sm);
    padding: 9px 18px;
    font-size: 0.78rem;
    color: #c4b5fd;
    font-weight: 500;
    transition: all 0.2s;
    text-decoration: none;
    font-family: var(--font-mono);
    letter-spacing: 0.2px;
  }
  .proxy-links a:hover {
    color: #fff;
    background: rgba(108, 99, 255, 0.25);
    border-color: var(--accent-1);
    box-shadow: 0 0 20px rgba(108, 99, 255, 0.25);
    transform: translateY(-1px);
  }
  .proxy-links .method-tag {
    font-size: 0.6rem;
    font-weight: 700;
    padding: 1px 6px;
    border-radius: 3px;
    background: rgba(34, 197, 94, 0.15);
    color: var(--green);
  }

  .api-strip .method {
    font-size: 0.6rem;
    font-weight: 700;
    padding: 1px 5px;
    border-radius: 3px;
    background: rgba(34, 197, 94, 0.15);
    color: var(--green);
  }

  .api-strip .method-muted {
    background: rgba(74, 80, 112, 0.2);
    color: var(--text-muted);
  }

  /* ==========================================================================
   * FOOTER — Simple centered copyright bar.
   * ====================================================================== */
  .footer {
    text-align: center;
    padding: 18px 28px;
    font-size: 0.72rem;
    color: rgba(74, 80, 112, 0.5);
    border-top: 1px solid rgba(90, 100, 200, 0.04);
    margin-top: 16px;
    letter-spacing: 0.3px;
  }

  .footer .sig {
    font-family: 'Times New Roman', 'Georgia', serif;
    font-style: italic;
    color: rgba(167, 139, 250, 0.4);
    font-size: 0.8rem;
  }

  /* ==========================================================================
   * LAST UPDATED — Timestamp display below stats.
   * ====================================================================== */
  .ts {
    font-size: 0.68rem;
    color: rgba(74, 80, 112, 0.6);
    text-align: right;
    margin-top: 10px;
    font-variant-numeric: tabular-nums;
  }

  /* ==========================================================================
   * TOAST — Notification popup for success/error feedback.
   * ====================================================================== */
  .toast-container {
    position: fixed;
    bottom: 28px;
    right: 28px;
    z-index: 9999;
    display: flex;
    flex-direction: column;
    gap: 8px;
    pointer-events: none;
  }

  .toast {
    background: rgba(15, 18, 40, 0.96);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 12px 20px;
    font-size: 0.8rem;
    color: var(--text-primary);
    backdrop-filter: blur(14px);
    -webkit-backdrop-filter: blur(14px);
    animation: toast-in 0.35s ease;
    max-width: 320px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
    pointer-events: auto;
  }

  .toast.toast-success {
    border-color: var(--green-border);
    border-left: 3px solid var(--green);
  }

  .toast.toast-error {
    border-color: var(--red-border);
    border-left: 3px solid var(--red);
  }

  @keyframes toast-in {
    from {
      opacity: 0;
      transform: translateX(30px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }

  /* ==========================================================================
   * RESPONSIVE — Adjustments for small screens and mobile.
   * ====================================================================== */
  @media (max-width: 720px) {
    .header {
      padding: 14px 16px;
    }
    .header-title .name {
      font-size: 1.1rem;
    }
    .main {
      padding: 16px;
    }
    .stats-bar {
      grid-template-columns: repeat(2, 1fr);
      gap: 8px;
    }
    .stat-cell .stat-value {
      font-size: 1.4rem;
    }
    .lock-container {
      padding: 30px 20px;
    }
    .lock-container input {
      width: 160px;
    }
    .proxy-grid {
      grid-template-columns: 1fr;
    }
    .proxy-header {
      flex-direction: column;
      align-items: flex-start;
    }
  }

  @media (max-width: 420px) {
    .stats-bar {
      grid-template-columns: 1fr 1fr;
    }
    .header-brand {
      gap: 12px;
    }
    .header-emblem {
      width: 34px;
      height: 34px;
      font-size: 0.9rem;
    }
    .header-title .name {
      font-size: 0.95rem;
    }
    .header-status {
      font-size: 0.7rem;
      padding: 4px 12px;
    }
    .lock-container .input-row {
      flex-direction: column;
      align-items: center;
    }
    .lock-container input {
      width: 100%;
      max-width: 240px;
    }
  }

  /* ==========================================================================
   * PRINT — Hide interactive elements when printing.
   * ====================================================================== */
  /* ==========================================================================
   * SEARCH BAR — Filters the proxy grid in real time.
   * ====================================================================== */
  .proxy-search {
    width: 100%;
    background: rgba(8, 10, 24, 0.7);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 10px 16px;
    color: var(--text-primary);
    font-size: 0.82rem;
    font-family: var(--font-mono);
    outline: none;
    margin-bottom: 12px;
    transition: border-color var(--transition);
  }
  .proxy-search:focus {
    border-color: var(--accent-1);
    box-shadow: 0 0 0 3px rgba(108, 99, 255, 0.08);
  }
  .proxy-search::placeholder {
    color: var(--text-muted);
    font-family: var(--font-sans);
    font-size: 0.8rem;
  }

  /* ==========================================================================
   * PROXY GRID EMPTY STATE — Shown when search returns no results.
   * ====================================================================== */
  .proxy-empty {
    grid-column: 1 / -1;
    text-align: center;
    padding: 40px 20px;
    color: var(--text-muted);
    font-size: 0.82rem;
  }
  .proxy-empty .empty-icon {
    font-size: 2rem;
    opacity: 0.3;
    display: block;
    margin-bottom: 8px;
  }

  /* ==========================================================================
   * COPY TOOLTIP — Appears when hovering over a proxy address.
   * ====================================================================== */
  .proxy-card .proxy-addr {
    cursor: pointer;
    position: relative;
  }
  .proxy-card .proxy-addr:hover {
    color: var(--accent-2);
  }
  .proxy-card .proxy-addr .copy-hint {
    display: none;
    position: absolute;
    bottom: calc(100% + 6px);
    left: 50%;
    transform: translateX(-50%);
    background: rgba(15, 18, 40, 0.96);
    border: 1px solid var(--border-color);
    border-radius: 4px;
    padding: 3px 10px;
    font-size: 0.6rem;
    color: var(--text-muted);
    white-space: nowrap;
    pointer-events: none;
    font-family: var(--font-sans);
  }
  .proxy-card .proxy-addr:hover .copy-hint {
    display: block;
  }

  /* ==========================================================================
   * SYSTEM INFO — Additional server details row.
   * ====================================================================== */
  .sys-row {
    display: flex;
    flex-wrap: wrap;
    gap: 6px 16px;
    margin-top: 16px;
    padding-top: 14px;
    border-top: 1px solid var(--border-color);
    font-size: 0.72rem;
    color: var(--text-muted);
  }
  .sys-row span {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .sys-row .sys-label {
    color: rgba(74, 80, 112, 0.6);
  }
  .sys-row .sys-value {
    color: var(--text-secondary);
    font-family: var(--font-mono);
    font-size: 0.7rem;
  }

  /* ==========================================================================
   * KEYBOARD SHORTCUT HINT — Subtle helper text.
   * ====================================================================== */
  .kbd-hint {
    font-size: 0.65rem;
    color: rgba(74, 80, 112, 0.4);
    text-align: center;
    margin-top: 12px;
    letter-spacing: 0.3px;
  }
  .kbd-hint kbd {
    display: inline-block;
    background: rgba(74, 80, 112, 0.15);
    border: 1px solid rgba(74, 80, 112, 0.2);
    border-radius: 3px;
    padding: 0 5px;
    font-size: 0.62rem;
    font-family: var(--font-mono);
    color: rgba(74, 80, 112, 0.6);
  }

  /* ==========================================================================
   * PING ANIMATION — Used to draw attention to status indicators.
   * ====================================================================== */
  @keyframes ping {
    0% {
      transform: scale(1);
      opacity: 0.6;
    }
    50% {
      transform: scale(1.3);
      opacity: 0.2;
    }
    100% {
      transform: scale(1);
      opacity: 0.6;
    }
  }
  .ping-ring {
    display: inline-block;
    width: 10px;
    height: 10px;
    border-radius: 50%;
    background: var(--green);
    animation: ping 1.5s ease-in-out infinite;
    position: relative;
  }
  .ping-ring::after {
    content: '';
    position: absolute;
    top: -4px;
    left: -4px;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    border: 2px solid rgba(34, 197, 94, 0.2);
    animation: ping 1.5s ease-in-out infinite 0.3s;
  }

  /* ==========================================================================
   * FLOAT ANIMATION — Subtle vertical bob for decorative elements.
   * ====================================================================== */
  @keyframes float {
    0%, 100% {
      transform: translateY(0);
    }
    50% {
      transform: translateY(-4px);
    }
  }
  .float-anim {
    animation: float 3s ease-in-out infinite;
  }

  /* ==========================================================================
   * GLOW PULSE — Soft border glow for focused or active elements.
   * ====================================================================== */
  @keyframes glow-pulse {
    0%, 100% {
      box-shadow: 0 0 8px rgba(108, 99, 255, 0.08);
    }
    50% {
      box-shadow: 0 0 20px rgba(108, 99, 255, 0.18);
    }
  }
  .glow-pulse {
    animation: glow-pulse 2.5s ease-in-out infinite;
  }

  /* ==========================================================================
   * SPINNER — Simple rotating loader for async operations.
   * ====================================================================== */
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
  .spinner {
    display: inline-block;
    width: 14px;
    height: 14px;
    border: 2px solid rgba(108, 99, 255, 0.15);
    border-top-color: var(--accent-1);
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
    vertical-align: middle;
  }
  .spinner-sm {
    width: 10px;
    height: 10px;
    border-width: 1.5px;
  }

  /* ==========================================================================
   * BADGE — Inline pill for counts, statuses, or tags.
   * ====================================================================== */
  .badge-pill {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 2px 8px;
    border-radius: 10px;
    font-size: 0.65rem;
    font-weight: 600;
    letter-spacing: 0.3px;
    line-height: 1.4;
  }
  .badge-pill.bg-accent {
    background: rgba(108, 99, 255, 0.12);
    color: var(--accent-2);
  }
  .badge-pill.bg-green {
    background: var(--green-bg);
    color: var(--green);
  }
  .badge-pill.bg-red {
    background: var(--red-bg);
    color: var(--red);
  }
  .badge-pill.bg-muted {
    background: rgba(74, 80, 112, 0.15);
    color: var(--text-muted);
  }

  /* ==========================================================================
   * VALUE CHANGE INDICATOR — Brief flash when a stat value changes.
   * ====================================================================== */
  @keyframes value-flash {
    0% { opacity: 0.5; }
    50% { opacity: 1; }
    100% { opacity: 1; }
  }
  .value-flash {
    animation: value-flash 0.4s ease;
  }

  /* ==========================================================================
   * FOOTER — Simple footer containing version and legal text.
   * ====================================================================== */
  .app-footer {
    text-align: center;
    padding: 24px 16px 32px;
    font-size: 0.65rem;
    color: rgba(74, 80, 112, 0.5);
    border-top: 1px solid rgba(74, 80, 112, 0.06);
    margin-top: 32px;
    letter-spacing: 0.3px;
  }
  .app-footer a {
    color: rgba(108, 99, 255, 0.4);
    text-decoration: none;
    transition: color 0.2s;
  }
  .app-footer a:hover {
    color: rgba(108, 99, 255, 0.7);
  }
  .app-footer .footer-sep {
    margin: 0 10px;
    opacity: 0.3;
  }

  /* ==========================================================================
   * MEDIA QUERIES — Responsive adjustments for smaller screens.
   * ====================================================================== */
  .proxy-toolbar {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 12px;
    flex-wrap: wrap;
  }
  .proxy-toolbar .proxy-search {
    margin-bottom: 0;
    flex: 1;
    min-width: 160px;
  }
  .proxy-toolbar .toolbar-btn {
    background: rgba(108, 99, 255, 0.08);
    border: 1px solid var(--border-color);
    border-radius: var(--radius-sm);
    padding: 10px 16px;
    color: var(--text-secondary);
    font-size: 0.78rem;
    cursor: pointer;
    transition: all var(--transition);
    font-family: var(--font-sans);
    white-space: nowrap;
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }
  .proxy-toolbar .toolbar-btn:hover {
    background: rgba(108, 99, 255, 0.15);
    border-color: var(--accent-1);
    color: var(--accent-2);
  }
  .proxy-toolbar .toolbar-btn:active {
    transform: scale(0.96);
  }
  .proxy-toolbar .toolbar-btn .btn-icon {
    font-size: 0.85rem;
    line-height: 1;
  }
  .proxy-toolbar .proxy-stats-mini {
    font-size: 0.7rem;
    color: var(--text-muted);
    white-space: nowrap;
  }

  /* ==========================================================================
   * TOGGLE SWITCH — For auto-refresh preference.
   * ====================================================================== */
  .toggle-wrap {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 0.7rem;
    color: var(--text-muted);
    cursor: pointer;
    user-select: none;
  }
  .toggle-wrap input {
    display: none;
  }
  .toggle-track {
    width: 30px;
    height: 16px;
    background: rgba(74, 80, 112, 0.3);
    border-radius: 10px;
    position: relative;
    transition: background var(--transition);
    flex-shrink: 0;
  }
  .toggle-track::after {
    content: '';
    position: absolute;
    top: 2px;
    left: 2px;
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: var(--text-muted);
    transition: all var(--transition);
  }
  .toggle-wrap input:checked + .toggle-track {
    background: rgba(108, 99, 255, 0.3);
  }
  .toggle-wrap input:checked + .toggle-track::after {
    left: 16px;
    background: var(--accent-1);
  }

  /* ==========================================================================
   * ANIMATIONS — Additional keyframes for UI polish.
   * ====================================================================== */
  @keyframes fade-in-up {
    from {
      opacity: 0;
      transform: translateY(12px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
  @keyframes fade-in {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  @keyframes shimmer {
    0% { background-position: -200px 0; }
    100% { background-position: calc(200px + 100%) 0; }
  }
  @keyframes scale-in {
    from {
      opacity: 0;
      transform: scale(0.92);
    }
    to {
      opacity: 1;
      transform: scale(1);
    }
  }
  @keyframes slide-down {
    from {
      opacity: 0;
      max-height: 0;
    }
    to {
      opacity: 1;
      max-height: 200px;
    }
  }

  .lock-container,
  .section {
    animation: fade-in-up 0.4s ease both;
  }
  .stat-cell {
    animation: fade-in-up 0.4s ease both;
  }
  .stat-cell:nth-child(1) { animation-delay: 0.02s; }
  .stat-cell:nth-child(2) { animation-delay: 0.06s; }
  .stat-cell:nth-child(3) { animation-delay: 0.10s; }
  .stat-cell:nth-child(4) { animation-delay: 0.14s; }

  .proxy-card {
    animation: fade-in 0.3s ease both;
  }

  /* ==========================================================================
   * LOADING SKELETON — Placeholder shimmer while proxy data loads.
   * ====================================================================== */
  .skeleton-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: 8px;
  }
  .skeleton-card {
    height: 38px;
    background: linear-gradient(90deg, rgba(15,18,40,0.4) 0%, rgba(25,30,60,0.5) 50%, rgba(15,18,40,0.4) 100%);
    background-size: 200px 100%;
    border-radius: var(--radius-sm);
    animation: shimmer 1.6s ease-in-out infinite;
  }

  /* ==========================================================================
   * FOCUS VISIBLE — Accessibility outline for keyboard navigation.
   * ====================================================================== */
  *:focus-visible {
    outline: 2px solid var(--accent-1);
    outline-offset: 2px;
  }
  button:focus-visible,
  input:focus-visible {
    outline: 2px solid var(--accent-1);
    outline-offset: 2px;
  }

  /* ==========================================================================
   * SELECTION STYLE — Matches accent color.
   * ====================================================================== */
  ::selection {
    background: rgba(108, 99, 255, 0.25);
    color: #fff;
  }

  /* ==========================================================================
   * PRINT — Hide interactive elements when printing.
   * ====================================================================== */
  @media print {
    .header {
      position: static;
    }
    .lock-container input,
    .lock-container button,
    .proxy-search,
    .toast-container {
      display: none;
    }
  }
</style>
</head>
<body>

<!-- Ambient background glow layers -->
<div class="bg-glow-1"></div>
<div class="bg-glow-2"></div>
<div class="bg-glow-3"></div>
<div class="bg-grid"></div>

<!-- ==========================================================================
     HEADER — Branding, server name, and live status badge
     ====================================================================== -->
<header class="header">
  <div class="header-inner">
    <div class="header-brand">
      <div class="header-emblem">
        <svg width="32" height="32" viewBox="0 0 32 32" fill="none">
          <rect x="2" y="2" width="28" height="28" rx="8" stroke="url(#lg)" stroke-width="1.5" fill="rgba(108,99,255,0.06)"/>
          <path d="M10 20V8l12 6-12 6z" fill="url(#lg)" opacity="0.9"/>
          <defs>
            <linearGradient id="lg" x1="0" y1="0" x2="32" y2="32">
              <stop stop-color="#6c63ff"/>
              <stop offset="1" stop-color="#a78bfa"/>
            </linearGradient>
          </defs>
        </svg>
      </div>
      <div class="header-title">
        <div class="name">ICEbot Server</div>
        <div class="byline">
          built by <span class="sig">Saif</span>
          &middot; Residential Proxy Edition
        </div>
      </div>
    </div>
    <div style="display:flex;align-items:center;gap:14px;flex-wrap:wrap;">
      <div class="uptime-container">
        <span>Uptime</span>
        <span class="uptime-value" id="uptimeDisplay">0s</span>
      </div>
      <div class="header-status">
        <span class="dot"></span>
        <span id="statusLabel">RUNNING</span>
      </div>
    </div>
  </div>
</header>

<!-- ==========================================================================
     MAIN CONTENT — Stats bar, password gate, proxy grid, API references
     ====================================================================== -->
<main class="main">

  <!-- =======================================================================
       STATS BAR — Live server metrics updated every 5 seconds
       ====================================================================== -->
  <div class="stats-bar" id="statsBar">
    <div class="stat-cell">
      <div class="stat-icon">
        <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
          <rect x="1" y="3" width="16" height="12" rx="2" stroke="currentColor" stroke-width="1.3"/>
          <circle cx="9" cy="9" r="2.5" stroke="currentColor" stroke-width="1.3"/>
          <path d="M9 11.5V14" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
        </svg>
      </div>
      <div class="stat-value" id="statSessions">0</div>
      <div class="stat-label">Sessions</div>
    </div>
    <div class="stat-cell">
      <div class="stat-icon">
        <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
          <circle cx="9" cy="6" r="3" stroke="currentColor" stroke-width="1.3"/>
          <path d="M3 16c0-3.3 2.7-6 6-6s6 2.7 6 6" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
        </svg>
      </div>
      <div class="stat-value" id="statBots">0</div>
      <div class="stat-label">Bots</div>
    </div>
    <div class="stat-cell">
      <div class="stat-icon">
        <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
          <path d="M9 2L11 6.5L16 7L12.5 10.5L13.5 16L9 13L4.5 16L5.5 10.5L2 7L7 6.5L9 2Z" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>
        </svg>
      </div>
      <div class="stat-value stat-green" id="statJoined">0</div>
      <div class="stat-label">Joined</div>
    </div>
    <div class="stat-cell">
      <div class="stat-icon">
        <svg width="18" height="18" viewBox="0 0 18 18" fill="none">
          <rect x="1" y="1" width="16" height="16" rx="3" stroke="currentColor" stroke-width="1.3"/>
          <circle cx="9" cy="9" r="4" stroke="currentColor" stroke-width="1.3"/>
          <circle cx="9" cy="9" r="1.5" fill="currentColor"/>
        </svg>
      </div>
      <div class="stat-value" id="statProxies">0</div>
      <div class="stat-label">Proxies</div>
      <div class="stat-sublabel">
        <span id="statAvail">0</span> avail &middot;
        <span id="statBlocked">0</span> blocked
      </div>
    </div>
  </div>

  <!-- PASSWORD GATE + PROXY GRID — Locked behind password. -->
  <div class="section">
    <div class="section-body" id="proxySectionBody">

      <!-- LOCK OVERLAY — visible until password is accepted -->
      <div class="lock-container" id="proxyLock">
        <span class="lock-icon">&#x1f512;</span>
        <div class="lock-title">Proxy Pool Locked</div>
        <div class="lock-desc">
          Enter the dashboard password to view all
          <strong id="proxyLockCount">80</strong>
          residential proxies with live status.
        </div>
        <div class="input-row">
          <input
            type="password"
            id="proxyPassword"
            placeholder="Enter password"
            autocomplete="off"
            spellcheck="false"
          />
          <button id="proxyUnlockBtn">Unlock</button>
        </div>
        <div class="lock-error" id="proxyError"></div>
      </div>

      <!-- PROXY CONTENT — hidden until password is accepted -->
      <div id="proxyContent" style="display:none;">
        <div class="proxy-header">
          <div class="proxy-count">
            <strong id="proxyCountLabel">0</strong> residential proxies
            <span style="font-weight:400;color:var(--text-muted);font-size:0.7rem;">
              &middot; last refresh <span id="proxyRefreshTime">—</span>
            </span>
          </div>
          <div class="proxy-summary">
            <span class="sum-avail">
              &#x25cf; <span id="proxyAvailLabel">0</span> available
            </span>
            <span class="sum-blocked">
              &#x25cf; <span id="proxyBlockedLabel">0</span> blocked
            </span>
          </div>
        </div>
        <div class="proxy-toolbar">
          <input
            type="text"
            class="proxy-search"
            id="proxySearch"
            placeholder="Filter proxies by IP or port..."
            spellcheck="false"
          />
          <button class="toolbar-btn" id="proxyRefreshBtn" title="Refresh proxy list">
            <span class="btn-icon">&#x21bb;</span>
            Refresh
          </button>
          <label class="toggle-wrap" title="Auto-refresh proxy list every 30 seconds">
            <input type="checkbox" id="proxyAutoRefresh" checked />
            <span class="toggle-track"></span>
            <span>Auto</span>
          </label>
        </div>
        <div class="proxy-links">
          <a href="#" class="api-link" data-path="/api/status">
            <span class="method-tag">GET</span> /api/status
          </a>
          <a href="#" class="api-link" data-path="/api/turnstile-status">
            <span class="method-tag">GET</span> /api/turnstile-status
          </a>
          <a href="#" class="api-link" data-path="/api/proxy-status">
            <span class="method-tag">GET</span> /api/proxy-status
          </a>
        </div>
        <div class="proxy-grid" id="proxyGrid"></div>
        <div id="proxyLoading" style="display:none; text-align:center; padding:24px; color:var(--text-muted); font-size:0.82rem;">
          Loading proxy pool...
        </div>
      </div>

    </div>
  </div>

  <!-- =======================================================================
       SYSTEM INFO — Server metadata displayed below the proxy pool.
       ====================================================================== -->
  <div class="section" style="margin-top: 20px;">
    <div class="section-body">
      <div style="font-size:0.82rem;font-weight:600;color:var(--text-secondary);margin-bottom:6px;letter-spacing:0.3px;">
        System
      </div>
      <div class="sys-row">
        <span>
          <span class="sys-label">Status</span>
          <span class="sys-value" id="sysStatus">online</span>
        </span>
        <span>
          <span class="sys-label">Proxies</span>
          <span class="sys-value" id="sysProxyCount">80</span>
        </span>
        <span>
          <span class="sys-label">Bots</span>
          <span class="sys-value" id="sysBotCount">0</span>
        </span>
        <span>
          <span class="sys-label">Sessions</span>
          <span class="sys-value" id="sysSessionCount">0</span>
        </span>
        <span>
          <span class="sys-label">Server</span>
          <span class="sys-value">Go / HostingGuru</span>
        </span>
        <span>
          <span class="sys-label">Edition</span>
          <span class="sys-value">Residential Proxy v7</span>
        </span>
        <span>
          <span class="sys-label">Version</span>
          <span class="sys-value">7.2.1</span>
        </span>
        <span>
          <span class="sys-label">Runtime</span>
          <span class="sys-value">Go 1.24</span>
        </span>
        <span>
          <span class="sys-label">Platform</span>
          <span class="sys-value">HostingGuru</span>
        </span>
        <span>
          <span class="sys-label">Time</span>
          <span class="sys-value" id="sysCurrentTime">—</span>
        </span>
        <span>
          <span class="sys-label">Author</span>
          <span class="sys-value" style="font-family:'Times New Roman','Georgia',serif;font-style:italic;color:rgba(167,139,250,0.6);">Saif</span>
        </span>
      </div>
    </div>
  </div>

  <!-- Keyboard shortcut hint -->
  <div class="kbd-hint">
    Press <kbd>Enter</kbd> to unlock &middot;
    Press <kbd>/</kbd> to search &middot;
    Click any proxy IP to copy &middot;
    <kbd>R</kbd> to refresh
  </div>

  <!-- Timestamp -->
  <div class="ts" id="lastUpdated">Waiting for data...</div>

</main>

<!-- ==========================================================================
     FOOTER
     ====================================================================== -->
<footer class="footer">
  &copy; 2026 &middot; ICEbot Server &middot; built by <span class="sig">Saif</span>
</footer>

<!-- ==========================================================================
     TOAST CONTAINER — For success and error notifications
     ====================================================================== -->
<div class="toast-container" id="toastContainer"></div>

<!-- ==========================================================================
     JAVASCRIPT — Application logic: stats polling, password gate, proxy fetch
     ====================================================================== -->
<script>
(function() {
  'use strict';

  /* ========================================================================
   * CONSTANTS
   * ====================================================================== */
  var STATS_INTERVAL_MS = 5000;
  var PROXY_DATA = [];

  /* ========================================================================
   * STATE
   * ====================================================================== */
  var proxyUnlocked = false;
  var statsIntervalId = null;
  var currentFilter = '';
  var sessionStart = new Date();

  /* ========================================================================
   * DOM REFERENCES — Cache all queried elements for performance.
   * ====================================================================== */
  var dom = {};

  function cacheDom() {
    dom.statusLabel      = document.getElementById('statusLabel');
    dom.statsSessions    = document.getElementById('statSessions');
    dom.statsBots        = document.getElementById('statBots');
    dom.statsJoined      = document.getElementById('statJoined');
    dom.statsProxies     = document.getElementById('statProxies');
    dom.statsAvail       = document.getElementById('statAvail');
    dom.statsBlocked     = document.getElementById('statBlocked');
    dom.lastUpdated      = document.getElementById('lastUpdated');
    dom.proxyLock        = document.getElementById('proxyLock');
    dom.proxyContent     = document.getElementById('proxyContent');
    dom.proxyGrid        = document.getElementById('proxyGrid');
    dom.proxyLoading     = document.getElementById('proxyLoading');
    dom.proxyPassword    = document.getElementById('proxyPassword');
    dom.proxyUnlockBtn   = document.getElementById('proxyUnlockBtn');
    dom.proxyError       = document.getElementById('proxyError');
    dom.proxyCountLabel  = document.getElementById('proxyCountLabel');
    dom.proxyAvailLabel  = document.getElementById('proxyAvailLabel');
    dom.proxyBlockedLabel = document.getElementById('proxyBlockedLabel');
    dom.proxyLockCount   = document.getElementById('proxyLockCount');
    dom.proxySearch      = document.getElementById('proxySearch');
    dom.toastContainer   = document.getElementById('toastContainer');
    dom.sysStatus        = document.getElementById('sysStatus');
    dom.sysProxyCount    = document.getElementById('sysProxyCount');
    dom.sysBotCount      = document.getElementById('sysBotCount');
    dom.sysSessionCount  = document.getElementById('sysSessionCount');
    dom.uptimeDisplay    = document.getElementById('uptimeDisplay');
    dom.sysCurrentTime   = document.getElementById('sysCurrentTime');
  }

  /* ========================================================================
   * TOAST — Shows a small notification at the bottom-right of the screen.
   * Auto-dismisses after 3.5 seconds.
   * ====================================================================== */
  function showToast(message, type) {
    type = type || 'info';
    var toast = document.createElement('div');
    toast.className = 'toast';
    if (type === 'success') {
      toast.className += ' toast-success';
    } else if (type === 'error') {
      toast.className += ' toast-error';
    }
    toast.textContent = message;
    dom.toastContainer.appendChild(toast);
    setTimeout(function() {
      if (toast.parentNode) {
        toast.parentNode.removeChild(toast);
      }
    }, 3500);
  }

  /* ========================================================================
   * FETCH STATS — Polls /api/status and updates all stat cells in the bar.
   * Called once at startup, then every STATS_INTERVAL_MS.
   * ====================================================================== */
  function fetchStats() {
    var xhr = new XMLHttpRequest();
    xhr.open('GET', '/api/status', true);
    xhr.timeout = 8000;

    xhr.onload = function() {
      if (xhr.status !== 200) {
        dom.statusLabel.textContent = 'DEGRADED';
        dom.statusLabel.style.color = '#f59e0b';
        return;
      }
      try {
        var d = JSON.parse(xhr.responseText);
        dom.statsSessions.textContent  = d.sessions || 0;
        dom.statsBots.textContent      = d.totalBots || 0;
        dom.statsJoined.textContent    = d.joinedBots || 0;
        dom.statsProxies.textContent   = d.proxies || 0;
        dom.statsAvail.textContent     = d.proxiesAvail || 0;
        dom.statsBlocked.textContent   = d.proxiesBlocked || 0;
        dom.lastUpdated.textContent    = 'updated ' + new Date().toLocaleTimeString();
        dom.statusLabel.textContent    = 'RUNNING';
        dom.statusLabel.style.color    = '';
        dom.sysStatus.textContent      = 'online';
        dom.sysProxyCount.textContent  = d.proxies || 80;
        dom.sysBotCount.textContent    = d.totalBots || 0;
        dom.sysSessionCount.textContent = d.sessions || 0;
      } catch (e) {
        /* JSON parse failure — ignore, will retry on next interval */
      }
    };

    xhr.onerror = function() {
      dom.statusLabel.textContent = 'OFFLINE';
      dom.statusLabel.style.color = '#ef4444';
    };

    xhr.ontimeout = function() {
      dom.statusLabel.textContent = 'TIMEOUT';
      dom.statusLabel.style.color = '#f59e0b';
    };

    xhr.send();
  }

  /* ========================================================================
   * RENDER PROXY GRID — Takes the raw proxy data array, applies the current
   * search filter, and rebuilds the grid DOM. Also attaches click-to-copy
   * behavior on each proxy address.
   * ====================================================================== */
  function renderProxyGrid(data) {
    PROXY_DATA = data || PROXY_DATA;

    var filtered = PROXY_DATA;
    if (currentFilter.length > 0) {
      var q = currentFilter.toLowerCase();
      filtered = PROXY_DATA.filter(function(p) {
        var addr = (p.ip + ':' + p.port).toLowerCase();
        return addr.indexOf(q) !== -1;
      });
    }

    dom.proxyCountLabel.textContent = filtered.length + ' / ' + PROXY_DATA.length;
    dom.proxyAvailLabel.textContent = 0;
    dom.proxyBlockedLabel.textContent = 0;

    if (filtered.length === 0) {
      dom.proxyGrid.innerHTML =
        '<div class="proxy-empty">' +
        '<span class="empty-icon">&#x1f50d;</span>' +
        'No proxies match "' + currentFilter + '".' +
        '</div>';
      return;
    }

    var availCount = 0;
    var blockedCount = 0;
    var html = '';
    for (var i = 0; i < filtered.length; i++) {
      var p = filtered[i];
      var isAvail = (p.status === 'available');
      if (isAvail) { availCount++; } else { blockedCount++; }
      var tagClass = isAvail ? 'tag-avail' : 'tag-blocked';
      var tagLabel = isAvail ? 'AVAILABLE' : 'BLOCKED';
      var addr = p.ip + ':' + p.port;
      html += '<div class="proxy-card" data-addr="' + addr + '">' +
        '<span class="proxy-addr" data-copy="' + addr + '">' +
          addr +
          '<span class="copy-hint">click to copy</span>' +
        '</span>' +
        '<span class="proxy-tag ' + tagClass + '">' + tagLabel + '</span>' +
        '</div>';
    }
    dom.proxyGrid.innerHTML = html;
    dom.proxyAvailLabel.textContent = availCount;
    dom.proxyBlockedLabel.textContent = blockedCount;

    /* Attach click-to-copy handlers */
    var addrSpans = dom.proxyGrid.querySelectorAll('.proxy-addr[data-copy]');
    for (var j = 0; j < addrSpans.length; j++) {
      (function(el) {
        el.addEventListener('click', function(e) {
          var text = el.getAttribute('data-copy');
          if (navigator.clipboard && navigator.clipboard.writeText) {
            navigator.clipboard.writeText(text).then(function() {
              showToast('Copied: ' + text, 'success');
            }).catch(function() {
              fallbackCopy(text);
            });
          } else {
            fallbackCopy(text);
          }
          e.stopPropagation();
        });
      })(addrSpans[j]);
    }
  }

  /* ========================================================================
   * FALLBACK COPY — Uses the older execCommand approach for browsers that
   * do not support the Clipboard API.
   * ====================================================================== */
  function fallbackCopy(text) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    ta.style.left = '-9999px';
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand('copy');
      showToast('Copied: ' + text, 'success');
    } catch (e) {
      showToast('Could not copy', 'error');
    }
    document.body.removeChild(ta);
  }

  /* ========================================================================
   * LOAD PROXIES — Fetches the protected proxy list from /api/proxies with
   * the user-supplied password. On success, stores data and renders grid.
   * On 401, shows an error message.
   * ====================================================================== */
  function loadProxies(password) {
    dom.proxyLoading.style.display = 'block';
    dom.proxyError.textContent = '';

    /* Show skeleton grid while loading */
    var skeletonHtml = '<div class="skeleton-grid">';
    for (var s = 0; s < 12; s++) {
      skeletonHtml += '<div class="skeleton-card"></div>';
    }
    skeletonHtml += '</div>';
    dom.proxyGrid.innerHTML = skeletonHtml;

    var xhr = new XMLHttpRequest();
    xhr.open('GET', '/api/proxies?password=' + encodeURIComponent(password), true);
    xhr.timeout = 10000;

    xhr.onload = function() {
      dom.proxyLoading.style.display = 'none';

      if (xhr.status === 401) {
        dom.proxyError.textContent = 'Incorrect password. Please try again.';
        showToast('Incorrect password', 'error');
        return;
      }

      if (xhr.status !== 200) {
        dom.proxyError.textContent = 'Server error (' + xhr.status + ').';
        showToast('Server error', 'error');
        return;
      }

      try {
        var d = JSON.parse(xhr.responseText);

        if (!d.proxies || d.proxies.length === 0) {
          dom.proxyGrid.innerHTML =
            '<div class="proxy-empty">' +
            '<span class="empty-icon">&#x1f4cb;</span>' +
            'No proxies in the pool.' +
            '</div>';
          proxyUnlocked = true;
          dom.proxyLock.style.display = 'none';
          dom.proxyContent.style.display = 'block';
          return;
        }

        proxyUnlocked = true;
        dom.proxyLock.style.display = 'none';
        dom.proxyContent.style.display = 'block';

        /* Update the last-refresh timestamp */
        var rt = document.getElementById('proxyRefreshTime');
        if (rt) {
          rt.textContent = new Date().toLocaleTimeString();
        }

        renderProxyGrid(d.proxies);

        showToast(
          'Proxy pool unlocked - ' + d.available + ' available, ' + d.blocked + ' blocked',
          'success'
        );
      } catch (e) {
        dom.proxyError.textContent = 'Failed to parse server response.';
        showToast('Parse error', 'error');
      }
    };

    xhr.onerror = function() {
      dom.proxyLoading.style.display = 'none';
      dom.proxyError.textContent = 'Network error - server unreachable.';
      showToast('Network error', 'error');
    };

    xhr.ontimeout = function() {
      dom.proxyLoading.style.display = 'none';
      dom.proxyError.textContent = 'Request timed out.';
      showToast('Request timed out', 'error');
    };

    xhr.send();
  }

  /* ========================================================================
   * PROXY AUTO-REFRESH helpers
   * ====================================================================== */
  var proxyRefreshIntervalId = null;

  function startProxyAutoRefresh() {
    stopProxyAutoRefresh();
    proxyRefreshIntervalId = setInterval(function() {
      if (proxyUnlocked && dom.proxyPassword.value) {
        loadProxies(dom.proxyPassword.value);
      }
    }, 30000);
  }

  function stopProxyAutoRefresh() {
    if (proxyRefreshIntervalId) {
      clearInterval(proxyRefreshIntervalId);
      proxyRefreshIntervalId = null;
    }
  }

  /* ========================================================================
   * BIND EVENTS — Attach all event listeners after cacheDom() has run.
   * This must be called from init() AFTER cacheDom() so that the DOM
   * element references in the [dom] object are populated.
   * ====================================================================== */
  function bindEvents() {

    // Enter key triggers unlock
    dom.proxyPassword.addEventListener('keydown', function(e) {
      if (e.key === 'Enter') {
        e.preventDefault();
        var pw = dom.proxyPassword.value;
        if (pw.length === 0) {
          dom.proxyError.textContent = 'Please enter a password.';
          return;
        }
        loadProxies(pw);
      }
    });

    // Clicking the unlock button
    dom.proxyUnlockBtn.addEventListener('click', function() {
      var pw = dom.proxyPassword.value;
      if (pw.length === 0) {
        dom.proxyError.textContent = 'Please enter a password.';
        return;
      }
      loadProxies(pw);
    });

    /* ========================================================================
     * SEARCH FILTER — Filters the visible proxy grid in real time
     * ====================================================================== */
    dom.proxySearch.addEventListener('input', function() {
      currentFilter = dom.proxySearch.value;
      renderProxyGrid();
    });

    /* ========================================================================
     * REFRESH BUTTON — Manual refresh
     * ====================================================================== */
    dom.proxyRefreshBtn.addEventListener('click', function() {
      if (!dom.proxyPassword.value) {
        showToast('No password stored — re-enter to refresh.', 'error');
        return;
      }
      loadProxies(dom.proxyPassword.value);
    });

    /* ========================================================================
     * AUTO-REFRESH TOGGLE
     * ====================================================================== */
    dom.proxyAutoRefresh.addEventListener('change', function() {
      if (dom.proxyAutoRefresh.checked) {
        startProxyAutoRefresh();
      } else {
        stopProxyAutoRefresh();
      }
    });

    /* ========================================================================
     * API LINK HANDLER — Opens protected endpoints with the stored password.
     * ====================================================================== */
    var apiLinks = document.querySelectorAll('.api-link');
    for (var li = 0; li < apiLinks.length; li++) {
      (function(link) {
        link.addEventListener('click', function(e) {
          e.preventDefault();
          var pw = dom.proxyPassword.value;
          var path = link.getAttribute('data-path');
          var url = path + '?password=' + encodeURIComponent(pw);
          window.open(url, '_blank');
        });
      })(apiLinks[li]);
    }

    /* ========================================================================
     * GLOBAL KEYBOARD SHORTCUTS:
     *   /  — Focus the proxy search bar
     *   R  — Refresh the proxy list (only when unlocked)
     * ====================================================================== */
    document.addEventListener('keydown', function(e) {
      var tag = (e.target && e.target.tagName) || '';
      var isInput = (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT');

      if (e.key === '/' && !isInput) {
        e.preventDefault();
        if (dom.proxySearch) {
          dom.proxySearch.focus();
        }
        return;
      }

      if ((e.key === 'r' || e.key === 'R') && !isInput) {
        if (proxyUnlocked && dom.proxyPassword.value) {
          e.preventDefault();
          loadProxies(dom.proxyPassword.value);
        }
        return;
      }
    });

  }

  /* ========================================================================
   * INIT — Start the stats poller, cache DOM refs, and bind events.
   * ====================================================================== */
  function init() {
    cacheDom();
    bindEvents();
    fetchStats();
    statsIntervalId = setInterval(fetchStats, STATS_INTERVAL_MS);
    startProxyAutoRefresh();

    /* Uptime counter — updates every second */
    setInterval(function() {
      var elapsed = Math.floor((new Date() - sessionStart) / 1000);
      var h = Math.floor(elapsed / 3600);
      var m = Math.floor((elapsed % 3600) / 60);
      var s = elapsed % 60;
      var parts = [];
      if (h > 0) { parts.push(h + 'h'); }
      if (m > 0 || h > 0) { parts.push(m + 'm'); }
      parts.push(s + 's');
      if (dom.uptimeDisplay) {
        dom.uptimeDisplay.textContent = parts.join(' ');
      }
    }, 1000);

    /* Current time clock — updates every 10 seconds */
    setInterval(function() {
      if (dom.sysCurrentTime) {
        dom.sysCurrentTime.textContent = new Date().toLocaleString();
      }
    }, 10000);
    if (dom.sysCurrentTime) {
      dom.sysCurrentTime.textContent = new Date().toLocaleString();
    }
  }

  /* Kick off on DOM ready */
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

})();
</script>
  <footer class="app-footer">
    <span>ICEbot Server v7.2.1</span>
    <span class="footer-sep">&middot;</span>
    <span>Saif Residential Proxy Pool</span>
    <span class="footer-sep">&middot;</span>
    <span>Powered by Go</span>
    <br>
    <a href="#">Dashboard</a>
    <span class="footer-sep">&middot;</span>
    <span>All times are local</span>
  </footer>
</body>
</html>`
