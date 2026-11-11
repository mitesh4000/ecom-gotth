// Package migrations embeds the SQL schema so the compiled server can create
// or upgrade its database without the source tree being present at runtime.
package migrations

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/mitesh4000/pushti_sangar/server/internal/database"
)

//go:embed *.sql
var files embed.FS

// Apply runs the embedded schema migration against db.
func Apply(db *sql.DB) error {
	sqlBytes, err := files.ReadFile("001_init.sql")
	if err != nil {
		return fmt.Errorf("read embedded migration: %w", err)
	}
	return database.MigrateSQL(db, string(sqlBytes))
}
