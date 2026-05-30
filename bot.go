package main

import (
	"context"
	"crypto/tls"
	"errors"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

// blankZero and blankOne are invisible Unicode characters used in nick generation
// (U+2800 Braille blank, U+3164 Hangul filler).
var blankZero = "\u2800"
var blankOne = "\u3164"

// proxyFailures tracks consecutive disconnect failures per proxy IP.
// After N failures the proxy is rotated (blocked) so new connections
// pick a fresh proxy instead of reusing a dead one.
var proxyFailures sync.Map // map[string]int

// sanitizeNick removes invisible suffix characters from a nickname for log
// display. The invisible characters (U+2800, U+3164) used in nick generation
// render as garbled Unicode in terminals and log files.
func sanitizeNick(nick string) string {
	if nick == "" {
		return ""
	}
	result := strings.ReplaceAll(nick, blankZero, "_")
	result = strings.ReplaceAll(result, blankOne, ".")
	return result
}

type Bot struct {
	session        *Session
	numericId      int
	garticId       atomic.Int64
	garticLongId   string
	nick           string
	proxyIp        string
	proxyTier      int
	premiumProxy   *PremiumProxy
	idioma         string // non-empty = public room join (language number)
	ws             *websocket.Conn
	wsMu           sync.Mutex
	alive          atomic.Bool
	joinSent       bool
	joinConfirmed  atomic.Bool
	joinedAt       time.Time
	exitedManually atomic.Bool
	lastMsg        atomic.Int64
	cancel         context.CancelFunc
}

// ConnectJob is a unit of work for the connect worker pool.
type ConnectJob struct {
	s              *Session
	bot            *Bot
	botNumId       int
	room           string
	nick           string
	avatar         string
	joinMessages   []JoinMessage
	serverOverride string
	idioma         string
}

// IsAlive returns true if the bot's WebSocket is still open
func (b *Bot) IsAlive() bool {
	return b.alive.Load()
}

// SendRaw sends a raw string message on the bot's WebSocket.
// Returns true if the message was sent successfully.
func (b *Bot) SendRaw(msg string) bool {
	b.wsMu.Lock()
	defer b.wsMu.Unlock()
	if b.ws != nil {
		b.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
		err := b.ws.WriteMessage(websocket.TextMessage, []byte(msg))
		b.ws.SetWriteDeadline(time.Time{})
		if err != nil {
			b.alive.Store(false)
			return false
		}
		return true
	}
	return false
}

// CloseWS gently closes the bot's WebSocket without cancelling the context.
func (b *Bot) CloseWS() {
	b.alive.Store(false)
	b.wsMu.Lock()
	if b.ws != nil {
		gid := b.garticId.Load()
		if gid > 0 {
			b.ws.SetWriteDeadline(time.Now().Add(1 * time.Second))
			b.ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("42[24,%d]", gid)))
			b.ws.SetWriteDeadline(time.Time{})
			time.Sleep(50 * time.Millisecond)
		}
		b.ws.Close()
		b.ws = nil
	}
	b.wsMu.Unlock()
}

// Destroy closes the bot's WebSocket and cancels its goroutine context.
func (b *Bot) Destroy() {
	b.alive.Store(false)
	if b.cancel != nil {
		b.cancel()
	}
	b.wsMu.Lock()
	if b.ws != nil {
		gid := b.garticId.Load()
		if gid > 0 {
			b.ws.SetWriteDeadline(time.Now().Add(1 * time.Second))
			b.ws.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("42[24,%d]", gid)))
			b.ws.SetWriteDeadline(time.Time{})
			time.Sleep(50 * time.Millisecond)
		}
		b.ws.Close()
		b.ws = nil
	}
	b.wsMu.Unlock()
}

// createBot performs the full bot creation flow.
func createBot(s *Session, room, baseName, nickMode, avatar string, customNicks []CustomNick, joinMessages []JoinMessage, serverOverride, idioma string) int {
	// Room-full guard: if 5+ consecutive bots got "room is full" rejection,
	// skip creating more bots to avoid wasted handshakes.
	if atomic.LoadInt32(&s.consecutiveRoomFull) >= 5 {
		s.Send(map[string]interface{}{
			"event": "roomFull",
			"msg":   "Room appears full — stopping bot creation",
		})
		return 0
	}

	s.mu.Lock()
	s.botCounter++
	botNumId := s.botCounter
	nr := generateNick(baseName, nickMode, customNicks, &s.queueNumber)
	bot := &Bot{
		session:   s,
		numericId: botNumId,
		nick:      nr.Nick,
		idioma:    idioma,
	}
	s.bots[botNumId] = bot
	s.mu.Unlock()

	// Determine avatar
	var finalAvatar string
	if nr.Avatar != nil {
		finalAvatar = fmt.Sprintf("%d", *nr.Avatar)
	} else if avatar == "random" {
		finalAvatar = fmt.Sprintf("%d", rand.Intn(37))
	} else if avatar == "null" {
		finalAvatar = "null"
	} else {
		finalAvatar = avatar
	}

	s.Send(map[string]interface{}{
		"event":     "botConnecting",
		"numericId": botNumId,
		"nick":      nr.Nick,
	})

	select {
	case s.connectPool <- ConnectJob{s, bot, botNumId, room, nr.Nick, finalAvatar, joinMessages, serverOverride, idioma}:
	case <-s.connectDone:
		s.mu.Lock()
		delete(s.bots, botNumId)
		s.mu.Unlock()
	}

	return botNumId
}

