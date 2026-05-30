package main

import (
	"encoding/json"
	"os"
	"sync"
)

// ============================================================================
// rooms.go — Persistent room settings storage
// Saves/restores per-room configuration (autoRejoin, autofarm, options, etc.)
// so settings survive server restarts and extension reconnects.
// ============================================================================

const roomConfigFile = "rooms_config.json"

// RoomPersist holds all per-room settings that should survive restarts.
type RoomPersist struct {
	Room           string          `json:"room"`
	AutoRejoin     bool            `json:"autoRejoin"`
	AutoRejoinCfg  *AutoJoinConfig `json:"autoRejoinCfg,omitempty"`
	Autofarm       bool            `json:"autofarm"`
	PrivateMode    bool            `json:"privateMode"`
	AnswerReveal   bool            `json:"answerReveal"`
	Rejoin         bool            `json:"rejoin"`
	KeepEmpty      bool            `json:"keepEmpty"`
	KeepEmptyCount int             `json:"keepEmptyCount"`
}

var (
	roomPersistMu  sync.RWMutex
	roomPersistMap = make(map[string]*RoomPersist)
)

// LoadRoomPersistence reads rooms_config.json at startup.
func LoadRoomPersistence() {
	data, err := os.ReadFile(roomConfigFile)
	if err != nil {
		return // first run — no config yet
	}
	var m map[string]*RoomPersist
	if err := json.Unmarshal(data, &m); err != nil {
		return
	}
	roomPersistMu.Lock()
	roomPersistMap = m
	roomPersistMu.Unlock()
}

// SaveRoomPersistence writes the current map to disk (in a goroutine).
func SaveRoomPersistence() {
	roomPersistMu.RLock()
	data, err := json.MarshalIndent(roomPersistMap, "", "  ")
	roomPersistMu.RUnlock()
	if err != nil {
		return
	}
	_ = os.WriteFile(roomConfigFile, data, 0644)
}

// GetRoomPersist returns the saved settings for a room (nil if none).
func GetRoomPersist(room string) *RoomPersist {
	if room == "" {
		return nil
	}
	roomPersistMu.RLock()
	defer roomPersistMu.RUnlock()
	return roomPersistMap[room]
}

// PutRoomPersist saves the current session settings for its room.
// Reads all fields from the live session so callers don't have to assemble them.
func PutRoomPersist(s *Session) {
	s.mu.RLock()
	room := s.room
	arcfg := s.autoRejoinConfig
	af := s.autofarm
	pm := s.privateMode
	ar := s.answerReveal
	s.mu.RUnlock()

	if room == "" {
		return
	}

	rejoinMu.RLock()
	rj := rejoinRooms[room]
	rejoinMu.RUnlock()

	keepEmptyMu.RLock()
	ke := keepEmptyRooms[room]
	kc := keepEmptyCounts[room]
	keepEmptyMu.RUnlock()

	p := &RoomPersist{
		Room:           room,
		AutoRejoin:     s.autoRejoin.Load(),
		AutoRejoinCfg:  arcfg,
		Autofarm:       af,
		PrivateMode:    pm,
		AnswerReveal:   ar,
		Rejoin:         rj,
		KeepEmpty:      ke,
		KeepEmptyCount: kc,
	}

	roomPersistMu.Lock()
	roomPersistMap[room] = p
	roomPersistMu.Unlock()

	go SaveRoomPersistence()
}

// ApplyRoomPersist restores persisted settings to a freshly created session.
func ApplyRoomPersist(s *Session) {
	p := GetRoomPersist(s.room)
	if p == nil {
		return
	}

	s.mu.Lock()
	s.autofarm = p.Autofarm
	s.privateMode = p.PrivateMode
	s.answerReveal = p.AnswerReveal
	if p.AutoRejoinCfg != nil {
		s.autoRejoinConfig = p.AutoRejoinCfg
	}
	s.mu.Unlock()

	s.autoRejoin.Store(p.AutoRejoin)

	rejoinMu.Lock()
	rejoinRooms[s.room] = p.Rejoin
	rejoinMu.Unlock()

	keepEmptyMu.Lock()
	keepEmptyRooms[s.room] = p.KeepEmpty
	keepEmptyCounts[s.room] = p.KeepEmptyCount
	keepEmptyMu.Unlock()
}
