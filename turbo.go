package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

type TurboConfig struct {
	Room   string
	Server string
	Target int
	Idioma string // non-empty = public room join
}

type TurboEntry struct {
	ws            *websocket.Conn
	wsMu          sync.Mutex
	proxyIp       string
	proxyTier     int           // TierCroxy, TierPremium, or TierDirect
	premiumProxy  *PremiumProxy // non-nil for TierPremium
	releaseOnce   sync.Once     // ensures proxy slot released exactly once
	ready         atomic.Bool
	server        string
	code          string
	cookieHeader  string
	cancel        context.CancelFunc
	done          chan struct{} // closed when read goroutine exits
	activated     atomic.Bool  // set true when consumed for a join — turbo goroutine transitions to bot mode

	// Activation data — written by turboActivateSocket BEFORE activated.Store(true),
	// read by turbo goroutine AFTER activated.Load() returns true.
	// Go memory model: atomic Store/Load provides happens-before ordering.
	actBot      *Bot
	actCtx      context.Context
	actNumId    int
	actNick     string
	actAvatar   string
	actJoinMsgs []JoinMessage
}

// releaseProxy releases the proxy slot if not already activated (bot owns the slot).
// Safe to call from multiple goroutines (e.g., StopTurbo + goroutine defer).
func (e *TurboEntry) releaseProxy() {
	if e.activated.Load() {
		return // bot owns the slot
	}
	e.releaseOnce.Do(func() {
		if e.premiumProxy != nil {
			e.premiumProxy.release()
		}
		if e.proxyTier == TierDirect {
			releaseDirectSlot()
		}
	})
}

func (e *TurboEntry) sendRaw(msg string) {
	e.wsMu.Lock()
	defer e.wsMu.Unlock()
	if e.ws != nil {
		e.ws.WriteMessage(websocket.TextMessage, []byte(msg))
	}
}

func turboSendStatus(s *Session) {
	s.turboMu.Lock()
	if !s.turboMode {
		// Don't send status when turbo is off — stale events would
		// make the extension re-check the toggle and restart turbo.
		s.turboMu.Unlock()
		return
	}
	ready := 0
	for _, e := range s.turboPool {
		if e.ready.Load() {
			ready++
		}
	}
	connecting := s.turboConnecting
	s.turboMu.Unlock()

	s.Send(map[string]interface{}{
		"event":      "turboStatus",
		"ready":      ready,
		"connecting": connecting,
	})
}

// turboSignalReplenish sends a non-blocking signal to the replenisher goroutine.
// Safe to call even when turbo is off (channel may be nil).
func turboSignalReplenish(s *Session) {
	s.turboMu.Lock()
	ch := s.turboReplenishCh
	s.turboMu.Unlock()
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default: // already signaled
		}
	}
}

func turboReplenish(s *Session) {
	s.turboMu.Lock()
	if !s.turboMode || s.turboConfig == nil {
		s.turboMu.Unlock()
		return
	}
	target := s.turboConfig.Target
	readyCount := 0
	for _, e := range s.turboPool {
		if e.ready.Load() {
			readyCount++
		}
	}
	needed := target - readyCount - s.turboConnecting
	room := s.turboConfig.Room
	serverOverride := s.turboConfig.Server
	idioma := s.turboConfig.Idioma
	s.turboMu.Unlock()

	if needed <= 0 {
		return
	}

	for n := 0; n < needed; n++ {
		proxy := pickProxy()
		if proxy == nil {
			continue
		}

		s.turboMu.Lock()
		s.turboConnecting++
		s.turboMu.Unlock()
		turboSendStatus(s)

		// All connections go through residential proxy via HTTP CONNECT tunnel
		go turboConnectOneResident(s, proxy, room, serverOverride, idioma)
	}
}

