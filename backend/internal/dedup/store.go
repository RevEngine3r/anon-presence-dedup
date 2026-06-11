package dedup

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/RevEngine3r/anon-presence-dedup/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
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
	cfg config.DedupConfig
	mu  sync.Mutex

	viewDedup     map[int64]*ViewEntry
	pendingViews  map[int64]int64
	reactionDedup map[int64]map[ReactionKey]struct{}
	pendingReacts map[int64]map[string]int64
}

func NewStore(cfg config.DedupConfig) *Store {
	return &Store{
		cfg:           cfg,
		viewDedup:     make(map[int64]*ViewEntry),
		pendingViews:  make(map[int64]int64),
		reactionDedup: make(map[int64]map[ReactionKey]struct{}),
		pendingReacts: make(map[int64]map[string]int64),
	}
}

// RecordView deduplicates and increments the pending view counter.
func (s *Store) RecordView(messageID int64, clientID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.viewDedup[messageID]
	if !ok {
		entry = &ViewEntry{Clients: make(map[string]struct{})}
		s.viewDedup[messageID] = entry
	}
	if _, seen := entry.Clients[clientID]; seen {
		return false
	}
	entry.Clients[clientID] = struct{}{}
	entry.LastTouched = time.Now()
	s.pendingViews[messageID]++
	return true
}

// RecordReaction deduplicates and increments the pending reaction counter.
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

// RunFlusher flushes pending counts to Postgres on the configured interval.
func (s *Store) RunFlusher(pool *pgxpool.Pool) {
	ticker := time.NewTicker(s.cfg.FlushInterval)
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
			`UPDATE messages SET view_count = view_count + $1 WHERE id = $2`, inc, msgID)
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
				`INSERT INTO reactions (message_id, emoji, count) VALUES ($1, $2, $3)
				 ON CONFLICT (message_id, emoji)
				 DO UPDATE SET count = reactions.count + EXCLUDED.count`,
				msgID, emoji, inc)
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

// RunCleaner removes stale entries on the configured interval.
func (s *Store) RunCleaner() {
	ticker := time.NewTicker(s.cfg.CleanInterval)
	defer ticker.Stop()
	for range ticker.C {
		s.clean()
	}
}

func (s *Store) clean() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.cfg.ViewTTL)
	for msgID, entry := range s.viewDedup {
		if entry.LastTouched.Before(cutoff) {
			delete(s.viewDedup, msgID)
		}
	}

	if len(s.viewDedup) > s.cfg.MaxEntries {
		type kv struct {
			id int64
			ts time.Time
		}
		entries := make([]kv, 0, len(s.viewDedup))
		for id, e := range s.viewDedup {
			entries = append(entries, kv{id, e.LastTouched})
		}
		for len(s.viewDedup) > s.cfg.MaxEntries {
			oldest := 0
			for i, e := range entries {
				if e.ts.Before(entries[oldest].ts) {
					oldest = i
				}
			}
			delete(s.viewDedup, entries[oldest].id)
			entries = append(entries[:oldest], entries[oldest+1:]...)
		}
	}
}
