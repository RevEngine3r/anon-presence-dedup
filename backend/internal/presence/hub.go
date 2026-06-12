package presence

import (
	"encoding/json"
	"sync"
)

// Hub manages WebSocket clients and broadcasts messages.
type Hub struct {
	mu           sync.Mutex
	clients      map[*Client]struct{}
	presenceRefs map[string]int
	onlineCount  int
}

func NewHub() *Hub {
	return &Hub{
		clients:      make(map[*Client]struct{}),
		presenceRefs: make(map[string]int),
	}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	h.presenceRefs[c.ClientID]++
	if h.presenceRefs[c.ClientID] == 1 {
		h.onlineCount++
		h.broadcastPresenceLocked()
	}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	h.presenceRefs[c.ClientID]--
	if h.presenceRefs[c.ClientID] <= 0 {
		delete(h.presenceRefs, c.ClientID)
		h.onlineCount--
		if h.onlineCount < 0 { h.onlineCount = 0 }
		h.broadcastPresenceLocked()
	}
}

func (h *Hub) Broadcast(v any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.broadcastLocked(v)
}

func (h *Hub) broadcastPresenceLocked() {
	h.broadcastLocked(map[string]any{
		"type":   "PRESENCE_UPDATE",
		"online": h.onlineCount,
	})
}

func (h *Hub) broadcastLocked(v any) {
	data, err := json.Marshal(v)
	if err != nil { return }
	for c := range h.clients {
		select {
		case c.Send <- data:
		default:
			delete(h.clients, c)
			close(c.Send)
		}
	}
}
