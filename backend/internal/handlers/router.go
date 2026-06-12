package handlers

import (
	"net/http"

	"github.com/RevEngine3r/anon-presence-dedup/internal/dedup"
	"github.com/RevEngine3r/anon-presence-dedup/internal/presence"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(store *dedup.Store, hub *presence.Hub, pool *pgxpool.Pool, adminUser, adminPass string) http.Handler {
	mux := http.NewServeMux()

	h := &handler{
		store:     store,
		hub:       hub,
		pool:      pool,
		adminUser: adminUser,
		adminPass: adminPass,
	}

	// --- public ---
	mux.HandleFunc("GET /api/channels", h.listChannels)
	mux.HandleFunc("GET /api/channels/{id}/messages", h.listMessages)
	mux.HandleFunc("POST /api/messages/{id}/view", h.recordView)
	mux.HandleFunc("POST /api/messages/{id}/react", h.recordReaction)
	mux.HandleFunc("/ws", h.serveWS)

	// --- admin ---
	mux.HandleFunc("POST /api/admin/login", h.adminLogin)
	mux.HandleFunc("POST /api/channels", h.adminOnly(h.createChannel))
	mux.HandleFunc("PATCH /api/channels/{id}", h.adminOnly(h.updateChannel))
	mux.HandleFunc("POST /api/channels/{id}/messages", h.adminOnly(h.createMessage))
	mux.HandleFunc("PATCH /api/messages/{id}", h.adminOnly(h.updateMessage))
	mux.HandleFunc("DELETE /api/messages/{id}", h.adminOnly(h.deleteMessage))
	mux.HandleFunc("DELETE /api/channels/{id}/messages", h.adminOnly(h.clearMessages))

	// --- static SPA (must be last) ---
	mux.Handle("/", http.FileServer(http.Dir("./frontend")))

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Client-ID, X-Admin-Token")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
