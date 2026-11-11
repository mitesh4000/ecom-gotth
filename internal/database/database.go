package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	// modernc.org/sqlite DSN
	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn += "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite writer safety
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}

	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func Migrate(db *sql.DB, migrationPath string) error {
	sqlBytes, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	return MigrateSQL(db, string(sqlBytes))
}

// MigrateSQL applies a SQL migration document. It is used by the embedded
// migration package so a compiled binary does not need the source tree.
func MigrateSQL(db *sql.DB, migrationSQL string) error {
	// Split on statement boundaries carefully enough for this schema
	stmts := splitSQL(migrationSQL)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			// Ignore "already exists" so migrate is idempotent on restart
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			return fmt.Errorf("migrate stmt failed: %w\n---\n%s", err, truncate(stmt, 200))
		}
	}
	return tx.Commit()
}

func splitSQL(s string) []string {
	var out []string
	var b strings.Builder
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if strings.HasSuffix(trimmed, ";") {
			out = append(out, b.String())
			b.Reset()
		}
	}
	if rest := strings.TrimSpace(b.String()); rest != "" {
		out = append(out, rest)
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
