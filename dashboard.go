package main

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ============================================================================
// dashboard.go — ICEbot Server Web Dashboard  v8.0 "PHANTOM"
//
// A fully self-contained single-page dashboard with:
//   - Matrix-rain canvas background
//   - Animated HUD stat cards with live counters
//   - Proxy pool with 2D canvas health ring
//   - Security: rate-limiting (5 attempts → 60s lockout),
//     timing-safe password compare, session tokens, auto-lock,
//     inactivity watcher, anti-clickjack headers
//   - Real-time line charts (proxy history, session history)
//   - Bulk copy, CSV export, per-column sort
//   - Keyboard shortcuts, custom cursor, scanline overlay
//
// Password: saif1234
// ============================================================================

const dashboardPassword = "saif1234"

// ---------------------------------------------------------------------------
// Rate-limiter for the proxy API endpoint
// ---------------------------------------------------------------------------

type loginAttempt struct {
	count     int
	lockedAt  time.Time
	lastTry   time.Time
}

var (
	loginMu      sync.Mutex
	loginAttempts = make(map[string]*loginAttempt)
)

const (
	maxLoginAttempts = 5
	lockoutDuration  = 60 * time.Second
)

// checkRateLimit returns (allowed bool, retryAfterSeconds int).
func checkRateLimit(ip string) (bool, int) {
	loginMu.Lock()
	defer loginMu.Unlock()

	a, ok := loginAttempts[ip]
	if !ok {
		loginAttempts[ip] = &loginAttempt{}
		return true, 0
	}

	// Reset count if last try was over 10 minutes ago
	if time.Since(a.lastTry) > 10*time.Minute {
		a.count = 0
		a.lockedAt = time.Time{}
	}

	if !a.lockedAt.IsZero() {
		remaining := lockoutDuration - time.Since(a.lockedAt)
		if remaining > 0 {
			return false, int(remaining.Seconds()) + 1
		}
		// Lockout expired
		a.count = 0
		a.lockedAt = time.Time{}
	}

	return true, 0
}

func recordFailedLogin(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()

	a, ok := loginAttempts[ip]
	if !ok {
		a = &loginAttempt{}
		loginAttempts[ip] = a
	}
	a.count++
	a.lastTry = time.Now()
	if a.count >= maxLoginAttempts {
		a.lockedAt = time.Now()
	}
}

func resetLoginAttempts(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginAttempts, ip)
}

// clientIP extracts the real IP from X-Forwarded-For or RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	ip := r.RemoteAddr
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == ':' {
			return ip[:i]
		}
	}
	return ip
}

// timingSafeEqual wraps subtle.ConstantTimeCompare for string passwords.
func timingSafeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// apiPasswordFormHTML is a minimal styled page shown when a protected API
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

// requirePassword checks the ?password= query parameter against the dashboard
// password using a timing-safe comparison. If missing or wrong it writes the
// password form to w and returns false. On correct password it returns true.
func requirePassword(w http.ResponseWriter, r *http.Request) bool {
	pw := r.URL.Query().Get("password")
	if pw == "" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(apiPasswordFormHTML))
		return false
	}
	if !timingSafeEqual(pw, dashboardPassword) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(apiPasswordFormHTML))
		return false
	}
	return true
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Anti-clickjacking
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy",
		"default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: 'self'; connect-src 'self'")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(dashboardHTML))
}

// handleProxiesAPI serves the proxy list as JSON.
//
// Security:
//   - Rate-limited: 5 failed attempts per IP → 60s lockout
//   - Timing-safe password compare via crypto/subtle
//   - Anti-clickjack + CORS headers
//
// Response:
//
//	{
//	  "total": 80, "available": 65, "blocked": 15,
//	  "lockout": 0,          // seconds remaining if locked out (else 0)
//	  "attemptsLeft": 5,     // attempts before lockout
//	  "proxies": [...]
//	}
func handleProxiesAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")

	ip := clientIP(r)

	// Check rate limit first
	allowed, retryAfter := checkRateLimit(ip)
	if !allowed {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":   "too many attempts — locked out",
			"lockout": retryAfter,
		})
		return
	}

	password := r.URL.Query().Get("password")
	if !timingSafeEqual(password, dashboardPassword) {
		recordFailedLogin(ip)
		// Re-check to get updated remaining attempts
		_, remaining := checkRateLimit(ip)
		loginMu.Lock()
		left := 0
		if a, ok := loginAttempts[ip]; ok {
			left = maxLoginAttempts - a.count
			if left < 0 {
				left = 0
			}
		}
		loginMu.Unlock()

		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":        "unauthorized — invalid or missing password",
			"lockout":      remaining,
			"attemptsLeft": left,
		})
		return
	}

	// Password correct — reset rate limit
	resetLoginAttempts(ip)

	total, avail, blocked := ProxyStats()

	proxies := make([]map[string]interface{}, 0, len(residentialProxies))

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

	sort.Slice(proxies, func(i, j int) bool {
		si := proxies[i]["status"].(string)
		sj := proxies[j]["status"].(string)
		if si != sj {
			return si == "available"
		}
		return proxies[i]["ip"].(string) < proxies[j]["ip"].(string)
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"total":        total,
		"available":    avail,
		"blocked":      blocked,
		"attemptsLeft": maxLoginAttempts,
		"lockout":      0,
		"proxies":      proxies,
	})
}

// ============================================================================
// Dashboard HTML — PHANTOM Edition
// ============================================================================

var dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow">
<title>ICEbot · PHANTOM Dashboard</title>
<style>
/* ============================================================
   RESET
   ============================================================ */
*,*::before,*::after{margin:0;padding:0;box-sizing:border-box}
html{font-size:16px;scroll-behavior:smooth;-webkit-font-smoothing:antialiased}

/* ============================================================
   DESIGN TOKENS
   ============================================================ */
:root{
  /* Background layers */
  --void:      #020408;
  --base:      #050a14;
  --surface:   #080f1e;
  --raised:    #0c1628;
  --elevated:  #111d35;

  /* Accent system — cyan/teal primary, gold secondary */
  --cyan:      #00e5ff;
  --cyan-dim:  #0099bb;
  --cyan-glow: rgba(0,229,255,0.15);
  --cyan-soft: rgba(0,229,255,0.07);
  --teal:      #1de9b6;
  --teal-dim:  #0dba90;
  --gold:      #ffd740;
  --gold-dim:  #c9a400;
  --gold-glow: rgba(255,215,64,0.12);
  --red:       #ff1744;
  --red-dim:   #b71c1c;
  --red-glow:  rgba(255,23,68,0.12);
  --green:     #00e676;
  --green-dim: #00a84e;
  --green-glow:rgba(0,230,118,0.10);
  --amber:     #ffab00;
  --purple:    #d500f9;

  /* Text */
  --text-1: #e8f4ff;
  --text-2: #7aa8d2;
  --text-3: #3d6080;
  --text-4: #1e3550;

  /* Borders */
  --border-1: rgba(0,229,255,0.12);
  --border-2: rgba(0,229,255,0.22);
  --border-3: rgba(0,229,255,0.40);

  /* Typography */
  --font-display: 'Courier New',Courier,monospace;
  --font-body:    'Courier New',Courier,monospace;
  --font-mono:    'Courier New',Courier,monospace;

  /* Radius */
  --r-xs: 3px;
  --r-sm: 6px;
  --r-md: 10px;
  --r-lg: 16px;

  /* Shadows / glow */
  --shadow-cyan: 0 0 24px rgba(0,229,255,0.18), 0 0 2px rgba(0,229,255,0.6);
  --shadow-gold: 0 0 24px rgba(255,215,64,0.18), 0 0 2px rgba(255,215,64,0.6);
  --shadow-card: 0 4px 40px rgba(0,0,0,0.6);

  /* Transitions */
  --t-fast: 0.15s cubic-bezier(0.4,0,0.2,1);
  --t-med:  0.3s  cubic-bezier(0.4,0,0.2,1);
  --t-slow: 0.6s  cubic-bezier(0.4,0,0.2,1);
}

/* ============================================================
   BASE
   ============================================================ */
body{
  font-family:var(--font-body);
  background:var(--void);
  color:var(--text-1);
  min-height:100vh;
  display:flex;
  flex-direction:column;
  overflow-x:hidden;
  cursor:none;
}

a{color:var(--cyan);text-decoration:none}
a:hover{color:var(--text-1)}

::selection{background:rgba(0,229,255,0.25);color:#fff}

::-webkit-scrollbar{width:4px;height:4px}
::-webkit-scrollbar-track{background:var(--void)}
::-webkit-scrollbar-thumb{background:var(--border-2);border-radius:2px}
::-webkit-scrollbar-thumb:hover{background:var(--border-3)}

/* ============================================================
   CUSTOM CURSOR
   ============================================================ */
#cursor{
  position:fixed;width:12px;height:12px;
  border:1.5px solid var(--cyan);
  border-radius:50%;pointer-events:none;
  transform:translate(-50%,-50%);
  z-index:99999;transition:transform var(--t-fast),border-color var(--t-fast);
  mix-blend-mode:screen;
}
#cursor-ring{
  position:fixed;width:36px;height:36px;
  border:1px solid rgba(0,229,255,0.3);
  border-radius:50%;pointer-events:none;
  transform:translate(-50%,-50%);
  z-index:99998;
  transition:width 0.2s,height 0.2s,border-color 0.2s;
}
body.cursor-click #cursor{transform:translate(-50%,-50%) scale(0.5);border-color:var(--gold)}
body.cursor-hover #cursor{transform:translate(-50%,-50%) scale(1.5);border-color:var(--teal)}
body.cursor-hover #cursor-ring{width:48px;height:48px;border-color:rgba(29,233,182,0.3)}

/* ============================================================
   CANVAS LAYERS
   ============================================================ */
#matrix-canvas{
  position:fixed;top:0;left:0;width:100%;height:100%;
  z-index:-3;opacity:0.35;pointer-events:none;
}
#grid-canvas{
  position:fixed;top:0;left:0;width:100%;height:100%;
  z-index:-2;opacity:0.5;pointer-events:none;
}

/* Scanline overlay */
.scanlines{
  position:fixed;top:0;left:0;width:100%;height:100%;
  z-index:-1;pointer-events:none;
  background:repeating-linear-gradient(
    0deg,
    transparent 0,transparent 2px,
    rgba(0,0,0,0.08) 2px,rgba(0,0,0,0.08) 4px
  );
  animation:scanline-drift 12s linear infinite;
}
@keyframes scanline-drift{
  0%{background-position:0 0}
  100%{background-position:0 100vh}
}

/* Vignette */
.vignette{
  position:fixed;top:0;left:0;width:100%;height:100%;
  z-index:0;pointer-events:none;
  background:radial-gradient(ellipse at center,transparent 40%,rgba(2,4,8,0.7) 100%);
}

/* ============================================================
   GLITCH ANIMATION
   ============================================================ */
