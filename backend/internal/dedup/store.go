package dedup

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	flushInterval   = 30 * time.Second
	cleanInterval   = 15 * time.Minute
	viewTTL         = 24 * time.Hour
	maxDedupEntries = 2000
)

// ViewEntry holds the set of clientIDs that have viewed a message.
type ViewEntry struct {
	Clients     map[string]struct{}
	LastTouched time.Time
}

// ReactionKey uniquely identifies a (clientID, emoji) pair per message.
type ReactionKey struct {
	ClientID string
	Emoji    string
}

// Store is the in-memory deduplication state.
type Store struct {
	mu sync.Mutex

	viewDedup    map[int64]*ViewEntry        // messageID -> entry
	pendingViews map[int64]int64             // messageID -> increment
	reactionDedup map[int64]map[ReactionKey]struct{} // messageID -> set of (clientID,emoji)
	pendingReacts map[int64]map[string]int64 // messageID -> emoji -> increment
}

func NewStore() *Store {
	return &Store{
		viewDedup:     make(map[int64]*ViewEntry),
		pendingViews:  make(map[int64]int64),
		reactionDedup: make(map[int64]map[ReactionKey]struct{}),
		pendingReacts: make(map[int64]map[string]int64),
	}
}

// RecordView deduplicates and increments the pending view counter.
// Returns true if the view was new (not a duplicate).
func (s *Store) RecordView(messageID int64, clientID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.viewDedup[messageID]
	if !ok {
		entry = &ViewEntry{Clients: make(map[string]struct{})}
		s.viewDedup[messageID] = entry
	}

	if _, seen := entry.Clients[clientID]; seen {
		return false // duplicate
	}

	entry.Clients[clientID] = struct{}{}
	entry.LastTouched = time.Now()
	s.pendingViews[messageID]++
	return true
}

// RecordReaction deduplicates and increments the pending reaction counter.
// Returns true if the reaction was new.
func (s *Store) RecordReaction(messageID int64, clientID, emoji string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := ReactionKey{ClientID: clientID, Emoji: emoji}

	if _, ok := s.reactionDedup[messageID]; !ok {
		s.reactionDedup[messageID] = make(map[ReactionKey]struct{})
	}
	if _, seen := s.reactionDedup[messageID][key]; seen {
		return false
	}
	s.reactionDedup[messageID][key] = struct{}{}

	if _, ok := s.pendingReacts[messageID]; !ok {
		s.pendingReacts[messageID] = make(map[string]int64)
	}
	s.pendingReacts[messageID][emoji]++
	return true
}

// RunFlusher flushes pending counts to Postgres every 30 seconds.
func (s *Store) RunFlusher(pool *pgxpool.Pool) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for range ticker.C {
		s.flushViews(pool)
		s.flushReactions(pool)
	}
}

func (s *Store) flushViews(pool *pgxpool.Pool) {
	s.mu.Lock()
	snapshot := s.pendingViews
	s.pendingViews = make(map[int64]int64)
	s.mu.Unlock()

	if len(snapshot) == 0 {
		return
	}

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Printf("flushViews begin tx: %v", err)
		return
	}
	defer tx.Rollback(ctx)

	for msgID, inc := range snapshot {
		if inc == 0 {
			continue
		}
		_, err = tx.Exec(ctx,
			`UPDATE messages SET view_count = view_count + $1 WHERE id = $2`,
			inc, msgID,
		)
		if err != nil {
			log.Printf("flushViews update msgID=%d: %v", msgID, err)
			return
		}
	}

	if err = tx.Commit(ctx); err != nil {
		log.Printf("flushViews commit: %v", err)
	}
}

func (s *Store) flushReactions(pool *pgxpool.Pool) {
	s.mu.Lock()
	snapshot := s.pendingReacts
	s.pendingReacts = make(map[int64]map[string]int64)
	s.mu.Unlock()

	if len(snapshot) == 0 {
		return
	}

	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Printf("flushReactions begin tx: %v", err)
		return
	}
	defer tx.Rollback(ctx)

	for msgID, emojiMap := range snapshot {
		for emoji, inc := range emojiMap {
			if inc == 0 {
				continue
			}
			_, err = tx.Exec(ctx,
				`INSERT INTO reactions (message_id, emoji, count)
				 VALUES ($1, $2, $3)
				 ON CONFLICT (message_id, emoji)
				 DO UPDATE SET count = reactions.count + EXCLUDED.count`,
				msgID, emoji, inc,
			)
			if err != nil {
				log.Printf("flushReactions upsert: %v", err)
				return
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		log.Printf("flushReactions commit: %v", err)
	}
}

// RunCleaner removes stale ViewDedupMap entries every 15 minutes.
func (s *Store) RunCleaner() {
	ticker := time.NewTicker(cleanInterval)
	defer ticker.Stop()
	for range ticker.C {
		s.clean()
	}
}

func (s *Store) clean() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-viewTTL)

	for msgID, entry := range s.viewDedup {
		if entry.LastTouched.Before(cutoff) {
			delete(s.viewDedup, msgID)
		}
	}

	// Enforce max entries: remove oldest if over limit.
	if len(s.viewDedup) > maxDedupEntries {
		// collect and sort by LastTouched, remove oldest
		type entry struct {
			id  int64
			ts  time.Time
		}
		entries := make([]entry, 0, len(s.viewDedup))
		for id, e := range s.viewDedup {
			entries = append(entries, entry{id, e.LastTouched})
		}
		// simple selection: remove entries until within limit
		for len(s.viewDedup) > maxDedupEntries {
			oldestIdx := 0
			for i, e := range entries {
				if e.ts.Before(entries[oldestIdx].ts) {
					oldestIdx = i
				}
			}
			delete(s.viewDedup, entries[oldestIdx].id)
			entries = append(entries[:oldestIdx], entries[oldestIdx+1:]...)
		}
	}
}
