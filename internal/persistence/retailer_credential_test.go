package persistence

import (
	"context"
	"testing"
)

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
	if string(cred.Payload) != string(payload) {
		t.Errorf("Payload = %s, want %s", cred.Payload, payload)
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
	if string(cred.Payload) != `"new"` {
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