@keyframes glitch-1{
  0%,100%{clip-path:inset(0 0 95% 0);transform:translate(-2px,0)}
  25%{clip-path:inset(40% 0 50% 0);transform:translate(2px,0)}
  50%{clip-path:inset(80% 0 5% 0);transform:translate(-1px,0)}
  75%{clip-path:inset(10% 0 85% 0);transform:translate(1px,0)}
}
@keyframes glitch-2{
  0%,100%{clip-path:inset(50% 0 30% 0);transform:translate(2px,0)}
  25%{clip-path:inset(5% 0 90% 0);transform:translate(-2px,0)}
  50%{clip-path:inset(65% 0 20% 0);transform:translate(1px,0)}
  75%{clip-path:inset(85% 0 8% 0);transform:translate(-1px,0)}
}
.glitch{position:relative;display:inline-block}
.glitch::before,.glitch::after{
  content:attr(data-text);
  position:absolute;top:0;left:0;
  color:inherit;background:transparent;
}
.glitch::before{color:var(--cyan);animation:glitch-1 4s infinite;opacity:0.7}
.glitch::after{color:var(--red);animation:glitch-2 4s infinite 0.1s;opacity:0.7}

/* ============================================================
   TYPING CURSOR
   ============================================================ */
@keyframes blink{0%,100%{opacity:1}50%{opacity:0}}
.type-cursor::after{
  content:'_';color:var(--cyan);
  animation:blink 1.1s step-end infinite;
  margin-left:1px;font-size:inherit;
}

/* ============================================================
   ANIMATED BORDERS
   ============================================================ */
@keyframes border-flow{
  0%{background-position:0% 50%}
  50%{background-position:100% 50%}
  100%{background-position:0% 50%}
}
.border-animated{
  position:relative;
}
.border-animated::before{
  content:'';position:absolute;inset:-1px;
  border-radius:inherit;
  background:linear-gradient(135deg,var(--cyan),var(--teal),var(--purple),var(--cyan));
  background-size:400% 400%;
  animation:border-flow 4s ease infinite;
  z-index:-1;
  opacity:0;transition:opacity var(--t-med);
}
.border-animated:hover::before{opacity:1}

/* ============================================================
   CORNER DECORATIONS
   ============================================================ */
.hud-corners{position:relative}
.hud-corners::before,.hud-corners::after{
  content:'';position:absolute;
  width:14px;height:14px;
  border-color:var(--cyan);
  border-style:solid;
  opacity:0.6;
}
.hud-corners::before{top:0;left:0;border-width:2px 0 0 2px;border-radius:2px 0 0 0}
.hud-corners::after{bottom:0;right:0;border-width:0 2px 2px 0;border-radius:0 0 2px 0}
.hud-corners-tr::before{top:0;right:0;left:auto;border-width:2px 2px 0 0;border-radius:0 2px 0 0}
.hud-corners-bl::after{bottom:0;left:0;right:auto;border-width:0 0 2px 2px;border-radius:0 0 0 2px}

/* ============================================================
   HEADER
   ============================================================ */
.header{
  position:sticky;top:0;z-index:200;
  background:linear-gradient(180deg,rgba(5,10,20,0.98),rgba(5,10,20,0.85));
  border-bottom:1px solid var(--border-1);
  backdrop-filter:blur(20px);
  -webkit-backdrop-filter:blur(20px);
  padding:0 28px;
  height:64px;
  display:flex;align-items:center;
}
.header-inner{
  max-width:1320px;margin:0 auto;
  width:100%;
  display:flex;align-items:center;
  justify-content:space-between;gap:16px;
  flex-wrap:wrap;
}

/* Logo mark */
.logo-mark{
  width:40px;height:40px;flex-shrink:0;
  border:1px solid var(--border-2);
  border-radius:var(--r-sm);
  background:linear-gradient(135deg,rgba(0,229,255,0.08),rgba(0,0,0,0));
  display:flex;align-items:center;justify-content:center;
  box-shadow:var(--shadow-cyan);
  position:relative;overflow:hidden;
}
.logo-mark::after{
  content:'';position:absolute;inset:0;
  background:linear-gradient(135deg,transparent 40%,rgba(0,229,255,0.08));
}
.logo-mark svg{position:relative;z-index:1}

.brand-name{
  font-size:1.3rem;font-weight:700;
  letter-spacing:0.1em;text-transform:uppercase;
  color:var(--text-1);line-height:1;
}
.brand-sub{
  font-size:0.6rem;color:var(--text-3);
  letter-spacing:0.25em;text-transform:uppercase;
  margin-top:3px;
}
.brand-sub .author{
  color:var(--gold);font-style:normal;
  letter-spacing:0.15em;
}

/* Status pill */
.status-pill{
  display:flex;align-items:center;gap:7px;
  padding:5px 14px;border-radius:20px;
  font-size:0.68rem;font-weight:700;
  letter-spacing:0.15em;text-transform:uppercase;
}
.status-pill.online{
  background:rgba(0,230,118,0.08);
  border:1px solid rgba(0,230,118,0.2);
  color:var(--green);
}
.status-pill.offline{
  background:var(--red-glow);
  border:1px solid rgba(255,23,68,0.2);
  color:var(--red);
}
.status-pill.degraded{
  background:var(--gold-glow);
  border:1px solid rgba(255,215,64,0.2);
  color:var(--gold);
}
.status-dot{
  width:6px;height:6px;border-radius:50%;
  background:currentColor;flex-shrink:0;
}
.status-dot.pulse{animation:status-pulse 2s ease-in-out infinite}
@keyframes status-pulse{
  0%,100%{opacity:1;box-shadow:0 0 0 0 currentColor}
  50%{opacity:0.7;box-shadow:0 0 0 4px transparent}
}

/* Uptime display */
.uptime-badge{
  font-family:var(--font-mono);
  font-size:0.7rem;color:var(--text-3);
  border:1px solid var(--border-1);
  padding:4px 12px;border-radius:var(--r-xs);
  background:rgba(0,229,255,0.02);
  letter-spacing:0.05em;
}
.uptime-badge .uptime-val{color:var(--text-2)}

/* ============================================================
   MAIN CONTENT
   ============================================================ */
.main{
  max-width:1320px;margin:0 auto;
  padding:28px 28px 60px;
  flex:1;width:100%;
  position:relative;z-index:1;
}

/* ============================================================
   SECTION HEADER
   ============================================================ */
.section-label{
  display:flex;align-items:center;gap:10px;
  font-size:0.62rem;font-weight:700;
  letter-spacing:0.25em;text-transform:uppercase;
  color:var(--text-3);margin-bottom:16px;
}
.section-label::before{
  content:'';width:16px;height:1px;
  background:linear-gradient(90deg,var(--cyan),transparent);
}
.section-label::after{
  content:'';flex:1;height:1px;
  background:linear-gradient(90deg,var(--border-1),transparent);
}

/* ============================================================
   STAT CARDS
   ============================================================ */
.stats-grid{
  display:grid;
  grid-template-columns:repeat(auto-fill,minmax(170px,1fr));
  gap:12px;
  margin-bottom:28px;
}

.stat-card{
  background:var(--surface);
  border:1px solid var(--border-1);
  border-radius:var(--r-md);
  padding:18px 16px 14px;
  position:relative;overflow:hidden;
  transition:transform var(--t-med),border-color var(--t-med),box-shadow var(--t-med);
  cursor:default;
}
.stat-card:hover{
  transform:translateY(-3px);
  border-color:var(--border-2);
  box-shadow:0 8px 40px rgba(0,0,0,0.4),0 0 20px rgba(0,229,255,0.06);
}

/* Top accent bar */
.stat-card::after{
  content:'';position:absolute;
  top:0;left:0;right:0;height:2px;
  background:linear-gradient(90deg,transparent 0%,var(--cyan) 50%,transparent 100%);
  opacity:0.4;
  transition:opacity var(--t-med);
}
.stat-card:hover::after{opacity:0.8}

/* Background glow blob */
.stat-card .card-bg-glow{
  position:absolute;bottom:-20px;right:-20px;
  width:80px;height:80px;border-radius:50%;
  opacity:0.06;pointer-events:none;
  transition:opacity var(--t-med);
}
.stat-card:hover .card-bg-glow{opacity:0.12}

.stat-icon-wrap{
  width:28px;height:28px;
  border-radius:var(--r-xs);
  background:rgba(0,229,255,0.06);
  border:1px solid rgba(0,229,255,0.1);
  display:flex;align-items:center;justify-content:center;
  margin-bottom:12px;
}
.stat-icon-wrap svg{display:block}

.stat-num{
  font-size:2rem;font-weight:900;
  line-height:1;
  font-variant-numeric:tabular-nums;
  letter-spacing:-0.03em;
  margin-bottom:6px;
}
/* color variants */
.stat-num.c-cyan{color:var(--cyan)}
.stat-num.c-teal{color:var(--teal)}
.stat-num.c-green{color:var(--green)}
.stat-num.c-gold{color:var(--gold)}
.stat-num.c-red{color:var(--red)}
.stat-num.c-purple{color:var(--purple)}
.stat-num.c-default{
  background:linear-gradient(135deg,var(--text-1),var(--text-2));
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;
  background-clip:text;
}

.stat-label{
  font-size:0.6rem;color:var(--text-3);
  text-transform:uppercase;letter-spacing:0.2em;
  font-weight:700;
}
.stat-sub{
  font-size:0.6rem;color:var(--text-4);
  margin-top:4px;
  letter-spacing:0.05em;
}

/* Flash animation on value change */
@keyframes stat-flash{
  0%{filter:brightness(2) drop-shadow(0 0 8px currentColor)}
  100%{filter:none}
}
.stat-num.flash{animation:stat-flash 0.5s ease}

/* ============================================================
   CHARTS ROW
   ============================================================ */
.charts-row{
  display:grid;
  grid-template-columns:1fr 1fr 260px;
  gap:12px;
  margin-bottom:28px;
}
@media(max-width:900px){
  .charts-row{grid-template-columns:1fr 1fr}
}
@media(max-width:600px){
  .charts-row{grid-template-columns:1fr}
}

.chart-card{
  background:var(--surface);
  border:1px solid var(--border-1);
  border-radius:var(--r-md);
  padding:18px;
  position:relative;overflow:hidden;
}
.chart-card .card-title{
  font-size:0.62rem;text-transform:uppercase;
  letter-spacing:0.2em;color:var(--text-3);
  font-weight:700;margin-bottom:14px;
  display:flex;align-items:center;justify-content:space-between;
}
.chart-card .card-title .title-dot{
  width:6px;height:6px;border-radius:50%;
  background:var(--cyan);
  animation:status-pulse 2s ease-in-out infinite;
}

.chart-canvas-wrap{
  position:relative;
  height:100px;
}
.chart-canvas-wrap canvas{width:100%!important;height:100%!important}

/* Proxy health ring card */
.ring-card{
  background:var(--surface);
  border:1px solid var(--border-1);
  border-radius:var(--r-md);
  padding:18px;
  display:flex;flex-direction:column;
  align-items:center;justify-content:center;
  position:relative;overflow:hidden;
}
.ring-wrap{
  position:relative;
  width:140px;height:140px;
  flex-shrink:0;
}
.ring-canvas{display:block;width:140px;height:140px}
.ring-center{
  position:absolute;inset:0;
  display:flex;flex-direction:column;
  align-items:center;justify-content:center;
}
.ring-pct{
  font-size:1.8rem;font-weight:900;
  color:var(--cyan);
  line-height:1;
  font-variant-numeric:tabular-nums;
}
.ring-pct-label{
  font-size:0.55rem;text-transform:uppercase;
  letter-spacing:0.15em;color:var(--text-3);
  margin-top:3px;font-weight:700;
}
.ring-legend{
  margin-top:14px;width:100%;
}
.ring-legend-row{
  display:flex;align-items:center;justify-content:space-between;
  font-size:0.68rem;
  padding:3px 0;
  border-bottom:1px solid var(--border-1);
}
.ring-legend-row:last-child{border-bottom:none}
.ring-legend-dot{
  width:8px;height:8px;border-radius:50%;flex-shrink:0;
}
.ring-legend-name{color:var(--text-2);margin-left:6px;flex:1}
.ring-legend-val{
  font-family:var(--font-mono);
  font-size:0.72rem;color:var(--text-1);
  font-variant-numeric:tabular-nums;
}

