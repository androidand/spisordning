package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// RetailerCredential is a retailer's manually-refreshed elevated-auth
// credential — see db/migrations/000015_retailer_credential.sql. Payload is
// opaque to spisordning (e.g. ICA's ImportedCookie[] JSON) — it's stored and
// served back verbatim, never inspected.
type RetailerCredential struct {
	Retailer   string
	Tier       string // always "elevated" today; see the migration's CHECK.
	Payload    []byte // raw JSON, as uploaded.
	UploadedAt time.Time
}

// UpsertRetailerCredential stores retailer's current elevated credential,
// overwriting whatever was there before — there is exactly one "current"
// elevated session per retailer, not a history of them.
func (s *Store) UpsertRetailerCredential(ctx context.Context, retailer string, payload []byte) error {
	const q = `INSERT INTO retailer_credential (retailer, tier, payload, uploaded_at)
		VALUES ($1, 'elevated', $2, now())
		ON CONFLICT (retailer, tier) DO UPDATE SET payload = EXCLUDED.payload, uploaded_at = now()`
	if _, err := s.db.Exec(ctx, q, retailer, payload); err != nil {
		return fmt.Errorf("persistence: upsert retailer_credential: %w", err)
	}
	return nil
}

// GetRetailerCredential returns retailer's current elevated credential, or
// found=false when none has been uploaded yet.
func (s *Store) GetRetailerCredential(ctx context.Context, retailer string) (cred RetailerCredential, found bool, err error) {
	const q = `SELECT retailer, tier, payload, uploaded_at FROM retailer_credential
		WHERE retailer = $1 AND tier = 'elevated'`
	err = s.db.QueryRow(ctx, q, retailer).Scan(&cred.Retailer, &cred.Tier, &cred.Payload, &cred.UploadedAt)
	if err == pgx.ErrNoRows {
		return RetailerCredential{}, false, nil
	}
	if err != nil {
		return RetailerCredential{}, false, fmt.Errorf("persistence: get retailer_credential: %w", err)
	}
	return cred, true, nil
}
