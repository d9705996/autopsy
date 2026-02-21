package main

import (
	"embed"
	"log"
	"net/http"
	"os"

	"github.com/example/autopsy/internal/api"
	"github.com/example/autopsy/internal/auth"
	"github.com/example/autopsy/internal/store"
	"github.com/example/autopsy/internal/triage"
)

//go:embed web/*
var webFS embed.FS

func main() {
	addr := envOrDefault("AUTOPSY_ADDR", ":8080")
	adminUser := envOrDefault("AUTOPSY_ADMIN_USER", "admin")
	adminPassword := envOrDefault("AUTOPSY_ADMIN_PASSWORD", "admin")

	server := api.NewServer(store.NewMemoryStore(), triage.NewHeuristicAgent(), auth.New(adminUser, adminPassword), webFS)
	log.Printf("autopsy listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Router()); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
