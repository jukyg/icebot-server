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

// Session manages a single extension ↔ server connection.
type Session struct {
	id                 string
	ws                 *websocket.Conn
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
	aiChatEnabled      bool
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

	// Rejoin throttle: prevents auto-rejoin storms when multiple bots
	// disconnect simultaneously. Only one rejoin operation runs at a time,
	// with a minimum 500ms interval between them.
	rejoinInProgress atomic.Bool

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
		ws:          ws,
		bots:        make(map[int]*Bot),
		room:        room,
		srvCache:    &ServerInfoCache{},
		connectPool: make(chan ConnectJob, 500),
		connectDone: make(chan struct{}),
		autofarm: true,
	}
	s.autoRejoin.Store(true)

	for i := 0; i < connectWorkers; i++ {
		go s.connectWorker()
	}

	sessions[id] = s
	color.New(color.FgHiCyan).Printf("[Session %s] Created (room=%s)\n", id, room)

	s.Send(map[string]interface{}{
		"event":     "sessionCreated",
		"sessionId": id,
		"marked":    s.marked,
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

// Send sends a JSON event to the connected extension.
func (s *Session) Send(event map[string]interface{}) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	if s.ws == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	s.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	s.ws.WriteMessage(websocket.TextMessage, data)
	s.ws.SetWriteDeadline(time.Time{})
}

// SendGameEvent forwards an event only from the reporter bot.
func (s *Session) SendGameEvent(botNumId int, event map[string]interface{}) {
	s.mu.RLock()
	reporter := s.reporterBotId
	s.mu.RUnlock()
	if botNumId == reporter {
		s.Send(event)
	}
}

func (s *Session) handleAIChatResponse(senderGarticID int64, chatMsg string, senderBot *Bot, botNumId int) {
	entry := autoDeploy.Get(fmt.Sprintf("%d", senderGarticID))
	if entry == nil {
		return
	}
	if !entry.AIChat {
		return
	}

	persona := entry.AIPersona
	targetName := entry.Name

	s.mu.RLock()
	bots := make([]*Bot, 0, len(s.bots))
	for _, b := range s.bots {
		if b.IsAlive() && b.garticId.Load() != senderGarticID {
			bots = append(bots, b)
		}
	}
	s.mu.RUnlock()

	if len(bots) == 0 {
		return
	}

	delay := time.Duration(200+rand.Intn(600)) * time.Millisecond
	time.Sleep(delay)

	response := aiChatResponseForTarget(chatMsg, persona, targetName)
	if response == "" {
		return
	}

	responder := bots[rand.Intn(len(bots))]
	gid := responder.garticId.Load()
	if gid == 0 {
		return
	}
	responder.SendRaw(fmt.Sprintf(`42[11,%d,%s]`, int(gid), jsonString(response)))
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
// Returns true only if no other rejoin is in progress AND
// at least 500ms have elapsed since the last rejoin.
func (s *Session) tryBeginRejoin() bool {
	if !s.rejoinInProgress.CompareAndSwap(false, true) {
		return false
	}
	s.mu.Lock()
	since := time.Since(s.lastAutoRejoinTime)
	if since < 500*time.Millisecond {
		s.mu.Unlock()
		s.rejoinInProgress.Store(false)
		return false
	}
	s.lastAutoRejoinTime = time.Now()
	s.mu.Unlock()
	return true
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

// Reattach binds a new WebSocket to this session (e.g. on page refresh).
func (s *Session) Reattach(ws *websocket.Conn) {
	s.wsMu.Lock()
	oldWs := s.ws
	s.ws = ws
	s.wsMu.Unlock()

	if oldWs != nil {
		oldWs.Close()
	}

	s.mu.Lock()
	s.orphaned = false
	s.mu.Unlock()

	color.New(color.FgHiGreen).Printf("[Session %s] Reattached successfully! Syncing %d bots...\n", s.id, len(s.bots))

	s.Send(map[string]interface{}{
		"event":     "sessionCreated",
		"sessionId": s.id,
		"marked":    s.marked,
	})

	// Synchronize bots list instantly
	bots := s.GetBotList()
	s.Send(map[string]interface{}{
		"event": "botSync",
		"bots":  bots,
	})
}

// Close tears down the session ONLY if the connection matches the active one.
func (s *Session) CloseConn(ws *websocket.Conn) {
	s.wsMu.Lock()
	isCurrent := s.ws == ws
	s.wsMu.Unlock()

	if !isCurrent {
		// This is a defunct connection from a prior page reload — ignore it
		return
	}

	color.New(color.FgYellow).Printf("[Session %s] Extension disconnected\n", s.id)

	s.mu.Lock()
	hasAliveBots := false
	for _, b := range s.bots {
		if b.IsAlive() && b.joinConfirmed.Load() {
			hasAliveBots = true
			break
		}
	}
	if hasAliveBots {
		s.orphaned = true
		color.New(color.FgYellow).Printf("[Session %s] Orphaned — %d bots still alive in room %s (keeping alive permanently)\n",
			s.id, len(s.bots), s.room)
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()

	// No alive bots — full shutdown
	close(s.connectDone)
	s.DestroyAllBots()
	s.StopTurbo()

	sessionsMu.Lock()
	delete(sessions, s.id)
	sessionsMu.Unlock()
}

// joinWithTurbo creates N bots quickly, consuming from turbo pool first.
func (s *Session) joinWithTurbo(cfg *AutoJoinConfig, qty int) {
	s.isAutoJoining.Store(true)
	defer s.isAutoJoining.Store(false)

	// Small random jitter to prevent thundering herd when multiple
	// rejoin triggers fire within the same throttle window.
	time.Sleep(time.Duration(rand.Intn(200)) * time.Millisecond)

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

	qty := cfg.Target
	if qty <= 0 {
		qty = 1
	}
	for i := 0; i < qty; i++ {
		createBot(s, cfg.Room, cfg.Name, cfg.NickMode, cfg.Avatar, cfg.CustomNicks, cfg.JoinMessages, cfg.Server, cfg.Idioma)
		time.Sleep(50 * time.Millisecond)
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
		yellow.Printf("[%s] EXIT all bots\n", s.id)
		if s.autoJoinCancel != nil {
			s.autoJoinCancel()
			s.autoJoinCancel = nil
		}
		s.DestroyAllBots()
		s.Send(map[string]interface{}{"event": "botSync", "bots": []interface{}{}})

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
		s.mu.RUnlock()
		if ok && bot.IsAlive() && bot.joinConfirmed.Load() {
			bot.SendRaw(fmt.Sprintf(`42[11,%d,%s]`, int(bot.garticId.Load()), jsonString(text)))
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
				b.SendRaw(fmt.Sprintf(`42[11,%d,%s]`, gid, jsonString(text)))
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
				b.SendRaw(fmt.Sprintf(`42[11,%d,%s]`, int(b.garticId.Load()), jsonString(text)))
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

	case "autofarm":
		enabled, _ := msg["enabled"].(bool)
		s.mu.Lock()
		s.autofarm = enabled
		s.mu.Unlock()
		yellow.Printf("[%s] AUTOFARM %v\n", s.id, enabled)

	case "setPrivateMode":
		enabled, _ := msg["enabled"].(bool)
		s.mu.Lock()
		s.privateMode = enabled
		s.mu.Unlock()
		if enabled {
			s.currentDrawerId.Store(0)
		}
		yellow.Printf("[%s] PRIVATE MODE %v\n", s.id, enabled)

	case "setAIChat":
		enabled, _ := msg["enabled"].(bool)
		s.mu.Lock()
		s.aiChatEnabled = enabled
		s.mu.Unlock()
		color.New(color.FgHiGreen).Printf("[%s] AI CHAT %v\n", s.id, enabled)
		s.Send(map[string]interface{}{"event": "aiChatStatus", "enabled": enabled})

	default:
		if cmd != "" {
			color.New(color.FgHiBlack).Printf("[%s] Unknown cmd: %s\n", s.id, cmd)
		}
	}
}