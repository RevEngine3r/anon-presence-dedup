package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/RevEngine3r/anon-presence-dedup/internal/dedup"
	"github.com/RevEngine3r/anon-presence-dedup/internal/presence"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
)

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type handler struct {
	store     *dedup.Store
	hub       *presence.Hub
	pool      *pgxpool.Pool
	adminUser string
	adminPass string
}

// ─── helpers ────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func extractClientID(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Client-ID"))
}

func (h *handler) isAdmin(r *http.Request) bool {
	token := r.Header.Get("X-Admin-Token")
	parts := strings.SplitN(token, ":", 2)
	if len(parts) != 2 {
		return false
	}
	userOK := subtle.ConstantTimeCompare([]byte(parts[0]), []byte(h.adminUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(parts[1]), []byte(h.adminPass)) == 1
	return userOK && passOK
}

func (h *handler) adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.isAdmin(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func pathInt(r *http.Request, key string) (int64, bool) {
	v, err := strconv.ParseInt(r.PathValue(key), 10, 64)
	return v, err == nil
}

// ─── admin login ─────────────────────────────────────────────────────────────

func (h *handler) adminLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	userOK := subtle.ConstantTimeCompare([]byte(body.Username), []byte(h.adminUser)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(body.Password), []byte(h.adminPass)) == 1
	if !userOK || !passOK {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	// Return a simple token: user:pass (base64 encode in production with JWT)
	token := body.Username + ":" + body.Password
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// ─── channels ────────────────────────────────────────────────────────────────

type Channel struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Color       string    `json:"color"`
	Emoji       string    `json:"emoji"`
	LogoURL     *string   `json:"logo_url"`
	CreatedAt   time.Time `json:"created_at"`
	MsgCount    int       `json:"msg_count"`
}

func (h *handler) listChannels(w http.ResponseWriter, r *http.Request) {
	rows, err := h.pool.Query(r.Context(), `
		SELECT c.id, c.name, c.description, c.color, c.emoji, c.logo_url, c.created_at,
			   COUNT(m.id) AS msg_count
		FROM channels c
		LEFT JOIN messages m ON m.channel_id = c.id
		GROUP BY c.id ORDER BY c.id`)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	defer rows.Close()
	var list []Channel
	for rows.Next() {
		var ch Channel
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Color, &ch.Emoji,
			&ch.LogoURL, &ch.CreatedAt, &ch.MsgCount); err != nil {
			continue
		}
		list = append(list, ch)
	}
	if list == nil {
		list = []Channel{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *handler) createChannel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
		Emoji       string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	if body.Color == "" { body.Color = "#2f81f7" }
	if body.Emoji == "" { body.Emoji = "💬" }
	var ch Channel
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO channels (name,description,color,emoji) VALUES($1,$2,$3,$4)
		 RETURNING id,name,description,color,emoji,logo_url,created_at`,
		body.Name, body.Description, body.Color, body.Emoji,
	).Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Color, &ch.Emoji, &ch.LogoURL, &ch.CreatedAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

func (h *handler) updateChannel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Color       *string `json:"color"`
		Emoji       *string `json:"emoji"`
		LogoURL     *string `json:"logo_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	_, err := h.pool.Exec(r.Context(), `
		UPDATE channels SET
		  name        = COALESCE($2, name),
		  description = COALESCE($3, description),
		  color       = COALESCE($4, color),
		  emoji       = COALESCE($5, emoji),
		  logo_url    = COALESCE($6, logo_url)
		WHERE id=$1`,
		id, body.Name, body.Description, body.Color, body.Emoji, body.LogoURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ─── messages ────────────────────────────────────────────────────────────────

type Message struct {
	ID         int64             `json:"id"`
	ChannelID  int64             `json:"channel_id"`
	Content    string            `json:"content"`
	ImageURL   *string           `json:"image_url"`
	ViewCount  int64             `json:"view_count"`
	Reactions  map[string]int64  `json:"reactions"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

func (h *handler) listMessages(w http.ResponseWriter, r *http.Request) {
	chID, ok := pathInt(r, "id")
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	rows, err := h.pool.Query(r.Context(), `
		SELECT m.id, m.channel_id, m.content, m.image_url, m.view_count, m.created_at, m.updated_at
		FROM messages m WHERE m.channel_id=$1 ORDER BY m.created_at ASC`, chID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.ChannelID, &m.Content, &m.ImageURL,
			&m.ViewCount, &m.CreatedAt, &m.UpdatedAt); err != nil {
			continue
		}
		m.Reactions = map[string]int64{}
		msgs = append(msgs, m)
	}
	// fetch reactions for all msgs in one query
	if len(msgs) > 0 {
		ids := make([]int64, len(msgs))
		idxMap := map[int64]int{}
		for i, m := range msgs { ids[i] = m.ID; idxMap[m.ID] = i }
		rrows, _ := h.pool.Query(r.Context(),
			`SELECT message_id, emoji, count FROM reactions WHERE message_id = ANY($1)`, ids)
		if rrows != nil {
			defer rrows.Close()
			for rrows.Next() {
				var mid int64; var emoji string; var cnt int64
				if err := rrows.Scan(&mid, &emoji, &cnt); err == nil {
					msgs[idxMap[mid]].Reactions[emoji] = cnt
				}
			}
		}
	}
	if msgs == nil { msgs = []Message{} }
	writeJSON(w, http.StatusOK, msgs)
}

func (h *handler) createMessage(w http.ResponseWriter, r *http.Request) {
	chID, ok := pathInt(r, "id")
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	var body struct {
		Content  string  `json:"content"`
		ImageURL *string `json:"image_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if strings.TrimSpace(body.Content) == "" && body.ImageURL == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content or image required"})
		return
	}
	var m Message
	m.Reactions = map[string]int64{}
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO messages (channel_id, content, image_url) VALUES($1,$2,$3)
		 RETURNING id, channel_id, content, image_url, view_count, created_at, updated_at`,
		chID, body.Content, body.ImageURL,
	).Scan(&m.ID, &m.ChannelID, &m.Content, &m.ImageURL, &m.ViewCount, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	// broadcast new message via WS
	h.hub.Broadcast(map[string]any{
		"type":    "NEW_MESSAGE",
		"message": m,
	})
	writeJSON(w, http.StatusCreated, m)
}

func (h *handler) updateMessage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	_, err := h.pool.Exec(r.Context(),
		`UPDATE messages SET content=$2, updated_at=NOW() WHERE id=$1`, id, body.Content)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *handler) deleteMessage(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	// get channel_id first for WS broadcast
	var chID int64
	h.pool.QueryRow(r.Context(), `SELECT channel_id FROM messages WHERE id=$1`, id).Scan(&chID)

	_, err := h.pool.Exec(r.Context(), `DELETE FROM messages WHERE id=$1`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	h.hub.Broadcast(map[string]any{"type": "DELETE_MESSAGE", "id": id, "channel_id": chID})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *handler) clearMessages(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt(r, "id")
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	_, err := h.pool.Exec(r.Context(), `DELETE FROM messages WHERE channel_id=$1`, id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db error"})
		return
	}
	h.hub.Broadcast(map[string]any{"type": "CLEAR_MESSAGES", "channel_id": id})
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ─── views & reactions ───────────────────────────────────────────────────────

func (h *handler) recordView(w http.ResponseWriter, r *http.Request) {
	clientID := extractClientID(r)
	if clientID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing client id"})
		return
	}
	msgID, ok := pathInt(r, "id")
	if !ok || !h.messageExists(r.Context(), msgID) {
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
	msgID, ok := pathInt(r, "id")
	if !ok || !h.messageExists(r.Context(), msgID) {
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
	if h.store.RecordReaction(msgID, clientID, body.Emoji) {
		// new reaction — upsert in DB and broadcast
		var newCount int64
		h.pool.QueryRow(r.Context(),
			`INSERT INTO reactions (message_id, emoji, count) VALUES($1,$2,1)
			 ON CONFLICT (message_id,emoji) DO UPDATE SET count=reactions.count+1
			 RETURNING count`, msgID, body.Emoji).Scan(&newCount)
		h.hub.Broadcast(map[string]any{
			"type":  "REACTION_UPDATE",
			"id":    msgID,
			"emoji": body.Emoji,
			"count": newCount,
		})
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// ─── websocket ───────────────────────────────────────────────────────────────

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
	if err != nil { return }
	client := &presence.Client{ClientID: clientID, Conn: conn}
	client.InitSend(32)
	h.hub.Register(client)
	defer h.hub.Unregister(client)
	go client.WritePump()
	for { if _, _, err := conn.ReadMessage(); err != nil { break } }
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func (h *handler) messageExists(ctx context.Context, id int64) bool {
	var exists bool
	err := h.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM messages WHERE id=$1)`, id).Scan(&exists)
	return err == nil && exists
}

func isValidEmoji(s string) bool {
	if s == "" || len(s) > 32 { return false }
	for _, r := range s {
		if !unicode.IsGraphic(r) { return false }
	}
	return true
}
