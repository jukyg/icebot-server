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
	return fmt.Sprintf("http://%s:%s@%s:%s", p.Username, p.Password, p.IP, p.Port)
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
	{IP: "38.154.203.95", Port: "5863", Username: "zmvhoipo", Password: "jf1azmanzauy"},
	{IP: "198.105.121.200", Port: "6462", Username: "zmvhoipo", Password: "jf1azmanzauy"},
	{IP: "64.137.96.74", Port: "6641", Username: "zmvhoipo", Password: "jf1azmanzauy"},
	{IP: "209.127.138.10", Port: "5784", Username: "zmvhoipo", Password: "jf1azmanzauy"},
	{IP: "38.154.185.97", Port: "6370", Username: "zmvhoipo", Password: "jf1azmanzauy"},
	{IP: "84.247.60.125", Port: "6095", Username: "zmvhoipo", Password: "jf1azmanzauy"},
	{IP: "142.111.67.146", Port: "5611", Username: "zmvhoipo", Password: "jf1azmanzauy"},
	{IP: "191.96.254.138", Port: "6185", Username: "zmvhoipo", Password: "jf1azmanzauy"},
	{IP: "31.58.9.4", Port: "6077", Username: "zmvhoipo", Password: "jf1azmanzauy"},
	{IP: "64.137.10.153", Port: "5803", Username: "zmvhoipo", Password: "jf1azmanzauy"},
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
)

// blockDuration is how long a proxy stays blocked before retrying.
// Reduced to 10s for faster recovery
const blockDuration = 10 * time.Second

// maxConsecutiveFailuresBeforeEscalate escalates a proxy to a 5-minute block.
const maxConsecutiveFailuresBeforeEscalate = 3

// pickProxyByIndex returns a residential proxy by round-robin index.
// Does NOT check blocked state — used for deterministic initial assignment.
// Formula: proxyIndex = botNumericId % totalProxies
func pickProxyByIndex(index int) *ResidentialProxy {
	idx := index % len(residentialProxies)
	return &residentialProxies[idx]
}

// pickProxy returns a random non-blocked residential proxy.
// Falls back to any proxy if all are blocked.
func pickProxy() *ResidentialProxy {
	blockedMu.RLock()
	defer blockedMu.RUnlock()

	now := time.Now()
	var available []*ResidentialProxy
	for i := range residentialProxies {
		p := &residentialProxies[i]
		blockedAt, isBlocked := blockedProxies[p.IP]
		if !isBlocked || now.Sub(blockedAt) > blockDuration {
			available = append(available, p)
		}
	}

	if len(available) == 0 {
		// All blocked — pick random anyway (may have recovered)
		idx := rand.Intn(len(residentialProxies))
		return &residentialProxies[idx]
	}
	return available[rand.Intn(len(available))]
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
	blockedMu.Lock()
	proxyFailureCount[ip]++
	failures := proxyFailureCount[ip]
	if failures >= maxConsecutiveFailuresBeforeEscalate {
		proxyFailureCount[ip] = 0
		blockedProxies[ip] = time.Now().Add(5*time.Minute - blockDuration)
	} else {
		blockedProxies[ip] = time.Now()
	}
	blockedMu.Unlock()
}

// markProxyAuthFailed blocks a proxy that returns "Proxy Authentication Required".
// Auth can fail transiently due to provider rate-limiting. Blocks for 5 minutes
// instead of permanently, so the proxy gets a chance to recover.
func markProxyAuthFailed(ip string) {
	blockedMu.Lock()
	proxyFailureCount[ip]++
	const longBlock = 5 * time.Minute
	blockedProxies[ip] = time.Now().Add(longBlock - blockDuration)
	blockedMu.Unlock()
	fmt.Printf("  \x1b[33m⛔ Proxy %s blocked for 5m — authentication failed\x1b[0m\n", ip)
}

// markProxyBlocked429 blocks a proxy for 5 minutes (Cloudflare 429 / error 1015).
// Works by back-dating the entry so the effective unblock time is 5 minutes from now.
func markProxyBlocked429(ip string) {
	const longBlock = 5 * time.Minute
	blockedMu.Lock()
	// pickProxy unblocks when: now.Sub(blockedAt) > blockDuration(10s)
	// To block for longBlock(5min), set blockedAt = now + (longBlock - blockDuration)
	// so the proxy stays blocked until now + longBlock.
	blockedProxies[ip] = time.Now().Add(longBlock - blockDuration)
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
	blockedMu.Unlock()
}

// releaseDirectSlot is a no-op placeholder for TierDirect compatibility.
func releaseDirectSlot() {}

// proxyResetLoop runs every 60 seconds and clears all blocked proxy entries,
// allowing blocked proxies to recover and cycle back into the pool automatically.
func proxyResetLoop() {
	for {
		time.Sleep(60 * time.Second)
		blockedMu.Lock()
		blockedProxies = make(map[string]time.Time)
		proxyFailureCount = make(map[string]int)
		blockedMu.Unlock()
		color.New(color.FgHiCyan).Println("[Proxy Reset] All proxies unblocked and recycled into pool")
		LogInfo("Proxy", "All proxies unblocked — auto-reset cycle")
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
			"Proxy-Authorization: Basic %s\r\n"+
			"User-Agent: Mozilla/5.0 (Windows NT 10.0; Win64; x64)\r\n"+
			"\r\n",
		targetHost, targetHost, proxy.BasicAuth(),
	)
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