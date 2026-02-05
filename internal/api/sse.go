package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/htojiddinov77-png/worktime/internal/middleware"
)

// Event is a server-side message that should be delivered to:
// - the affected user (UserID)
// - and all admins (optional behavior implemented here)
type Event struct {
	Type   string      `json:"type"`
	UserID int64       `json:"user_id"` // affected user
	Data   interface{} `json:"data"`
}

type Hub struct {
	mu sync.RWMutex

	// userID -> set of clients (each client is a channel)
	userClients map[int64]map[chan Event]struct{}

	// admins get everything
	adminClients map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{
		userClients:  make(map[int64]map[chan Event]struct{}),
		adminClients: make(map[chan Event]struct{}),
	}
}

func (h *Hub) subscribeUser(userID int64) chan Event {
	ch := make(chan Event, 32) // buffered so slow clients don't block hub
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.userClients[userID] == nil {
		h.userClients[userID] = make(map[chan Event]struct{})
	}
	h.userClients[userID][ch] = struct{}{}
	return ch
}

func (h *Hub) subscribeAdmin() chan Event {
	ch := make(chan Event, 32)
	h.mu.Lock()
	h.adminClients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) unsubscribeUser(userID int64, ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if set := h.userClients[userID]; set != nil {
		delete(set, ch)
		if len(set) == 0 {
			delete(h.userClients, userID)
		}
	}
	close(ch)
}

func (h *Hub) unsubscribeAdmin(ch chan Event) {
	h.mu.Lock()
	delete(h.adminClients, ch)
	h.mu.Unlock()
	close(ch)
}

// Publish sends events to:
// - the affected user (evt.UserID)
// - and all admin clients
func (h *Hub) Publish(evt Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Deliver to affected user (if any)
	if evt.UserID != 0 {
		if set := h.userClients[evt.UserID]; set != nil {
			for ch := range set {
				select {
				case ch <- evt:
				default:
					// drop if client is slow
				}
			}
		}
	}

	// Deliver to all admins
	for ch := range h.adminClients {
		select {
		case ch <- evt:
		default:
			// drop if client is slow
		}
	}
}

func (h *Hub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	claims, ok := middleware.GetUser(r)
	if !ok || claims == nil || claims.Id == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // helps with nginx buffering

	// Subscribe based on role
	isAdmin := strings.EqualFold(claims.Role, "admin")

	var ch chan Event
	var unsub func()

	if isAdmin {
		ch = h.subscribeAdmin()
		unsub = func() { h.unsubscribeAdmin(ch) }
	} else {
		ch = h.subscribeUser(claims.Id)
		unsub = func() { h.unsubscribeUser(claims.Id, ch) }
	}
	defer unsub()

	// Tell client we're connected
	fmt.Fprintf(w, "event: connected\ndata: {\"ok\":true}\n\n")
	flusher.Flush()

	// Keepalive to reduce proxy idle disconnects
	keepAlive := time.NewTicker(10 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return

		case <-keepAlive.C:
			// Comment frame (valid SSE). Client ignores it but keeps connection alive.
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()

		case evt, ok := <-ch:
			if !ok {
				return
			}

			b, err := json.Marshal(evt.Data)
			if err != nil {
				// If marshal fails, skip sending this event
				continue
			}

			fmt.Fprintf(w, "event: %s\n", evt.Type)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}
