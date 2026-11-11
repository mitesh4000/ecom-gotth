package main

import (
	"log"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/mitesh4000/pushti_sangar/server/internal/auth"
	"github.com/mitesh4000/pushti_sangar/server/internal/config"
	"github.com/mitesh4000/pushti_sangar/server/internal/database"
	"github.com/mitesh4000/pushti_sangar/server/internal/models"
)

func main() {
	_ = godotenv.Load()
	_ = godotenv.Load(".env")
	cfg := config.Load()

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	mig := filepath.Join("migrations", "001_init.sql")
	_ = database.Migrate(db, mig)

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

	users := []struct {
		name, email, pass, role string
	}{
		{"Admin", "admin@pushtishangar.com", "Admin@123", "admin"},
		{"Manager", "manager@pushtishangar.com", "Manager@123", "manager"},
		{"Support", "user@pushtishangar.com", "User@123", "user"},
	}
	for _, u := range users {
		var exists int
		_ = db.QueryRow(`SELECT COUNT(1) FROM "User" WHERE email = ?`, u.email).Scan(&exists)
		if exists > 0 {
			continue
		}
		hash, _ := auth.HashPassword(u.pass)
		uid := models.NewID()
		_, err := db.Exec(`INSERT INTO "User" (id, name, email, password, photo, active, createdAt, updatedAt)
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

	customers := []struct {
		username, email, pass string
	}{
		{"Aarohi", "aarohi@example.com", "Customer@123"},
		{"Vihaan", "vihaan@example.com", "Customer@123"},
		{"Diya", "diya@example.com", "Customer@123"},
	}
	for _, c := range customers {
		var exists int
		_ = db.QueryRow(`SELECT COUNT(1) FROM "Customer" WHERE email = ?`, c.email).Scan(&exists)
		if exists > 0 {
			continue
		}
		hash, _ := auth.HashPassword(c.pass)
		cid := models.NewID()
		_, err := db.Exec(`INSERT INTO "Customer"
			(id, username, email, address, phone, password, paymentMethods, preferences, active, deleted, createdAt, updatedAt)
			VALUES (?, ?, ?, '[]', NULL, ?, '[]', '{}', 1, 0, ?, ?)`, cid, c.username, c.email, hash, now, now)
		if err != nil {
			log.Printf("customer %s: %v", c.email, err)
			continue
		}
		log.Printf("seeded customer %s", c.email)
	}

	_, _ = db.Exec(`INSERT OR IGNORE INTO "LatestInvoice" (id, lattestInvoice, updatedAt) VALUES ('1', 0, ?)`, now)
	log.Println("seed complete")
}
