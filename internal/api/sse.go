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
	// mu protects the shared maps below from concurrent access.
	// Why? Because many goroutines may:
	// - connect/disconnect clients (subscribe/unsubscribe)
	// - publish events (Publish)
	// at the same time.
	mu sync.RWMutex

	// userID -> set of clients (each client is a channel)
	// The reminder: map[chan Event]struct{} is a "set" in Go.
	// We don't care about values, only whether the key exists.
	userClients map[int64]map[chan Event]struct{}

	// admins get everything
	// Also a set: each admin connection has its own channel.
	adminClients map[chan Event]struct{}
}

func NewHub() *Hub {
	return &Hub{
		userClients:  make(map[int64]map[chan Event]struct{}),
		adminClients: make(map[chan Event]struct{}),
	}
}

// subscribeUser -> Normal user connected
func (h *Hub) subscribeUser(userID int64) chan Event {
	// Each browser/tab gets its own channel ("mailbox").
	// Buffer size 32 means we can queue up to 32 events for this client
	// without blocking Publish().
	ch := make(chan Event, 32)

	// Lock because we're MODIFYING shared maps (write operation).
	// Lock() blocks other goroutines from writing/reading these maps while we change them.
	h.mu.Lock()
	// defer makes sure Unlock runs even if we return early later (safe cleanup pattern).
	defer h.mu.Unlock()

	// Ensure the user's set exists (user can have multiple tabs/devices).
	if h.userClients[userID] == nil {
		h.userClients[userID] = make(map[chan Event]struct{})
	}

	// Add this client's channel into the user's set.
	h.userClients[userID][ch] = struct{}{}
	return ch
}

// subscribeAdmin -> Admin connected
func (h *Hub) subscribeAdmin() chan Event {
	ch := make(chan Event, 32)

	// Lock because we're MODIFYING shared map.
	h.mu.Lock()
	h.adminClients[ch] = struct{}{}
	h.mu.Unlock()

	return ch
}

// lose connection , for example after user logged out or close the tab
func (h *Hub) unsubscribeUser(userID int64, ch chan Event) {
	// Lock because we're MODIFYING shared maps.
	h.mu.Lock()
	defer h.mu.Unlock()

	if set := h.userClients[userID]; set != nil {
		// Remove this connection (channel) from the user's set.
		delete(set, ch)

		// If user has no more active connections (no tabs/devices),
		// remove the user entry entirely to keep map small/clean.
		if len(set) == 0 {
			delete(h.userClients, userID)
		}
	}

	// Closing the channel signals "no more events will be delivered" to this client.
	// Also prevents goroutine leaks and makes reads return ok=false.
	close(ch)
}

func (h *Hub) unsubscribeAdmin(ch chan Event) {
	// Lock because we're MODIFYING shared map.
	h.mu.Lock()
	delete(h.adminClients, ch)
	h.mu.Unlock()

	close(ch)
}

// Publish sends events to:
// - the affected user (evt.UserID)
// - and all admin clients
func (h *Hub) Publish(evt Event) {
	// RLock because we are only READING from the maps (not changing them).
	// RLock allows multiple readers at once, but blocks writers (subscribe/unsubscribe)
	// while reading is happening.
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Deliver to affected user (if any)
	if evt.UserID != 0 {
		if set := h.userClients[evt.UserID]; set != nil {
			for ch := range set {
				// Non-blocking send:
				// - if the client's channel has space, send the event
				// - if it's full, default triggers and we DROP the event
				// This prevents one slow client from blocking the whole system.
				select {
				case ch <- evt:
				default:
					// drop if client is slow (channel buffer full)
					// SSE is "best-effort live updates"; the UI can always refresh via REST.
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
	// SSE requires flushing data immediately.
	// http.Flusher is the interface that allows us to Flush().
	// flush ma'nosi immediately javob yuborish
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Auth: only logged-in users can open an SSE stream.
	claims, ok := middleware.GetUser(r)
	if !ok || claims == nil || claims.Id == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream") // treat this stream of events
	w.Header().Set("Cache-Control", "no-cache")         // don't cache this response
	w.Header().Set("Connection", "keep-alive")          // it helps tcp connection open
	w.Header().Set("X-Accel-Buffering", "no")           // helps with nginx buffering (don't delay/collect chunks)

	// Subscribe based on role
	isAdmin := strings.EqualFold(claims.Role, "admin")

	var ch chan Event
	var unsub func()

	if isAdmin {
		ch = h.subscribeAdmin()
		// Store the correct cleanup function for this connection.
		unsub = func() { h.unsubscribeAdmin(ch) }
	} else {
		ch = h.subscribeUser(claims.Id)
		unsub = func() { h.unsubscribeUser(claims.Id, ch) }
	}

	// Always cleanup when the connection ends (disconnect/crash/timeout).
	// If we forget this: memory leak + stale channels.
	defer unsub()

	// Tell client we're connected.
	// SSE format rules:
	// - "event:" line is optional (event name)
	// - "data:" line is payload (can repeat for multi-line data)
	// - blank line "\n\n" ends one SSE message/frame
	fmt.Fprintf(w, "event: connected\ndata: {\"ok\":true}\n\n")
	flusher.Flush() // push it to browser immediately

	// Keepalive stops proxies/nginx from killing silent connection.
	keepAlive := time.NewTicker(10 * time.Second)
	defer keepAlive.Stop()

	for {
		select {
		// If client disconnects, Go cancels the request context.
		// We exit, then defer unsub() runs to cleanup.
		case <-r.Context().Done(): // client disconnects, exit functions and use defer unsub()
			return

		case <-keepAlive.C:
			// Comment frame (valid SSE):
			// Lines starting with ":" are ignored by EventSource,
			// but they count as traffic for proxies (prevents idle timeout).
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()

		case evt, ok := <-ch:
			// If the channel is closed, ok=false → connection should end.
			if !ok {
				return
			}

			// Convert payload to JSON text so it can go into "data:".
			// Note: you're sending only evt.Data, not the full Event struct.
			b, err := json.Marshal(evt.Data) // convert evt.Data into JSON, write SSE Format, Flush
			if err != nil {
				// If marshal fails, skip sending this event
				continue
			}

			// Send SSE frame:
			// event: <type>
			// data: <json>
			// blank line ends frame
			fmt.Fprintf(w, "event: %s\n", evt.Type)
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		}
	}
}
