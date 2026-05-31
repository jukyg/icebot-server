package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

// ============================================================================
// proxy.go — Residential proxy management with IP:PORT:USER:PASS authentication
// Routes all bot traffic through rotating residential proxies.
// ============================================================================

// Proxy tier constants
const (
	TierResidential = 0 // residential proxy (primary)
	TierCroxy       = 0 // alias for CroxyProxy-style (same tier)
	TierPremium     = 1 // premium proxy (future use)
	TierDirect      = 2 // direct connection (fallback)
)

// PremiumProxy is a placeholder for future premium proxy support.
type PremiumProxy struct {
	IP      string
	Port    string
	proxyURL_ string
	slots   chan struct{}
}

func (p *PremiumProxy) release() {
	if p != nil && p.slots != nil {
		select {
		case p.slots <- struct{}{}:
		default:
		}
	}
}

func (p *PremiumProxy) proxyURL() string {
	return p.proxyURL_
}

// ResidentialProxy represents a single residential proxy with credentials.
type ResidentialProxy struct {
	IP       string
	Port     string
	Username string
	Password string
}

// Address returns the full host:port string for this proxy.
func (p *ResidentialProxy) Address() string {
	return fmt.Sprintf("%s:%s", p.IP, p.Port)
}

// ProxyURL returns the HTTP proxy URL with embedded credentials.
func (p *ResidentialProxy) ProxyURL() string {
	if p.Username != "" {
		return fmt.Sprintf("http://%s:%s@%s:%s", p.Username, p.Password, p.IP, p.Port)
	}
	return fmt.Sprintf("http://%s:%s", p.IP, p.Port)
}

// BasicAuth returns the Base64-encoded "username:password" for Proxy-Authorization.
func (p *ResidentialProxy) BasicAuth() string {
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", p.Username, p.Password)))
}

// ============================================================================
// ProxySelection — returned by turbo.go's pickProxy
// ============================================================================
type ProxySelection struct {
	Tier       int
	CroxyIP    string
	Residential *ResidentialProxy
	Premium    *PremiumProxy
}

// ============================================================================
// Proxy pool — residential proxies with rotating credentials
// ============================================================================
var residentialProxies []ResidentialProxy

// proxySeeds are the known working residential proxies with credentials.
var proxySeeds = []ResidentialProxy{
	{IP: "38.154.203.95", Port: "5863", Username: "fjdiujcj", Password: "bamvb22jtfi7"},
	{IP: "198.105.121.200", Port: "6462", Username: "fjdiujcj", Password: "bamvb22jtfi7"},
	{IP: "64.137.96.74", Port: "6641", Username: "fjdiujcj", Password: "bamvb22jtfi7"},
	{IP: "209.127.138.10", Port: "5784", Username: "fjdiujcj", Password: "bamvb22jtfi7"},
	{IP: "38.154.185.97", Port: "6370", Username: "fjdiujcj", Password: "bamvb22jtfi 7"},
	{IP: "84.247.60.125", Port: "6095", Username: "fjdiujcj", Password: "bamvb22jtfi7"},
	{IP: "142.111.67.146", Port: "5611", Username: "fjdiujcj", Password: "bamvb22jtfi7"},
	{IP: "191.96.254.138", Port: "6185", Username: "fjdiujcj", Password: "bamvb22jtfi7"},
	{IP: "31.58.9.4", Port: "6077", Username: "fjdiujcj", Password: "bamvb22jtfi7"},
	{IP: "104.239.107.47", Port: "5699", Username: "fjdiujcj", Password: "bamvb22jtfi7"},
}

func init() {
	// Use the known working proxies directly — no IP fuzzing.
	// These are actual proxy servers with valid credentials.
	residentialProxies = make([]ResidentialProxy, len(proxySeeds))
	copy(residentialProxies, proxySeeds)
}