// ============================================================================
// botConnectViaProxy — Main connection dispatcher called by session connectWorker.
// Routes through residential proxies using HTTP CONNECT tunnel for all rooms.
// This supports both PRIVATE and PUBLIC (idioma) rooms correctly.
// ============================================================================
func botConnectViaProxy(s *Session, bot *Bot, botNumId int, room, nick, finalAvatar string, joinMessages []JoinMessage, serverOverride, idioma string) {
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	// Overall connect timeout: give up after 120s instead of trying every proxy.
	// If all proxies are blocked this prevents a 50-worker pileup on retries.
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer connectCancel()

	var lastErr error
	totalProxies := len(residentialProxies)
	maxAttempts := totalProxies
	if maxAttempts > 5 {
		maxAttempts = 5 // limit to 5 proxy attempts per bot connect
	}
	if totalProxies == 0 {
		removeBotOnFail(s, bot, botNumId, "no_proxy_available")
		return
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if connectCtx.Err() != nil {
			lastErr = connectCtx.Err()
			break
		}
		proxy := pickProxyByIndex(botNumId + attempt - 1)
		bot.proxyIp = proxy.Address()
		bot.proxyTier = TierResidential

		if !s.quietLogs.Load() && attempt > 1 {
			color.New(color.FgYellow).Printf("  Bot %d: Retrying connection (Attempt %d/%d) via proxy %s...\n", botNumId, attempt, totalProxies, proxy.Address())
		}

		// ----------------------------------------------------------------
		// Step 1: Fetch server info through the residential proxy (HTTP)
		// ----------------------------------------------------------------
		var fetchURL string
		if idioma != "" {
			fetchURL = fmt.Sprintf("https://gartic.io/server/?check=1&v3=1&lang=%s", idioma)
		} else {
			fetchURL = fmt.Sprintf("https://gartic.io/server/?check=1&v3=1&room=%s", room)
		}

		proxyClient := newProxiedHTTPClient(proxy)
		fetchCtx, fetchCancel := context.WithTimeout(connectCtx, 8*time.Second)
		req, err := http.NewRequestWithContext(fetchCtx, "GET", fetchURL, nil)
		if err != nil {
			fetchCancel()
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		req.Header.Set("Origin", "https://gartic.io")
		req.Header.Set("Referer", "https://gartic.io/")

		resp, err := proxyClient.Do(req)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				if !s.quietLogs.Load() {
					red.Printf("  Bot %d: proxy %s timed out (8s)\n", botNumId, proxy.Address())
				}
			} else if !s.quietLogs.Load() {
				red.Printf("  Bot %d: fetch failed via proxy %s: %s\n", botNumId, proxy.Address(), err.Error())
			}
			errStr := err.Error()
			if strings.Contains(errStr, "407") || strings.Contains(errStr, "Authentication Required") {
				markProxyAuthFailed(proxy.IP)
			} else {
				markProxyBlocked(proxy.IP)
			}
			fetchCancel()
			lastErr = err
			continue
		}
		fetchCancel()

		// Detect Cloudflare 429/403 immediately — block proxy for 5 minutes
		if resp.StatusCode == 429 || resp.StatusCode == 403 {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if !s.quietLogs.Load() {
				red.Printf("  Bot %d: proxy %s returned HTTP %d (Cloudflare block)\n", botNumId, proxy.Address(), resp.StatusCode)
			}
			markProxyBlocked429(proxy.IP)
			lastErr = fmt.Errorf("http_%d", resp.StatusCode)
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		text := string(bodyBytes)

		// ----------------------------------------------------------------
		// Step 2: Parse server redirect from response
		// ----------------------------------------------------------------
		if !strings.Contains(text, "https://") || !strings.Contains(text, "gartic.io") || !strings.Contains(text, "?c=") {
			if !s.quietLogs.Load() {
				red.Printf("  Bot %d: proxy %s returned invalid response or Cloudflare block (1015/429)\n", botNumId, proxy.Address())
			}
			markProxyBlocked429(proxy.IP)
			lastErr = fmt.Errorf("invalid_server_response")
			continue
		}

		extractedServer := strings.Split(strings.Split(text, "https://")[1], ".")[0]
		finalServer := extractedServer
		if serverOverride != "" {
			finalServer = "server" + serverOverride
		}
		code := strings.Split(text, "?c=")[1]
		if idx := strings.IndexAny(code, " \r\n\"&"); idx > 0 {
			code = code[:idx]
		}

		var cookieParts []string
		for _, c := range resp.Header.Values("Set-Cookie") {
			nameVal := strings.Split(c, ";")[0]
			if nameVal != "" && !strings.HasSuffix(nameVal, "=") {
				cookieParts = append(cookieParts, nameVal)
			}
		}
		botCookieHeader := strings.Join(cookieParts, "; ")

		s.srvCache.set(finalServer, code, proxy.IP, botCookieHeader)

		// ----------------------------------------------------------------
		// Step 3: Connect WebSocket through residential proxy (CONNECT tunnel)
		// ----------------------------------------------------------------
		wsURL := fmt.Sprintf("wss://%s.gartic.io/socket.io/?c=%s&EIO=3&transport=websocket", finalServer, code)

		dialer := newManualProxiedWSDialer(proxy)
		headers := http.Header{}
		headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		headers.Set("Origin", "https://gartic.io")
		if botCookieHeader != "" {
			headers.Set("Cookie", botCookieHeader)
		}

		ws, wsResp, err := dialer.Dial(wsURL, headers)
		if err != nil {
			if !s.quietLogs.Load() {
				detail := ""
				if wsResp != nil {
					bodySnip, _ := io.ReadAll(io.LimitReader(wsResp.Body, 200))
					wsResp.Body.Close()
					detail = fmt.Sprintf(" [HTTP %d: %s]", wsResp.StatusCode, strings.TrimSpace(string(bodySnip)))
				}
				red.Printf("  Bot %d: WS connect failed via %s: %s%s\n", botNumId, proxy.Address(), err.Error(), detail)
			}
			errStr := err.Error()
			if strings.Contains(errStr, "407") || strings.Contains(errStr, "Authentication Required") {
				markProxyAuthFailed(proxy.IP)
			} else if wsResp != nil && wsResp.StatusCode == 429 {
				markProxyBlocked429(proxy.IP)
			} else {
				markProxyBlocked(proxy.IP)
			}
			lastErr = err
			continue
		}

		s.mu.RLock()
		_, tracked := s.bots[botNumId]
		s.mu.RUnlock()
		if !tracked {
			ws.Close()
			return
		}

		ctx, cancel := context.WithCancel(context.Background())
		bot.ws = ws
		bot.alive.Store(true)
		bot.cancel = cancel

		if !s.quietLogs.Load() {
			green.Printf("  Bot %d connected via proxy %s: %s\n", botNumId, proxy.Address(), sanitizeNick(nick))
		}

		go botMessageLoop(ctx, s, bot, botNumId, room, nick, finalAvatar, joinMessages)
		return
	}

	errMsg := "connection_failed"
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	removeBotOnFail(s, bot, botNumId, errMsg)
}

