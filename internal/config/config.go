package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port         string
	Env          string
	DatabaseURL  string
	JWTSecret    string
	FrontendURL  string
	AdminURL     string
	CORSOrigins  []string
	EmailHost    string
	EmailPort    int
	EmailUser    string
	EmailPass    string
	UploadsDir   string
	TokenTTL     time.Duration
}

func Load() *Config {
	emailPort, _ := strconv.Atoi(getEnv("EMAIL_PORT", "587"))
	origins := strings.Split(getEnv("CORS_ORIGINS", ""), ",")
	var clean []string
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o != "" {
			clean = append(clean, o)
		}
	}

	dbURL := getEnv("DATABASE_URL", "file:./data/dev.db")
	// Normalize prisma-style SQLite URL for database/sql
	dbURL = strings.TrimPrefix(dbURL, "file:")

	return &Config{
		Port:        getEnv("PORT", "5000"),
		Env:         getEnv("NODE_ENV", getEnv("ENV", "development")),
		DatabaseURL: dbURL,
		JWTSecret:   getEnv("JWT_SECRET", "change-me-development-only"),
		FrontendURL: getEnv("FRONTEND_URL", "http://localhost:3000"),
		AdminURL:    getEnv("ADMIN_URL", "http://localhost:3001"),
		CORSOrigins: clean,
		EmailHost:   getEnv("EMAIL_HOST", ""),
		EmailPort:   emailPort,
		EmailUser:   getEnv("EMAIL_USER", ""),
		EmailPass:   getEnv("EMAIL_PASS", ""),
		UploadsDir:  getEnv("UPLOADS_DIR", "./uploads"),
		TokenTTL:    24 * time.Hour,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
