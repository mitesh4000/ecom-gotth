package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// DevelopmentJWTSecret is intentionally weak and must never be used in
	// production. It exists only so a fresh development checkout starts.
	DevelopmentJWTSecret = "change-me-development-only"

	minProductionSecretLength = 32
)

type Config struct {
	Port        string
	Env         string
	DatabaseURL string
	JWTSecret   string
	FrontendURL string
	AdminURL    string
	CORSOrigins []string
	EmailHost   string
	EmailPort   int
	EmailUser   string
	EmailPass   string
	UploadsDir  string
	TokenTTL    time.Duration

	// MaxBodyBytes bounds request bodies. Uploads use the same limit.
	MaxBodyBytes int64
	// CookieSecure controls the Secure flag on auth cookies. It defaults to
	// true in production and false in development.
	CookieSecure bool
	// TrustProxy enables X-Forwarded-For/X-Real-IP handling. Leave disabled
	// unless the server is behind a trusted reverse proxy.
	TrustProxy bool
	// PaymentProvider is the configured payment gateway identifier. When it is
	// empty the payment endpoint fails closed instead of pretending to succeed.
	PaymentProvider string
	// PaymentSecret is the shared secret used to verify provider callbacks.
	// Both provider and secret must be set for payments to be accepted.
	PaymentSecret string
	// AuthStatus selects the HTTP status used for authentication failures.
	// "legacy" preserves the original 200-with-success:false contract; "strict"
	// returns 401/403. Production defaults to strict.
	AuthStatus string
	// CSRFMode controls protection for cookie-authenticated unsafe requests.
	// "off" disables the check, "same-origin" rejects cross-origin browser
	// requests and requires a double-submit token for allowed cross-origin
	// requests, and "strict" requires the token for every unsafe cookie-auth
	// request. The default is "same-origin".
	CSRFMode string
}

func Load() *Config {
	emailPort, _ := strconv.Atoi(getEnv("EMAIL_PORT", "587"))
	origins := splitList(getEnv("CORS_ORIGINS", ""))

	dbURL := getEnv("DATABASE_URL", "file:./data/dev.db")
	// Normalize prisma-style SQLite URL for database/sql
	dbURL = strings.TrimPrefix(dbURL, "file:")

	env := getEnv("NODE_ENV", getEnv("ENV", "development"))
	cfg := &Config{
		Port:            getEnv("PORT", "5000"),
		Env:             env,
		DatabaseURL:     dbURL,
		JWTSecret:       getEnv("JWT_SECRET", DevelopmentJWTSecret),
		FrontendURL:     getEnv("FRONTEND_URL", "http://localhost:3000"),
		AdminURL:        getEnv("ADMIN_URL", "http://localhost:3001"),
		CORSOrigins:     origins,
		EmailHost:       getEnv("EMAIL_HOST", ""),
		EmailPort:       emailPort,
		EmailUser:       getEnv("EMAIL_USER", ""),
		EmailPass:       getEnv("EMAIL_PASS", ""),
		UploadsDir:      getEnv("UPLOADS_DIR", "./uploads"),
		TokenTTL:        24 * time.Hour,
		MaxBodyBytes:    32 << 20,
		TrustProxy:      boolEnv("TRUST_PROXY", false),
		PaymentProvider: getEnv("PAYMENT_PROVIDER", ""),
		PaymentSecret:   getEnv("PAYMENT_SECRET", ""),
		AuthStatus:      strings.ToLower(getEnv("AUTH_STATUS", defaultAuthStatus(env))),
		CSRFMode:        strings.ToLower(getEnv("CSRF_MODE", "same-origin")),
	}
	cfg.CookieSecure = boolEnv("COOKIE_SECURE", cfg.IsProduction())

	if len(cfg.CORSOrigins) == 0 {
		for _, origin := range []string{cfg.FrontendURL, cfg.AdminURL} {
			origin = strings.TrimRight(strings.TrimSpace(origin), "/")
			if origin != "" {
				cfg.CORSOrigins = append(cfg.CORSOrigins, origin)
			}
		}
	}
	return cfg
}

// IsProduction reports whether the process is configured for production.
func (c *Config) IsProduction() bool {
	env := strings.ToLower(strings.TrimSpace(c.Env))
	return env == "production" || env == "prod"
}

// Validate fails closed when production settings are missing or insecure.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.Port) == "" {
		return errors.New("PORT must not be empty")
	}
	if c.TokenTTL <= 0 || c.TokenTTL > 30*24*time.Hour {
		return errors.New("token TTL must be between 1 second and 30 days")
	}
	if c.MaxBodyBytes <= 0 {
		return errors.New("max body size must be positive")
	}
	if c.AuthStatus != "legacy" && c.AuthStatus != "strict" {
		return fmt.Errorf("AUTH_STATUS must be legacy or strict, got %q", c.AuthStatus)
	}
	switch c.CSRFMode {
	case "", "off", "same-origin", "strict":
	default:
		return fmt.Errorf("CSRF_MODE must be off, same-origin, or strict, got %q", c.CSRFMode)
	}
	if (strings.TrimSpace(c.PaymentProvider) == "") != (strings.TrimSpace(c.PaymentSecret) == "") {
		return errors.New("PAYMENT_PROVIDER and PAYMENT_SECRET must be set together")
	}
	if c.PaymentSecret != "" && len(c.PaymentSecret) < 16 {
		return errors.New("PAYMENT_SECRET must be at least 16 characters")
	}
	if c.IsProduction() {
		if len(c.JWTSecret) < minProductionSecretLength {
			return fmt.Errorf("JWT_SECRET must be at least %d characters in production", minProductionSecretLength)
		}
		if c.JWTSecret == DevelopmentJWTSecret {
			return errors.New("JWT_SECRET must not use the development default in production")
		}
		if len(c.CORSOrigins) == 0 {
			return errors.New("CORS_ORIGINS must be set explicitly in production")
		}
		for _, origin := range c.CORSOrigins {
			if origin == "*" {
				return errors.New("CORS_ORIGINS must not contain * in production")
			}
			if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "http://127.0.0.1") {
				return fmt.Errorf("CORS origin %q is a development origin and is not allowed in production", origin)
			}
		}
		if !c.CookieSecure {
			return errors.New("COOKIE_SECURE must be true in production")
		}
	}
	return nil
}

// IsStrictAuth reports whether auth failures should use 401/403.
func (c *Config) IsStrictAuth() bool {
	return c.AuthStatus == "strict"
}

func defaultAuthStatus(env string) string {
	if strings.EqualFold(env, "production") || strings.EqualFold(env, "prod") {
		return "strict"
	}
	return "legacy"
}

func splitList(value string) []string {
	var out []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(strings.TrimRight(item, "/"))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func boolEnv(key string, fallback bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
