# Anonymous Client Deduplication & Presence System

A production-grade system for anonymous client identity, in-memory deduplication, and real-time presence tracking.

## Stack
- **Backend:** Go (no Redis, pure in-memory maps)
- **Frontend:** React SPA (TypeScript + Vite)
- **Database:** PostgreSQL
- **Proxy:** nginx
- **Future:** Kotlin Compose Android client

## Architecture

```
┌─────────────────────────────────────────────────────┐
│  React SPA / Kotlin Compose                         │
│  • UUID v4 in localStorage / DataStore              │
│  • X-Client-ID header on every request              │
│  • Local seen_views set (IntersectionObserver)      │
└────────────────────┬────────────────────────────────┘
                     │ HTTP + WebSocket
┌────────────────────▼────────────────────────────────┐
│  nginx (reverse proxy + static files)               │
│  • /api/* → Go backend (HTTP/1.1 keepalive)         │
│  • /ws    → Go backend (WebSocket upgrade)           │
│  • /      → React SPA dist (try_files)               │
└────────────────────┬────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────┐
│  Go Backend                                          │
│  • In-memory ViewDedupMap  (sync.Mutex)              │
│  • In-memory PresenceMap   (sync.Mutex)              │
│  • 30s flush → Postgres batch UPDATE                 │
│  • 15min RAM cleanup (24h TTL, max 2000 entries)     │
└────────────────────┬────────────────────────────────┘
                     │
┌────────────────────▼────────────────────────────────┐
│  PostgreSQL                                          │
│  messages(view_count BIGINT)                         │
│  reactions(message_id, emoji, count)                 │
└─────────────────────────────────────────────────────┘
```

## Quickstart

```bash
# Backend
cd backend
go mod tidy
POSTGRES_DSN="postgres://user:pass@localhost/db?sslmode=disable" go run ./cmd/server

# Frontend
cd frontend
npm install
npm run dev

# Production (Docker Compose)
docker compose up -d
```

## nginx Setup (bare metal)

```bash
# 1. Build frontend
cd frontend && npm run build
sudo mkdir -p /var/www/anon-presence
sudo cp -r dist /var/www/anon-presence/

# 2. Install config
sudo cp nginx.conf /etc/nginx/sites-available/anon-presence
sudo ln -s /etc/nginx/sites-available/anon-presence /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx

# 3. TLS with certbot
sudo certbot --nginx -d example.com
```

## Invariant

> A client event is uniquely identified by `(clientID, messageID, optional emoji)`.  
> Duplicated events MUST NOT increment counters more than once.

This holds across multiple tabs, reconnections, rapid scrolls, refreshes, and Android app restarts.

## Spec

See [SPEC.md](./SPEC.md) for the full production specification.
