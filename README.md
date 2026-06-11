# Anonymous Client Deduplication & Presence System

A production-grade system for anonymous client identity, in-memory deduplication, and real-time presence tracking.

## Stack
- **Backend:** Go (no Redis, pure in-memory maps)
- **Frontend:** React SPA (TypeScript + Vite)
- **Database:** PostgreSQL
- **Proxy:** Caddy
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
```

## Invariant

> A client event is uniquely identified by `(clientID, messageID, optional emoji)`.  
> Duplicated events MUST NOT increment counters more than once.

This holds across multiple tabs, reconnections, rapid scrolls, refreshes, and Android app restarts.

## Spec

See [SPEC.md](./SPEC.md) for the full production specification.
