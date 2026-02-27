// Package db manages database connections and schema migrations.
// It supports two drivers: "sqlite" (pure-Go, no external process) and
// "postgres" (PostgreSQL via pgx/v5).
package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"math"

	"github.com/d9705996/autopsy/internal/config"
	"github.com/d9705996/autopsy/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// New opens the database, runs migrations, and returns:
//   - a *gorm.DB for use by all application packages
//   - a *pgxpool.Pool only when Driver=="postgres", else nil (used by River)
func New(ctx context.Context, cfg *config.DBConfig) (*gorm.DB, *pgxpool.Pool, error) {
	switch cfg.Driver {
	case "postgres":
		return openPostgres(ctx, cfg)
	default:
		gormDB, err := openSQLite(cfg)
		return gormDB, nil, err
	}
}

// openSQLite opens (or creates) the SQLite database file and runs AutoMigrate.
func openSQLite(cfg *config.DBConfig) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(cfg.File), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Enable WAL mode for better concurrent read performance.
	if err := db.Exec("PRAGMA journal_mode=WAL").Error; err != nil {
		return nil, fmt.Errorf("set wal mode: %w", err)
	}
	if err := db.Exec("PRAGMA foreign_keys=ON").Error; err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	// AutoMigrate creates / updates tables to match the model structs.
	if err := db.AutoMigrate(
		&model.Organization{},
		&model.User{},
		&model.RefreshToken{},
	); err != nil {
		return nil, fmt.Errorf("sqlite automigrate: %w", err)
	}
	return db, nil
}

// openPostgres opens a GORM Postgres connection via pgx/v5/stdlib and also
// returns a raw pgxpool.Pool for use by the River job queue.
func openPostgres(ctx context.Context, cfg *config.DBConfig) (*gorm.DB, *pgxpool.Pool, error) {
	// Build pgxpool for River (and also used as the stdlib connection).
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, nil, fmt.Errorf("parse db dsn: %w", err)
	}
	if cfg.MaxConns > math.MaxInt32 {
		return nil, nil, fmt.Errorf("DB_MAX_CONNS %d exceeds maximum value (%d)", cfg.MaxConns, math.MaxInt32)
	}
	poolCfg.MaxConns = int32(cfg.MaxConns)

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, nil, fmt.Errorf("create pgxpool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}

	// Apply SQL migrations before opening GORM so the schema is ready.
	if err := runPostgresMigrations(cfg.DSN); err != nil {
		pool.Close()
		return nil, nil, err
	}

	// Open a GORM DB backed by pgx/stdlib (reuses the pgx connection config).
	sqlDB := stdlib.OpenDBFromPool(pool)
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("open gorm/postgres: %w", err)
	}

	return gormDB, pool, nil
}

// runPostgresMigrations applies all pending SQL migrations via golang-migrate.
func runPostgresMigrations(dsn string) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("load migration source: %w", err)
	}

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse dsn for migrations: %w", err)
	}
	sqlDB := stdlib.OpenDB(*poolCfg.ConnConfig)
	defer func() { _ = sqlDB.Close() }()

	driver, err := migratepostgres.WithInstance(sqlDB, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("postgres migrate driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

// DBPinger wraps *gorm.DB and satisfies the health.Pinger interface.
type DBPinger struct {
	db *gorm.DB
}

// NewPinger returns a DBPinger that can be passed to health.New.
func NewPinger(db *gorm.DB) *DBPinger {
	return &DBPinger{db: db}
}

// Ping checks database connectivity.
func (p *DBPinger) Ping(ctx context.Context) error {
	sqlDB, err := p.db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	return sqlDB.PingContext(ctx)
}
