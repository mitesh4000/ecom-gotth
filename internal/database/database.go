package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// _txlock=immediate makes every transaction take the write lock at BEGIN, so
// two checkout transactions can never both read the same stock and then race
// to write it. Combined with busy_timeout, concurrent writers queue instead of
// failing or overselling.
const sqlitePragmas = "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)&_txlock=immediate"

func Open(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}

	// modernc.org/sqlite DSN. Pragmas are applied per connection, which is
	// important because SQLite settings such as foreign_keys are connection
	// scoped.
	dsn := dbPath
	if strings.Contains(dsn, "?") {
		dsn += "&" + sqlitePragmas
	} else {
		dsn += "?" + sqlitePragmas
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// WAL allows readers and the single writer to coexist. Write transactions
	// are serialised by _txlock=immediate rather than by forcing the whole
	// process onto one connection: a single connection would deadlock any
	// handler that queries the database while iterating an open result set.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(0)

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
	return migrateSQL(db, migrationSQL, true)
}

// MigrateSQLVersion applies one migration once and records its version. The
// legacy 001 schema is allowed to report "already exists" so existing
// databases can be adopted without being recreated.
func MigrateSQLVersion(db *sql.DB, version, migrationSQL string, ignoreExists bool) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS "schema_migrations" (
		"version" TEXT NOT NULL PRIMARY KEY,
		"appliedAt" DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	var applied int
	if err := db.QueryRow(`SELECT COUNT(1) FROM "schema_migrations" WHERE version = ?`, version).Scan(&applied); err != nil {
		return fmt.Errorf("check migration %s: %w", version, err)
	}
	if applied > 0 {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range splitSQL(migrationSQL) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			if ignoreExists && strings.Contains(strings.ToLower(err.Error()), "already exists") {
				continue
			}
			return fmt.Errorf("migrate %s failed: %w\n---\n%s", version, err, truncate(stmt, 200))
		}
	}
	if _, err := tx.Exec(`INSERT INTO "schema_migrations" (version) VALUES (?)`, version); err != nil {
		return fmt.Errorf("record migration %s: %w", version, err)
	}
	return tx.Commit()
}

func migrateSQL(db *sql.DB, migrationSQL string, ignoreExists bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, stmt := range splitSQL(migrationSQL) {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			if ignoreExists && strings.Contains(strings.ToLower(err.Error()), "already exists") {
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
		if strings.HasPrefix(trimmed, "--") || trimmed == "" {
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