// blockedProxies tracks proxies that have failed recently.
var (
	blockedMu          sync.RWMutex
	blockedProxies     = make(map[string]time.Time)
	proxyFailureCount  = make(map[string]int) // consecutive failures per IP
	proxyScore         = make(map[string]int) // health score: higher = more reliable (capped ±100)
	proxyTotalAttempts = make(map[string]int) // total pick count for scoring weight
)

// blockDuration is how long a proxy stays blocked before retrying.
// Reduced to 10s for faster recovery
const blockDuration = 10 * time.Second

// maxConsecutiveFailuresBeforeEscalate escalates a proxy to a 5-minute block.
const maxConsecutiveFailuresBeforeEscalate = 3

// pickProxyByIndex returns a residential proxy by round-robin index.
// Skips blocked proxies by scanning forward. Falls back to the original
// index if all are blocked (may have recovered since the block was set).
func pickProxyByIndex(index int) *ResidentialProxy {
	total := len(residentialProxies)
	if total == 0 {
		return nil
	}

	blockedMu.RLock()
	defer blockedMu.RUnlock()

	now := time.Now()
	start := index % total
	for i := 0; i < total; i++ {
		idx := (start + i) % total
		p := &residentialProxies[idx]
		blockedAt, isBlocked := blockedProxies[p.IP]
		if !isBlocked || now.Sub(blockedAt) > blockDuration {
			return p
		}
	}
	// All blocked — return original index anyway
	return &residentialProxies[start]
}

// pickProxy returns a random non-blocked residential proxy, weighted by
// health score. Proxies with higher scores (more successes than failures)
// are selected more often. Falls back to any proxy if all are blocked.
func pickProxy() *ResidentialProxy {
	blockedMu.RLock()
	defer blockedMu.RUnlock()

	now := time.Now()
	var available []*ResidentialProxy
	var weights []int
	totalWeight := 0
	for i := range residentialProxies {
		p := &residentialProxies[i]
		blockedAt, isBlocked := blockedProxies[p.IP]
		if !isBlocked || now.Sub(blockedAt) > blockDuration {
			available = append(available, p)
			// Score-based weight: base 1 + score/25, minimum 1, maximum 5
			score := proxyScore[p.IP]
			w := 1 + score/25
			if w < 1 {
				w = 1
			}
			if w > 5 {
				w = 5
			}
			weights = append(weights, w)
			totalWeight += w
		}
	}

	if len(available) == 0 {
		// All blocked — pick random anyway (may have recovered)
		idx := rand.Intn(len(residentialProxies))
		return &residentialProxies[idx]
	}

	// Weighted random selection
	r := rand.Intn(totalWeight)
	for i, w := range weights {
		r -= w
		if r < 0 {
			return available[i]
		}
	}
	return available[len(available)-1]
}

// pickProxySelection returns a ProxySelection for use by turbo.go.
func pickProxySelection() *ProxySelection {
	p := pickProxy()
	if p == nil {
		return nil
	}
	return &ProxySelection{
		Tier:        TierResidential,
		CroxyIP:     p.IP,
		Residential: p,
	}
}

// findProxyByIP returns the proxy for a given IP address, or nil.
func findProxyByIP(ip string) *ResidentialProxy {
	for i := range residentialProxies {
		if residentialProxies[i].IP == ip {
			return &residentialProxies[i]
		}
	}
	return nil
}

// markProxyBlocked adds a proxy IP to the blocked set with a timestamp.
// Tracks consecutive failures — after 3+ failures in a row, escalates to
// a 5-minute block (same as markProxyBlocked429) so consistently dead
// proxies are not retried every 10 seconds.
func markProxyBlocked(ip string) {
	host := ip
	if h, _, err := net.SplitHostPort(ip); err == nil {
		host = h
	}
	blockedMu.Lock()
	proxyFailureCount[host]++
	failures := proxyFailureCount[host]
	proxyScore[host] -= 5
	if proxyScore[host] < -100 {
		proxyScore[host] = -100
	}
	if failures >= maxConsecutiveFailuresBeforeEscalate {
		proxyFailureCount[host] = 0
		blockedProxies[host] = time.Now().Add(5*time.Minute - blockDuration)
	} else {
		blockedProxies[host] = time.Now()
	}
	blockedMu.Unlock()
}

