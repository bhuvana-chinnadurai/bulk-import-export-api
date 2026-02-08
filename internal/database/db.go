package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bulk-import-export-api/internal/config"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

// DB wraps the sql.DB connection with additional functionality
type DB struct {
	*sql.DB
	log zerolog.Logger
}

// New creates a new database connection with connection pooling
func New(cfg *config.DatabaseConfig, log zerolog.Logger) (*DB, error) {
	db, err := sql.Open("postgres", cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.MaxLifetime)

	// Test connection with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	wrapper := &DB{
		DB:  db,
		log: log.With().Str("component", "database").Logger(),
	}

	wrapper.log.Info().
		Str("host", cfg.Host).
		Str("database", cfg.Name).
		Int("max_open_conns", cfg.MaxOpenConns).
		Msg("Database connection established")

	return wrapper, nil
}

// RunMigrations executes all pending migrations using golang-migrate
func (db *DB) RunMigrations(migrationsPath string) error {
	db.log.Info().Str("path", migrationsPath).Msg("Running database migrations")

	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	db.log.Info().
		Uint("version", version).
		Bool("dirty", dirty).
		Msg("Migrations completed")

	return nil
}

// MigrateDown rolls back the last migration
func (db *DB) MigrateDown(migrationsPath string) error {
	db.log.Info().Str("path", migrationsPath).Msg("Rolling back last migration")

	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to rollback migration: %w", err)
	}

	db.log.Info().Msg("Migration rolled back")
	return nil
}

// MigrateToVersion migrates to a specific version
func (db *DB) MigrateToVersion(migrationsPath string, version uint) error {
	db.log.Info().Uint("version", version).Msg("Migrating to specific version")

	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Migrate(version); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to migrate to version %d: %w", version, err)
	}

	return nil
}

// HealthCheck verifies the database connection is healthy
func (db *DB) HealthCheck(ctx context.Context) error {
	return db.PingContext(ctx)
}

// Stats returns database connection pool statistics
func (db *DB) Stats() sql.DBStats {
	return db.DB.Stats()
}
