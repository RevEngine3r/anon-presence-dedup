package handlers

import (
	"net/http"

	"github.com/RevEngine3r/anon-presence-dedup/internal/dedup"
	"github.com/RevEngine3r/anon-presence-dedup/internal/presence"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewRouter(store *dedup.Store, hub *presence.Hub, pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()

	h := &handler{store: store, hub: hub, pool: pool}

	mux.HandleFunc("POST /api/messages/{id}/view", h.recordView)
	mux.HandleFunc("POST /api/messages/{id}/react", h.recordReaction)
	mux.HandleFunc("/ws", h.serveWS)

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Client-ID")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