// markProxyAuthFailed blocks a proxy that returns "Proxy Authentication Required".
// Auth can fail transiently due to provider rate-limiting. Blocks for 5 minutes
// instead of permanently, so the proxy gets a chance to recover.
func markProxyAuthFailed(ip string) {
	host := ip
	if h, _, err := net.SplitHostPort(ip); err == nil {
		host = h
	}
	blockedMu.Lock()
	proxyFailureCount[host]++
	proxyScore[host] -= 10
	if proxyScore[host] < -100 {
		proxyScore[host] = -100
	}
	const longBlock = 5 * time.Minute
	blockedProxies[host] = time.Now().Add(longBlock - blockDuration)
	blockedMu.Unlock()
	fmt.Printf("  \x1b[33m⛔ Proxy %s blocked for 5m — authentication failed\x1b[0m\n", ip)
}

// markProxyBlocked429 blocks a proxy for 5 minutes (Cloudflare 429 / error 1015).
// Works by back-dating the entry so the effective unblock time is 5 minutes from now.
func markProxyBlocked429(ip string) {
	host := ip
	if h, _, err := net.SplitHostPort(ip); err == nil {
		host = h
	}
	const longBlock = 5 * time.Minute
	blockedMu.Lock()
	proxyScore[host] -= 15
	if proxyScore[host] < -100 {
		proxyScore[host] = -100
	}
	// pickProxy unblocks when: now.Sub(blockedAt) > blockDuration(10s)
	// To block for longBlock(5min), set blockedAt = now + (longBlock - blockDuration)
	// so the proxy stays blocked until now + longBlock.
	blockedProxies[host] = time.Now().Add(longBlock - blockDuration)
	blockedMu.Unlock()
}

// markProxySuccess increases the health score of a proxy (called on successful join).
// Score is capped at +100 to prevent old data from dominating.
func markProxySuccess(ip string) {
	host := ip
	if h, _, err := net.SplitHostPort(ip); err == nil {
		host = h
	}
	blockedMu.Lock()
	proxyTotalAttempts[host]++
	proxyScore[host] += 10
	if proxyScore[host] > 100 {
		proxyScore[host] = 100
	}
	blockedMu.Unlock()
}

// unblockProxy removes a proxy from the blocked set (called on successful join).
// Also resets the consecutive failure counter so the proxy starts fresh.
func unblockProxy(ip string) {
	// Strip port if present
	host := ip
	if h, _, err := net.SplitHostPort(ip); err == nil {
		host = h
	}
	blockedMu.Lock()
	delete(blockedProxies, host)
	delete(proxyFailureCount, host)
	proxyScore[host] += 5 // gradual recovery bonus on unblock
	if proxyScore[host] > 100 {
		proxyScore[host] = 100
	}
	blockedMu.Unlock()
}

// releaseDirectSlot is a no-op placeholder for TierDirect compatibility.
func releaseDirectSlot() {}

// proxyResetLoop runs every 60 seconds and removes only proxies whose block
// has expired (handles the 10s default block). Proxies escalated to 5-minute
// blocks (3+ consecutive failures) are preserved — they stay blocked until
// the full duration expires. This prevents repeatedly retrying a dead proxy.
func proxyResetLoop() {
	for {
		time.Sleep(60 * time.Second)
		blockedMu.Lock()
		now := time.Now()
		for ip, blockedAt := range blockedProxies {
			if now.Sub(blockedAt) > blockDuration {
				delete(blockedProxies, ip)
				delete(proxyFailureCount, ip)
				// Score persists — decays naturally via markProxyBlocked calls
				// if the proxy is still problematic; otherwise it stays positive.
			}
		}
		total := len(blockedProxies)
		blockedMu.Unlock()
		if total > 0 {
			color.New(color.FgHiCyan).Printf("[Proxy Reset] %d proxies still blocked (escalated), expired entries cleaned\n", total)
		}
	}
}

