package presence

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/RevEngine3r/anon-presence-dedup/internal/config"
	"github.com/gorilla/websocket"
)

// Client represents a WebSocket connection.
type Client struct {
	ClientID string
	Conn     *websocket.Conn
	send     chan []byte
}

// InitSend initialises the buffered send channel.
func (c *Client) InitSend(bufSize int) { c.send = make(chan []byte, bufSize) }

// WritePump pumps messages from the send channel to the WebSocket.
func (c *Client) WritePump() {
	defer c.Conn.Close()
	for msg := range c.send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

// Hub manages all WebSocket connections and presence state.
type Hub struct {
	cfg config.WebSocketConfig
	mu  sync.Mutex

	presenceRefs map[string]int
	onlineCount  int

	clients     map[*Client]struct{}
	register    chan *Client
	unregister  chan *Client
	broadcastCh chan []byte
}

func NewHub(cfg config.WebSocketConfig) *Hub {
	return &Hub{
		cfg:          cfg,
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
			changed := h.presenceRefs[c.ClientID] == 1
			if changed {
				h.onlineCount++
			}
			h.mu.Unlock()
			if changed {
				h.broadcastPresence()
			}

		case c := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, c)
			close(c.send)
			h.presenceRefs[c.ClientID]--
			changed := h.presenceRefs[c.ClientID] == 0
			if changed {
				delete(h.presenceRefs, c.ClientID)
				h.onlineCount--
			}
			h.mu.Unlock()
			if changed {
				h.broadcastPresence()
			}

		case msg := <-h.broadcastCh:
			h.mu.Lock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Register(c *Client)   { h.register <- c }
func (h *Hub) Unregister(c *Client) { h.unregister <- c }

func (h *Hub) Broadcast(msg []byte) {
	select {
	case h.broadcastCh <- msg:
	default:
		log.Println("hub: broadcast channel full, dropping")
	}
}

func (h *Hub) broadcastPresence() {
	h.mu.Lock()
	count := h.onlineCount
	h.mu.Unlock()
	msg, _ := json.Marshal(map[string]any{"type": "PRESENCE_UPDATE", "online": count})
	h.Broadcast(msg)
}

func (h *Hub) BroadcastReaction(msgID int64, emoji string, count int64) {
	msg, _ := json.Marshal(map[string]any{
		"type": "REACTION_UPDATE", "id": msgID, "emoji": emoji, "count": count,
	})
	h.Broadcast(msg)
}
