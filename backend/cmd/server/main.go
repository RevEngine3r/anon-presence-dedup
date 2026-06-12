package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/RevEngine3r/anon-presence-dedup/internal/db"
	"github.com/RevEngine3r/anon-presence-dedup/internal/dedup"
	"github.com/RevEngine3r/anon-presence-dedup/internal/handlers"
	"github.com/RevEngine3r/anon-presence-dedup/internal/presence"
)

func main() {
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" { dsn = "postgres://postgres:postgres@localhost:5432/binahayat?sslmode=disable" }

	adminUser := os.Getenv("ADMIN_USER")
	if adminUser == "" { adminUser = "admin" }
	adminPass := os.Getenv("ADMIN_PASS")
	if adminPass == "" { adminPass = "admin1234" }

	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" { addr = ":8080" }

	pool, err := db.Connect(ctx, dsn)
	if err != nil { log.Fatalf("db connect: %v", err) }
	defer pool.Close()

	store := dedup.NewStore()
	hub   := presence.NewHub()

	go store.RunFlusher(ctx, pool, 30*time.Second)
	go store.RunCleaner(ctx, 15*time.Minute)

	router := handlers.NewRouter(store, hub, pool, adminUser, adminPass)

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