/* ============================================================
   PROXY SECTION
   ============================================================ */
.proxy-section{
  background:var(--surface);
  border:1px solid var(--border-1);
  border-radius:var(--r-lg);
  overflow:hidden;
  box-shadow:var(--shadow-card);
  margin-bottom:28px;
  position:relative;
}

/* Lock panel */
.lock-panel{
  padding:60px 40px;
  text-align:center;
  position:relative;
}
.lock-hexagon{
  width:80px;height:80px;
  margin:0 auto 24px;
  position:relative;
  display:flex;align-items:center;justify-content:center;
}
.lock-hexagon svg{display:block}
.lock-hexagon .hex-rings{
  position:absolute;inset:-16px;
  animation:hex-rotate 8s linear infinite;
  opacity:0.4;
}
@keyframes hex-rotate{
  0%{transform:rotate(0deg)}100%{transform:rotate(360deg)}
}
.lock-title{
  font-size:1.1rem;font-weight:700;
  color:var(--text-1);margin-bottom:8px;
  letter-spacing:0.05em;
}
.lock-desc{
  font-size:0.75rem;color:var(--text-3);
  max-width:380px;margin:0 auto 28px;
  line-height:1.7;
}
.lock-desc strong{color:var(--text-2)}

/* Password form */
.pw-form{
  display:flex;gap:10px;justify-content:center;flex-wrap:wrap;
  max-width:420px;margin:0 auto;
}
.pw-input{
  background:rgba(2,4,8,0.8);
  border:1px solid var(--border-2);
  border-radius:var(--r-sm);
  padding:11px 18px;
  color:var(--text-1);
  font-family:var(--font-mono);
  font-size:0.85rem;
  letter-spacing:0.15em;
  outline:none;
  width:220px;
  transition:border-color var(--t-fast),box-shadow var(--t-fast);
}
.pw-input:focus{
  border-color:var(--cyan);
  box-shadow:0 0 0 3px rgba(0,229,255,0.1),var(--shadow-cyan);
}
.pw-input::placeholder{
  color:var(--text-4);letter-spacing:0.05em;
  font-family:var(--font-body);
}

.unlock-btn{
  background:linear-gradient(135deg,rgba(0,229,255,0.12),rgba(0,229,255,0.06));
  border:1px solid var(--border-2);
  border-radius:var(--r-sm);
  padding:11px 28px;
  color:var(--cyan);
  font-family:var(--font-body);
  font-size:0.8rem;
  font-weight:700;
  letter-spacing:0.15em;
  text-transform:uppercase;
  cursor:pointer;
  transition:all var(--t-med);
  position:relative;overflow:hidden;
}
.unlock-btn::before{
  content:'';position:absolute;inset:0;
  background:linear-gradient(135deg,rgba(0,229,255,0.15),transparent);
  opacity:0;transition:opacity var(--t-fast);
}
.unlock-btn:hover{
  border-color:var(--cyan);
  box-shadow:var(--shadow-cyan);
  color:var(--text-1);
}
.unlock-btn:hover::before{opacity:1}
.unlock-btn:active{transform:scale(0.96)}
.unlock-btn:disabled{
  opacity:0.4;cursor:not-allowed;
  pointer-events:none;
}

/* Security messages */
.lock-msg{
  font-size:0.72rem;
  min-height:1.5em;margin-top:12px;
  letter-spacing:0.05em;
}
.lock-msg.error{color:var(--red)}
.lock-msg.info{color:var(--gold)}
.lock-msg.success{color:var(--green)}

/* Attempt dots */
.attempt-dots{
  display:flex;gap:6px;justify-content:center;margin-top:14px;
}
.attempt-dot{
  width:8px;height:8px;border-radius:50%;
  background:var(--border-2);
  transition:background var(--t-fast);
}
.attempt-dot.used{background:var(--red)}
.attempt-dot.ok{background:var(--green)}

/* Lockout countdown */
.lockout-overlay{
  display:none;
  position:absolute;inset:0;
  background:rgba(2,4,8,0.94);
  z-index:10;
  align-items:center;justify-content:center;
  flex-direction:column;
  gap:16px;
  backdrop-filter:blur(4px);
}
.lockout-overlay.active{display:flex}
.lockout-count{
  font-size:4rem;font-weight:900;
  color:var(--red);
  font-variant-numeric:tabular-nums;
  text-shadow:0 0 30px var(--red);
}
.lockout-label{
  font-size:0.7rem;text-transform:uppercase;
  letter-spacing:0.3em;color:var(--text-3);
}

/* ============================================================
   PROXY CONTENT (unlocked)
   ============================================================ */
.proxy-content{display:none}
.proxy-content.visible{display:block}

.proxy-topbar{
  padding:16px 20px 0;
  display:flex;align-items:center;
  justify-content:space-between;
  flex-wrap:wrap;gap:12px;
  border-bottom:1px solid var(--border-1);
  padding-bottom:14px;
}
.proxy-stats-row{
  display:flex;align-items:center;gap:16px;
  font-size:0.72rem;
}
.proxy-stat-item{
  display:flex;align-items:center;gap:6px;
}
.psb{width:8px;height:8px;border-radius:50%}
.psb-green{background:var(--green);box-shadow:0 0 6px var(--green)}
.psb-red{background:var(--red);box-shadow:0 0 6px var(--red)}
.psb-total{background:var(--cyan);box-shadow:0 0 6px var(--cyan)}
.proxy-stat-num{
  font-weight:700;color:var(--text-1);
  font-variant-numeric:tabular-nums;
}
.proxy-stat-name{color:var(--text-3)}

/* Toolbar */
.proxy-toolbar{
  padding:12px 20px;
  display:flex;align-items:center;
  gap:10px;flex-wrap:wrap;
  border-bottom:1px solid var(--border-1);
}
.proxy-search{
  flex:1;min-width:180px;
  background:rgba(2,4,8,0.7);
  border:1px solid var(--border-1);
  border-radius:var(--r-sm);
  padding:8px 14px;
  color:var(--text-1);
  font-family:var(--font-mono);
  font-size:0.78rem;
  outline:none;
  transition:border-color var(--t-fast);
}
.proxy-search:focus{border-color:var(--cyan);box-shadow:0 0 0 2px rgba(0,229,255,0.08)}
.proxy-search::placeholder{color:var(--text-4);font-family:var(--font-body)}

.filter-btn{
  background:transparent;
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  padding:7px 14px;
  color:var(--text-3);
  font-size:0.68rem;font-weight:700;
  letter-spacing:0.1em;text-transform:uppercase;
  cursor:pointer;
  transition:all var(--t-fast);
  font-family:var(--font-body);
}
.filter-btn:hover{border-color:var(--border-2);color:var(--text-2)}
.filter-btn.active{
  background:rgba(0,229,255,0.08);
  border-color:var(--cyan);color:var(--cyan);
}
.filter-btn.active-green{
  background:rgba(0,230,118,0.08);
  border-color:rgba(0,230,118,0.4);color:var(--green);
}
.filter-btn.active-red{
  background:var(--red-glow);
  border-color:rgba(255,23,68,0.4);color:var(--red);
}

.tb-gap{flex:1}

.icon-btn{
  background:transparent;
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  padding:7px 12px;
  color:var(--text-3);
  font-size:0.7rem;font-weight:700;
  letter-spacing:0.08em;
  cursor:pointer;
  transition:all var(--t-fast);
  display:flex;align-items:center;gap:6px;
  font-family:var(--font-body);
  text-transform:uppercase;
}
.icon-btn:hover{border-color:var(--border-2);color:var(--text-2)}
.icon-btn svg{flex-shrink:0}

/* Auto-refresh toggle */
.ar-toggle{
  display:flex;align-items:center;gap:7px;
  font-size:0.65rem;color:var(--text-3);
  text-transform:uppercase;letter-spacing:0.1em;
  cursor:pointer;user-select:none;
}
.ar-toggle input{display:none}
.ar-track{
  width:32px;height:17px;
  background:var(--raised);
  border:1px solid var(--border-1);
  border-radius:9px;position:relative;
  transition:background var(--t-fast);
}
.ar-track::after{
  content:'';
  position:absolute;top:2px;left:2px;
  width:11px;height:11px;border-radius:50%;
  background:var(--text-3);
  transition:left var(--t-fast),background var(--t-fast);
}
.ar-toggle input:checked+.ar-track{
  background:rgba(0,229,255,0.15);
  border-color:rgba(0,229,255,0.3);
}
.ar-toggle input:checked+.ar-track::after{
  left:17px;background:var(--cyan);
}

/* ============================================================
   PROXY GRID
   ============================================================ */
.proxy-grid-wrap{padding:16px 20px 20px}

.proxy-grid{
  display:grid;
  grid-template-columns:repeat(auto-fill,minmax(280px,1fr));
  gap:6px;
}

.proxy-card{
  background:rgba(2,4,8,0.5);
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  padding:9px 14px;
  display:flex;align-items:center;justify-content:space-between;
  font-family:var(--font-mono);
  font-size:0.77rem;
  transition:all var(--t-fast);
  cursor:pointer;
  position:relative;overflow:hidden;
}
.proxy-card:hover{
  background:rgba(0,229,255,0.04);
  border-color:var(--border-2);
  transform:translateX(4px);
}
.proxy-card:active{transform:translateX(2px) scale(0.99)}
.proxy-card::before{
  content:'';position:absolute;left:0;top:0;bottom:0;
  width:2px;opacity:0;
  transition:opacity var(--t-fast);
}
.proxy-card.avail::before{background:var(--green);box-shadow:0 0 8px var(--green)}
.proxy-card.blocked::before{background:var(--red);box-shadow:0 0 8px var(--red)}
.proxy-card:hover::before{opacity:1}

.proxy-addr{color:var(--text-2);letter-spacing:0.03em}
.proxy-port{color:var(--text-3)}
.proxy-colon{color:var(--text-4)}

.proxy-status-tag{
  font-size:0.55rem;font-weight:700;
  padding:2px 8px;border-radius:3px;
  text-transform:uppercase;letter-spacing:0.12em;
  font-family:var(--font-body);
  flex-shrink:0;
}
.tag-avail{
  background:rgba(0,230,118,0.1);
  border:1px solid rgba(0,230,118,0.2);
  color:var(--green);
}
.tag-blocked{
  background:rgba(255,23,68,0.1);
  border:1px solid rgba(255,23,68,0.2);
  color:var(--red);
}

/* Copied flash */
@keyframes copy-flash{
  0%{background:rgba(0,229,255,0.15)}
  100%{background:rgba(2,4,8,0.5)}
}
.proxy-card.copied{animation:copy-flash 0.6s ease}

