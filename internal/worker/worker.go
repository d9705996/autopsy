// Package worker bootstraps the River job queue.
package worker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// HealthCheckArgs is a trivial job used to validate queue wiring.
type HealthCheckArgs struct{}

// Kind returns the unique job type identifier for health check jobs.
func (HealthCheckArgs) Kind() string { return "health_check" }

type healthCheckWorker struct {
	river.WorkerDefaults[HealthCheckArgs]
	log *slog.Logger
}

func (w *healthCheckWorker) Work(_ context.Context, _ *river.Job[HealthCheckArgs]) error {
	w.log.Debug("health check job executed")
	return nil
}

// Queue is the interface exposed by both the real River client and noopQueue.
type Queue interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// Client wraps river.Client and exposes a Start/Stop lifecycle.
type Client struct {
	client *river.Client[pgx.Tx]
	log    *slog.Logger
}

// Start begins processing queued jobs.
func (c *Client) Start(ctx context.Context) error { return c.client.Start(ctx) }

// Stop gracefully shuts down the worker client.
func (c *Client) Stop(ctx context.Context) error { return c.client.Stop(ctx) }

// noopQueue is used when River is unavailable (e.g. DB_DRIVER=sqlite).
type noopQueue struct{ log *slog.Logger }

func (n *noopQueue) Start(_ context.Context) error {
	n.log.Info("worker queue disabled (sqlite driver — River requires postgres)")
	return nil
}
func (n *noopQueue) Stop(_ context.Context) error { return nil }

// New creates a queue implementation appropriate for the given driver.
//   - "postgres": returns a fully-functional River client backed by pool.
//   - anything else: returns a no-op queue that logs a startup notice.
//
// pool may be nil when driver != "postgres".
func New(ctx context.Context, pool *pgxpool.Pool, driver string, concurrency int, log *slog.Logger) (Queue, error) {
	if driver != "postgres" {
		return &noopQueue{log: log}, nil
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &healthCheckWorker{log: log})

	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: concurrency},
		},
		Workers: workers,
		Logger:  log,
	})
	if err != nil {
		return nil, fmt.Errorf("create river client: %w", err)
	}
	return &Client{client: client, log: log}, nil
}

// MigrateRiver runs River's built-in schema migrations against the given pool.
// Only call this when DB_DRIVER=postgres.
func MigrateRiver(ctx context.Context, db *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(db), nil)
	if err != nil {
		return fmt.Errorf("create river migrator: %w", err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("run river migrations: %w", err)
	}
	return nil
}
