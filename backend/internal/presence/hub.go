package presence

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const broadcastThrottle = 350 * time.Millisecond // ~3 msgs/sec max per type

// Client represents a WebSocket connection.
type Client struct {
	ClientID string
	Conn     *websocket.Conn
	send     chan []byte
}

// Hub manages all WebSocket connections and presence state.
type Hub struct {
	mu sync.Mutex

	// presenceRefs counts active WS connections per clientID.
	presenceRefs   map[string]int
	onlineCount    int

	clients        map[*Client]struct{}
	register       chan *Client
	unregister     chan *Client
	broadcastCh    chan []byte

	lastPresenceBroadcast time.Time
}

func NewHub() *Hub {
	return &Hub{
		presenceRefs: make(map[string]int),
		clients:      make(map[*Client]struct{}),
		register:     make(chan *Client, 64),
		unregister:   make(chan *Client, 64),
		broadcastCh:  make(chan []byte, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.presenceRefs[c.ClientID]++
			if h.presenceRefs[c.ClientID] == 1 {
				h.onlineCount++
				h.mu.Unlock()
				h.broadcastPresence()
			} else {
				h.mu.Unlock()
			}

		case c := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, c)
			close(c.send)
			h.presenceRefs[c.ClientID]--
			if h.presenceRefs[c.ClientID] == 0 {
				delete(h.presenceRefs, c.ClientID)
				h.onlineCount--
				h.mu.Unlock()
				h.broadcastPresence()
			} else {
				h.mu.Unlock()
			}

		case msg := <-h.broadcastCh:
			h.mu.Lock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// slow client — drop message
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Register(c *Client) { h.register <- c }
func (h *Hub) Unregister(c *Client) { h.unregister <- c }

// Broadcast sends a message to all connected clients (throttled).
func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcastCh <- msg:
	default:
		log.Println("presence hub: broadcast channel full, dropping")
	}
}

func (h *Hub) broadcastPresence() {
	h.mu.Lock()
	count := h.onlineCount
	h.mu.Unlock()

	msg, _ := json.Marshal(map[string]any{
		"type":   "PRESENCE_UPDATE",
		"online": count,
	})
	h.Broadcast(msg)
}

// BroadcastReaction broadcasts a reaction update (throttled to ~3/sec).
func (h *Hub) BroadcastReaction(msgID int64, emoji string, count int64) {
	msg, _ := json.Marshal(map[string]any{
		"type":  "REACTION_UPDATE",
		"id":    msgID,
		"emoji": emoji,
		"count": count,
	})
	h.Broadcast(msg)
}

// WritePump pumps messages from the send channel to the WebSocket.
func (c *Client) WritePump() {
	defer c.Conn.Close()
	for msg := range c.send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}
