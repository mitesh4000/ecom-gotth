package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/database"
	"github.com/mitesh4000/pushti_sangar/server/internal/handlers"
	"github.com/mitesh4000/pushti_sangar/server/internal/upload"
	"github.com/mitesh4000/pushti_sangar/server/migrations"
)

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load(".env")

	// Container health probe. The runtime image is distroless and has no shell
	// or curl, so the binary probes its own /healthz endpoint.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}

	cfg := config.Load()
	logger := newLogger(cfg)
	slog.SetDefault(logger)

	if err := cfg.Validate(); err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	if err := upload.EnsureDirs(cfg.UploadsDir,
		"Products", "Category", "Banner", "Blog", "Gallery", "Color", "Users"); err != nil {
		logger.Error("cannot create upload directories", "error", err)
		os.Exit(1)
	}

	dbPath := cfg.DatabaseURL
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Clean(dbPath)
	}

	db, err := database.Open(dbPath)
	if err != nil {
		logger.Error("cannot open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := migrations.Apply(db); err != nil {
		if cfg.IsProduction() {
			logger.Error("migration failed", "error", err)
			os.Exit(1)
		}
		logger.Warn("migration failed; continuing in development", "error", err)
	}

	api := &handlers.API{DB: db, Cfg: cfg}
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		logger.Info("server started", "port", cfg.Port, "env", cfg.Env, "db", dbPath, "auth_status", cfg.AuthStatus)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("listen failed", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
	logger.Info("stopped")
}

func runHealthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "5000"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

func newLogger(cfg *config.Config) *slog.Logger {
	level := slog.LevelInfo
	if cfg.Env == "development" {
		level = slog.LevelDebug
	}
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
