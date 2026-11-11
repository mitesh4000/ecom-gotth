package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/database"
	"github.com/mitesh4000/pushti_sangar/server/internal/models"
	"github.com/mitesh4000/pushti_sangar/server/migrations"
)

// Development-only credentials. Production refuses to run without explicit
// SEED_*_PASSWORD environment variables.
const (
	devAdminPassword    = "Admin@12345"
	devManagerPassword  = "Manager@12345"
	devUserPassword     = "User@12345"
	devCustomerPassword = "Customer@12345"
)

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load(".env")
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := migrations.Apply(db); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	// Roles
	roles := []string{"admin", "manager", "user"}
	roleIDs := map[string]string{}
	for _, r := range roles {
		id := models.NewID()
		_, err := db.Exec(`INSERT OR IGNORE INTO "Role" (id, role, permissions) VALUES (?, ?, '[]')`, id, r)
		if err != nil {
			log.Printf("role %s: %v", r, err)
		}
		var existing string
		if db.QueryRow(`SELECT id FROM "Role" WHERE role = ?`, r).Scan(&existing) == nil {
			roleIDs[r] = existing
		} else {
			roleIDs[r] = id
		}
	}

	adminPassword := seedPassword(cfg, "SEED_ADMIN_PASSWORD", devAdminPassword)
	managerPassword := seedPassword(cfg, "SEED_MANAGER_PASSWORD", devManagerPassword)
	userPassword := seedPassword(cfg, "SEED_USER_PASSWORD", devUserPassword)

	users := []struct {
		name, email, pass, role string
	}{
		{"Admin", "admin@pushtishangar.com", adminPassword, "admin"},
		{"Manager", "manager@pushtishangar.com", managerPassword, "manager"},
		{"Support", "user@pushtishangar.com", userPassword, "user"},
	}
	for _, u := range users {
		var exists int
		_ = db.QueryRow(`SELECT COUNT(1) FROM "User" WHERE email = ?`, u.email).Scan(&exists)
		if exists > 0 {
			continue
		}
		hash, err := auth.HashPassword(u.pass)
		if err != nil {
			log.Printf("hash user %s: %v", u.email, err)
			continue
		}
		uid := models.NewID()
		_, err = db.Exec(`INSERT INTO "User" (id, name, email, password, photo, active, createdAt, updatedAt)
			VALUES (?, ?, ?, ?, '', 1, ?, ?)`, uid, u.name, u.email, hash, now, now)
		if err != nil {
			log.Printf("user %s: %v", u.email, err)
			continue
		}
		if rid, ok := roleIDs[u.role]; ok {
			_, _ = db.Exec(`INSERT INTO "UserRole" (id, userId, roleId) VALUES (?, ?, ?)`, models.NewID(), uid, rid)
		}
		log.Printf("seeded user %s", u.email)
	}

	// Demo customers are useful for local development but must never be
	// created automatically in production.
	if !cfg.IsProduction() {
		customerPassword := seedPassword(cfg, "SEED_CUSTOMER_PASSWORD", devCustomerPassword)
		customers := []struct {
			username, email, pass string
		}{
			{"Aarohi", "aarohi@example.com", customerPassword},
			{"Vihaan", "vihaan@example.com", customerPassword},
			{"Diya", "diya@example.com", customerPassword},
		}
		for _, c := range customers {
			var exists int
			_ = db.QueryRow(`SELECT COUNT(1) FROM "Customer" WHERE email = ?`, c.email).Scan(&exists)
			if exists > 0 {
				continue
			}
			hash, err := auth.HashPassword(c.pass)
			if err != nil {
				log.Printf("hash customer %s: %v", c.email, err)
				continue
			}
			cid := models.NewID()
			_, err = db.Exec(`INSERT INTO "Customer"
				(id, username, email, address, phone, password, paymentMethods, preferences, active, deleted, createdAt, updatedAt)
				VALUES (?, ?, ?, '[]', NULL, ?, '[]', '{}', 1, 0, ?, ?)`, cid, c.username, c.email, hash, now, now)
			if err != nil {
				log.Printf("customer %s: %v", c.email, err)
				continue
			}
			log.Printf("seeded customer %s", c.email)
		}
	}

	_, _ = db.Exec(`INSERT OR IGNORE INTO "LatestInvoice" (id, lattestInvoice, updatedAt) VALUES ('1', 0, ?)`, now)
	log.Println("seed complete")
}

// seedPassword returns the environment-provided password. In production an
// explicit value is mandatory; development falls back to a documented demo
// password.
func seedPassword(cfg *config.Config, key, devDefault string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value != "" {
		return value
	}
	if cfg.IsProduction() {
		log.Fatalf("%s must be set in production", key)
	}
	return devDefault
}
