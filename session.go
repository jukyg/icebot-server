package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/gorilla/websocket"
)

// ============================================================================
// session.go — Extension ↔ Server session management
// Each browser tab that connects gets its own Session with its own bot pool.
// ============================================================================

// connectWorkers controls how many concurrent bot connections are processed.
// Increased to 50 for maximum speed.
const connectWorkers = 50

// AutoJoinConfig holds the configuration for auto-rejoin and autojoin.
type AutoJoinConfig struct {
	Room           string
	Name           string
	Nick           string // alias for Name
	NickMode       string
	CustomNicks    []CustomNick
	Avatar         string
	JoinMessages   []JoinMessage
	ServerOverride string
	Server         string // alias for ServerOverride
	Target         int
	Idioma         string
}

// AutoRejoinConfig is an alias kept for compatibility with older code paths.
type AutoRejoinConfig = AutoJoinConfig

// Global room registries for rejoin/keepEmpty — cross-session, room-keyed.
var (
	rejoinRooms     = make(map[string]bool)
	keepEmptyRooms  = make(map[string]bool)
	keepEmptyCounts = make(map[string]int)
	rejoinMu        sync.RWMutex
	keepEmptyMu     sync.RWMutex
)

// MaxWSConns limits the number of concurrent WebSocket connections per session
// to prevent resource exhaustion from rapid page reloads.
const maxWSConns = 10

// Session manages an extension ↔ server connection. Supports multiple
// concurrent WebSocket connections (fan-out) so browser tabs monitoring
// the same room do not trigger reconnection loops — each tab's WS is
// added to wsSet and removed when the tab closes.
type Session struct {
	id                 string
	wsSet              map[*websocket.Conn]bool
	wsMu               sync.Mutex
	mu                 sync.RWMutex
	bots               map[int]*Bot
	botCounter         int
	queueNumber        int
	room               string
	orphaned           bool
	autoRejoin         atomic.Bool
	autoRejoinConfig   *AutoJoinConfig
	lastAutoRejoinTime time.Time
	autofarm           bool
	privateMode        bool
	answerReveal       bool

	currentDrawerId    atomic.Int64
	currentDrawWord    string
	reporterBotId      int
	isAutoJoining      atomic.Bool
	quietLogs          atomic.Bool
	marked             bool
	srvCache           *ServerInfoCache
	connectPool        chan ConnectJob
	connectDone        chan struct{}
	autoJoinCancel     func()

	// Room-full detection: when N consecutive bots get "room is full" (code 3),
	// new bot creation is stopped until a successful join or new JOIN command.
	consecutiveRoomFull int32

	createdAt          time.Time

	// Rejoin throttle: prevents auto-rejoin storms when multiple bots
	// disconnect simultaneously. Only one rejoin operation runs at a time,
	// with exponential backoff on repeated failures.
	rejoinInProgress atomic.Bool
	rejoinBackoff    atomic.Int64 // nanoseconds, doubles on each failed attempt

	// Turbo mode fields
	turboMu           sync.Mutex
	turboMode         bool
	turboConfig       *TurboConfig
	turboPool         []*TurboEntry
	turboConnecting   int
	turboStopCh       chan struct{}
	turboReplenishCh  chan struct{}
	turboDefaultConfig *TurboConfig
}

// Session registry
var (
	sessionsMu sync.RWMutex
	sessions   = make(map[string]*Session)
)

// keepBotsAlive monitors the session and relaunches bots whenever all of
// them die while autoRejoin is enabled.  Run once per session with:
//   go s.keepBotsAlive()
func (s *Session) keepBotsAlive() {
	ticker := time.NewTicker(90 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !s.autoRejoin.Load() {
			continue
		}

		// Detect and kill stale bots (no message received in 2+ minutes)
		now := time.Now().Unix()
		var staleBots []*Bot
		s.mu.RLock()
		for _, b := range s.bots {
			last := b.lastMsg.Load()
			if last > 0 && now-last > 120 {
				staleBots = append(staleBots, b)
			}
		}
		s.mu.RUnlock()
		for _, b := range staleBots {
			color.New(color.FgYellow).Printf("[keepBotsAlive] Killing stale bot %d (room=%s) — last msg %ds ago\n",
				b.numericId, s.room, now-b.lastMsg.Load())
			b.Destroy()
		}

		s.mu.RLock()
		alive := 0
		for _, b := range s.bots {
			if b.IsAlive() && b.joinConfirmed.Load() {
				alive++
			}
		}
		cfg := s.autoRejoinConfig
		s.mu.RUnlock()

		if alive > 0 || cfg == nil || cfg.Room == "" {
			continue
		}

		if s.tryBeginRejoin() {
			color.New(color.FgCyan).Printf("[keepBotsAlive] Room %s: no alive bots — relaunching\n", cfg.Room)
			go func(c *AutoJoinConfig) {
				defer s.endRejoin()
				s.startRejoinAutoJoin(c)
			}(cfg)
		}
	}
}

