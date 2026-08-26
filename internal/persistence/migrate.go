package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/androidand/spisordning/db"
)

// goose reads migrations from the embedded db.FS "migrations" root. Goose keeps
// the base FS globally, so it is set once for the process.
func init() {
	goose.SetBaseFS(db.FS)
}

// MigrateUp applies all pending embedded Goose migrations to the database named
// by cfg. It is the only supported schema-mutation path; serve never mutates.
func MigrateUp(ctx context.Context, cfg Config) error {
	dbh, err := openSQL(ctx, cfg)
	if err != nil {
		return err
	}
	defer dbh.Close()
	if err := goose.UpContext(ctx, dbh, "migrations"); err != nil {
		return fmt.Errorf("persistence: migrate up: %w", err)
	}
	return nil
}

// MigrateStatus prints the applied/pending migration status for cfg's database.
func MigrateStatus(ctx context.Context, cfg Config) error {
	dbh, err := openSQL(ctx, cfg)
	if err != nil {
		return err
	}
	defer dbh.Close()
	if err := goose.StatusContext(ctx, dbh, "migrations"); err != nil {
		return fmt.Errorf("persistence: migrate status: %w", err)
	}
	return nil
}

// Seed applies every embedded db/seeds/*.sql file idempotently. Seeds are not
// numbered migrations and do not participate in goose_db_version.
func Seed(ctx context.Context, cfg Config) error {
	dbh, err := openSQL(ctx, cfg)
	if err != nil {
		return err
	}
	defer dbh.Close()
	files, err := fs.Glob(db.FS, "seeds/*.sql")
	if err != nil {
		return fmt.Errorf("persistence: list seeds: %w", err)
	}
	for _, name := range files {
		raw, err := fs.ReadFile(db.FS, name)
		if err != nil {
			return fmt.Errorf("persistence: read seed %s: %w", name, err)
		}
		if err := execScript(ctx, dbh, string(raw)); err != nil {
			return fmt.Errorf("persistence: apply seed %s: %w", name, err)
		}
	}
	return nil
}

// MigrationsPending reports how many embedded migrations have not been applied
// to cfg's database. A missing goose_db_version table (fresh database) means
// every migration is pending. It never mutates the schema.
func MigrationsPending(ctx context.Context, cfg Config) (int, error) {
	dbh, err := openSQL(ctx, cfg)
	if err != nil {
		return 0, err
	}
	defer dbh.Close()
	entries, err := fs.Glob(db.FS, "migrations/*.sql")
	if err != nil {
		return 0, fmt.Errorf("persistence: list migrations: %w", err)
	}
	total := len(entries)
	var applied int
	// goose_db_version carries a base row (version_id = 0) in addition to one
	// row per applied migration, so count only real migration versions.
	err = dbh.QueryRowContext(ctx, `SELECT count(*) FROM goose_db_version WHERE version_id > 0`).Scan(&applied)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "42P01" { // undefined_table
			return total, nil
		}
		return 0, fmt.Errorf("persistence: read goose_db_version: %w", err)
	}
	pending := total - applied
	if pending < 0 {
		pending = 0
	}
	return pending, nil
}

// openSQL opens a *sql.DB on the pgx driver and verifies connectivity.
func openSQL(ctx context.Context, cfg Config) (*sql.DB, error) {
	dbh, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("persistence: open db: %w", err)
	}
	if err := dbh.PingContext(ctx); err != nil {
		_ = dbh.Close()
		return nil, fmt.Errorf("persistence: ping: %w", err)
	}
	return dbh, nil
}

// execScript runs each statement of a SQL script individually (database/sql /
// pgx stdlib is single-statement). BEGIN/COMMIT wrappers are dropped.
func execScript(ctx context.Context, dbh *sql.DB, script string) error {
	for _, stmt := range splitSQL(script) {
		t := strings.TrimSpace(stmt)
		if strings.EqualFold(t, "BEGIN") || strings.EqualFold(t, "COMMIT") {
			continue
		}
		if _, err := dbh.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// splitSQL splits a SQL script into individual statements, honoring single-quoted
// strings, -- line comments, and /* */ block comments, so a semicolon inside a
// string or comment does not terminate a statement.
func splitSQL(script string) []string {
	var stmts []string
	var b strings.Builder
	inStr := false
	inLine := false
	inBlock := false
	for i := 0; i < len(script); i++ {
		c := script[i]
		switch {
		case inLine:
			if c == '\n' {
				inLine = false
			}
		case inBlock:
			if c == '*' && i+1 < len(script) && script[i+1] == '/' {
				inBlock = false
				i++
			}
		case inStr:
			b.WriteByte(c)
			if c == '\'' {
				if i+1 < len(script) && script[i+1] == '\'' {
					b.WriteByte('\'')
					i++
				} else {
					inStr = false
				}
			}
		default:
			switch {
			case c == '-' && i+1 < len(script) && script[i+1] == '-':
				inLine = true
				i++
			case c == '/' && i+1 < len(script) && script[i+1] == '*':
				inBlock = true
				i++
			case c == '\'':
				inStr = true
				b.WriteByte(c)
			case c == ';':
				stmts = append(stmts, b.String())
				b.Reset()
			default:
				b.WriteByte(c)
			}
		}
	}
	if t := strings.TrimSpace(b.String()); t != "" {
		stmts = append(stmts, b.String())
	}
	out := make([]string, 0, len(stmts))
	for _, s := range stmts {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}
