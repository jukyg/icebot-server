package main

import (
	"encoding/json"
	"net/http"
	"sync"
)

type TrackedEntry struct {
	TrackedID   string   `json:"trackedID"`
	Name        string   `json:"name"`
	Identifiers []map[string]string `json:"identifiers"`
	Enabled     bool     `json:"enabled"`
	Message     string   `json:"message"`
	Kick        bool     `json:"kick"`
	Loyalty     bool     `json:"loyalty"`
	AIChat      bool     `json:"aiChat"`
	AIPersona   string   `json:"aiPersona"`
}

type AutoDeployRegistry struct {
	mu          sync.RWMutex
	entries     map[string]*TrackedEntry
	master      bool
	immuneRooms map[string]bool
	forcedRooms map[string]bool
}

var autoDeploy = &AutoDeployRegistry{
	entries:     make(map[string]*TrackedEntry),
	master:      false,
	immuneRooms: make(map[string]bool),
	forcedRooms: make(map[string]bool),
}

func (r *AutoDeployRegistry) Upsert(e *TrackedEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries[e.TrackedID] = e
}

func (r *AutoDeployRegistry) Delete(trackedID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.entries, trackedID)
}

func (r *AutoDeployRegistry) Get(trackedID string) *TrackedEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.entries[trackedID]
}

func (r *AutoDeployRegistry) GetAll() []*TrackedEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]*TrackedEntry, 0, len(r.entries))
	for _, e := range r.entries {
		res = append(res, e)
	}
	return res
}

func (r *AutoDeployRegistry) SetMaster(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.master = enabled
}

func (r *AutoDeployRegistry) Master() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.master
}

func (r *AutoDeployRegistry) SetImmuneRoom(room string, immune bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if immune {
		r.immuneRooms[room] = true
	} else {
		delete(r.immuneRooms, room)
	}
}

func (r *AutoDeployRegistry) SetForcedRoom(room string, forced bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if forced {
		r.forcedRooms[room] = true
	} else {
		delete(r.forcedRooms, room)
	}
}

func (r *AutoDeployRegistry) ImmuneRooms() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]string, 0, len(r.immuneRooms))
	for c := range r.immuneRooms {
		res = append(res, c)
	}
	return res
}

func (r *AutoDeployRegistry) ForcedRooms() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]string, 0, len(r.forcedRooms))
	for c := range r.forcedRooms {
		res = append(res, c)
	}
	return res
}

func handleAutoDeployList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries":      autoDeploy.GetAll(),
		"masterEnabled": autoDeploy.Master(),
		"immuneRooms":  autoDeploy.ImmuneRooms(),
		"forcedRooms":  autoDeploy.ForcedRooms(),
	})
}

func handleAutoDeployUpsert(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var e TrackedEntry
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if e.TrackedID == "" {
		http.Error(w, "missing trackedID", http.StatusBadRequest)
		return
	}
	autoDeploy.Upsert(&e)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func handleAutoDeployDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	autoDeploy.Delete(id)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func handleAutoDeployMaster(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	enabled := r.URL.Query().Get("enabled") == "true"
	autoDeploy.SetMaster(enabled)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"enabled": enabled})
}

func handleAutoDeployImmuneRooms(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "missing room", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case "POST":
		autoDeploy.SetImmuneRoom(room, true)
	case "DELETE":
		autoDeploy.SetImmuneRoom(room, false)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

func handleAutoDeployForcedRooms(w http.ResponseWriter, r *http.Request) {
	room := r.URL.Query().Get("room")
	if room == "" {
		http.Error(w, "missing room", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case "POST":
		autoDeploy.SetForcedRoom(room, true)
	case "DELETE":
		autoDeploy.SetForcedRoom(room, false)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}