/* Empty state */
.proxy-empty{
  grid-column:1/-1;
  padding:48px 20px;text-align:center;
  color:var(--text-4);font-size:0.78rem;
}
.proxy-empty .empty-icon{
  font-size:2rem;opacity:0.3;
  display:block;margin-bottom:12px;
}

/* Skeleton shimmer */
@keyframes shimmer{
  0%{background-position:-400px 0}
  100%{background-position:400px 0}
}
.proxy-skel{
  height:38px;border-radius:var(--r-xs);
  background:linear-gradient(90deg,var(--raised) 0%,var(--elevated) 50%,var(--raised) 100%);
  background-size:400px 100%;
  animation:shimmer 1.5s infinite;
}

/* ============================================================
   SYSTEM INFO CARD
   ============================================================ */
.sys-card{
  background:var(--surface);
  border:1px solid var(--border-1);
  border-radius:var(--r-lg);
  padding:20px 24px;
  margin-bottom:28px;
  overflow:hidden;
  position:relative;
}
.sys-grid{
  display:grid;
  grid-template-columns:repeat(auto-fill,minmax(200px,1fr));
  gap:0;
}
.sys-item{
  padding:8px 16px;
  display:flex;align-items:center;justify-content:space-between;
  border-right:1px solid var(--border-1);
  border-bottom:1px solid var(--border-1);
  font-size:0.7rem;
}
.sys-item:last-child{border-right:none}
.sys-key{color:var(--text-3);text-transform:uppercase;letter-spacing:0.12em}
.sys-val{
  color:var(--text-2);
  font-family:var(--font-mono);font-size:0.68rem;
  text-align:right;
}
.sys-val.author-val{
  color:var(--gold);
  font-style:italic;
  font-family:'Georgia',serif;
}

/* ============================================================
   API REFERENCE BAR
   ============================================================ */
.api-bar{
  display:flex;flex-wrap:wrap;gap:8px;
  margin-bottom:28px;
}
.api-chip{
  display:inline-flex;align-items:center;gap:7px;
  background:rgba(0,229,255,0.03);
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  padding:7px 14px;
  font-size:0.7rem;color:var(--text-3);
  font-family:var(--font-mono);
  cursor:pointer;
  transition:all var(--t-fast);
}
.api-chip:hover{border-color:var(--border-2);color:var(--text-2);background:rgba(0,229,255,0.05)}
.api-chip .method{
  font-size:0.55rem;font-weight:900;font-family:var(--font-body);
  padding:1px 5px;border-radius:2px;
  background:rgba(0,230,118,0.12);
  color:var(--green);letter-spacing:0.08em;
}

/* ============================================================
   KBD HINTS
   ============================================================ */
.kbd-row{
  display:flex;flex-wrap:wrap;gap:8px;
  justify-content:center;margin:20px 0 0;
  font-size:0.62rem;color:var(--text-4);
  letter-spacing:0.08em;
}
.kbd-pair{display:flex;align-items:center;gap:5px}
kbd{
  display:inline-block;
  background:rgba(0,229,255,0.04);
  border:1px solid var(--border-1);
  border-bottom:2px solid var(--border-2);
  border-radius:3px;padding:1px 6px;
  font-size:0.6rem;font-family:var(--font-mono);
  color:var(--text-3);
}

/* ============================================================
   FOOTER
   ============================================================ */
.footer{
  text-align:center;
  padding:20px 28px;
  font-size:0.65rem;color:var(--text-4);
  border-top:1px solid rgba(0,229,255,0.04);
  letter-spacing:0.12em;text-transform:uppercase;
  position:relative;z-index:1;
}
.footer .sig{
  font-family:'Georgia',serif;
  font-style:italic;text-transform:none;
  color:rgba(255,215,64,0.4);letter-spacing:0.05em;
}

/* ============================================================
   TOAST
   ============================================================ */
#toast-root{
  position:fixed;bottom:28px;right:28px;
  z-index:9999;display:flex;flex-direction:column;
  gap:8px;pointer-events:none;
}
.toast{
  background:rgba(8,15,30,0.96);
  border:1px solid var(--border-1);
  border-radius:var(--r-xs);
  padding:11px 18px;
  font-size:0.75rem;color:var(--text-1);
  backdrop-filter:blur(12px);
  max-width:300px;pointer-events:auto;
  animation:toast-in 0.3s ease;
  position:relative;overflow:hidden;
}
.toast::after{
  content:'';position:absolute;
  bottom:0;left:0;height:2px;
  animation:toast-drain 3.5s linear forwards;
}
.toast.t-ok{border-left:2px solid var(--green)}
.toast.t-ok::after{background:var(--green)}
.toast.t-err{border-left:2px solid var(--red)}
.toast.t-err::after{background:var(--red)}
.toast.t-info{border-left:2px solid var(--cyan)}
.toast.t-info::after{background:var(--cyan)}
@keyframes toast-in{
  from{opacity:0;transform:translateX(40px)}
  to{opacity:1;transform:translateX(0)}
}
@keyframes toast-drain{
  from{width:100%}to{width:0}
}

/* ============================================================
   REFRESH SPINNER
   ============================================================ */
@keyframes spin{from{transform:rotate(0)}to{transform:rotate(360deg)}}
.spinning{animation:spin 0.6s linear infinite}

/* ============================================================
   FADE-IN-UP ENTRANCE
   ============================================================ */
@keyframes fade-up{
  from{opacity:0;transform:translateY(16px)}
  to{opacity:1;transform:translateY(0)}
}
.fade-up{animation:fade-up 0.5s ease both}
.stat-card:nth-child(1){animation-delay:0.04s}
.stat-card:nth-child(2){animation-delay:0.08s}
.stat-card:nth-child(3){animation-delay:0.12s}
.stat-card:nth-child(4){animation-delay:0.16s}
.stat-card:nth-child(5){animation-delay:0.20s}
.stat-card:nth-child(6){animation-delay:0.24s}
.stat-card:nth-child(7){animation-delay:0.28s}
.stat-card:nth-child(8){animation-delay:0.32s}

/* ============================================================
   PROGRESS BAR
   ============================================================ */
.mini-bar-wrap{
  height:3px;background:var(--raised);
  border-radius:2px;margin-top:8px;overflow:hidden;
}
.mini-bar{
  height:100%;border-radius:2px;
  transition:width 0.6s cubic-bezier(0.4,0,0.2,1);
}
.mini-bar.c-green{background:linear-gradient(90deg,var(--green-dim),var(--green))}
.mini-bar.c-red{background:linear-gradient(90deg,var(--red-dim),var(--red))}
.mini-bar.c-cyan{background:linear-gradient(90deg,var(--cyan-dim),var(--cyan))}

/* ============================================================
   TIMESTAMP
   ============================================================ */
.ts-row{
  font-size:0.62rem;color:var(--text-4);
  text-align:right;margin-top:8px;
  letter-spacing:0.06em;
  font-family:var(--font-mono);
}

/* ============================================================
   RESPONSIVE
   ============================================================ */
@media(max-width:720px){
  .header{padding:0 16px;height:54px}
  .main{padding:16px 16px 48px}
  .stats-grid{grid-template-columns:repeat(2,1fr);gap:8px}
  .stat-num{font-size:1.6rem}
  .lock-panel{padding:40px 20px}
  .proxy-grid{grid-template-columns:1fr}
}

/* ============================================================
   PRINT
   ============================================================ */
