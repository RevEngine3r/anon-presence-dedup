package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/RevEngine3r/anon-presence-dedup/internal/dedup"
	"github.com/RevEngine3r/anon-presence-dedup/internal/presence"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type handler struct {
	store *dedup.Store
	hub   *presence.Hub
	pool  *pgxpool.Pool
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func extractClientID(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Client-ID"))
}

func (h *handler) recordView(w http.ResponseWriter, r *http.Request) {
	clientID := extractClientID(r)
	if clientID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing client id"})
		return
	}

	rawID := r.PathValue("id")
	msgID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	if !h.messageExists(r.Context(), msgID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	h.store.RecordView(msgID, clientID)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *handler) recordReaction(w http.ResponseWriter, r *http.Request) {
	clientID := extractClientID(r)
	if clientID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing client id"})
		return
	}

	rawID := r.PathValue("id")
	msgID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	var body struct {
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || !isValidEmoji(body.Emoji) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid emoji"})
		return
	}

	if !h.messageExists(r.Context(), msgID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	h.store.RecordReaction(msgID, clientID, body.Emoji)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *handler) serveWS(w http.ResponseWriter, r *http.Request) {
	clientID := r.URL.Query().Get("client_id")
	if clientID == "" {
		clientID = r.Header.Get("X-Client-ID")
	}
	if clientID == "" {
		http.Error(w, `{"error":"missing client id"}`, http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &presence.Client{
		ClientID: clientID,
		Conn:     conn,
	}
	// initialise send channel via exported field
	client.InitSend(32)

	h.hub.Register(client)
	defer h.hub.Unregister(client)

	go client.WritePump()

	// read pump — discard incoming, detect close
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (h *handler) messageExists(ctx context.Context, id int64) bool {
	var exists bool
	err := h.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE id=$1)`, id).Scan(&exists)
	return err == nil && exists
}

// isValidEmoji returns true if the string is a non-empty emoji or short text.
func isValidEmoji(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for _, r := range s {
		// allow emoji ranges, ASCII punctuation-like chars
		if !unicode.IsGraphic(r) {
			return false
		}
	}
	return true
}
