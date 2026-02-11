package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/bulk-import-export-api/internal/api"
	"github.com/bulk-import-export-api/internal/config"
	"github.com/bulk-import-export-api/internal/database"
	"github.com/bulk-import-export-api/internal/repository"
	"github.com/bulk-import-export-api/internal/service"
	"github.com/bulk-import-export-api/pkg/logger"
)

func main() {
	// Initialize logger
	log := logger.New()
	log.Info().Msg("Starting Bulk Import/Export API server...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Initialize database
	db, err := database.New(&cfg.Database, log)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	// Run migrations
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "./migrations"
	}
	if err := db.RunMigrations(migrationsPath); err != nil {
		log.Fatal().Err(err).Msg("Failed to run database migrations")
	}

	// Initialize repositories
	repos := repository.New(db)

	// Initialize services
	services := service.NewServices(repos, cfg, log)

	// Start background job processor
	go services.Job.StartProcessor(context.Background())
	log.Info().Msg("Background job processor started")

	// Initialize router
	router := api.NewRouter(services, cfg, log)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.ReadTimeout,
	}

	// Start server in goroutine
	go func() {
		log.Info().Str("port", cfg.Server.Port).Msg("Server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	// Stop job processor
	services.Job.StopProcessor()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	log.Info().Msg("Server exited gracefully")
}
