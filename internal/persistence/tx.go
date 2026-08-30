package persistence

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// ErrNoRows is the sentinel error used when a query returns no rows.
// It is re-exported from pgx so that callers in the service layer can
// use errors.Is(err, persistence.ErrNoRows) without importing pgx.
var ErrNoRows = pgx.ErrNoRows

// Row is the minimal interface that QueryRow returns. It is satisfied by
// pgx.Row (which has Scan(dest ...interface{}) error).
type Row interface {
	Scan(dest ...interface{}) error
}

// Tx is the minimal transaction interface that the service layer needs.
// It is satisfied by pgx.Tx except for the QueryRow return type: pgx.Tx
// returns pgx.Row while this interface declares Row. Since pgx.Row
// satisfies Row, the wrapper (txAdapter) bridges the gap.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (interface{}, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) Row
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

// txAdapter wraps pgx.Tx so that QueryRow returns our Row interface
// instead of pgx.Row. This lets the service layer depend on Tx without
// importing pgx.
type txAdapter struct{ inner pgx.Tx }

func (t txAdapter) Exec(ctx context.Context, sql string, args ...interface{}) (interface{}, error) {
	res, err := t.inner.Exec(ctx, sql, args...)
	return res, err
}

func (t txAdapter) QueryRow(ctx context.Context, sql string, args ...interface{}) Row {
	return t.inner.QueryRow(ctx, sql, args...)
}

func (t txAdapter) Commit(ctx context.Context) error {
	return t.inner.Commit(ctx)
}

func (t txAdapter) Rollback(ctx context.Context) error {
	return t.inner.Rollback(ctx)
}
