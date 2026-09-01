package persistence

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestProbeUUIDEncoding(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	u := uuid.New()

	// A. basic connectivity
	var one int
	err := s.db.QueryRow(ctx, `SELECT 1`).Scan(&one)
	t.Logf("SELECT 1: got=%d err=%v", one, err)

	// A2. what db/schema/catalog does the Go conn see?
	var dbname, searchpath string
	err = s.db.QueryRow(ctx, `SELECT current_database(), current_schema()`).Scan(&dbname, &searchpath)
	t.Logf("current_database=%q current_schema=%q err=%v", dbname, searchpath, err)
	var coltype string
	err = s.db.QueryRow(ctx, `SELECT format_type(a.atttypid, a.atttypmod) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relname='shopping_list' AND a.attname='id' AND n.nspname=current_schema() AND NOT a.attisdropped`).Scan(&coltype)
	t.Logf("catalog shopping_list.id type=%q err=%v", coltype, err)

	// B. text param
	var txt string
	err = s.db.QueryRow(ctx, `SELECT $1::text`, "hello").Scan(&txt)
	t.Logf("SELECT $1::text (string): got=%q err=%v", txt, err)

	// C. uuid cast, string value
	var uu uuid.UUID
	err = s.db.QueryRow(ctx, `SELECT $1::uuid`, u.String()).Scan(&uu)
	t.Logf("SELECT $1::uuid (string): got=%q err=%v", uu, err)

	// D. uuid cast, uuid value
	err = s.db.QueryRow(ctx, `SELECT $1::uuid`, u).Scan(&uu)
	t.Logf("SELECT $1::uuid (uuid.UUID): got=%q err=%v", uu, err)

	// E. no cast, let server infer from target (into uuid var)
	err = s.db.QueryRow(ctx, `SELECT $1`, u.String()).Scan(&uu)
	t.Logf("SELECT $1 (string, scan uuid): got=%q err=%v", uu, err)

	// F. bigint param, int value
	var bi int64
	err = s.db.QueryRow(ctx, `SELECT $1::bigint`, int64(42)).Scan(&bi)
	t.Logf("SELECT $1::bigint (int64): got=%d err=%v", bi, err)

	// G. insert with explicit ::uuid cast
	_, err = s.db.Exec(ctx, `INSERT INTO shopping_list (id, name, status) VALUES ($1::uuid, 'probe', 'active')`, u.String())
	t.Logf("INSERT id=$1::uuid (string): err=%v", err)
	_, _ = s.db.Exec(ctx, `DELETE FROM shopping_list WHERE id = $1`, u.String())

	// H. insert with uuid.UUID value and explicit cast
	_, err = s.db.Exec(ctx, `INSERT INTO shopping_list (id, name, status) VALUES ($1::uuid, 'probe', 'active')`, u)
	t.Logf("INSERT id=$1::uuid (uuid.UUID): err=%v", err)
	_, _ = s.db.Exec(ctx, `DELETE FROM shopping_list WHERE id = $1`, u.String())
}
