package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/RevEngine3r/anon-presence-dedup/internal/config"
	"github.com/RevEngine3r/anon-presence-dedup/internal/db"
	"github.com/RevEngine3r/anon-presence-dedup/internal/dedup"
	"github.com/RevEngine3r/anon-presence-dedup/internal/handlers"
	"github.com/RevEngine3r/anon-presence-dedup/internal/presence"
)

func main() {
	cfgPath := flag.String("config", "", "path to server.yml (default: ./server.yml)")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := db.Connect(cfg.Postgres)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer pool.Close()

	dedupStore := dedup.NewStore(cfg.Dedup)
	presenceHub := presence.NewHub(cfg.WebSocket)

	go dedupStore.RunFlusher(pool)
	go dedupStore.RunCleaner()
	go presenceHub.Run()

	mux := handlers.NewRouter(dedupStore, presenceHub, pool)

	srv := &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	log.Printf("server listening on %s", cfg.Server.Addr)
	log.Fatal(srv.ListenAndServe())
}