// turboConnectOneResident connects a turbo pool entry through a residential proxy
// using HTTP CONNECT tunnel (same mechanism as normal bots, but pre-connects the socket).
func turboConnectOneResident(s *Session, proxy *ResidentialProxy, room, serverOverride, idioma string) {
	red := color.New(color.FgRed)

	// Step 1: Fetch server info through the residential proxy
	var fetchURL string
	if idioma != "" {
		fetchURL = fmt.Sprintf("https://gartic.io/server/?check=1&v3=1&lang=%s", idioma)
	} else {
		fetchURL = fmt.Sprintf("https://gartic.io/server/?check=1&v3=1&room=%s", room)
	}

	proxyClient := newProxiedHTTPClient(proxy)
	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		s.turboMu.Lock()
		s.turboConnecting--
		s.turboMu.Unlock()
		turboSendStatus(s)
		turboSignalReplenish(s)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://gartic.io")
	req.Header.Set("Referer", "https://gartic.io/")

	resp, err := proxyClient.Do(req)
	if err != nil {
		if !s.quietLogs.Load() {
			red.Printf("  Turbo: fetch failed via proxy %s: %s\n", proxy.Address(), err.Error())
		}
		markProxyBlocked(proxy.IP)
		s.turboMu.Lock()
		s.turboConnecting--
		s.turboMu.Unlock()
		turboSendStatus(s)
		turboSignalReplenish(s)
		return
	}

	// Detect Cloudflare 429/403 immediately — block proxy for 5 minutes
	if resp.StatusCode == 429 || resp.StatusCode == 403 {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if !s.quietLogs.Load() {
			red.Printf("  Turbo: proxy %s returned HTTP %d (Cloudflare block)\n", proxy.Address(), resp.StatusCode)
		}
		markProxyBlocked429(proxy.IP)
		s.turboMu.Lock()
		s.turboConnecting--
		s.turboMu.Unlock()
		turboSendStatus(s)
		turboSignalReplenish(s)
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		s.turboMu.Lock()
		s.turboConnecting--
		s.turboMu.Unlock()
		turboSendStatus(s)
		turboSignalReplenish(s)
		return
	}

	s.turboMu.Lock()
	if !s.turboMode {
		s.turboConnecting--
		s.turboMu.Unlock()
		return
	}
	s.turboMu.Unlock()

	text := string(bodyBytes)
	if !strings.Contains(text, "https://") || !strings.Contains(text, "gartic.io") || !strings.Contains(text, "?c=") {
		if !s.quietLogs.Load() {
			red.Printf("  Turbo: proxy %s returned invalid/blocked response\n", proxy.Address())
		}
		markProxyBlocked429(proxy.IP)
		s.turboMu.Lock()
		s.turboConnecting--
		s.turboMu.Unlock()
		turboSendStatus(s)
		turboSignalReplenish(s)
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

	// Step 2: Connect WebSocket through residential proxy (CONNECT tunnel)
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
		if wsResp != nil {
			bodySnip, _ := io.ReadAll(io.LimitReader(wsResp.Body, 200))
			wsResp.Body.Close()
			if !s.quietLogs.Load() {
				red.Printf("  Turbo: WS failed via proxy %s: %s [HTTP %d: %s]\n",
					proxy.Address(), err.Error(), wsResp.StatusCode, strings.TrimSpace(string(bodySnip)))
			}
		}
		if wsResp != nil && wsResp.StatusCode == 429 {
			markProxyBlocked429(proxy.IP)
		} else {
			markProxyBlocked(proxy.IP)
		}
		s.turboMu.Lock()
		s.turboConnecting--
		s.turboMu.Unlock()
		turboSendStatus(s)
		turboSignalReplenish(s)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &TurboEntry{
		ws:           ws,
		proxyIp:      proxy.Address(),
		proxyTier:    TierResidential,
		server:       finalServer,
		code:         code,
		cookieHeader: botCookieHeader,
		cancel:       cancel,
		done:         make(chan struct{}),
	}

	s.turboMu.Lock()
	s.turboConnecting--
	if !s.turboMode {
		// Turbo was turned off while we were connecting — discard
		s.turboMu.Unlock()
		cancel()
		ws.Close()
		return
	}
	s.turboPool = append(s.turboPool, entry)
	s.turboMu.Unlock()

	turboStartReadLoop(s, entry, ctx)
}

// turboStartReadLoop starts the WebSocket read loop for a turbo pool entry.
// It handles the socket.io handshake, pings, and waits for activation.
// When activated (turboActivateSocket sets entry.activated), this goroutine
// transitions in-place to botMessageLoop.
func turboStartReadLoop(s *Session, entry *TurboEntry, ctx context.Context) {
	defer close(entry.done)
	defer func() {
		entry.wsMu.Lock()
		if entry.ws != nil {
			entry.ws.Close()
			entry.ws = nil
		}
		entry.wsMu.Unlock()
		entry.releaseProxy()

		// Remove from pool if still there
		s.turboMu.Lock()
		for i, e := range s.turboPool {
			if e == entry {
				s.turboPool = append(s.turboPool[:i], s.turboPool[i+1:]...)
				break
			}
		}
		s.turboMu.Unlock()
		turboSendStatus(s)
		turboSignalReplenish(s)
	}()

	ws := entry.ws
	ws.SetReadLimit(4 << 20)
	ws.SetPingHandler(func(data string) error {
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return ws.WriteControl(websocket.PongMessage,
			[]byte(data), time.Now().Add(4*time.Second))
	})
	ws.SetReadDeadline(time.Now().Add(60 * time.Second))

	turboDone := make(chan struct{})
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
			case <-turboDone:
				return
			}
		}
	}()
	defer close(turboDone)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, raw, err := ws.ReadMessage()
		if err != nil {
			return
		}
		msg := string(raw)
		ws.SetReadDeadline(time.Now().Add(60 * time.Second))

		// Check if this entry has been activated (consumed for a join)
		if entry.activated.Load() {
			// Transition to botMessageLoop in-place
			bot := entry.actBot
			botMessageLoop(entry.actCtx, s, bot, entry.actNumId, "", entry.actNick, entry.actAvatar, entry.actJoinMsgs, msg)
			return
		}

		// Socket.io handshake — respond with namespace connect
		if strings.HasPrefix(msg, "0{") || msg == "0" {
			entry.sendRaw("40")
			continue
		}

		// Namespace connect confirmation — mark as ready
		if msg == "40" {
			entry.ready.Store(true)
			turboSendStatus(s)
			continue
		}

		// Socket.io application-level heartbeat — MUST reply instantly
		if msg == "2" {
			ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
			ws.WriteMessage(websocket.TextMessage, []byte("3"))
			ws.SetReadDeadline(time.Now().Add(60 * time.Second))
			continue
		}
	}
}