// GetOrCreateSession retrieves an existing session by room code, or creates one if not found.
// IMPORTANT: This now matches orphaned sessions too, so when the user leaves the room
// and comes back, their bots are still running and they regain full control.
func GetOrCreateSession(ws *websocket.Conn, room string) *Session {
	sessionsMu.Lock()
	defer sessionsMu.Unlock()

	if room != "" {
		// First pass: try to find an active (non-orphaned) session for this room
		for _, s := range sessions {
			s.mu.RLock()
			match := s.room == room && !s.orphaned
			s.mu.RUnlock()
			if match {
				color.New(color.FgHiGreen).Printf("[Session Registry] Reattaching to active session %s for room %s\n", s.id, room)
				s.Reattach(ws)
				return s
			}
		}
		// Second pass: try to find an ORPHANED session for this room (bots still alive!)
		for _, s := range sessions {
			s.mu.RLock()
			match := s.room == room && s.orphaned
			s.mu.RUnlock()
			if match {
				color.New(color.FgHiGreen).Printf("[Session Registry] ★ Recovering orphaned session %s for room %s — bots are still alive!\n", s.id, room)
				s.Reattach(ws)
				return s
			}
		}
	}

	// Create new session
	id := fmt.Sprintf("s_%d_%d", time.Now().UnixMilli(), rand.Intn(10000))
	s := &Session{
		id:          id,
		wsSet:       map[*websocket.Conn]bool{ws: true},
		bots:        make(map[int]*Bot),
		room:        room,
		srvCache:    &ServerInfoCache{},
		connectPool: make(chan ConnectJob, 500),
		connectDone: make(chan struct{}),
		privateMode: true,
		createdAt:   time.Now(),
	}
	s.autoRejoin.Store(true)
	s.autoRejoinConfig = &AutoJoinConfig{
		Room:     room,
		Name:     "Bot",
		Nick:     "Bot",
		NickMode: "0",
		Avatar:   "0",
		Target:   10,
	}

	for i := 0; i < connectWorkers; i++ {
		go s.connectWorker()
	}

	sessions[id] = s
	go s.keepBotsAlive()
	color.New(color.FgHiCyan).Printf("[Session %s] Created (room=%s)\n", id, room)

	// Restore any previously saved settings for this room so the owner's
	// options (autofarm, privateMode, autoRejoin target, etc.) come back
	// automatically when they return to the same room.
	ApplyRoomPersist(s)

	s.mu.RLock()
	af := s.autofarm
	pm := s.privateMode
	ar := s.answerReveal
	s.mu.RUnlock()

	s.Send(map[string]interface{}{
		"event":        "sessionCreated",
		"sessionId":    id,
		"marked":       s.marked,
		"autoRejoin":   s.autoRejoin.Load(),
		"autofarm":     af,
		"privateMode":  pm,
		"answerReveal": ar,
	})

	return s
}

// connectWorker processes bot connect jobs from the pool.
// Each worker picks a residential proxy and connects through it.
func (s *Session) connectWorker() {
	for {
		select {
		case job := <-s.connectPool:
			// Connect through residential proxy — no stagger, all bots hit instantly
			botConnectViaProxy(job.s, job.bot, job.botNumId, job.room, job.nick, job.avatar, job.joinMessages, job.serverOverride, job.idioma)
		case <-s.connectDone:
			return
		}
	}
}

// Send broadcasts a JSON event to ALL active extension connections (fan-out).
// Writes are serialized per-connection by the gorilla library's internal mutex;
// the wsMu prevents interleaving across connections during broadcast.
func (s *Session) Send(event map[string]interface{}) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	if len(s.wsSet) == 0 {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	for ws := range s.wsSet {
		ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
		ws.WriteMessage(websocket.TextMessage, data)
		ws.SetWriteDeadline(time.Time{})
	}
}

// sendTo sends a JSON event to a single WebSocket connection (bypasses fan-out).
// Used during reattachment to initialize the new tab without re-syncing existing tabs.
func (s *Session) sendTo(ws *websocket.Conn, event map[string]interface{}) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	ws.WriteMessage(websocket.TextMessage, data)
	ws.SetWriteDeadline(time.Time{})
}

// SendGameEvent forwards an event from the reporter bot to ALL active sessions
// monitoring the same room. This ensures bots deployed via admin/auto-deploy
// (which may be in a headless session with no WebSocket) still reach any user
// who is actively monitoring the room.
func (s *Session) SendGameEvent(botNumId int, event map[string]interface{}) {
	s.mu.RLock()
	reporter := s.reporterBotId
	room := s.room
	s.mu.RUnlock()
	if botNumId != reporter {
		return
	}
	// Always send to our own session first
	s.Send(event)
	if room == "" {
		return
	}
	// Broadcast to all other sessions with live WebSockets for the same room
	sessionsMu.RLock()
	var targets []*Session
	for _, other := range sessions {
		if other == s {
			continue
		}
		other.mu.RLock()
		match := other.room == room && !other.orphaned
		other.mu.RUnlock()
		if !match {
			continue
		}
		other.wsMu.Lock()
		hasWS := len(other.wsSet) > 0
		other.wsMu.Unlock()
		if hasWS {
			targets = append(targets, other)
		}
	}
	sessionsMu.RUnlock()
	for _, t := range targets {
		t.Send(event)
	}
}

