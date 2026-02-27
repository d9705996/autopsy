// Autopsy — Incident Response Management Platform
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	autopsyapi "github.com/d9705996/autopsy/internal/api"
	"github.com/d9705996/autopsy/internal/api/handler"
	"github.com/d9705996/autopsy/internal/config"
	"github.com/d9705996/autopsy/internal/db"
	"github.com/d9705996/autopsy/internal/health"
	"github.com/d9705996/autopsy/internal/observability"
	"github.com/d9705996/autopsy/internal/seed"
	"github.com/d9705996/autopsy/internal/version"
	"github.com/d9705996/autopsy/internal/worker"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// --- Observability -------------------------------------------------------
	obs, log, err := observability.New(ctx, &observability.Config{
		ServiceName:    "autopsy",
		ServiceVersion: version.Version,
		LogLevel:       cfg.Log.Level,
		LogFormat:      cfg.Log.Format,
		OTLPEndpoint:   cfg.OTel.OTLPEndpoint,
	})
	if err != nil {
		return fmt.Errorf("init observability: %w", err)
	}
	defer obs.Shutdown(context.Background())
	slog.SetDefault(log)
	log.Info("starting autopsy", "version", version.Version, "commit", version.Commit, "db_driver", cfg.DB.Driver)

	// --- Database ------------------------------------------------------------
	// db.New opens the connection, runs migrations (AutoMigrate for SQLite,
	// golang-migrate for Postgres), and returns the GORM handle plus an
	// optional pgxpool (non-nil only for postgres, used by River).
	gormDB, pool, err := db.New(ctx, &cfg.DB)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	if pool != nil {
		defer pool.Close()
	}
	log.Info("database ready", "driver", cfg.DB.Driver)

	// --- Seed admin ----------------------------------------------------------
	if err := seed.EnsureAdmin(ctx, gormDB, seed.AdminOptions{
		Email:    cfg.App.SeedAdminEmail,
		Password: cfg.App.SeedAdminPassword,
	}, log); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	// --- Worker queue --------------------------------------------------------
	// River migrations only run when Postgres is available.
	if pool != nil {
		if err := worker.MigrateRiver(ctx, pool); err != nil {
			return fmt.Errorf("river migrations: %w", err)
		}
		log.Info("river migrations applied")
	}

	wq, err := worker.New(ctx, pool, cfg.DB.Driver, cfg.Worker.Concurrency, log)
	if err != nil {
		return fmt.Errorf("create worker: %w", err)
	}
	if err := wq.Start(ctx); err != nil {
		return fmt.Errorf("start worker: %w", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := wq.Stop(stopCtx); err != nil {
			log.Error("worker stop error", "err", err)
		}
	}()

	// --- HTTP routes ---------------------------------------------------------
	healthHandler := health.New(db.NewPinger(gormDB))
	authHandler := handler.NewAuthHandler(gormDB, cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL)

	mux := http.NewServeMux()
	autopsyapi.RegisterRoutes(mux, healthHandler, authHandler, cfg.JWT.Secret)
	// Prometheus metrics endpoint
	mux.Handle("GET /metrics", promhttp.Handler())

	// SPA: serve embedded frontend from ui/dist
	registerSPA(mux, log)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTP.Port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// --- Start server --------------------------------------------------------
	log.Info("http server listening", "addr", srv.Addr)
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("server stopped cleanly")
	return nil
}