// StopTurbo closes all turbo pool sockets.
// Safe: StopTurbo and turboConsumeForJoin are both called from the ReadLoop
// (single goroutine per session), so they cannot be in-flight simultaneously.
func (s *Session) StopTurbo() {
	s.turboMu.Lock()
	s.turboMode = false
	s.turboConfig = nil
	for _, e := range s.turboPool {
		e.releaseProxy()
		e.cancel()
		e.wsMu.Lock()
		if e.ws != nil {
			e.ws.Close()
		}
		e.wsMu.Unlock()
	}
	s.turboPool = nil
	s.turboConnecting = 0
	if s.turboStopCh != nil {
		close(s.turboStopCh)
		s.turboStopCh = nil
	}
	s.turboReplenishCh = nil
	s.turboMu.Unlock()
}

// startTurboFromConfig starts the turbo pool from a TurboConfig.
func (s *Session) startTurboFromConfig(cfg *TurboConfig) {
	s.StopTurbo()
	s.turboMu.Lock()
	s.turboMode = true
	s.turboConfig = &TurboConfig{
		Room:   cfg.Room,
		Server: cfg.Server,
		Target: cfg.Target,
		Idioma: cfg.Idioma,
	}
	s.turboReplenishCh = make(chan struct{}, 1)
	s.turboMu.Unlock()
	turboReplenish(s)
	s.startTurboLoops()
	turboSendStatus(s)
}

// resumeTurboIfDefault restarts turbo from the saved default config (if any).
// Called after autojoin ends to restore the pre-autojoin turbo state.
func (s *Session) resumeTurboIfDefault() {
	s.turboMu.Lock()
	cfg := s.turboDefaultConfig
	alreadyOn := s.turboMode
	s.turboMu.Unlock()
	if cfg != nil && !alreadyOn {
		s.startTurboFromConfig(cfg)
	}
}


// startTurboLoops starts keepalive (15s), health check (30s), and replenisher loops
func (s *Session) startTurboLoops() {
	stopCh := make(chan struct{})
	s.turboMu.Lock()
	s.turboStopCh = stopCh
	s.turboMu.Unlock()

	// Keepalive: ping all ready sockets every 15s
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				s.turboMu.Lock()
				pool := make([]*TurboEntry, len(s.turboPool))
				copy(pool, s.turboPool)
				s.turboMu.Unlock()
				for _, e := range pool {
					if e.ready.Load() {
						e.sendRaw("2")
					}
				}
			}
		}
	}()

	// Health: replenish every 30s as safety net
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				turboReplenish(s)
			}
		}
	}()

	// Replenisher: reacts instantly to signal channel
	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-s.turboReplenishCh:
				turboReplenish(s)
			}
		}
	}()
}

