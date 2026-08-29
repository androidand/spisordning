package persistence

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

// jsonEqual compares two JSON byte strings semantically rather than
// byte-for-byte — Postgres's JSONB column re-serializes with its own
// canonical whitespace (e.g. spaces after ":"/","), so an exact string
// comparison against what was uploaded is the wrong check even when the data
// round-trips correctly.
func jsonEqual(t *testing.T, a, b []byte) bool {
	t.Helper()
	var av, bv any
	if err := json.Unmarshal(a, &av); err != nil {
		t.Fatalf("jsonEqual: unmarshal a: %v (a=%s)", err, a)
	}
	if err := json.Unmarshal(b, &bv); err != nil {
		t.Fatalf("jsonEqual: unmarshal b: %v (b=%s)", err, b)
	}
	return reflect.DeepEqual(av, bv)
}

func TestRetailerCredential_UploadThenGet(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "retailer_credential")

	if _, found, err := s.GetRetailerCredential(ctx, "ica"); err != nil {
		t.Fatalf("GetRetailerCredential (before upload): %v", err)
	} else if found {
		t.Fatal("expected found=false before any upload")
	}

	payload := []byte(`[{"name":"sid","value":"abc","domain":"ica.se"}]`)
	if err := s.UpsertRetailerCredential(ctx, "ica", payload); err != nil {
		t.Fatalf("UpsertRetailerCredential: %v", err)
	}

	cred, found, err := s.GetRetailerCredential(ctx, "ica")
	if err != nil {
		t.Fatalf("GetRetailerCredential (after upload): %v", err)
	}
	if !found {
		t.Fatal("expected found=true after upload")
	}
	if cred.Retailer != "ica" {
		t.Errorf("Retailer = %q, want ica", cred.Retailer)
	}
	if cred.Tier != "elevated" {
		t.Errorf("Tier = %q, want elevated", cred.Tier)
	}
	if !jsonEqual(t, cred.Payload, payload) {
		t.Errorf("Payload = %s, want (semantically) %s", cred.Payload, payload)
	}
	if cred.UploadedAt.IsZero() {
		t.Error("UploadedAt should be set")
	}
}

func TestRetailerCredential_UploadOverwritesPrevious(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "retailer_credential")

	if err := s.UpsertRetailerCredential(ctx, "ica", []byte(`"old"`)); err != nil {
		t.Fatalf("UpsertRetailerCredential (1st): %v", err)
	}
	if err := s.UpsertRetailerCredential(ctx, "ica", []byte(`"new"`)); err != nil {
		t.Fatalf("UpsertRetailerCredential (2nd): %v", err)
	}

	cred, found, err := s.GetRetailerCredential(ctx, "ica")
	if err != nil || !found {
		t.Fatalf("GetRetailerCredential: found=%v err=%v", found, err)
	}
	if !jsonEqual(t, cred.Payload, []byte(`"new"`)) {
		t.Errorf("Payload = %s, want the overwritten value %q", cred.Payload, `"new"`)
	}
}

func TestRetailerCredential_DifferentRetailersAreIndependent(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "retailer_credential")

	if err := s.UpsertRetailerCredential(ctx, "ica", []byte(`"ica-cred"`)); err != nil {
		t.Fatalf("UpsertRetailerCredential (ica): %v", err)
	}

	if _, found, err := s.GetRetailerCredential(ctx, "willys"); err != nil {
		t.Fatalf("GetRetailerCredential (willys): %v", err)
	} else if found {
		t.Error("willys should have no credential — uploading for ica must not affect it")
	}
}
