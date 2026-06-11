# Production Specification — Anonymous Client Deduplication & Presence System
*(for Go backend + React SPA + future Kotlin Compose app)*

## 1. Identity Model

### 1.1 Client Identity Requirement
Every anonymous client (web or Android) SHALL have a **stable UUID string** acting as a persistent per-installation identity.

### 1.2 Local Persistence
- **Web:** Store UUID in `localStorage` under key `"client_id"`.
- **Android (Compose):** Store UUID in `DataStore` or `SharedPreferences` under key `"client_id"`.

### 1.3 Transport
Every HTTP request MUST include the header:
```
X-Client-ID: <uuid>
```
Every WebSocket connection MUST include the same UUID via query string `client_id=<uuid>` or header `X-Client-ID: <uuid>`.

The backend MUST reject any request missing this identity.

---

## 2. In-Memory Deduplication (Go server)

### 2.1 Purpose
Prevent counting duplicate views or reactions from the same client for the same message.

### 2.2 Data Structures

**ViewDedupMap:** `map[messageID] -> *ViewEntry`  
**ViewEntry:** `{ Clients map[string]struct{}, LastTouched time.Time }`  
**PendingViews:** `map[messageID] -> incrementCount`  
**ReactionDedup:** `map[messageID] -> map[clientID] -> map[emoji]bool`  
**PresenceMap:** `map[clientID] -> activeConnectionCount`  
**onlineUserCount:** `int`

All maps protected by `sync.Mutex`.

---

## 3. Behavior

### 3.1 View Recording
`POST /api/messages/{id}/view` with `X-Client-ID`:
1. Validate clientID.
2. Look up or create ViewEntry.
3. If already seen → DO NOTHING.
4. If new → insert clientID, increment pendingViews, update LastTouched.

Returns `200 { "ok": true }`. Does NOT return updated count.

### 3.2 Reaction Recording
`POST /api/messages/{id}/react` with `{ "emoji": "<string>" }`:
1. Validate emoji.
2. Deduplicate using RAM map.
3. If new → increment pendingReactions.

Returns `200 { "ok": true }`.

### 3.3 Presence Tracking
On WS connect: increment presenceRefs[clientID]; if was 0 → increment onlineUserCount.  
On WS disconnect: decrement presenceRefs[clientID]; if becomes 0 → decrement onlineUserCount.

Broadcast `PRESENCE_UPDATE` only when count changes.

---

## 4. Periodic Tasks

### 4.1 View Flush (every 30s)
1. Snapshot + reset pendingViews.
2. `UPDATE messages SET view_count = view_count + $2 WHERE id = $1` in transaction.

### 4.2 RAM Cleanup (every 15min)
1. Remove ViewDedupMap entries older than 24h.
2. Optional: cap at 2000 most recent message IDs.

---

## 5. Database Schema

```sql
ALTER TABLE messages ADD COLUMN IF NOT EXISTS view_count BIGINT NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS reactions (
  message_id BIGINT REFERENCES messages(id) ON DELETE CASCADE,
  emoji      TEXT    NOT NULL,
  count      BIGINT  NOT NULL DEFAULT 0,
  PRIMARY KEY (message_id, emoji)
);
```

---

## 9. System Constraints
1. No Redis — all dedup in RAM.
2. No session cookies — rely solely on X-Client-ID.
3. No per-user DB writes — aggregated counters only.
4. MVP: single instance acceptable.
5. Only Go + Postgres + Caddy.

---

## 10. Performance Guarantees
- O(1) dedup check per view.
- O(1) presence increment/decrement.
- RAM < 50MB for 2000 messages × 6000 users.
- DB writes amortized via 30s batches.
- WS traffic throttled < 3 msgs/sec per event type.
