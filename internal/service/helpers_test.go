package service_test

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/mitesh4000/pushti_sangar/server/internal/database"
	"github.com/mitesh4000/pushti_sangar/server/internal/models"
	"github.com/mitesh4000/pushti_sangar/server/migrations"
)

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := migrations.Apply(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func insertCustomer(t *testing.T, db *sql.DB, id, email string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.Exec(`INSERT INTO "Customer"
		(id, username, email, address, password, paymentMethods, preferences, active, deleted, createdAt, updatedAt)
		VALUES (?, ?, ?, '[]', 'x', '[]', '{}', 1, 0, ?, ?)`, id, id, email, now, now)
	if err != nil {
		t.Fatalf("insert customer: %v", err)
	}
}

func insertProduct(t *testing.T, db *sql.DB, id string, price float64, stock *int) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := db.Exec(`INSERT INTO "Product"
		(id, name, pricesCalculatedPrice, stockQuantity, isActive, createdAt, updatedAt)
		VALUES (?, ?, ?, ?, 1, ?, ?)`, id, id, price, stock, now, now)
	if err != nil {
		t.Fatalf("insert product: %v", err)
	}
}

func insertStockRow(t *testing.T, db *sql.DB, productID string, quantity int) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO "Stock" (id, productId, quantity, currentPricePerUnit, date)
		VALUES (?, ?, ?, 0, ?)`, models.NewID(), productID, quantity, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		t.Fatalf("insert stock: %v", err)
	}
}

func productStock(t *testing.T, db *sql.DB, productID string) int {
	t.Helper()
	var qty sql.NullInt64
	if err := db.QueryRow(`SELECT stockQuantity FROM "Product" WHERE id = ?`, productID).Scan(&qty); err != nil {
		t.Fatalf("read product stock: %v", err)
	}
	return int(qty.Int64)
}

func stockTableTotal(t *testing.T, db *sql.DB, productID string) int {
	t.Helper()
	var qty sql.NullInt64
	if err := db.QueryRow(`SELECT SUM(quantity) FROM "Stock" WHERE productId = ?`, productID).Scan(&qty); err != nil {
		t.Fatalf("read stock table: %v", err)
	}
	return int(qty.Int64)
}

func countRows(t *testing.T, db *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(1) FROM "` + table + `"`).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}