// ForEachBot runs a function on each alive bot.
func (s *Session) ForEachBot(fn func(*Bot)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, b := range s.bots {
		if b.IsAlive() {
			fn(b)
		}
	}
}

// electReporterLocked picks the first joined bot as reporter. Must hold s.mu.
func (s *Session) electReporterLocked() {
	s.reporterBotId = 0
	for id, b := range s.bots {
		if b.joinConfirmed.Load() {
			s.reporterBotId = id
			return
		}
	}
}

// tryBeginRejoin attempts to start a rejoin operation.
// Returns true only if no other rejoin is in progress AND enough time
// has elapsed since the last attempt. Uses exponential backoff:
//   1st retry: 3s   2nd: 6s   3rd: 12s   4th: 24s   5th+: 60s
// Reset on successful bot join (see case "5" in botMessageLoop).
func (s *Session) tryBeginRejoin() bool {
	if !s.rejoinInProgress.CompareAndSwap(false, true) {
		return false
	}
	s.mu.Lock()
	since := time.Since(s.lastAutoRejoinTime)
	backoff := time.Duration(s.rejoinBackoff.Load())
	if backoff == 0 {
		backoff = 3 * time.Second
	}
	if since < backoff {
		s.mu.Unlock()
		s.rejoinInProgress.Store(false)
		return false
	}
	s.lastAutoRejoinTime = time.Now()
	// Double backoff for next attempt (cap at 60s)
	next := backoff * 2
	if next > 60*time.Second {
		next = 60 * time.Second
	}
	s.rejoinBackoff.Store(int64(next))
	s.mu.Unlock()
	return true
}

// resetRejoinBackoff clears the rejoin backoff to minimum (3s).
// Called whenever a bot successfully joins the room.
func (s *Session) resetRejoinBackoff() {
	s.mu.Lock()
	s.rejoinBackoff.Store(int64(3 * time.Second))
	s.mu.Unlock()
}

// endRejoin releases the rejoin throttle.
func (s *Session) endRejoin() {
	s.rejoinInProgress.Store(false)
}

// DestroyAllBots closes all bot connections.
func (s *Session) DestroyAllBots() {
	s.mu.Lock()
	bots := make([]*Bot, 0, len(s.bots))
	for _, b := range s.bots {
		bots = append(bots, b)
	}
	s.bots = make(map[int]*Bot)
	s.reporterBotId = 0
	s.mu.Unlock()

	for _, b := range bots {
		b.exitedManually.Store(true)
		b.Destroy()
	}
}

// GetBotList returns a list of bot info for the extension.
func (s *Session) GetBotList() []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]map[string]interface{}, 0, len(s.bots))
	for _, b := range s.bots {
		if b.joinConfirmed.Load() {
			list = append(list, map[string]interface{}{
				"numericId": b.numericId,
				"garticId":  int(b.garticId.Load()),
				"nick":      b.nick,
			})
		}
	}
	return list
}

// Reattach binds a new WebSocket to this session (e.g. on page refresh or new tab).
// Unlike the old single-connection approach, this ADDS the connection to the set
// without closing existing ones — preventing cross-tab reconnection loops.
func (s *Session) Reattach(ws *websocket.Conn) {
	s.wsMu.Lock()
	if len(s.wsSet) >= maxWSConns {
		s.wsMu.Unlock()
		color.New(color.FgYellow).Printf("[Session %s] Reattachment rejected — too many connections (%d)\n", s.id, maxWSConns)
		ws.Close()
		return
	}
	s.wsSet[ws] = true
	totalConns := len(s.wsSet)
	s.wsMu.Unlock()

	s.mu.Lock()
	s.orphaned = false
	s.mu.Unlock()

	color.New(color.FgHiGreen).Printf("[Session %s] New connection added (total %d connections). Syncing %d bots...\n",
		s.id, totalConns, len(s.bots))

	s.mu.RLock()
	af := s.autofarm
	pm := s.privateMode
	ar := s.answerReveal
	s.mu.RUnlock()

	rejoinMu.RLock()
	rj := rejoinRooms[s.room]
	rejoinMu.RUnlock()
	keepEmptyMu.RLock()
	ke := keepEmptyRooms[s.room]
	kc := keepEmptyCounts[s.room]
	keepEmptyMu.RUnlock()

	// Send initialization to the NEW connection only (not broadcast)
	s.sendTo(ws, map[string]interface{}{
		"event":          "sessionCreated",
		"sessionId":      s.id,
		"marked":         s.marked,
		"autoRejoin":     s.autoRejoin.Load(),
		"autofarm":       af,
		"privateMode":    pm,
		"answerReveal":   ar,
		"rejoin":         rj,
		"keepEmpty":      ke,
		"keepEmptyCount": kc,
	})

	// Synchronize bots list to the new connection
	bots := s.GetBotList()
	s.sendTo(ws, map[string]interface{}{
		"event": "botSync",
		"bots":  bots,
	})
}