// ============================================================================
// botConnect — Legacy CroxyProxy path (kept for turbo.go compatibility)
// ============================================================================
func botConnect(s *Session, bot *Bot, botNumId int, ip, room, nick, finalAvatar string, joinMessages []JoinMessage, serverOverride string, useCache bool, idioma string) {
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)

	cookieHeader := getCookieForProxy(ip)

	var finalServer, code, botCookieHeader string
	usedCache := false

	// Try server-info cache first
	if useCache {
		cachedServer, cachedCode, cachedIPCookies, ok := s.srvCache.get(ip)
		if ok {
			finalServer = cachedServer
			if serverOverride != "" {
				finalServer = "server" + serverOverride
			}
			code = cachedCode
			var parts []string
			if cachedIPCookies != "" {
				parts = append(parts, cachedIPCookies)
			}
			if cookieHeader != "" {
				parts = append(parts, cookieHeader)
			}
			botCookieHeader = strings.Join(parts, "; ")
			usedCache = true
		}
	}

	if !usedCache {
		var fetchURL string
		if idioma != "" {
			fetchURL = fmt.Sprintf("https://%s/server/?check=1&v3=1&lang=%s&__cpo=aHR0cHM6Ly9nYXJ0aWMuaW8#", ip, idioma)
		} else {
			fetchURL = fmt.Sprintf("https://%s/server/?check=1&v3=1&room=%s&__cpo=aHR0cHM6Ly9nYXJ0aWMuaW8#", ip, room)
		}
		req, err := http.NewRequest("GET", fetchURL, nil)
		if err != nil {
			removeBotOnFail(s, bot, botNumId, "request_create_failed")
			return
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		req.Header.Set("Origin", "https://gartic.io")
		req.Header.Set("Referer", "https://gartic.io/")
		if cookieHeader != "" {
			req.Header.Set("Cookie", cookieHeader)
		}

		resp, err := httpClient.Do(req)
		if err != nil {
			if !s.quietLogs.Load() {
				red.Printf("  Bot %d: fetch failed for %s: %s\n", botNumId, ip, err.Error())
			}
			removeBotOnFail(s, bot, botNumId, err.Error())
			return
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		text := string(bodyBytes)

		if !strings.Contains(text, "https://") || !strings.Contains(text, "gartic.io") || !strings.Contains(text, "?c=") {
			if !s.quietLogs.Load() {
				red.Printf("  Bot %d: proxy %s returned invalid response\n", botNumId, ip)
			}
			removeBotOnFail(s, bot, botNumId, "invalid_proxy_response")
			return
		}

		extractedServer := strings.Split(strings.Split(text, "https://")[1], ".")[0]
		finalServer = extractedServer
		if serverOverride != "" {
			finalServer = "server" + serverOverride
		}
		code = strings.Split(text, "?c=")[1]
		if idx := strings.IndexAny(code, " \r\n\"&"); idx > 0 {
			code = code[:idx]
		}

		var cookieParts []string
		for _, c := range resp.Header.Values("Set-Cookie") {
			nameVal := strings.Split(c, ";")[0]
			if nameVal != "" && !strings.HasSuffix(nameVal, "=") {
				cookieParts = append(cookieParts, nameVal)
			}
		}
		ipCookies := strings.Join(cookieParts, "; ")
		if cookieHeader != "" {
			cookieParts = append(cookieParts, cookieHeader)
		}
		botCookieHeader = strings.Join(cookieParts, "; ")
		s.srvCache.set(extractedServer, code, ip, ipCookies)
	}

	innerURL := fmt.Sprintf("wss://%s.gartic.io/socket.io/?c=%s&EIO=3&transport=websocket", finalServer, code)
	encodedURL := base64.StdEncoding.EncodeToString([]byte(innerURL))
	wsURL := fmt.Sprintf("wss://%s/__cpw.php?u=%s&o=aHR0cHM6Ly9nYXJ0aWMuaW8=", ip, encodedURL)

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		NetDialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		HandshakeTimeout: 10 * time.Second,
	}
	headers := http.Header{}
	headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	if botCookieHeader != "" {
		headers.Set("Cookie", botCookieHeader)
	}

	ws, wsResp, err := dialer.Dial(wsURL, headers)
	if err != nil {
		if !s.quietLogs.Load() {
			detail := ""
			if wsResp != nil {
				bodySnip, _ := io.ReadAll(io.LimitReader(wsResp.Body, 200))
				wsResp.Body.Close()
				detail = fmt.Sprintf(" [HTTP %d: %s]", wsResp.StatusCode, strings.TrimSpace(string(bodySnip)))
			}
			red.Printf("  Bot %d: WS connect failed for %s: %s%s\n", botNumId, ip, err.Error(), detail)
		}
		markProxyBlocked(ip)
		if usedCache {
			s.srvCache.invalidate()
			botConnect(s, bot, botNumId, ip, room, nick, finalAvatar, joinMessages, serverOverride, false, idioma)
			return
		}
		removeBotOnFail(s, bot, botNumId, "ws_connect_failed")
		return
	}

	s.mu.RLock()
	_, tracked := s.bots[botNumId]
	s.mu.RUnlock()
	if !tracked {
		ws.Close()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	bot.ws = ws
	bot.alive.Store(true)
	bot.cancel = cancel

	if !s.quietLogs.Load() {
		green.Printf("  Bot %d connected: %s\n", botNumId, sanitizeNick(nick))
	}

	go botMessageLoop(ctx, s, bot, botNumId, room, nick, finalAvatar, joinMessages)
}

// ============================================================================
// botConnectHTTP — HTTP Connect path used by turbo pool (direct/premium proxy)
// ============================================================================
func botConnectHTTP(s *Session, bot *Bot, botNumId int, proxyURL interface{}, proxyIP string, proxyTier int, premiumProxy *PremiumProxy, room, nick, finalAvatar string, joinMessages []JoinMessage, serverOverride, idioma string) {
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)

	var fetchURL string
	if idioma != "" {
		fetchURL = fmt.Sprintf("https://gartic.io/server/?check=1&v3=1&lang=%s", idioma)
	} else {
		fetchURL = fmt.Sprintf("https://gartic.io/server/?check=1&v3=1&room=%s", room)
	}

	var client *http.Client
	if proxyURL != nil {
		if _, ok := proxyURL.(string); ok {
			proxy := findProxyByIP(proxyIP)
			if proxy != nil {
				client = newProxiedHTTPClient(proxy)
			}
		}
	}
	if client == nil {
		client = httpClient
	}

	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		removeBotOnFail(s, bot, botNumId, "request_create_failed")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://gartic.io")
	req.Header.Set("Referer", "https://gartic.io/")

	resp, err := client.Do(req)
	if err != nil {
		if !s.quietLogs.Load() {
			red.Printf("  Bot %d: HTTP fetch failed: %s\n", botNumId, err)
		}
		removeBotOnFail(s, bot, botNumId, "fetch_failed")
		return
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	text := string(bodyBytes)

	if !strings.Contains(text, "https://") || !strings.Contains(text, "gartic.io") || !strings.Contains(text, "?c=") {
		if !s.quietLogs.Load() {
			red.Printf("  Bot %d: invalid server response\n", botNumId)
		}
		removeBotOnFail(s, bot, botNumId, "invalid_server_response")
		return
	}

	extractedServer := strings.Split(strings.Split(text, "https://")[1], ".")[0]
	finalServer := extractedServer
	if serverOverride != "" {
		finalServer = "server" + serverOverride
	}
	code := strings.Split(text, "?c=")[1]
	if idx := strings.IndexAny(code, " \r\n\"&"); idx > 0 {
		code = code[:idx]
	}

	var cookieParts []string
	for _, c := range resp.Header.Values("Set-Cookie") {
		nameVal := strings.Split(c, ";")[0]
		if nameVal != "" && !strings.HasSuffix(nameVal, "=") {
			cookieParts = append(cookieParts, nameVal)
		}
	}
	botCookieHeader := strings.Join(cookieParts, "; ")

	wsURL := fmt.Sprintf("wss://%s.gartic.io/socket.io/?c=%s&EIO=3&transport=websocket", finalServer, code)

	var wsDialer *websocket.Dialer
	proxy := findProxyByIP(proxyIP)
	if proxy != nil {
		wsDialer = newManualProxiedWSDialer(proxy)
	} else {
		wsDialer = &websocket.Dialer{
			TLSClientConfig:  &tls.Config{InsecureSkipVerify: true},
			HandshakeTimeout: 15 * time.Second,
		}
	}

	headers := http.Header{}
	headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	headers.Set("Origin", "https://gartic.io")
	if botCookieHeader != "" {
		headers.Set("Cookie", botCookieHeader)
	}

	ws, wsResp, err := wsDialer.Dial(wsURL, headers)
	if err != nil {
		if !s.quietLogs.Load() {
			detail := ""
			if wsResp != nil {
				bodySnip, _ := io.ReadAll(io.LimitReader(wsResp.Body, 200))
				wsResp.Body.Close()
				detail = fmt.Sprintf(" [HTTP %d: %s]", wsResp.StatusCode, strings.TrimSpace(string(bodySnip)))
			}
			red.Printf("  Bot %d: WS connect failed: %s%s\n", botNumId, err.Error(), detail)
		}
		removeBotOnFail(s, bot, botNumId, "ws_connect_failed")
		return
	}

	s.mu.RLock()
	_, tracked := s.bots[botNumId]
	s.mu.RUnlock()
	if !tracked {
		ws.Close()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	bot.ws = ws
	bot.proxyIp = proxyIP
	bot.proxyTier = proxyTier
	bot.premiumProxy = premiumProxy
	bot.alive.Store(true)
	bot.cancel = cancel

	if !s.quietLogs.Load() {
		green.Printf("  Bot %d connected (HTTP): %s\n", botNumId, sanitizeNick(nick))
	}

	go botMessageLoop(ctx, s, bot, botNumId, room, nick, finalAvatar, joinMessages)
}

// ============================================================================
// botMessageLoop — The main bot read loop. Handles all socket.io events.
// ============================================================================
func botMessageLoop(ctx context.Context, s *Session, bot *Bot, botNumId int, room, nick, finalAvatar string, joinMessages []JoinMessage, firstMsg ...string) {
	red := color.New(color.FgRed)
	cyan := color.New(color.FgCyan)

	ws := bot.ws
	ws.SetReadLimit(4 << 20) // 4MB max message
	ws.SetPingHandler(func(data string) error {
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return ws.WriteControl(websocket.PongMessage,
			[]byte(data), time.Now().Add(4*time.Second))
	})
	ws.SetReadDeadline(time.Now().Add(60 * time.Second))

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				deadline := time.Now().Add(4 * time.Second)
				if err := ws.WriteControl(websocket.PingMessage,
					nil, deadline); err != nil {
					return
				}
				ws.SetReadDeadline(time.Now().Add(60 * time.Second))
			case <-done:
				return
			}
		}
	}()
	defer close(done)
	defer func() {
		// Release premium proxy slot
		if bot.premiumProxy != nil {
			bot.premiumProxy.release()
			bot.premiumProxy = nil
		}

		uptime := 0
		if bot.joinConfirmed.Load() {
			uptime = int(time.Since(bot.joinedAt).Seconds())
		}
		if !s.quietLogs.Load() {
			displayNick := bot.nick
			if displayNick == "" {
				displayNick = nick
			}
			red.Printf("  Bot %d (%s) disconnected — uptime=%ds proxy=%s joined=%v\n",
				botNumId, sanitizeNick(displayNick), uptime, bot.proxyIp, bot.joinConfirmed.Load())
		}

		// Rotate proxy after repeated failures
		if bot.proxyIp != "" && bot.joinConfirmed.Load() {
			proxyIP := bot.proxyIp
			if shouldRotateProxy(proxyIP) {
				color.New(color.FgYellow).Printf("  Bot %d: proxy %s failed 3 times — blocking for 20m\n", botNumId, proxyIP)
				markProxyBlocked(proxyIP)
			}
		}

		if bot.exitedManually.Load() {
			return
		}

		wasJoined := bot.joinConfirmed.Load()

		s.mu.Lock()
		if botNumId == s.reporterBotId {
			s.electReporterLocked()
		}
		bot.Destroy()
		delete(s.bots, botNumId)

		isOrph := s.orphaned
		autoRejoinOn := s.autoRejoin.Load()
		isAutoJoining := s.isAutoJoining.Load()

		// Count remaining joined bots (only when we might auto-rejoin)
		joinedRemaining := 0
		if wasJoined && !isOrph && autoRejoinOn {
			for _, b := range s.bots {
				if b.joinConfirmed.Load() {
					joinedRemaining++
				}
			}
		}
		s.mu.Unlock()

		// Send disconnect notification to extension (only if not auto-joining)
		if !isAutoJoining {
			s.Send(map[string]interface{}{
				"event":     "botDisconnected",
				"numericId": botNumId,
				"nick":      bot.nick,
			})
		}

		// Auto-rejoin logic (persistent bots — NEVER DISCONNECT mode):
		// Only fires for bots that had already confirmed joining (wasJoined).
		// This preserves the normal manual-join flow: if a bot fails during
		// the initial handshake it does NOT trigger startRejoinAutoJoin, which
		// would otherwise conflict with the user's in-progress join attempt.
		// Throttled via tryBeginRejoin so only one rejoin runs at a time.
		if wasJoined && autoRejoinOn && !isAutoJoining {
			if s.tryBeginRejoin() {
				defer s.endRejoin()
				s.mu.RLock()
				cfg := s.autoRejoinConfig
				s.mu.RUnlock()
				if cfg != nil && cfg.Idioma == "" {
					qty := cfg.Target
					if qty <= 0 {
						qty = 1
					}
					// Count remaining joined bots to decide restart vs. top-up
					joinedRemaining := 0
					s.mu.RLock()
					for _, b := range s.bots {
						if b.joinConfirmed.Load() {
							joinedRemaining++
						}
					}
					s.mu.RUnlock()
					if joinedRemaining == 0 {
						cyan.Printf("[auto-rejoin] Bot died, no observers — starting autojoin (room=%s)\n", cfg.Room)
						go s.startRejoinAutoJoin(cfg)
					} else {
						cyan.Printf("[auto-rejoin] Bot died — dupe join x%d (room=%s)\n", qty, cfg.Room)
						go func() {
							time.Sleep(time.Duration(2000+rand.Intn(3000)) * time.Millisecond)
							s.joinWithTurbo(cfg, qty)
						}()
					}
				}
			}
		}
	}()

	// Join confirmation timeout (90s — increased for slow proxies)
	joinTimer := time.AfterFunc(90*time.Second, func() {
		if !bot.joinConfirmed.Load() {
			bot.Destroy()
		}
	})
	defer joinTimer.Stop()

	// Application-level heartbeat every 12s — sends 42[42,garticId] to:
	// 1. Keep the proxy TCP tunnel alive (prevent NAT/proxy half-open timeout)
	// 2. Detect dead connections early via SendRaw failure
	// 3. Signal to Gartic this bot is active (anti-AFK)
	// This is the equivalent of wsProxy.js's Worker-backed keepalive.
	go func() {
		hbTicker := time.NewTicker(12 * time.Second)
		defer hbTicker.Stop()
		for {
			select {
			case <-hbTicker.C:
				gid := int(bot.garticId.Load())
				if gid > 0 && bot.alive.Load() {
					if !bot.SendRaw(fmt.Sprintf("42[42,%d]", gid)) {
						return
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Private Mode answer loop: sends the answer with a safe buffer (400ms ticker)
	// to avoid Gartic's anti-bot kicking while still answering within <1s.
	go func() {
		pmTicker := time.NewTicker(400 * time.Millisecond)
		defer pmTicker.Stop()
		for {
			select {
			case <-pmTicker.C:
				if !bot.alive.Load() {
					continue
				}
				s.mu.RLock()
				pm := s.privateMode
				word := s.currentDrawWord
				s.mu.RUnlock()
				if !pm || word == "" {
					continue
				}
				gid := bot.garticId.Load()
				if gid == 0 {
					continue
				}
				if !bot.SendRaw(fmt.Sprintf(`42[13,%d,%s]`, int(gid), jsonString(word))) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	var pendingMsg string
	if len(firstMsg) > 0 {
		pendingMsg = firstMsg[0]
	}

	for {
		var msg string
		if pendingMsg != "" {
			msg = pendingMsg
			pendingMsg = ""
		} else {
			ws.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, raw, err := ws.ReadMessage()
			if err != nil {
				return
			}
			msg = string(raw)
			ws.SetReadDeadline(time.Now().Add(60 * time.Second))
			bot.lastMsg.Store(time.Now().Unix())
		}

		// Socket.io handshake
		if msg == "40" {
			if bot.joinSent {
				continue
			}
			bot.joinSent = true
			tok := FetchTurnstileToken()
			if tok == "" {
				if !s.quietLogs.Load() {
					red.Printf("  Bot %d: no Turnstile token — aborting\n", botNumId)
				}
				bot.Destroy()
				return
			}
			var joinPayload string
			if bot.idioma != "" {
				joinPayload = fmt.Sprintf(`42[1,{"v":20000,"token":"%s","nick":"%s","avatar":%s,"platform":2,"idioma":%s}]`,
					tok, nick, finalAvatar, bot.idioma)
			} else {
				sala := room
				if len(room) > 2 {
					sala = room[2:]
				}
				joinPayload = fmt.Sprintf(`42[3,{"v":20000,"token":"%s","nick":"%s","avatar":%s,"platform":2,"sala":"%s"}]`,
					tok, nick, finalAvatar, sala)
			}
			bot.SendRaw(joinPayload)
			go ReleaseTurnstileToken(tok)
			continue
		}

		// Socket.io application-level heartbeat — MUST reply instantly
		if msg == "2" {
			ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
			ws.WriteMessage(websocket.TextMessage, []byte("3"))
			ws.SetReadDeadline(time.Now().Add(60 * time.Second))
			continue
		}

		// Socket.io open acknowledgement
		if strings.HasPrefix(msg, "0{") || msg == "0" {
			continue
		}

		// Parse socket.io event
		if !strings.HasPrefix(msg, "42") {
			continue
		}

		var parsed []interface{}
		if err := json.Unmarshal([]byte(msg[2:]), &parsed); err != nil {
			continue
		}
		if len(parsed) == 0 {
			continue
		}
		eventType := fmt.Sprintf("%v", parsed[0])

		switch eventType {
		case "5": // Join confirmed
			bot.joinConfirmed.Store(true)
			bot.joinedAt = time.Now()
			joinTimer.Stop()
			unblockProxy(bot.proxyIp)
			markProxySuccess(bot.proxyIp)
			s.resetRejoinBackoff()
			atomic.StoreInt32(&s.consecutiveRoomFull, 0)
			LogSuccess("Bot", fmt.Sprintf("Bot #%d (%s) joined room %s [garticId=%d]", botNumId, bot.nick, room, bot.garticId.Load()))

			if len(parsed) > 1 {
				bot.garticLongId = fmt.Sprintf("%v", parsed[1])
			}
			if len(parsed) > 2 {
				if id, ok := parsed[2].(float64); ok {
					bot.garticId.Store(int64(id))
					// Log the garticId assignment for debugging
					if !s.quietLogs.Load() {
						cyan.Printf("[Bot %d] Joined successfully! Assigned garticId=%d\n", botNumId, int64(id))
					}
				}
			} else if !s.quietLogs.Load() {
				red.Printf("[Bot %d] WARNING: Join confirmed but no garticId in response!\n", botNumId)
			}

			// Post-join signal — REQUIRED
			bot.SendRaw(fmt.Sprintf("42[46,%d]", int(bot.garticId.Load())))

			// Send join messages
			gid := int(bot.garticId.Load())
			for _, m := range joinMessages {
				switch m.Type {
				case "broadcast":
					bot.SendRaw(fmt.Sprintf(`42[13,%d,%s]`, gid, jsonString(m.Msg)))
				case "message":
					bot.SendRaw(fmt.Sprintf(`42[13,%d,%s]`, gid, jsonString(m.Msg)))
				case "answer":
					bot.SendRaw(fmt.Sprintf(`42[13,%d,%s]`, gid, jsonString(m.Msg)))
				}
			}

			s.mu.Lock()
			if s.reporterBotId == 0 {
				s.reporterBotId = botNumId
			}
			s.mu.Unlock()

			if !s.isAutoJoining.Load() {
				s.Send(map[string]interface{}{
					"event":     "botJoined",
					"numericId": botNumId,
					"garticId":  int(bot.garticId.Load()),
					"nick":      bot.nick,
					"room":      room,
				})
				if len(parsed) > 5 {
					if users, ok := parsed[5].([]interface{}); ok {
						s.Send(map[string]interface{}{
							"event":  "roomUsers",
							"users":  users,
							"source": botNumId,
						})
					}
				}
			}

		case "23": // User joined
			if len(parsed) > 1 {
				s.SendGameEvent(botNumId, map[string]interface{}{
					"event":  "userJoined",
					"user":   parsed[1],
					"source": botNumId,
				})
			}

		case "24": // User left
			if len(parsed) > 1 {
				s.SendGameEvent(botNumId, map[string]interface{}{
					"event":  "userLeft",
					"userId": parsed[1],
					"source": botNumId,
				})

				if s.autoRejoin.Load() {
					s.mu.RLock()
					cfg := s.autoRejoinConfig
					isOrph := s.orphaned
					s.mu.RUnlock()
					if isOrph && cfg != nil && cfg.Idioma == "" && s.tryBeginRejoin() {
						qty := cfg.Target
						if qty <= 0 {
							qty = 1
						}
						cyan.Printf("[auto-rejoin] User left — dupe join x%d (room=%s)\n", qty, room)
						go func() {
							time.Sleep(time.Duration(2000+rand.Intn(3000)) * time.Millisecond)
							s.joinWithTurbo(cfg, qty)
							s.endRejoin()
						}()
					}
				}
			}

		case "11": // Chat message
			if len(parsed) > 2 {
				chatUserID := fmt.Sprintf("%v", parsed[1])
				chatMsg := fmt.Sprintf("%v", parsed[2])

				s.SendGameEvent(botNumId, map[string]interface{}{
					"event":  "chatMessage",
					"userId": chatUserID,
					"msg":    chatMsg,
					"source": botNumId,
				})

			}

		case "16": // Turn signal — drawing round started
			s.SendGameEvent(botNumId, map[string]interface{}{
				"event":  "turnSignal",
				"data":   parsed,
				"source": botNumId,
			})
			s.mu.RLock()
			af := s.autofarm
			priv := s.privateMode
			ar := s.answerReveal
			s.mu.RUnlock()
			// Extract the word from parsed[1].
			// Gartic may send flat [16, "word", ...] or nested [16, ["word", ...]].
			// Handle both to be safe — try direct string first, then nested array.
			var word string
			if len(parsed) > 1 {
				if w, ok := parsed[1].(string); ok {
					word = w
				} else if arr, ok := parsed[1].([]interface{}); ok && len(arr) > 0 {
					if w, ok := arr[0].(string); ok {
						word = w
					}
				}
			}
			// 0) Answer Reveal — send word to extension if our bot is the drawer
			if ar && word != "" {
				s.Send(map[string]interface{}{
					"event":  "answerReveal",
					"word":   word,
					"botId":  botNumId,
					"botNumericId": botNumId,
				})
				if !s.quietLogs.Load() {
					cyan.Printf("[Bot %d] Answer Reveal: word=%q\n", botNumId, word)
				}
			}
			// 1) Store the word for the Private Mode answer loop
			if priv && word != "" {
				s.mu.Lock()
				s.currentDrawWord = word
				s.mu.Unlock()
				if !s.quietLogs.Load() {
					cyan.Printf("[Bot %d] Stored word=%q for answer loop\n", botNumId, word)
				}
			}
			// 2) Take the pencil in both modes to hold the turn
			currentGarticId := bot.garticId.Load()
			if (af || priv) && currentGarticId != 0 {
				if !s.quietLogs.Load() {
					cyan.Printf("[Bot %d] Taking pencil (garticId=%d) af=%v priv=%v\n", botNumId, currentGarticId, af, priv)
				}
				go func(gid int64) {
					time.Sleep(150 * time.Millisecond)
					bot.SendRaw(fmt.Sprintf("42[34,%d]", int(gid)))
				}(currentGarticId)
			} else if currentGarticId == 0 && !s.quietLogs.Load() {
				red.Printf("[Bot %d] WARNING: Cannot act - garticId is 0!\n", botNumId)
			}

		case "45": // Incoming kick vote (someone voted to kick THIS bot)
			// NOTE: This is NOT a confirmation of our outgoing vote.
			// Gartic sends event 45 only to the TARGET of the kick vote,
			// not back to the voter. When WE initiate a kick, our bots
			// never receive event 45 back — only the target player does.
			// These events show when OTHER players vote to kick one of
			// our bots. See session.go "kick" for the send-side tracking.
			if len(parsed) > 2 {
				s.SendGameEvent(botNumId, map[string]interface{}{
					"event":    "kickVote",
					"voterId":  parsed[1],
					"targetId": parsed[2],
					"source":   botNumId,
				})
			}

		case "6": // Kicked/removed from room
			if !bot.joinConfirmed.Load() {
				subCode := "?"
				reason := "unknown"
				if len(parsed) > 1 {
					subCode = fmt.Sprintf("%v", parsed[1])
					switch subCode {
					case "3":
						reason = "room is full"
						atomic.AddInt32(&s.consecutiveRoomFull, 1)
					case "4":
						reason = "already playing (fingerprint dedup)"
					case "5":
						reason = "Turnstile token rejected"
					case "7":
						reason = "banned"
					}
				}
			if !s.quietLogs.Load() {
				red.Printf("  Bot %d: gartic rejected join with code %s (%s)\n", botNumId, subCode, reason)
			}
			LogWarn("Bot", fmt.Sprintf("Bot #%d rejected: code %s (%s)", botNumId, subCode, reason))
			bot.Destroy()
				return
			}
			if s.autoRejoin.Load() {
				if s.orphaned {
					s.mu.RLock()
					cfg := s.autoRejoinConfig
					s.mu.RUnlock()
					if cfg != nil && cfg.Idioma == "" && s.tryBeginRejoin() {
						qty := cfg.Target
						if qty <= 0 {
							qty = 1
						}
						cyan.Printf("[auto-rejoin] Bot kicked — dupe join x%d (room=%s)\n", qty, room)
						go func() {
							time.Sleep(time.Duration(2000+rand.Intn(3000)) * time.Millisecond)
							s.joinWithTurbo(cfg, qty)
							s.endRejoin()
						}()
					}
				}
			}

		case "42": // AFK check
			if bot.garticId.Load() != 0 {
				bot.SendRaw(fmt.Sprintf("42[42,%d]", int(bot.garticId.Load())))
			}

		case "47": // Report prompt — intentionally ignored
			// Do NOT auto-respond to event 47 (report prompt).
			// Responding with 42[35] here would report whoever is drawing,
			// which is the opposite of what autofarm should do.
			// Manual report is still available via the REPORT button.
		}
	}
}

// shouldRotateProxy tracks consecutive failures per proxy and returns
// true when a proxy should be rotated (blocked) after 3+ failures.
func shouldRotateProxy(proxy string) bool {
	val, _ := proxyFailures.LoadOrStore(proxy, 0)
	count := val.(int) + 1
	proxyFailures.Store(proxy, count)
	if count >= 3 {
		proxyFailures.Delete(proxy)
		return true
	}
	return false
}

func removeBotOnFail(s *Session, bot *Bot, botNumId int, reason string) {
	s.mu.Lock()
	delete(s.bots, botNumId)
	s.mu.Unlock()
	if !s.isAutoJoining.Load() {
		s.Send(map[string]interface{}{
			"event":     "botFailed",
			"numericId": botNumId,
			"reason":    reason,
		})
	}
}