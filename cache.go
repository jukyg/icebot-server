package main

import (
	"sync"
	"time"
)

// ============================================================================
// cache.go — Server info cache for reusing room tokens across bots
// Avoids redundant HTTP fetches when creating multiple bots for the same room.
// ============================================================================

// ServerInfoCache caches the server name, code, and cookies for a proxy IP.
// Each session has its own cache instance to avoid cross-session interference.
type ServerInfoCache struct {
	mu        sync.RWMutex
	server    string    // e.g. "server05"
	code      string    // e.g. "966568"
	proxyIP   string    // which proxy IP this cache entry belongs to
	cookies   string    // any cookies returned by gartic for this IP
	fetchedAt time.Time // when this cache entry was created
}

// get returns cached server info for the given proxy IP.
// Returns false if cache is empty, stale (>60s), or for a different IP.
func (c *ServerInfoCache) get(ip string) (server, code, cookies string, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.server == "" || c.proxyIP != ip {
		return "", "", "", false
	}
	if time.Since(c.fetchedAt) > 60*time.Second {
		return "", "", "", false
	}
	return c.server, c.code, c.cookies, true
}

// set stores server info for a proxy IP.
func (c *ServerInfoCache) set(server, code, ip, cookies string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.server = server
	c.code = code
	c.proxyIP = ip
	c.cookies = cookies
	c.fetchedAt = time.Now()
}

// invalidate clears the cache, forcing fresh fetches.
func (c *ServerInfoCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.server = ""
	c.code = ""
	c.proxyIP = ""
	c.cookies = ""
}