// turboConsumeForJoin consumes ready turbo sockets for a join command.
// Returns how many were successfully activated (bot created + join sent).
// Failed activations are not counted — caller should backfill with normal joins.
func (s *Session) turboConsumeForJoin(room, name, nickMode, avatar string, customNicks []CustomNick, joinMessages []JoinMessage, qty int, idioma string) int {
	s.turboMu.Lock()
	var readyEntries []*TurboEntry
	for _, e := range s.turboPool {
		if e.ready.Load() && e.ws != nil {
			readyEntries = append(readyEntries, e)
		}
	}
	limit := qty
	if limit > len(readyEntries) {
		limit = len(readyEntries)
	}
	toActivate := readyEntries[:limit]
	// Remove from pool
	for _, e := range toActivate {
		for i, pe := range s.turboPool {
			if pe == e {
				s.turboPool = append(s.turboPool[:i], s.turboPool[i+1:]...)
				break
			}
		}
	}
	s.turboMu.Unlock()

	// Launch each activation in its own goroutine so the caller (cmdJoin
	// from the control-WS ReadLoop) returns immediately. Without this,
	// each turboActivateSocket blocks up to 60s waiting for a Turnstile
	// token, and N entries serialize to N*60s — locking up the ReadLoop
	// so the extension can't even send chat/exit while a join is in flight.
	for _, entry := range toActivate {
		s.mu.Lock()
		nr := generateNick(name, nickMode, customNicks, &s.queueNumber)
		s.mu.Unlock()

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

		go turboActivateSocket(s, entry, room, nr.Nick, finalAvatar, joinMessages, idioma)
	}

	if len(toActivate) > 0 {
		turboSendStatus(s)
		turboSignalReplenish(s)
	}

	// Optimistically report all activations as successful so cmdJoin's
	// backfill logic doesn't double-spawn bots. Some may fail at the
	// token-wait or join step; those self-clean.
	return len(toActivate)
}

// turboActivateSocket converts a turbo pool entry into a full bot.
// The turbo goroutine transforms in-place into botMessageLoop when it reads
// the next message and sees the activated flag. No SetReadDeadline, no goroutine
// switch — avoids gorilla/websocket's sticky readErr that corrupts the socket.
func turboActivateSocket(s *Session, entry *TurboEntry, room, nick, finalAvatar string, joinMessages []JoinMessage, idioma string) bool {
	// Check socket is still usable
	entry.wsMu.Lock()
	ws := entry.ws
	if ws == nil {
		entry.wsMu.Unlock()
		return false
	}
	entry.wsMu.Unlock()

	s.mu.Lock()
	s.botCounter++
	botNumId := s.botCounter
	bot := &Bot{
		session:      s,
		numericId:    botNumId,
		nick:         nick,
		proxyIp:      entry.proxyIp,
		proxyTier:    entry.proxyTier,
		premiumProxy: entry.premiumProxy,
		ws:           ws,
	}
	bot.alive.Store(true)
	s.bots[botNumId] = bot
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	bot.cancel = cancel

	// Send join packet immediately — write is safe concurrent with turbo
	// goroutine's read (gorilla supports one reader + one writer).
	bot.joinSent = true
	bot.idioma = idioma
	// On-demand fetch from moonfive — see bot.go:botMessageLoop. The
	// pre-connected turbo socket is left behind for the next click; only
	// the bot record is unwound.
	tok := FetchTurnstileToken()
	if tok == "" {
		// Moonfive empty — abort this turbo activation cleanly. We've
		// already created the bot record but never sent join and never
		// set entry.activated, so the turbo goroutine still owns the
		// socket. Just remove the bot record and cancel its context.
		s.mu.Lock()
		delete(s.bots, botNumId)
		s.mu.Unlock()
		cancel()
		return false
	}
	if idioma != "" {
		// Public room: join by language number
		bot.SendRaw(fmt.Sprintf(`42[1,{"v":20000,"token":"%s","nick":"%s","avatar":%s,"platform":2,"idioma":%s}]`,
			tok, nick, finalAvatar, idioma))
	} else {
		sala := room
		if len(room) > 2 {
			sala = room[2:]
		}
		bot.SendRaw(fmt.Sprintf(`42[3,{"v":20000,"token":"%s","nick":"%s","avatar":%s,"platform":2,"sala":"%s"}]`,
			tok, nick, finalAvatar, sala))
	}
	// Token is consumed by gartic the moment the join packet is in
	// flight — release from the relay's in-flight ledger.
	go ReleaseTurnstileToken(tok)

	// Store activation data BEFORE setting the flag.
	// Go memory model: writes before atomic Store are visible after
	// the corresponding atomic Load that observes the stored value.
	entry.actBot = bot
	entry.actCtx = ctx
	entry.actNumId = botNumId
	entry.actNick = nick
	entry.actAvatar = finalAvatar
	entry.actJoinMsgs = joinMessages

	// Signal turbo goroutine to transition to bot mode on its next read.
	// The goroutine checks this flag after every ReadMessage and calls
	// botMessageLoop in-place — same goroutine, same socket, no interruption.
	entry.activated.Store(true)

	return true
}