@media print{
  #matrix-canvas,#grid-canvas,.scanlines,.vignette,
  #cursor,#cursor-ring,.unlock-btn,input{display:none!important}
  body{background:#fff;color:#000}
}
</style>
</head>
<body>

<!-- Custom cursor -->
<div id="cursor"></div>
<div id="cursor-ring"></div>

<!-- Background layers -->
<canvas id="matrix-canvas"></canvas>
<canvas id="grid-canvas"></canvas>
<div class="scanlines"></div>
<div class="vignette"></div>

<!-- Toast container -->
<div id="toast-root"></div>

<!-- ================================================================
     HEADER
     ================================================================ -->
<header class="header">
  <div class="header-inner">
    <div style="display:flex;align-items:center;gap:14px">
      <div class="logo-mark">
        <svg width="22" height="22" viewBox="0 0 22 22" fill="none">
          <polygon points="11,2 20,7 20,15 11,20 2,15 2,7" stroke="#00e5ff" stroke-width="1.2" fill="rgba(0,229,255,0.06)"/>
          <polygon points="11,6 16,9 16,14 11,17 6,14 6,9" stroke="#1de9b6" stroke-width="0.8" fill="rgba(29,233,182,0.04)"/>
          <circle cx="11" cy="11" r="2" fill="#00e5ff" opacity="0.8"/>
        </svg>
      </div>
      <div>
        <div class="brand-name glitch" data-text="ICEbot">ICEbot</div>
        <div class="brand-sub">
          PHANTOM DASHBOARD · by <span class="author">Saif</span>
        </div>
      </div>
    </div>

    <div style="display:flex;align-items:center;gap:12px;flex-wrap:wrap">
      <div class="uptime-badge">
        UP <span class="uptime-val" id="uptimeDisplay">0s</span>
      </div>
      <div class="status-pill online" id="statusPill">
        <span class="status-dot pulse" id="statusDot"></span>
        <span id="statusLabel">RUNNING</span>
      </div>
    </div>
  </div>
</header>

<!-- ================================================================
     MAIN
     ================================================================ -->
<main class="main">

  <!-- STATS GRID -------------------------------------------------- -->
  <div class="section-label">Live Telemetry</div>

  <div class="stats-grid" id="statsGrid">
    <!-- card 0: sessions -->
    <div class="stat-card hud-corners fade-up">
      <div style="background:rgba(0,229,255,0.1)" class="card-bg-glow"></div>
      <div class="stat-icon-wrap">
        <svg width="16" height="16" fill="none" stroke="#00e5ff" stroke-width="1.5" viewBox="0 0 16 16">
          <rect x="1" y="2" width="14" height="11" rx="1.5"/>
          <path d="M5 13v1M11 13v1M3 15h10"/>
        </svg>
      </div>
      <div class="stat-num c-cyan type-cursor" id="statSessions" style="font-family:var(--font-mono)">0</div>
      <div class="stat-label">Sessions</div>
      <div class="mini-bar-wrap"><div class="mini-bar c-cyan" id="barSessions" style="width:0%"></div></div>
    </div>

    <!-- card 1: total bots -->
    <div class="stat-card hud-corners fade-up">
      <div style="background:rgba(29,233,182,0.1)" class="card-bg-glow"></div>
      <div class="stat-icon-wrap">
        <svg width="16" height="16" fill="none" stroke="#1de9b6" stroke-width="1.5" viewBox="0 0 16 16">
          <circle cx="8" cy="5" r="2.5"/>
          <path d="M2 14c0-3.3 2.7-6 6-6s6 2.7 6 6"/>
        </svg>
      </div>
      <div class="stat-num c-teal" id="statBots" style="font-family:var(--font-mono)">0</div>
      <div class="stat-label">Total Bots</div>
      <div class="mini-bar-wrap"><div class="mini-bar c-cyan" id="barBots" style="width:0%"></div></div>
    </div>

    <!-- card 2: joined bots -->
    <div class="stat-card hud-corners fade-up">
      <div style="background:rgba(0,230,118,0.1)" class="card-bg-glow"></div>
      <div class="stat-icon-wrap">
        <svg width="16" height="16" fill="none" stroke="#00e676" stroke-width="1.5" viewBox="0 0 16 16">
          <path d="M3 8l3.5 3.5L13 4"/>
        </svg>
      </div>
      <div class="stat-num c-green" id="statJoined" style="font-family:var(--font-mono)">0</div>
      <div class="stat-label">Joined</div>
      <div class="stat-sub" id="joinedPct">0% success rate</div>
      <div class="mini-bar-wrap"><div class="mini-bar c-green" id="barJoined" style="width:0%"></div></div>
    </div>

    <!-- card 3: total proxies -->
    <div class="stat-card hud-corners fade-up">
      <div style="background:rgba(0,229,255,0.08)" class="card-bg-glow"></div>
      <div class="stat-icon-wrap">
        <svg width="16" height="16" fill="none" stroke="#00e5ff" stroke-width="1.5" viewBox="0 0 16 16">
          <rect x="1" y="1" width="14" height="14" rx="2"/>
          <circle cx="8" cy="8" r="3.5"/>
          <circle cx="8" cy="8" r="1" fill="#00e5ff"/>
        </svg>
      </div>
      <div class="stat-num c-default" id="statProxies" style="font-family:var(--font-mono)">0</div>
      <div class="stat-label">Proxies</div>
      <div class="stat-sub">Pool size</div>
    </div>

    <!-- card 4: available proxies -->
    <div class="stat-card hud-corners fade-up">
      <div style="background:rgba(0,230,118,0.08)" class="card-bg-glow"></div>
      <div class="stat-icon-wrap">
        <svg width="16" height="16" fill="none" stroke="#00e676" stroke-width="1.5" viewBox="0 0 16 16">
          <path d="M8 3v2M8 11v2M3 8h2M11 8h2"/>
          <circle cx="8" cy="8" r="3.5"/>
        </svg>
      </div>
      <div class="stat-num c-green" id="statAvail" style="font-family:var(--font-mono)">0</div>
      <div class="stat-label">Available</div>
      <div class="mini-bar-wrap"><div class="mini-bar c-green" id="barAvail" style="width:0%"></div></div>
    </div>

    <!-- card 5: blocked proxies -->
    <div class="stat-card hud-corners fade-up">
      <div style="background:rgba(255,23,68,0.08)" class="card-bg-glow"></div>
      <div class="stat-icon-wrap">
        <svg width="16" height="16" fill="none" stroke="#ff1744" stroke-width="1.5" viewBox="0 0 16 16">
          <circle cx="8" cy="8" r="6"/>
          <path d="M5 5l6 6M11 5l-6 6"/>
        </svg>
      </div>
      <div class="stat-num c-red" id="statBlocked" style="font-family:var(--font-mono)">0</div>
      <div class="stat-label">Blocked</div>
      <div class="mini-bar-wrap"><div class="mini-bar c-red" id="barBlocked" style="width:0%"></div></div>
    </div>

    <!-- card 6: success rate -->
    <div class="stat-card hud-corners fade-up">
      <div style="background:rgba(255,215,64,0.08)" class="card-bg-glow"></div>
      <div class="stat-icon-wrap">
        <svg width="16" height="16" fill="none" stroke="#ffd740" stroke-width="1.5" viewBox="0 0 16 16">
          <path d="M8 2l1.5 4H14l-3.7 2.7 1.4 4.3L8 10.4l-3.7 2.6 1.4-4.3L2 6h4.5z"/>
        </svg>
      </div>
      <div class="stat-num c-gold" id="statAvailPct" style="font-family:var(--font-mono)">0%</div>
      <div class="stat-label">Proxy Health</div>
      <div class="mini-bar-wrap"><div class="mini-bar c-cyan" id="barHealth" style="width:0%"></div></div>
    </div>

    <!-- card 7: bots per session -->
    <div class="stat-card hud-corners fade-up">
      <div style="background:rgba(213,0,249,0.08)" class="card-bg-glow"></div>
      <div class="stat-icon-wrap">
        <svg width="16" height="16" fill="none" stroke="#d500f9" stroke-width="1.5" viewBox="0 0 16 16">
          <path d="M2 12l4-4 3 3 5-7"/>
        </svg>
      </div>
      <div class="stat-num c-purple" id="statBotsPerSession" style="font-family:var(--font-mono)">0</div>
      <div class="stat-label">Bots / Session</div>
    </div>
  </div>

  <!-- CHARTS ROW -------------------------------------------------- -->
  <div class="section-label">Activity Monitor</div>

  <div class="charts-row">
    <!-- Sessions chart -->
    <div class="chart-card fade-up hud-corners">
      <div class="card-title">
        Sessions history
        <span class="title-dot"></span>
      </div>
      <div class="chart-canvas-wrap">
        <canvas id="sessionsChart"></canvas>
      </div>
    </div>

    <!-- Proxies chart -->
    <div class="chart-card fade-up hud-corners">
      <div class="card-title">
        Proxy pool health
        <span class="title-dot" style="background:var(--green)"></span>
      </div>
      <div class="chart-canvas-wrap">
        <canvas id="proxyChart"></canvas>
      </div>
    </div>

    <!-- Health ring -->
    <div class="ring-card fade-up hud-corners">
      <div class="card-title" style="width:100%">
        Pool status
        <span class="title-dot" style="background:var(--gold)"></span>
      </div>
      <div class="ring-wrap">
        <canvas class="ring-canvas" id="ringCanvas" width="140" height="140"></canvas>
        <div class="ring-center">
          <div class="ring-pct" id="ringPct">0%</div>
          <div class="ring-pct-label">healthy</div>
        </div>
      </div>
      <div class="ring-legend" id="ringLegend">
        <div class="ring-legend-row">
          <div class="ring-legend-dot" style="background:var(--green)"></div>
          <span class="ring-legend-name">Available</span>
          <span class="ring-legend-val" id="legendAvail">0</span>
        </div>
        <div class="ring-legend-row">
          <div class="ring-legend-dot" style="background:var(--red)"></div>
          <span class="ring-legend-name">Blocked</span>
          <span class="ring-legend-val" id="legendBlocked">0</span>
        </div>
        <div class="ring-legend-row">
          <div class="ring-legend-dot" style="background:var(--text-4)"></div>
          <span class="ring-legend-name">Total</span>
          <span class="ring-legend-val" id="legendTotal">0</span>
        </div>
      </div>
    </div>
  </div>

  <!-- PROXY POOL -------------------------------------------------- -->
  <div class="section-label">Proxy Pool</div>

  <div class="proxy-section hud-corners fade-up" id="proxySection">

    <!-- Lockout overlay -->
    <div class="lockout-overlay" id="lockoutOverlay">
      <div class="lockout-count" id="lockoutCount">60</div>
      <div class="lockout-label">Too many failed attempts · Try again in</div>
      <div style="font-size:0.68rem;color:var(--text-4);letter-spacing:0.1em;text-transform:uppercase">seconds</div>
    </div>

    <!-- LOCK PANEL -->
    <div class="lock-panel" id="proxyLock">
      <div class="lock-hexagon">
        <!-- Rotating ring -->
        <svg class="hex-rings" width="80" height="80" viewBox="0 0 80 80" fill="none">
          <polygon points="40,4 72,22 72,58 40,76 8,58 8,22"
            stroke="rgba(0,229,255,0.3)" stroke-width="1" stroke-dasharray="4 4"/>
        </svg>
        <svg width="48" height="48" viewBox="0 0 48 48" fill="none">
          <polygon points="24,4 44,15 44,33 24,44 4,33 4,15"
            stroke="#00e5ff" stroke-width="1.5" fill="rgba(0,229,255,0.06)"/>
          <rect x="16" y="22" width="16" height="12" rx="2" fill="none" stroke="#00e5ff" stroke-width="1.5"/>
          <path d="M18 22v-4a6 6 0 0112 0v4" stroke="#00e5ff" stroke-width="1.5" stroke-linecap="round"/>
          <circle cx="24" cy="28" r="2" fill="#00e5ff"/>
        </svg>
      </div>

      <div class="lock-title type-cursor">Proxy Pool Locked</div>
      <div class="lock-desc">
        Authentication required to view
        <strong id="proxyLockCount">80</strong>
        residential proxies with live status.
        Max <strong>5</strong> attempts before 60s lockout.
      </div>

      <div class="pw-form">
        <input
          class="pw-input"
          type="password"
          id="proxyPassword"
          placeholder="Enter password"
          autocomplete="off"
          spellcheck="false"
          maxlength="64"
        />
        <button class="unlock-btn" id="proxyUnlockBtn">UNLOCK</button>
      </div>

      <div class="lock-msg" id="proxyMsg"></div>

      <!-- Attempt indicator dots -->
      <div class="attempt-dots" id="attemptDots">
        <div class="attempt-dot ok" id="dot0"></div>
        <div class="attempt-dot ok" id="dot1"></div>
        <div class="attempt-dot ok" id="dot2"></div>
        <div class="attempt-dot ok" id="dot3"></div>
        <div class="attempt-dot ok" id="dot4"></div>
      </div>
    </div>

    <!-- PROXY CONTENT (shown after unlock) -->
    <div class="proxy-content" id="proxyContent">

      <!-- Top bar: summary counts -->
      <div class="proxy-topbar">
        <div class="proxy-stats-row">
          <div class="proxy-stat-item">
            <div class="psb psb-total"></div>
            <span class="proxy-stat-num" id="pTotal">0</span>
            <span class="proxy-stat-name">total</span>
          </div>
          <div class="proxy-stat-item">
            <div class="psb psb-green"></div>
            <span class="proxy-stat-num" id="pAvail">0</span>
            <span class="proxy-stat-name">available</span>
          </div>
          <div class="proxy-stat-item">
            <div class="psb psb-red"></div>
            <span class="proxy-stat-num" id="pBlocked">0</span>
            <span class="proxy-stat-name">blocked</span>
          </div>
        </div>
        <div style="font-size:0.62rem;color:var(--text-4);letter-spacing:0.08em">
          last refresh <span id="proxyRefreshTime" style="color:var(--text-3)">—</span>
        </div>
      </div>

      <!-- Toolbar: search + filters + actions -->
      <div class="proxy-toolbar">
        <input
          type="text"
          class="proxy-search"
          id="proxySearch"
          placeholder="Search by IP or port..."
          spellcheck="false"
        />
        <button class="filter-btn active" id="filterAll">All</button>
        <button class="filter-btn" id="filterAvail">Available</button>
        <button class="filter-btn" id="filterBlocked">Blocked</button>

        <div class="tb-gap"></div>

        <button class="icon-btn" id="copyAllBtn" title="Copy all visible proxies">
          <svg width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 13 13">
            <rect x="1" y="3" width="8" height="9" rx="1"/>
            <path d="M4 3V2a1 1 0 011-1h6a1 1 0 011 1v8a1 1 0 01-1 1h-1"/>
          </svg>
          COPY ALL
        </button>
        <button class="icon-btn" id="exportBtn" title="Export proxies as CSV">
          <svg width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 13 13">
            <path d="M6.5 1v7M4 5.5L6.5 8 9 5.5"/>
            <path d="M2 9.5V11a1 1 0 001 1h7a1 1 0 001-1V9.5"/>
          </svg>
          EXPORT
        </button>
        <button class="icon-btn" id="proxyRefreshBtn" title="Refresh proxy list">
          <svg id="refreshIcon" width="13" height="13" fill="none" stroke="currentColor" stroke-width="1.5" viewBox="0 0 13 13">
            <path d="M11 3.5A5.5 5.5 0 102 8.5"/>
            <path d="M2 5V8.5H5.5"/>
          </svg>
          REFRESH
        </button>
        <label class="ar-toggle" title="Auto-refresh every 30s">
          <input type="checkbox" id="proxyAutoRefresh" checked />
          <span class="ar-track"></span>
          AUTO
        </label>
      </div>

      <!-- API Endpoints (visible after unlock) -->
      <div class="api-bar" id="apiBar">
        <div class="api-chip" data-path="/api/status">
          <span class="method">GET</span>/api/status
        </div>
        <div class="api-chip" data-path="/api/turnstile-status">
          <span class="method">GET</span>/api/turnstile-status
        </div>
        <div class="api-chip" data-path="/api/proxy-status">
          <span class="method">GET</span>/api/proxy-status
        </div>
        <div class="api-chip" data-path="/api/proxies">
          <span class="method">GET</span>/api/proxies
        </div>
      </div>

      <!-- Grid -->
      <div class="proxy-grid-wrap">
        <div class="proxy-grid" id="proxyGrid"></div>
      </div>

      <div class="ts-row" id="proxyTs">&nbsp;</div>
    </div>

  </div>

  <!-- SYSTEM INFO ------------------------------------------------- -->
  <div class="section-label">System</div>

  <div class="sys-card hud-corners fade-up">
    <div class="sys-grid" id="sysGrid">
      <div class="sys-item">
        <span class="sys-key">Status</span>
        <span class="sys-val" id="sysStatus">—</span>
      </div>
      <div class="sys-item">
        <span class="sys-key">Runtime</span>
        <span class="sys-val">Go 1.24</span>
      </div>
      <div class="sys-item">
        <span class="sys-key">Version</span>
        <span class="sys-val">v8.0 PHANTOM</span>
      </div>
      <div class="sys-item">
        <span class="sys-key">Platform</span>
        <span class="sys-val">HostingGuru</span>
      </div>
      <div class="sys-item">
        <span class="sys-key">Edition</span>
        <span class="sys-val">Residential Proxy</span>
      </div>
      <div class="sys-item">
        <span class="sys-key">Pool Size</span>
        <span class="sys-val" id="sysPool">80</span>
      </div>
      <div class="sys-item">
        <span class="sys-key">Bots</span>
        <span class="sys-val" id="sysBots">0</span>
      </div>
      <div class="sys-item">
        <span class="sys-key">Sessions</span>
        <span class="sys-val" id="sysSessions">0</span>
      </div>
      <div class="sys-item">
        <span class="sys-key">Time</span>
        <span class="sys-val" id="sysTime">—</span>
      </div>
      <div class="sys-item">
        <span class="sys-key">Author</span>
        <span class="sys-val author-val">Saif</span>
      </div>
    </div>
  </div>

  <!-- KBD HINTS -------------------------------------------------- -->
  <div class="kbd-row">
    <div class="kbd-pair"><kbd>Enter</kbd> unlock</div>
    <div class="kbd-pair"><kbd>/</kbd> search</div>
    <div class="kbd-pair"><kbd>R</kbd> refresh</div>
    <div class="kbd-pair"><kbd>C</kbd> copy all</div>
    <div class="kbd-pair"><kbd>E</kbd> export</div>
    <div class="kbd-pair"><kbd>1</kbd> all · <kbd>2</kbd> avail · <kbd>3</kbd> blocked</div>
  </div>

  <div class="ts-row" id="lastUpdated">Awaiting telemetry...</div>

</main>

<footer class="footer">
  &copy; 2026 &middot; ICEbot Server v8.0 &quot;PHANTOM&quot; &middot; built by <span class="sig">Saif</span>
</footer>

<!-- ================================================================
     JAVASCRIPT
     ================================================================ -->
<script>
(function(){
'use strict';

/* ==============================================================
   CUSTOM CURSOR
   ============================================================== */
var cur  = document.getElementById('cursor');
var ring = document.getElementById('cursor-ring');
var mx=0,my=0,rx=0,ry=0;

document.addEventListener('mousemove',function(e){
  mx=e.clientX;my=e.clientY;
  cur.style.left=mx+'px';cur.style.top=my+'px';
},true);

// Lag the ring for smooth effect
(function ringLoop(){
  rx+=(mx-rx)*0.14;
  ry+=(my-ry)*0.14;
  ring.style.left=rx+'px';ring.style.top=ry+'px';
  requestAnimationFrame(ringLoop);
})();

document.addEventListener('mousedown',function(){
  document.body.classList.add('cursor-click');
  setTimeout(function(){document.body.classList.remove('cursor-click')},200);
});

document.querySelectorAll('button,a,input,label,[role=button]').forEach(function(el){
  el.addEventListener('mouseenter',function(){document.body.classList.add('cursor-hover')});
  el.addEventListener('mouseleave',function(){document.body.classList.remove('cursor-hover')});
});

/* ==============================================================
   MATRIX RAIN
   ============================================================== */
(function(){
  var c=document.getElementById('matrix-canvas');
  var ctx=c.getContext('2d');
  var W,H,cols,drops;
  var chars='ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789@#$%&アイウエオカキクケコサシスセソタチツテトナニヌネノ';

  function resize(){
    W=c.width=window.innerWidth;
    H=c.height=window.innerHeight;
    cols=Math.floor(W/16);
    drops=new Array(cols).fill(1);
  }
  resize();
  window.addEventListener('resize',resize);

  function draw(){
    ctx.fillStyle='rgba(2,4,8,0.04)';
    ctx.fillRect(0,0,W,H);
    for(var i=0;i<drops.length;i++){
      var ch=chars[Math.floor(Math.random()*chars.length)];
      var bright=Math.random();
      if(bright>0.97){
        ctx.fillStyle='#ffffff';
      } else if(bright>0.90){
        ctx.fillStyle='#00e5ff';
      } else if(bright>0.6){
        ctx.fillStyle='rgba(0,229,255,0.5)';
      } else {
        ctx.fillStyle='rgba(0,150,180,0.25)';
      }
      ctx.font='14px monospace';
      ctx.fillText(ch,i*16,drops[i]*16);
      if(drops[i]*16>H && Math.random()>0.975){drops[i]=0;}
      drops[i]++;
    }
  }
  setInterval(draw,50);
})();

/* ==============================================================
   PERSPECTIVE GRID (canvas)
   ============================================================== */
(function(){
  var c=document.getElementById('grid-canvas');
  var ctx=c.getContext('2d');

  function resize(){c.width=window.innerWidth;c.height=window.innerHeight}
  resize();
  window.addEventListener('resize',resize);

  var offset=0;
  function draw(){
    var W=c.width,H=c.height;
    ctx.clearRect(0,0,W,H);
    offset=(offset+0.4)%80;

    // Horizontal lines
    ctx.strokeStyle='rgba(0,229,255,0.035)';
    ctx.lineWidth=1;
    for(var y=offset;y<H;y+=40){
      ctx.beginPath();ctx.moveTo(0,y);ctx.lineTo(W,y);ctx.stroke();
    }
    // Vertical lines
    for(var x=offset;x<W;x+=60){
      ctx.beginPath();ctx.moveTo(x,0);ctx.lineTo(x,H);ctx.stroke();
    }
    requestAnimationFrame(draw);
  }
  draw();
})();

/* ==============================================================
   MINI LINE CHARTS
   ============================================================== */
var CHART_POINTS = 30;
var sessionsHistory = new Array(CHART_POINTS).fill(0);
var proxyHistory    = new Array(CHART_POINTS).fill(100);

function drawLineChart(canvasId, data, color, fillColor){
  var canvas = document.getElementById(canvasId);
  if(!canvas) return;
  var ctx = canvas.getContext('2d');
  var W = canvas.clientWidth;
  var H = canvas.clientHeight;
  canvas.width = W;
  canvas.height = H;

  if(!W || !H) return;

  ctx.clearRect(0,0,W,H);

  var max = Math.max.apply(null, data);
  if(max === 0) max = 1;
  var min = Math.min.apply(null, data);
  var range = max - min || 1;

  var step = W / (data.length - 1);
  var pad = 6;

  // Build path
  ctx.beginPath();
  for(var i=0; i<data.length; i++){
    var x = i * step;
    var y = H - pad - ((data[i] - min) / range) * (H - pad*2);
    if(i===0) ctx.moveTo(x,y); else ctx.lineTo(x,y);
  }

  // Fill gradient
  var fill = ctx.createLinearGradient(0,0,0,H);
  fill.addColorStop(0, fillColor);
  fill.addColorStop(1,'rgba(0,0,0,0)');
  ctx.lineTo(W, H); ctx.lineTo(0, H); ctx.closePath();
  ctx.fillStyle = fill;
  ctx.fill();

  // Line
  ctx.beginPath();
  for(var i=0; i<data.length; i++){
    var x = i * step;
    var y = H - pad - ((data[i] - min) / range) * (H - pad*2);
    if(i===0) ctx.moveTo(x,y); else ctx.lineTo(x,y);
  }
  ctx.strokeStyle = color;
  ctx.lineWidth = 1.5;
  ctx.stroke();

  // Glow line
  ctx.shadowBlur = 8;
  ctx.shadowColor = color;
  ctx.stroke();
  ctx.shadowBlur = 0;

  // Last point dot
  var lastX = (data.length-1)*step;
  var lastY = H - pad - ((data[data.length-1]-min)/range)*(H-pad*2);
  ctx.beginPath();
  ctx.arc(lastX,lastY,3,0,Math.PI*2);
  ctx.fillStyle = color;
  ctx.fill();
}

/* ==============================================================
   RING CHART (donut)
   ============================================================== */
function drawRing(avail, total){
  var canvas = document.getElementById('ringCanvas');
  if(!canvas) return;
  var ctx = canvas.getContext('2d');
  var W = 140, H = 140;
  canvas.width = W; canvas.height = H;
  ctx.clearRect(0,0,W,H);

  var cx=W/2, cy=H/2, r=54, lw=12;
  var blocked = total - avail;
  var pct = total > 0 ? avail/total : 0;
  var TAU = Math.PI*2;
  var start = -Math.PI/2;

  // Track
  ctx.beginPath();
  ctx.arc(cx,cy,r,0,TAU);
  ctx.strokeStyle='rgba(0,229,255,0.07)';
  ctx.lineWidth=lw;
  ctx.stroke();

  if(total > 0){
    // Available arc
    ctx.beginPath();
    ctx.arc(cx,cy,r,start,start+TAU*pct);
    ctx.strokeStyle='#00e676';
    ctx.lineWidth=lw;
    ctx.lineCap='round';
    ctx.shadowBlur=10;
    ctx.shadowColor='#00e676';
    ctx.stroke();
    ctx.shadowBlur=0;

    // Blocked arc
    if(blocked > 0){
      ctx.beginPath();
      ctx.arc(cx,cy,r,start+TAU*pct,start+TAU);
      ctx.strokeStyle='#ff1744';
      ctx.lineWidth=lw;
      ctx.lineCap='round';
      ctx.shadowBlur=6;
      ctx.shadowColor='#ff1744';
      ctx.stroke();
      ctx.shadowBlur=0;
    }
  }

  // Inner glow ring
  ctx.beginPath();
  ctx.arc(cx,cy,r-lw/2-4,0,TAU);
  ctx.strokeStyle='rgba(0,229,255,0.04)';
  ctx.lineWidth=1;
  ctx.stroke();

  document.getElementById('ringPct').textContent = Math.round(pct*100)+'%';
  document.getElementById('legendAvail').textContent = avail;
  document.getElementById('legendBlocked').textContent = blocked;
  document.getElementById('legendTotal').textContent = total;
}

/* ==============================================================
   CONSTANTS
   ============================================================== */
var STATS_INTERVAL = 5000;
var PROXY_AUTO_INTERVAL = 30000;
var MAX_ATTEMPTS = 5;

/* ==============================================================
   STATE
   ============================================================== */
var state = {
  unlocked:        false,
  password:        '',
  filter:          'all',   // 'all' | 'available' | 'blocked'
  search:          '',
  proxies:         [],
  attemptsUsed:    0,
  locked:          false,
  lockoutEnd:      null,
  statsIntervalId: null,
  proxyIntervalId: null,
  sessionStart:    new Date()
};

/* ==============================================================
   DOM CACHE
   ============================================================== */
var D = {
  statusPill:       document.getElementById('statusPill'),
  statusLabel:      document.getElementById('statusLabel'),
  statusDot:        document.getElementById('statusDot'),
  uptimeDisplay:    document.getElementById('uptimeDisplay'),
  statSessions:     document.getElementById('statSessions'),
  statBots:         document.getElementById('statBots'),
  statJoined:       document.getElementById('statJoined'),
  statProxies:      document.getElementById('statProxies'),
  statAvail:        document.getElementById('statAvail'),
  statBlocked:      document.getElementById('statBlocked'),
  statAvailPct:     document.getElementById('statAvailPct'),
  statBPS:          document.getElementById('statBotsPerSession'),
  joinedPct:        document.getElementById('joinedPct'),
  barSessions:      document.getElementById('barSessions'),
  barBots:          document.getElementById('barBots'),
  barJoined:        document.getElementById('barJoined'),
  barAvail:         document.getElementById('barAvail'),
  barBlocked:       document.getElementById('barBlocked'),
  barHealth:        document.getElementById('barHealth'),
  lastUpdated:      document.getElementById('lastUpdated'),
  proxyLock:        document.getElementById('proxyLock'),
  proxyContent:     document.getElementById('proxyContent'),
  proxyGrid:        document.getElementById('proxyGrid'),
  proxyPassword:    document.getElementById('proxyPassword'),
  proxyUnlockBtn:   document.getElementById('proxyUnlockBtn'),
  proxyMsg:         document.getElementById('proxyMsg'),
  proxyLockCount:   document.getElementById('proxyLockCount'),
  proxySearch:      document.getElementById('proxySearch'),
  proxyRefreshTime: document.getElementById('proxyRefreshTime'),
  pTotal:           document.getElementById('pTotal'),
  pAvail:           document.getElementById('pAvail'),
  pBlocked:         document.getElementById('pBlocked'),
  proxyTs:          document.getElementById('proxyTs'),
  filterAll:        document.getElementById('filterAll'),
  filterAvail:      document.getElementById('filterAvail'),
  filterBlocked:    document.getElementById('filterBlocked'),
  proxyRefreshBtn:  document.getElementById('proxyRefreshBtn'),
  copyAllBtn:       document.getElementById('copyAllBtn'),
  exportBtn:        document.getElementById('exportBtn'),
  proxyAutoRefresh: document.getElementById('proxyAutoRefresh'),
  lockoutOverlay:   document.getElementById('lockoutOverlay'),
  lockoutCount:     document.getElementById('lockoutCount'),
  attemptDots:      [0,1,2,3,4].map(function(i){return document.getElementById('dot'+i)}),
  sysStatus:        document.getElementById('sysStatus'),
  sysPool:          document.getElementById('sysPool'),
  sysBots:          document.getElementById('sysBots'),
  sysSessions:      document.getElementById('sysSessions'),
  sysTime:          document.getElementById('sysTime'),
  toastRoot:        document.getElementById('toast-root')
};

/* ==============================================================
   TOAST
   ============================================================== */
function toast(msg, type){
  type = type || 'info';
  var t = document.createElement('div');
  t.className = 'toast t-'+(type==='success'?'ok':type==='error'?'err':'info');
  t.textContent = msg;
  D.toastRoot.appendChild(t);
  setTimeout(function(){if(t.parentNode)t.parentNode.removeChild(t)}, 3800);
}

/* ==============================================================
   ANIMATED COUNTER
   ============================================================== */
function animateTo(el, target, suffix){
  if(!el) return;
  suffix = suffix || '';
  var current = parseFloat(el.textContent) || 0;
  var diff = target - current;
  if(Math.abs(diff) < 0.5){el.textContent = target + suffix; return;}
  var steps = 20;
  var inc = diff / steps;
  var step = 0;
  var id = setInterval(function(){
    step++;
    current += inc;
    el.textContent = Math.round(current) + suffix;
    if(step >= steps){
      el.textContent = target + suffix;
      clearInterval(id);
      el.classList.add('flash');
      setTimeout(function(){el.classList.remove('flash')},600);
    }
  }, 16);
}

/* ==============================================================
   FETCH STATS
   ============================================================== */
function fetchStats(){
  var xhr = new XMLHttpRequest();
  xhr.open('GET','/api/status',true);
  xhr.timeout = 8000;

  xhr.onload = function(){
    if(xhr.status !== 200){
      setStatus('degraded');
      return;
    }
    try{
      var d = JSON.parse(xhr.responseText);
      var sessions = d.sessions  || 0;
      var bots     = d.totalBots || 0;
      var joined   = d.joinedBots || 0;
      var total    = d.proxies   || 0;
      var avail    = d.proxiesAvail   || 0;
      var blocked  = d.proxiesBlocked || 0;
      var healthPct= total > 0 ? Math.round(avail/total*100) : 0;
      var bps      = sessions > 0 ? (bots/sessions).toFixed(1) : '0';
      var jPct     = bots > 0 ? Math.round(joined/bots*100) : 0;

      animateTo(D.statSessions, sessions);
      animateTo(D.statBots, bots);
      animateTo(D.statJoined, joined);
      animateTo(D.statProxies, total);
      animateTo(D.statAvail, avail);
      animateTo(D.statBlocked, blocked);
      animateTo(D.statAvailPct, healthPct, '%');
      if(D.statBPS) D.statBPS.textContent = bps;
      if(D.joinedPct) D.joinedPct.textContent = jPct + '% success rate';

      // Mini bars
      var maxSess = Math.max(sessions, 1);
      if(D.barSessions) D.barSessions.style.width = Math.min(100, sessions/10*100)+'%';
      if(D.barBots)     D.barBots.style.width     = Math.min(100, bots/50*100)+'%';
      if(D.barJoined)   D.barJoined.style.width   = jPct+'%';
      if(D.barAvail)    D.barAvail.style.width     = healthPct+'%';
      if(D.barBlocked)  D.barBlocked.style.width   = (total>0?Math.round(blocked/total*100):0)+'%';
      if(D.barHealth)   D.barHealth.style.width    = healthPct+'%';

      // Charts history
      sessionsHistory.push(sessions);
      sessionsHistory.shift();
      proxyHistory.push(avail);
      proxyHistory.shift();
      drawLineChart('sessionsChart', sessionsHistory, '#00e5ff', 'rgba(0,229,255,0.08)');
      drawLineChart('proxyChart', proxyHistory, '#00e676', 'rgba(0,230,118,0.08)');
      drawRing(avail, total);

      // System panel
      if(D.sysPool)     D.sysPool.textContent     = total;
      if(D.sysBots)     D.sysBots.textContent      = bots;
      if(D.sysSessions) D.sysSessions.textContent  = sessions;
      if(D.sysStatus)   D.sysStatus.textContent    = 'ONLINE';

      // Update proxy lock count
      if(D.proxyLockCount) D.proxyLockCount.textContent = total;

      D.lastUpdated.textContent = 'updated ' + new Date().toLocaleTimeString();
      setStatus('online');

    } catch(e){ setStatus('degraded'); }
  };

  xhr.onerror   = function(){ setStatus('offline'); };
  xhr.ontimeout = function(){ setStatus('degraded'); };
  xhr.send();
}

function setStatus(s){
  D.statusPill.className = 'status-pill ' + s;
  D.statusDot.className  = 'status-dot' + (s==='online'?' pulse':'');
  D.statusLabel.textContent = s.toUpperCase();
  if(D.sysStatus) D.sysStatus.textContent = s.toUpperCase();
}

/* ==============================================================
   LOCKOUT TIMER
   ============================================================== */
var lockoutTimerId = null;

function startLockout(seconds){
  state.locked = true;
  state.lockoutEnd = Date.now() + seconds * 1000;
  D.lockoutOverlay.classList.add('active');
  D.proxyUnlockBtn.disabled = true;

  function tick(){
    var rem = Math.ceil((state.lockoutEnd - Date.now()) / 1000);
    if(rem <= 0){
      clearInterval(lockoutTimerId);
      state.locked = false;
      state.attemptsUsed = 0;
      D.lockoutOverlay.classList.remove('active');
      D.proxyUnlockBtn.disabled = false;
      D.proxyMsg.textContent = '';
      D.proxyMsg.className = 'lock-msg';
      updateAttemptDots(0);
    } else {
      D.lockoutCount.textContent = rem;
    }
  }
  tick();
  lockoutTimerId = setInterval(tick, 500);
}

function updateAttemptDots(used){
  for(var i=0;i<5;i++){
    if(!D.attemptDots[i]) continue;
    if(i < used){
      D.attemptDots[i].className = 'attempt-dot used';
    } else {
      D.attemptDots[i].className = 'attempt-dot ok';
    }
  }
}

/* ==============================================================
   LOAD PROXIES
   ============================================================== */
function loadProxies(pw, silent){
  if(state.locked) return;
  if(!silent){
    D.proxyGrid.innerHTML = '<div class="proxy-skel"></div>'.repeat(12).replace(/(proxy-skel)/g,'$1 fade-up');
    D.proxyMsg.textContent = '';
  }

  var xhr = new XMLHttpRequest();
  xhr.open('GET', '/api/proxies?password='+encodeURIComponent(pw), true);
  xhr.timeout = 12000;

  xhr.onload = function(){
    try{
      var d = JSON.parse(xhr.responseText);

      if(xhr.status === 429){
        // Rate limited
        var secs = d.lockout || 60;
        startLockout(secs);
        return;
      }

      if(xhr.status === 401){
        state.attemptsUsed++;
        updateAttemptDots(state.attemptsUsed);

        var left = d.attemptsLeft !== undefined ? d.attemptsLeft : MAX_ATTEMPTS - state.attemptsUsed;

        if(d.lockout && d.lockout > 0){
          startLockout(d.lockout);
          return;
        }

        setMsg('Incorrect password. '+left+' attempt'+(left===1?'':'s')+' remaining.','error');
        toast('Wrong password — '+left+' left','error');
        // Shake the input
        D.proxyPassword.style.borderColor='var(--red)';
        setTimeout(function(){D.proxyPassword.style.borderColor=''},800);
        return;
      }

      if(xhr.status !== 200){
        setMsg('Server error ('+xhr.status+').','error');
        return;
      }

      // SUCCESS
      state.attemptsUsed = 0;
      state.password = pw;
      state.unlocked = true;
      state.proxies  = d.proxies || [];

      if(!silent){
        D.proxyLock.style.display    = 'none';
        D.proxyContent.classList.add('visible');
        toast('Proxy pool unlocked — '+d.available+' available, '+d.blocked+' blocked','success');
        startProxyAutoRefresh();
      }

      D.proxyRefreshTime.textContent = new Date().toLocaleTimeString();
      D.pTotal.textContent   = d.total    || 0;
      D.pAvail.textContent   = d.available|| 0;
      D.pBlocked.textContent = d.blocked  || 0;
      D.proxyTs.textContent  = 'last fetch: '+new Date().toLocaleTimeString();

      renderProxyGrid();

    } catch(e){
      setMsg('Parse error.','error');
    }
  };

  xhr.onerror   = function(){setMsg('Network error.','error');};
  xhr.ontimeout = function(){setMsg('Request timed out.','error');};
  xhr.send();
}

function setMsg(msg, type){
  D.proxyMsg.textContent = msg;
  D.proxyMsg.className = 'lock-msg ' + (type||'');
}

/* ==============================================================
   RENDER PROXY GRID
   ============================================================== */
function renderProxyGrid(){
  var data = state.proxies;
  var q    = state.search.toLowerCase();
  var f    = state.filter;

  var filtered = data.filter(function(p){
    if(f === 'available' && p.status !== 'available') return false;
    if(f === 'blocked'   && p.status !== 'blocked')   return false;
    if(q){
      var addr = (p.ip+':'+p.port).toLowerCase();
      if(addr.indexOf(q) === -1) return false;
    }
    return true;
  });

  if(filtered.length === 0){
    D.proxyGrid.innerHTML =
      '<div class="proxy-empty">' +
      '<span class="empty-icon">&#9632;</span>' +
      'No proxies match your filter.' +
      '</div>';
    return;
  }

  var html = '';
  for(var i=0;i<filtered.length;i++){
    var p = filtered[i];
    var isAvail = p.status === 'available';
    var cls = isAvail ? 'avail' : 'blocked';
    var tagCls = isAvail ? 'tag-avail' : 'tag-blocked';
    var tagTxt = isAvail ? 'OK' : 'BLOCKED';
    var addr = p.ip+':'+p.port;
    html +=
      '<div class="proxy-card '+cls+'" data-addr="'+addr+'" data-ip="'+p.ip+'" data-port="'+p.port+'" title="Click to copy">' +
        '<span class="proxy-addr">'+
          '<span>'+p.ip+'</span>'+
          '<span class="proxy-colon">:</span>'+
          '<span class="proxy-port">'+p.port+'</span>'+
        '</span>' +
        '<span class="proxy-status-tag '+tagCls+'">'+tagTxt+'</span>' +
      '</div>';
  }

  D.proxyGrid.innerHTML = html;

  // Attach click-to-copy
  var cards = D.proxyGrid.querySelectorAll('.proxy-card');
  cards.forEach(function(card){
    card.addEventListener('click',function(){
      var addr = card.getAttribute('data-addr');
      copyText(addr, function(){
        card.classList.add('copied');
        setTimeout(function(){card.classList.remove('copied')},700);
        toast('Copied: '+addr,'success');
      });
    });
  });
}

/* ==============================================================
   COPY HELPERS
   ============================================================== */
function copyText(text, cb){
  if(navigator.clipboard && navigator.clipboard.writeText){
    navigator.clipboard.writeText(text).then(cb).catch(function(){fallbackCopy(text,cb)});
  } else {
    fallbackCopy(text,cb);
  }
}

function fallbackCopy(text,cb){
  var ta=document.createElement('textarea');
  ta.value=text;ta.style.cssText='position:fixed;opacity:0;left:-9999px';
  document.body.appendChild(ta);ta.select();
  try{document.execCommand('copy');if(cb)cb();}catch(e){toast('Copy failed','error');}
  document.body.removeChild(ta);
}

/* ==============================================================
   EXPORT CSV
   ============================================================== */
function exportProxies(){
  if(!state.proxies.length){toast('No proxies to export','error');return;}
  var rows = ['ip,port,status'];
  state.proxies.forEach(function(p){
    rows.push(p.ip+','+p.port+','+p.status);
  });
  var csv = rows.join('\n');
  var blob = new Blob([csv],{type:'text/csv'});
  var url = URL.createObjectURL(blob);
  var a = document.createElement('a');
  a.href=url;a.download='proxies_'+new Date().toISOString().split('T')[0]+'.csv';
  document.body.appendChild(a);a.click();
  document.body.removeChild(a);URL.revokeObjectURL(url);
  toast('Exported '+state.proxies.length+' proxies','success');
}

/* ==============================================================
   COPY ALL
   ============================================================== */
function copyAll(){
  var f = state.filter;
  var q = state.search.toLowerCase();
  var lines = state.proxies
    .filter(function(p){
      if(f==='available'&&p.status!=='available')return false;
      if(f==='blocked'&&p.status!=='blocked')return false;
      if(q&&(p.ip+':'+p.port).toLowerCase().indexOf(q)===-1)return false;
      return true;
    })
    .map(function(p){return p.ip+':'+p.port})
    .join('\n');

  if(!lines){toast('Nothing to copy','error');return;}
  copyText(lines,function(){
    toast('Copied '+lines.split('\n').length+' proxies','success');
  });
}

/* ==============================================================
   PROXY AUTO-REFRESH
   ============================================================== */
function startProxyAutoRefresh(){
  stopProxyAutoRefresh();
  if(D.proxyAutoRefresh.checked){
    state.proxyIntervalId = setInterval(function(){
      if(state.unlocked && state.password){
        loadProxies(state.password, true);
      }
    }, PROXY_AUTO_INTERVAL);
  }
}

function stopProxyAutoRefresh(){
  if(state.proxyIntervalId){clearInterval(state.proxyIntervalId);state.proxyIntervalId=null;}
}

/* ==============================================================
   FILTER BUTTONS
   ============================================================== */
function setFilter(f){
  state.filter = f;
  D.filterAll.className     = 'filter-btn' + (f==='all'?' active':'');
  D.filterAvail.className   = 'filter-btn' + (f==='available'?' active-green':'');
  D.filterBlocked.className = 'filter-btn' + (f==='blocked'?' active-red':'');
  renderProxyGrid();
}

/* ==============================================================
   BIND EVENTS
   ============================================================== */
function bindEvents(){

  // Unlock
  function doUnlock(){
    var pw = D.proxyPassword.value;
    if(!pw){setMsg('Enter a password.','error');return;}
    if(state.locked){return;}
    loadProxies(pw, false);
  }
  D.proxyUnlockBtn.addEventListener('click', doUnlock);
  D.proxyPassword.addEventListener('keydown',function(e){
    if(e.key==='Enter'){e.preventDefault();doUnlock();}
  });

  // Search
  D.proxySearch.addEventListener('input',function(){
    state.search = D.proxySearch.value;
    renderProxyGrid();
  });

  // Filters
  D.filterAll.addEventListener('click',    function(){setFilter('all')});
  D.filterAvail.addEventListener('click',  function(){setFilter('available')});
  D.filterBlocked.addEventListener('click',function(){setFilter('blocked')});

  // Refresh
  D.proxyRefreshBtn.addEventListener('click',function(){
    if(!state.password){toast('Re-enter password to refresh','error');return;}
    var icon = document.getElementById('refreshIcon');
    if(icon) icon.classList.add('spinning');
    loadProxies(state.password, false);
    setTimeout(function(){if(icon)icon.classList.remove('spinning')},800);
  });

  // Copy all
  D.copyAllBtn.addEventListener('click', copyAll);

  // Export
  D.exportBtn.addEventListener('click', exportProxies);

  // Auto-refresh toggle
  D.proxyAutoRefresh.addEventListener('change', startProxyAutoRefresh);

  // API chips — open with password if unlocked
  document.querySelectorAll('.api-chip').forEach(function(chip){
    chip.addEventListener('click',function(){
      var path = chip.getAttribute('data-path');
      var pw = state.password || '';
      var url = pw ? path+'?password='+encodeURIComponent(pw) : path;
      window.open(url,'_blank');
    });
  });

  // Global keyboard shortcuts
  document.addEventListener('keydown',function(e){
    var tag = (e.target && e.target.tagName)||'';
    var inInput = (tag==='INPUT'||tag==='TEXTAREA'||tag==='SELECT');

    if(e.key==='/'&&!inInput){
      e.preventDefault();
      if(state.unlocked) D.proxySearch.focus();
      else D.proxyPassword.focus();
      return;
    }

    if(!inInput){
      if(e.key==='r'||e.key==='R'){
        e.preventDefault();
        if(state.unlocked&&state.password){loadProxies(state.password,false);}
        return;
      }
      if(e.key==='c'||e.key==='C'){e.preventDefault();copyAll();return;}
      if(e.key==='e'||e.key==='E'){e.preventDefault();exportProxies();return;}
      if(e.key==='1'){setFilter('all');return;}
      if(e.key==='2'){setFilter('available');return;}
      if(e.key==='3'){setFilter('blocked');return;}
    }
  });
}

/* ==============================================================
   UPTIME CLOCK
   ============================================================== */
function startClocks(){
  setInterval(function(){
    // Uptime
    var elapsed = Math.floor((new Date()-state.sessionStart)/1000);
    var h=Math.floor(elapsed/3600);
    var m=Math.floor((elapsed%3600)/60);
    var s=elapsed%60;
    var parts=[];
    if(h)parts.push(h+'h');
    if(m||h)parts.push(m+'m');
    parts.push(s+'s');
    if(D.uptimeDisplay) D.uptimeDisplay.textContent = parts.join(' ');

    // System time
    if(D.sysTime) D.sysTime.textContent = new Date().toLocaleString();
  }, 1000);
}

/* ==============================================================
   INIT
   ============================================================== */
function init(){
  bindEvents();
  startClocks();

  // Initial draws
  drawLineChart('sessionsChart', sessionsHistory, '#00e5ff', 'rgba(0,229,255,0.08)');
  drawLineChart('proxyChart',    proxyHistory,    '#00e676', 'rgba(0,230,118,0.08)');
  drawRing(0, 80);

  // Start polling
  fetchStats();
  state.statsIntervalId = setInterval(fetchStats, STATS_INTERVAL);

  // Redraw charts on resize
  window.addEventListener('resize', function(){
    drawLineChart('sessionsChart', sessionsHistory, '#00e5ff', 'rgba(0,229,255,0.08)');
    drawLineChart('proxyChart',    proxyHistory,    '#00e676', 'rgba(0,230,118,0.08)');
  });
}

if(document.readyState==='loading'){
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}

})();
</script>
</body>
</html>`