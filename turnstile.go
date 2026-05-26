package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// ============================================================================
// turnstile.go — Cloudflare Turnstile token pool management
// Gartic.io requires a valid Turnstile token on WebSocket namespace auth.
// Tokens are pulled from a remote staging server and pooled locally.
// ============================================================================

// turnstileTokenTTL is how long a captured token is considered usable.
// Gartic's siteverify rejects tokens older than ~30-60s, we use 120s as buffer.
const turnstileTokenTTL = 120 * time.Second

// turnstileChannelSize caps how many tokens can be queued at once.
const turnstileChannelSize = 4096

// turnstileSourceURL is the staging server we PULL tokens from.
const turnstileSourceURL = "https://mohanadino.duckdns.org:8443/assign?wait=10"

// turnstilePullTarget is the local pool size we aim to keep filled.
const turnstilePullTarget = 2000

// turnstilePullClient is a dedicated HTTP client for the puller goroutine.
var turnstilePullClient = &http.Client{Timeout: 20 * time.Second}

// turnstileToken wraps a token string with its receipt timestamp for TTL checks.
type turnstileToken struct {
	Token    string
	Received time.Time
}

// Global token pool and metrics
var (
	turnstileChan        = make(chan turnstileToken, turnstileChannelSize)
	turnstileLastInMu    sync.RWMutex
	turnstileLastIn      time.Time
	turnstileTotalIn     atomic.Int64 // Total tokens received
	turnstileTotalOut    atomic.Int64 // Total tokens dispensed
	turnstileExpired     atomic.Int64 // Tokens discarded due to TTL
	turnstileDropped     atomic.Int64 // Tokens dropped (buffer full)
	turnstilePulledIn    atomic.Int64 // Tokens pulled from remote source
	turnstilePullErrors  atomic.Int64 // Pull HTTP errors
	turnstilePullEmpties atomic.Int64 // Pull returned empty/invalid token
)

// AddTurnstileToken queues a fresh token. Drops it if the buffer is full.
func AddTurnstileToken(tok string) {
	if tok == "" {
		return
	}
	select {
	case turnstileChan <- turnstileToken{Token: tok, Received: time.Now()}:
		turnstileLastInMu.Lock()
		turnstileLastIn = time.Now()
		turnstileLastInMu.Unlock()
		turnstileTotalIn.Add(1)
	default:
		turnstileDropped.Add(1)
	}
}

// PopTurnstileToken returns a fresh token immediately, or "" if none queued.
// Automatically discards expired tokens.
func PopTurnstileToken() string {
	for {
		select {
		case t := <-turnstileChan:
			if time.Since(t.Received) > turnstileTokenTTL {
				turnstileExpired.Add(1)
				continue
			}
			turnstileTotalOut.Add(1)
			return t.Token
		default:
			return ""
		}
	}
}

// PopTurnstileTokenWait waits up to maxWait for a fresh token.
// Returns "" if no valid token is available within the deadline.
func PopTurnstileTokenWait(maxWait time.Duration) string {
	if maxWait <= 0 {
		return PopTurnstileToken()
	}
	deadline := time.NewTimer(maxWait)
	defer deadline.Stop()
	for {
		select {
		case t := <-turnstileChan:
			if time.Since(t.Received) > turnstileTokenTTL {
				turnstileExpired.Add(1)
				continue
			}
			turnstileTotalOut.Add(1)
			return t.Token
		case <-deadline.C:
			return ""
		}
	}
}

// TurnstilePoolStatus returns counts for the status endpoint.
func TurnstilePoolStatus() (available int, lastIn time.Time) {
	turnstileLastInMu.RLock()
	lastIn = turnstileLastIn
	turnstileLastInMu.RUnlock()
	return len(turnstileChan), lastIn
}

// pullOneTokenFromSource fetches a single Turnstile token from the remote source.
func pullOneTokenFromSource() string {
	req, err := http.NewRequest("GET", turnstileSourceURL, nil)
	if err != nil {
		turnstilePullErrors.Add(1)
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := turnstilePullClient.Do(req)
	if err != nil {
		turnstilePullErrors.Add(1)
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		turnstilePullErrors.Add(1)
		return ""
	}
	var body struct {
		Success bool   `json:"success"`
		Token   string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		turnstilePullErrors.Add(1)
		return ""
	}
	// Tokens from the source start with "0." or "1." — validate shape
	if len(body.Token) < 80 {
		turnstilePullEmpties.Add(1)
		return ""
	}
	if body.Token[:2] != "0." && body.Token[:2] != "1." {
		turnstilePullEmpties.Add(1)
		return ""
	}
	turnstilePulledIn.Add(1)
	return body.Token
}

// StartTurnstilePuller launches multiple background goroutines that keep the
// local channel topped up from the token source in parallel.
func StartTurnstilePuller() {
	for i := 0; i < 20; i++ {
		go func() {
			for {
				// If pool is full enough, wait before pulling more
				if len(turnstileChan) >= turnstilePullTarget {
					time.Sleep(1 * time.Second)
					continue
				}
				tok := pullOneTokenFromSource()
				if tok != "" {
					AddTurnstileToken(tok)
					continue
				}
				// Failed to pull — back off slightly
				time.Sleep(2 * time.Second)
			}
		}()
	}
}

// CanAutoRejoin returns true if it's worth firing an auto-rejoin right now.
// Checks if tokens are available or were recently received.
func CanAutoRejoin() bool {
	if len(turnstileChan) > 0 {
		return true
	}
	turnstileLastInMu.RLock()
	lastIn := turnstileLastIn
	turnstileLastInMu.RUnlock()
	return !lastIn.IsZero() && time.Since(lastIn) < 60*time.Second
}

// ============================================================================
// HTTP handlers for the turnstile token API
// ============================================================================

// handleTurnstileToken accepts POST requests with a token to add to the pool.
func handleTurnstileToken(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if len(body.Token) < 80 {
		http.Error(w, "token shape invalid", http.StatusBadRequest)
		return
	}
	if body.Token[:2] != "0." && body.Token[:2] != "1." {
		http.Error(w, "token shape invalid", http.StatusBadRequest)
		return
	}
	AddTurnstileToken(body.Token)
	avail, _ := TurnstilePoolStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":        true,
		"available": avail,
	})
}

// handleTurnstileStatus returns the current token pool status as JSON.
func handleTurnstileStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	avail, lastIn := TurnstilePoolStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"available":   avail,
		"lastInUnix":  lastIn.Unix(),
		"lastInAgoMs": time.Since(lastIn).Milliseconds(),
		"ttlSec":      int(turnstileTokenTTL.Seconds()),
		"totalIn":     turnstileTotalIn.Load(),
		"totalOut":    turnstileTotalOut.Load(),
		"expired":     turnstileExpired.Load(),
		"dropped":     turnstileDropped.Load(),
		"pulledIn":    turnstilePulledIn.Load(),
		"pullErrors":  turnstilePullErrors.Load(),
		"pullEmpties": turnstilePullEmpties.Load(),
		"pullTarget":  turnstilePullTarget,
		"sourceURL":   turnstileSourceURL,
	})
}
