// Package migrations embeds the SQL schema so the compiled server can create
// or upgrade its database without the source tree being present at runtime.
package migrations

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/mitesh4000/pushti_sangar/server/internal/database"
)

//go:embed *.sql
var files embed.FS

// Apply runs every embedded migration that has not been recorded in
// schema_migrations. The legacy 001 schema is adopted idempotently; later
// migrations are additive and must succeed as a whole.
func Apply(db *sql.DB) error {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		sqlBytes, err := files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read embedded migration %s: %w", name, err)
		}
		// 001_init.sql predates migration tracking. Existing databases already
		// contain these tables, so duplicate-object errors are expected.
		ignoreExists := name == "001_init.sql"
		if err := database.MigrateSQLVersion(db, name, string(sqlBytes), ignoreExists); err != nil {
			return err
		}
	}
	return nil
}
