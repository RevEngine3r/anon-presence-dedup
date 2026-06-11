package main

import (
	"log"
	"net/http"
	"os"

	"github.com/RevEngine3r/anon-presence-dedup/internal/db"
	"github.com/RevEngine3r/anon-presence-dedup/internal/dedup"
	"github.com/RevEngine3r/anon-presence-dedup/internal/handlers"
	"github.com/RevEngine3r/anon-presence-dedup/internal/presence"
)

func main() {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		log.Fatal("POSTGRES_DSN env var required")
	}

	pool, err := db.Connect(dsn)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	dedupStore := dedup.NewStore()
	presenceHub := presence.NewHub()

	go dedupStore.RunFlusher(pool)
	go dedupStore.RunCleaner()
	go presenceHub.Run()

	mux := handlers.NewRouter(dedupStore, presenceHub, pool)

	addr := ":8080"
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
