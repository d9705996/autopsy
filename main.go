package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/example/autopsy/internal/api"
	"github.com/example/autopsy/internal/auth"
	"github.com/example/autopsy/internal/store"
	"github.com/example/autopsy/internal/triage"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

//go:embed web/*
var webFS embed.FS

func main() {
	addr := envOrDefault("AUTOPSY_ADDR", ":8080")
	adminUser := envOrDefault("AUTOPSY_ADMIN_USER", "admin")
	adminPassword := envOrDefault("AUTOPSY_ADMIN_PASSWORD", "admin")

	dbDriver := envOrDefault("AUTOPSY_DB_DRIVER", "sqlite")
	dbDSN := envOrDefault("AUTOPSY_DB_DSN", "file:autopsy.db?_pragma=busy_timeout(5000)")
	if dbDriver == "postgres" && dbDSN == "file:autopsy.db?_pragma=busy_timeout(5000)" {
		dbDSN = envOrDefault("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/autopsy?sslmode=disable")
	}

	repo, err := store.NewSQLStore(dbDriver, dbDSN)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer repo.Close()

	server := api.NewServer(repo, triage.NewHeuristicAgent(), auth.New(adminUser, adminPassword), webFS)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("autopsy listening on %s using %s", addr, dbDriver)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
