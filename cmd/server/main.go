package main

import (
	"context"
	"log"
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
)

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load(".env")

	cfg := config.Load()

	if err := upload.EnsureDirs(cfg.UploadsDir,
		"Products", "Category", "Banner", "Blog", "Gallery", "Color", "Users"); err != nil {
		log.Fatalf("uploads dirs: %v", err)
	}

	dbPath := cfg.DatabaseURL
	if !filepath.IsAbs(dbPath) {
		dbPath = filepath.Clean(dbPath)
	}

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	mig := filepath.Join("migrations", "001_init.sql")
	if _, err := os.Stat(mig); err == nil {
		if err := database.Migrate(db, mig); err != nil {
			log.Printf("migration warning: %v", err)
		}
	}

	api := &handlers.API{DB: db, Cfg: cfg}
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("Server Started on %s (env=%s, db=%s)", cfg.Port, cfg.Env, dbPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Println("bye")
}