// ProxyStats returns counts for the status API.
func ProxyStats() (total, available, blocked int) {
	blockedMu.RLock()
	defer blockedMu.RUnlock()

	now := time.Now()
	total = len(residentialProxies)
	blocked = 0
	for _, t := range blockedProxies {
		if now.Sub(t) <= blockDuration {
			blocked++
		}
	}
	available = total - blocked
	return
}

// ============================================================================
// HTTP client factory — routes through residential proxy
// ============================================================================
func newProxiedHTTPClient(proxy *ResidentialProxy) *http.Client {
	return &http.Client{
		Timeout: 8 * time.Second, // 8s to fail-fast and rotate proxies quickly
		Transport: &http.Transport{
			Proxy:               nil, // Disable transport proxy to let our manual DialContext do transparent tunneling
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			DisableKeepAlives:   true, // Disable keep-alives to enforce clean rotations per request
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialThroughProxy(proxy, addr)
			},
		},
	}
}

// ============================================================================
// WebSocket dialer factory — uses standard http.ProxyURL
// ============================================================================
func newProxiedWSDialer(proxy *ResidentialProxy) *websocket.Dialer {
	proxyURL, _ := url.Parse(proxy.ProxyURL())
	return &websocket.Dialer{
		Proxy: http.ProxyURL(proxyURL),
		NetDialContext: (&net.Dialer{
			Timeout:   20 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
		HandshakeTimeout: 30 * time.Second,
	}
}

// ============================================================================
// Manual CONNECT tunnel — most reliable for authenticated residential proxies
// ============================================================================
// dialThroughProxy establishes a TCP connection through the proxy using HTTP CONNECT.
// TCP keepalive (30s) is set to detect half-open connections through NAT/proxy timeouts.
// Single-shot — no retries. Caller (botConnectViaProxy) handles proxy rotation on failure.
func dialThroughProxy(proxy *ResidentialProxy, targetHost string) (net.Conn, error) {
	d := &net.Dialer{
		Timeout:   20 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	proxyConn, err := d.Dial("tcp", proxy.Address())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy %s: %w", proxy.Address(), err)
	}

	connectReq := fmt.Sprintf(
		"CONNECT %s HTTP/1.1\r\n"+
			"Host: %s\r\n"+
			"User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64)\r\n",
		targetHost, targetHost)
	if proxy.Username != "" {
		connectReq += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", proxy.BasicAuth())
	}
	connectReq += "\r\n"
	_, err = proxyConn.Write([]byte(connectReq))
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to send CONNECT: %w", err)
	}

	proxyConn.SetReadDeadline(time.Now().Add(20 * time.Second))
	resp, err := http.ReadResponse(bufio.NewReader(proxyConn), nil)
	if err != nil {
		proxyConn.Close()
		return nil, fmt.Errorf("failed to read CONNECT response: %w", err)
	}
	resp.Body.Close()
	proxyConn.SetReadDeadline(time.Time{})

	if resp.StatusCode == 407 {
		proxyConn.Close()
		return nil, fmt.Errorf("proxy CONNECT returned HTTP 407 (Proxy Authentication Required)")
	}
	if resp.StatusCode != 200 {
		proxyConn.Close()
		return nil, fmt.Errorf("proxy CONNECT returned HTTP %d", resp.StatusCode)
	}

	return proxyConn, nil
}

// newManualProxiedWSDialer creates a WebSocket dialer using manual CONNECT tunnel.
// This is the most reliable method for authenticated residential proxies.
func newManualProxiedWSDialer(proxy *ResidentialProxy) *websocket.Dialer {
	return &websocket.Dialer{
		NetDial: func(network, addr string) (net.Conn, error) {
			rawConn, err := dialThroughProxy(proxy, addr)
			if err != nil {
				return nil, err
			}
			return rawConn, nil
		},
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
		HandshakeTimeout: 30 * time.Second,
	}
}