// CloseConn removes a WebSocket connection from the session's fan-out set.
// If no connections remain and the session has no bots, it is destroyed.
// If bots remain, the session is orphaned (kept alive for reattachment).
func (s *Session) CloseConn(ws *websocket.Conn) {
	s.wsMu.Lock()
	delete(s.wsSet, ws)
	remaining := len(s.wsSet)
	s.wsMu.Unlock()

	color.New(color.FgYellow).Printf("[Session %s] Connection closed (%d remaining)\n", s.id, remaining)

	if remaining > 0 {
		// Other connections still active — session remains fully alive
		return
	}

	s.mu.Lock()
	// If ANY bots exist in the map (even connecting, not yet joined),
	// orphan the session instead of destroying it. Bots may be in the
	// middle of the connect pool handshake — killing the session would
	// lose them permanently.
	hasLiveBots := false
	for _, b := range s.bots {
		if b.IsAlive() {
			hasLiveBots = true
			break
		}
	}
	if hasLiveBots || len(s.bots) > 0 {
		s.orphaned = true
		color.New(color.FgYellow).Printf("[Session %s] Orphaned — %d bots tracked in room %s (keeping alive)\n",
			s.id, len(s.bots), s.room)
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	// No bots — full shutdown
	close(s.connectDone)
	s.DestroyAllBots()
	s.StopTurbo()

	sessionsMu.Lock()
	delete(sessions, s.id)
	sessionsMu.Unlock()
}

// sessionReaper periodically cleans up orphaned sessions that have no alive bots.
// Sessions with autoRejoin enabled are kept alive for up to 7 days — bots may
// be actively reconnecting in the background even when the extension is away.
func sessionReaper() {
	for {
		time.Sleep(60 * time.Second)
		sessionsMu.Lock()
		for id, s := range sessions {
			s.mu.RLock()
			isOrph := s.orphaned
			aliveCount := 0
			totalBots := len(s.bots)
			for _, b := range s.bots {
				if b.IsAlive() && b.joinConfirmed.Load() {
					aliveCount++
				}
			}
			age := time.Since(s.createdAt)
			autoRejoinOn := s.autoRejoin.Load()
			cfg := s.autoRejoinConfig
			s.mu.RUnlock()

			if !isOrph {
				continue
			}

			// If autoRejoin is on, keep the session alive for up to 7 days.
			// Bots may be reconnecting in the background.
			if autoRejoinOn {
				if age > 7*24*time.Hour {
					color.New(color.FgHiYellow).Printf("[Session Reaper] Retiring 7-day old session %s (room=%s)\n", id, s.room)
					delete(sessions, id)
					continue
				}
				// All bots died but autoRejoin is still on — restart them
				if aliveCount == 0 && totalBots == 0 && cfg != nil && cfg.Room != "" && cfg.Idioma == "" {
					color.New(color.FgHiCyan).Printf("[Session Reaper] No bots in autoRejoin session %s (room=%s) — relaunching\n", id, s.room)
					go func(sess *Session, c *AutoJoinConfig) {
						if sess.tryBeginRejoin() {
							defer sess.endRejoin()
							sess.startRejoinAutoJoin(c)
						}
					}(s, cfg)
				}
				continue
			}

			// No autoRejoin: clean up after 5 minutes with no alive bots
			if aliveCount == 0 && age > 5*time.Minute {
				color.New(color.FgHiYellow).Printf("[Session Reaper] Cleaning up orphaned session %s (room=%s) — no alive bots\n", id, s.room)
				delete(sessions, id)
			}
		}
		sessionsMu.Unlock()
	}
}

// joinWithTurbo creates N bots quickly, consuming from turbo pool first.
func (s *Session) joinWithTurbo(cfg *AutoJoinConfig, qty int) {
	s.isAutoJoining.Store(true)
	defer s.isAutoJoining.Store(false)

	turboUsed := 0
	s.turboMu.Lock()
	turboOn := s.turboMode
	s.turboMu.Unlock()
	if turboOn && cfg.Idioma == "" {
		turboUsed = s.turboConsumeForJoin(cfg.Room, cfg.Name, cfg.NickMode, cfg.Avatar, cfg.CustomNicks, cfg.JoinMessages, qty, cfg.Idioma)
	}
	for i := 0; i < qty-turboUsed; i++ {
		createBot(s, cfg.Room, cfg.Name, cfg.NickMode, cfg.Avatar, cfg.CustomNicks, cfg.JoinMessages, cfg.Server, cfg.Idioma)
	}
}

// startRejoinAutoJoin starts a rejoin auto-join loop.
func (s *Session) startRejoinAutoJoin(cfg *AutoJoinConfig) {
	s.isAutoJoining.Store(true)
	defer s.isAutoJoining.Store(false)

	// Wait for old sockets to fully drain before deploying new bots
	time.Sleep(time.Duration(2000+rand.Intn(3000)) * time.Millisecond)

	qty := cfg.Target
	if qty <= 0 {
		qty = 1
	}
	for i := 0; i < qty; i++ {
		createBot(s, cfg.Room, cfg.Name, cfg.NickMode, cfg.Avatar, cfg.CustomNicks, cfg.JoinMessages, cfg.Server, cfg.Idioma)
		if i < qty-1 {
			time.Sleep(time.Duration(800+rand.Intn(700)) * time.Millisecond)
		}
	}

	// Wait for all to join (up to 90s total)
	deadline := time.After(90 * time.Second)
	for {
		select {
		case <-deadline:
			goto done
		case <-time.After(500 * time.Millisecond):
			s.mu.RLock()
			joined := 0
			for _, b := range s.bots {
				if b.joinConfirmed.Load() {
					joined++
				}
			}
			s.mu.RUnlock()
			if joined >= qty {
				goto done
			}
		}
	}
done:
	bots := s.GetBotList()
	s.Send(map[string]interface{}{
		"event": "botSync",
		"bots":  bots,
	})
}

// findBotGlobally searches ALL sessions for a bot by numericId.
// Returns the bot and the session it belongs to.
func findBotGlobally(numericId int) (*Bot, *Session) {
	sessionsMu.RLock()
	defer sessionsMu.RUnlock()
	for _, sess := range sessions {
		sess.mu.RLock()
		bot, ok := sess.bots[numericId]
		sess.mu.RUnlock()
		if ok {
			return bot, sess
		}
	}
	return nil, nil
}

// HandleMessage processes a command from the extension.
func (s *Session) HandleMessage(raw []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}

	cmd, _ := msg["cmd"].(string)
	yellow := color.New(color.FgYellow)

	switch cmd {
	case "join":
		room := parseRoom(fmt.Sprintf("%v", msg["room"]))
		if room == "" {
			room = s.room
		}
		qty := int(getFloat(msg, "qty", 1))
		name := getString(msg, "name", "Bot")
		nickMode := getString(msg, "nickMode", "0")
		avatar := getString(msg, "avatar", "0")
		serverOverride := getString(msg, "server", "")
		idioma := getString(msg, "idioma", "")
		atomic.StoreInt32(&s.consecutiveRoomFull, 0)

		var customNicks []CustomNick
		if cn, ok := msg["customNicks"]; ok {
			if b, err := json.Marshal(cn); err == nil {
				json.Unmarshal(b, &customNicks)
			}
		}
		var joinMsgs []JoinMessage
		if jm, ok := msg["joinMessages"]; ok {
			if b, err := json.Marshal(jm); err == nil {
				json.Unmarshal(b, &joinMsgs)
			}
		}

		yellow.Printf("[%s] JOIN room=%s qty=%d nick=%s server=%s idioma=%s\n",
			s.id, room, qty, name, serverOverride, idioma)

		// Track the room on this session so orphan recovery works
		s.mu.Lock()
		if s.room == "" && room != "" {
			s.room = room
		}
		s.autoRejoinConfig = &AutoJoinConfig{
			Room: room, Name: name, Nick: name, NickMode: nickMode,
			CustomNicks: customNicks, Avatar: avatar,
			JoinMessages: joinMsgs, ServerOverride: serverOverride, Server: serverOverride,
			Target: qty, Idioma: idioma,
		}
		s.mu.Unlock()

		// Try turbo pool first (for private rooms only), then normal
		turboUsed := 0
		s.turboMu.Lock()
		turboOn := s.turboMode
		s.turboMu.Unlock()
		if turboOn && idioma == "" {
			turboUsed = s.turboConsumeForJoin(room, name, nickMode, avatar, customNicks, joinMsgs, qty, idioma)
		}
		for i := 0; i < qty-turboUsed; i++ {
			createBot(s, room, name, nickMode, avatar, customNicks, joinMsgs, serverOverride, idioma)
		}

	case "exit":
		yellow.Printf("[%s] EXIT — destroying all bots\n", s.id)
		if s.autoJoinCancel != nil {
			s.autoJoinCancel()
			s.autoJoinCancel = nil
		}
		s.StopTurbo()
		s.DestroyAllBots()
		s.Send(map[string]interface{}{
			"event": "botSync",
			"bots":  []map[string]interface{}{},
		})

	case "exitBot":
		numId := int(getFloat(msg, "numericId", 0))
		if numId == 0 {
			numId = int(getFloat(msg, "botId", 0))
		}
		s.mu.Lock()
		bot, ok := s.bots[numId]
		if ok {
			delete(s.bots, numId)
			if numId == s.reporterBotId {
				s.electReporterLocked()
			}
		}
		s.mu.Unlock()
		if !ok {
			bot, otherSess := findBotGlobally(numId)
			if bot != nil {
				otherSess.mu.Lock()
				delete(otherSess.bots, numId)
				if numId == otherSess.reporterBotId {
					otherSess.electReporterLocked()
				}
				otherSess.mu.Unlock()
				ok = true
			}
		}
		if ok {
			bot.exitedManually.Store(true)
			bot.Destroy()
			yellow.Printf("[%s] EXIT bot %d\n", s.id, numId)
			s.Send(map[string]interface{}{"event": "botDisconnected", "numericId": numId})
		}

	case "chat":
		numId := int(getFloat(msg, "numericId", 0))
		if numId == 0 {
			numId = int(getFloat(msg, "botId", 0))
		}
		text := getString(msg, "msg", "")
		if text == "" {
			break
		}
		s.mu.RLock()
		bot, ok := s.bots[numId]
		if !ok {
			gid := int(getFloat(msg, "garticId", 0))
			if gid > 0 {
				for _, b := range s.bots {
					if int(b.garticId.Load()) == gid {
						bot = b
						ok = true
						break
					}
				}
			}
		}
		s.mu.RUnlock()
		if !ok {
			bot, _ = findBotGlobally(numId)
			if bot != nil {
				ok = true
			}
		}
		if ok && bot.IsAlive() && bot.garticId.Load() != 0 {
			gid := int(bot.garticId.Load())
			if !bot.SendRaw(fmt.Sprintf(`42[13,%d,%s]`, gid, jsonString(text))) {
				yellow.Printf("[%s] Chat send failed for bot %d\n", s.id, numId)
			}
		} else {
			if !ok {
				yellow.Printf("[%s] Chat: bot %d not found\n", s.id, numId)
			} else if !bot.IsAlive() {
				yellow.Printf("[%s] Chat: bot %d not alive\n", s.id, numId)
			}
		}

	case "answer":
		numId := int(getFloat(msg, "botId", 0))
		text := getString(msg, "msg", "")
		if text == "" {
			break
		}
		s.mu.RLock()
		bot, ok := s.bots[numId]
		s.mu.RUnlock()
		if !ok {
			bot, _ = findBotGlobally(numId)
			if bot != nil {
				ok = true
			}
		}
		if ok && bot.IsAlive() && bot.joinConfirmed.Load() {
			bot.SendRaw(fmt.Sprintf(`42[13,%d,%s]`, int(bot.garticId.Load()), jsonString(text)))
		}

	case "broadcast":
		text := getString(msg, "msg", "")
		if text == "" {
			break
		}
		s.ForEachBot(func(b *Bot) {
			if b.joinConfirmed.Load() {
				gid := int(b.garticId.Load())
				b.SendRaw(fmt.Sprintf(`42[13,%d,%s]`, gid, jsonString(text)))
			}
		})

	case "broadcastMsg":
		text := getString(msg, "msg", "")
		if text == "" {
			break
		}
		s.ForEachBot(func(b *Bot) {
			if b.joinConfirmed.Load() {
				b.SendRaw(fmt.Sprintf(`42[13,%d,%s]`, int(b.garticId.Load()), jsonString(text)))
			}
		})

	case "broadcastAnswer":
		text := getString(msg, "msg", "")
		if text == "" {
			break
		}
		s.ForEachBot(func(b *Bot) {
			if b.joinConfirmed.Load() {
				b.SendRaw(fmt.Sprintf(`42[13,%d,%s]`, int(b.garticId.Load()), jsonString(text)))
			}
		})

	case "report":
		// Staggered delay to prevent rate limiting (10ms between each bot)
		botIndex := 0
		s.ForEachBot(func(b *Bot) {
			if b.joinConfirmed.Load() {
				delay := time.Duration(botIndex*10) * time.Millisecond
				botIndex++
				go func(bot *Bot, d time.Duration) {
					time.Sleep(d)
					bot.SendRaw(fmt.Sprintf("42[35,%d]", int(bot.garticId.Load())))
				}(b, delay)
			}
		})

	case "jump":
		s.ForEachBot(func(b *Bot) {
			if b.joinConfirmed.Load() {
				b.SendRaw(fmt.Sprintf("42[34,%d]", int(b.garticId.Load())))
			}
		})

	case "tips":
		s.ForEachBot(func(b *Bot) {
			if b.joinConfirmed.Load() {
				b.SendRaw(fmt.Sprintf("42[32,%d]", int(b.garticId.Load())))
			}
		})

	case "afkConfirm":
		s.ForEachBot(func(b *Bot) {
			if b.joinConfirmed.Load() {
				b.SendRaw(fmt.Sprintf("42[42,%d]", int(b.garticId.Load())))
			}
		})

	case "kick":
		// Accept targetUserId as either string or number (JSON unmarshals numbers as float64)
		var targetId string
		if tid, ok := msg["targetUserId"].(string); ok {
			targetId = tid
		} else if tidNum, ok := msg["targetUserId"].(float64); ok {
			targetId = fmt.Sprintf("%.0f", tidNum)
		}
		if targetId == "" || targetId == "0" {
			break
		}
		// Snapshot all alive bots with valid garticIds
		s.mu.RLock()
		var kickBots []*Bot
		for _, b := range s.bots {
			if b.IsAlive() && b.joinConfirmed.Load() && b.garticId.Load() > 0 {
				kickBots = append(kickBots, b)
			}
		}
		s.mu.RUnlock()

		yellow.Printf("[%s] KICK target=%s — launching %d bots\n", s.id, targetId, len(kickBots))

		// Shuffle bot order so the same proxies don't always vote first.
		rand.Shuffle(len(kickBots), func(i, j int) {
			kickBots[i], kickBots[j] = kickBots[j], kickBots[i]
		})

		// Dynamic Human-Like Burst Stagger (150-250ms per bot) in a single
		// background goroutine so the main handler returns immediately.
		//
		// 50µs micro-stagger triggered Gartic's anti-flood: all 6 frames hit
		// Gartic's socket cluster within the same micro-burst window, and the
		// anti-spam filter silently dropped 5 of 6 — only 3 votes from bot #13
		// registered.
		//
		// By spacing each bot's vote by a random 150-250ms interval, Gartic
		// sees realistic human-like pacing across independent connections.
		// Total window for 15 bots: ~2.25-3.75s — fast enough to complete
		// before the vote session expires, slow enough to bypass anti-flood.
		go func(kickBots []*Bot, target string) {
			var votesSent atomic.Int32
			total := len(kickBots)
			for i, b := range kickBots {
				if i > 0 {
					stagger := time.Duration(150+rand.Intn(100)) * time.Millisecond
					time.Sleep(stagger)
				}
				gid := int(b.garticId.Load())
				if i == 0 {
					payload := fmt.Sprintf(`42[45,%d,["%s",true]]`, gid, target)
					yellow.Printf("[KICK] First payload: %s (%d bytes)\n", payload, len(payload))
				}
				if gid == 0 {
					continue
				}
				b.SendRaw(fmt.Sprintf(`42[45,%d,["%s",true]]`, gid, target))
				if !b.IsAlive() {
					if !s.quietLogs.Load() {
						color.New(color.FgHiMagenta).Printf("  [KICK] Bot #%d (%s) gid=%d proxy=%s ✗ WRITE FAILED — connection dead\n",
							b.numericId, sanitizeNick(b.nick), gid, b.proxyIp)
					}
					continue
				}
				n := int(votesSent.Add(1))
				if !s.quietLogs.Load() {
					color.New(color.FgHiMagenta).Printf("  [KICK] Bot #%d (%s) gid=%d proxy=%s voted ✓ (%d/%d)\n",
						b.numericId, sanitizeNick(b.nick), gid, b.proxyIp, n, total)
				}
				s.Send(map[string]interface{}{
					"event":   "kickProgress",
					"voteNum": n,
					"total":   total,
					"botId":   b.numericId,
					"nick":    b.nick,
					"proxyIp": b.proxyIp,
				})
			}
			finalCount := int(votesSent.Load())
			s.Send(map[string]interface{}{
				"event":     "kickComplete",
				"targetId":  target,
				"votesSent": finalCount,
				"totalBots": total,
			})
			yellow.Printf("[%s] KICK complete — %d/%d votes sent for target %s\n",
				s.id, finalCount, total, target)
		}(kickBots, targetId)

	case "getDrawAll":
		s.ForEachBot(func(b *Bot) {
			if b.joinConfirmed.Load() {
				b.SendRaw(fmt.Sprintf("42[34,%d]", int(b.garticId.Load())))
			}
		})

	case "getBotList":
		bots := s.GetBotList()
		s.Send(map[string]interface{}{"event": "botList", "bots": bots})

	case "turboOn":
		room := parseRoom(getString(msg, "room", s.room))
		qty := int(getFloat(msg, "qty", 10))
		serverOverride := getString(msg, "server", "")
		idioma := getString(msg, "idioma", "")
		yellow.Printf("[%s] TURBO ON room=%s qty=%d idioma=%s\n", s.id, room, qty, idioma)

		cfg := &TurboConfig{
			Room:   room,
			Server: serverOverride,
			Target: qty,
			Idioma: idioma,
		}
		s.turboMu.Lock()
		s.turboDefaultConfig = cfg
		s.turboMu.Unlock()
		s.startTurboFromConfig(cfg)

	case "turboOff":
		yellow.Printf("[%s] TURBO OFF\n", s.id)
		s.StopTurbo()
		// Do NOT destroy active bots, they stay in the room!
		s.Send(map[string]interface{}{"event": "turboStatus", "ready": 0, "connecting": 0})
		// Sync the active bots to ensure the UI keeps showing them
		bots := s.GetBotList()
		s.Send(map[string]interface{}{"event": "botSync", "bots": bots})

	case "autoRejoin":
		enabled, _ := msg["enabled"].(bool)
		s.autoRejoin.Store(enabled)
		if enabled {
			room := parseRoom(getString(msg, "room", s.room))
			name := getString(msg, "name", "Bot")
			s.mu.Lock()
			s.autoRejoinConfig = &AutoJoinConfig{
				Room: room, Name: name, Nick: name,
				NickMode: getString(msg, "nickMode", "0"),
				Avatar:   getString(msg, "avatar", "0"),
				Target:   int(getFloat(msg, "qty", 1)),
				Idioma:   getString(msg, "idioma", ""),
			}
			if cn, ok := msg["customNicks"]; ok {
				if b, err := json.Marshal(cn); err == nil {
					json.Unmarshal(b, &s.autoRejoinConfig.CustomNicks)
				}
			}
			if jm, ok := msg["joinMessages"]; ok {
				if b, err := json.Marshal(jm); err == nil {
					json.Unmarshal(b, &s.autoRejoinConfig.JoinMessages)
				}
			}
			s.mu.Unlock()
		}
		yellow.Printf("[%s] AUTO-REJOIN %v\n", s.id, enabled)
		s.Send(map[string]interface{}{"event": "autoRejoinStatus", "enabled": enabled})
		go PutRoomPersist(s)

	case "autojoin":
		room := parseRoom(getString(msg, "room", s.room))
		qty := int(getFloat(msg, "qty", 10))
		name := getString(msg, "name", "Bot")
		nickMode := getString(msg, "nickMode", "0")
		avatar := getString(msg, "avatar", "0")
		idioma := getString(msg, "idioma", "")

		var customNicks []CustomNick
		if cn, ok := msg["customNicks"]; ok {
			if b, err := json.Marshal(cn); err == nil {
				json.Unmarshal(b, &customNicks)
			}
		}
		var joinMsgs []JoinMessage
		if jm, ok := msg["joinMessages"]; ok {
			if b, err := json.Marshal(jm); err == nil {
				json.Unmarshal(b, &joinMsgs)
			}
		}

		yellow.Printf("[%s] AUTO-JOIN room=%s qty=%d\n", s.id, room, qty)
		s.Send(map[string]interface{}{"event": "autoJoinStarted", "target": qty})

		if s.autoJoinCancel != nil {
			s.autoJoinCancel()
		}
		done := make(chan struct{})
		s.autoJoinCancel = func() { close(done) }

		go func() {
			s.isAutoJoining.Store(true)
			defer s.isAutoJoining.Store(false)
			for i := 0; i < qty; i++ {
				select {
				case <-done:
					return
				default:
				}
				createBot(s, room, name, nickMode, avatar, customNicks, joinMsgs, "", idioma)
				time.Sleep(150 * time.Millisecond)
			}
			time.Sleep(8 * time.Second)
			bots := s.GetBotList()
			s.Send(map[string]interface{}{"event": "botSync", "bots": bots})
			s.Send(map[string]interface{}{"event": "autoJoinStopped"})
		}()

	case "stopAutojoin":
		yellow.Printf("[%s] STOP AUTO-JOIN\n", s.id)
		if s.autoJoinCancel != nil {
			s.autoJoinCancel()
			s.autoJoinCancel = nil
		}
		s.isAutoJoining.Store(false)
		s.Send(map[string]interface{}{"event": "autoJoinStopped"})

	case "markRoom":
		enabled, _ := msg["enabled"].(bool)
		s.mu.Lock()
		s.marked = enabled
		s.mu.Unlock()
		s.Send(map[string]interface{}{"event": "markStatus", "marked": enabled})

	case "keepEmpty":
		enabled, _ := msg["enabled"].(bool)
		count := int(getFloat(msg, "count", 1))
		keepEmptyMu.Lock()
		keepEmptyRooms[s.room] = enabled
		keepEmptyCounts[s.room] = count
		keepEmptyMu.Unlock()
		s.Send(map[string]interface{}{
			"event":     "keepEmptyStatus",
			"keepEmpty": enabled,
			"count":     count,
		})
		go PutRoomPersist(s)

	case "rejoin":
		enabled, _ := msg["enabled"].(bool)
		rejoinMu.Lock()
		rejoinRooms[s.room] = enabled
		rejoinMu.Unlock()
		s.Send(map[string]interface{}{
			"event":  "rejoinStatus",
			"rejoin": enabled,
		})
		go PutRoomPersist(s)

	case "autofarm":
		enabled, _ := msg["enabled"].(bool)
		s.mu.Lock()
		s.autofarm = enabled
		s.mu.Unlock()
		yellow.Printf("[%s] AUTOFARM %v\n", s.id, enabled)
		go PutRoomPersist(s)

	case "setPrivateMode":
		enabled, _ := msg["enabled"].(bool)
		s.mu.Lock()
		s.privateMode = enabled
		s.mu.Unlock()
		if enabled {
			s.currentDrawerId.Store(0)
		}
		yellow.Printf("[%s] PRIVATE MODE %v\n", s.id, enabled)
		go PutRoomPersist(s)

	case "answerReveal":
		enabled, _ := msg["enabled"].(bool)
		s.mu.Lock()
		s.answerReveal = enabled
		s.mu.Unlock()
		yellow.Printf("[%s] ANSWER REVEAL %v\n", s.id, enabled)
		go PutRoomPersist(s)

	default:
		if cmd != "" {
			color.New(color.FgHiBlack).Printf("[%s] Unknown cmd: %s\n", s.id, cmd)
		}
	}
}