package retailer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/androidand/spisordning/internal/httpclient"
)

// TestTierFor_ICA verifies ICA's tier declaration: the anonymous ecom surface
// is basic (never stale) and the account-bound writes are elevated (can stale).
func TestTierFor_ICA(t *testing.T) {
	c := NewICA("http://localhost:8403")

	basic := []Operation{OpResolve, OpSearch, OpBarcode, OpOffers}
	for _, op := range basic {
		if got := c.TierFor(op); got != AuthBasic {
			t.Errorf("ICA %s = %s, want %s", op, got, AuthBasic)
		}
	}

	elevated := []Operation{OpCreateList, OpSyncList, OpToCart, OpBonus}
	for _, op := range elevated {
		if got := c.TierFor(op); got != AuthElevated {
			t.Errorf("ICA %s = %s, want %s", op, got, AuthElevated)
		}
	}
}

// TestTierFor_SingleTierRetailers verifies Willys and Hemköp are single-tier:
// every operation is basic, so the tier concept is a no-op for them.
func TestTierFor_SingleTierRetailers(t *testing.T) {
	cases := []struct {
		name string
		c    *Client
	}{
		{"willys", New("http://localhost:8402")},
		{"hemkop", NewHemkop("http://localhost:8404")},
	}
	all := []Operation{OpResolve, OpSearch, OpCreateList, OpSyncList, OpToCart, OpBarcode, OpBonus, OpOffers}
	for _, tc := range cases {
		for _, op := range all {
			if got := tc.c.TierFor(op); got != AuthBasic {
				t.Errorf("%s %s = %s, want %s (single-tier)", tc.name, op, got, AuthBasic)
			}
		}
	}
}

// TestIsStaleCredential verifies the canonical stale signal is keyed off the
// HTTP status code (401/403), not any other status.
func TestIsStaleCredential(t *testing.T) {
	stale := []int{http.StatusUnauthorized, http.StatusForbidden}
	for _, code := range stale {
		if !IsStaleCredential(code) {
			t.Errorf("IsStaleCredential(%d) = false, want true", code)
		}
	}
	notStale := []int{http.StatusOK, http.StatusCreated, http.StatusNotFound, http.StatusBadGateway, http.StatusInternalServerError}
	for _, code := range notStale {
		if IsStaleCredential(code) {
			t.Errorf("IsStaleCredential(%d) = true, want false", code)
		}
	}
}

// TestIsElevatedStale_StatusError verifies detection works off the status code
// carried by an httpclient.StatusError — the canonical 401 signals stale, a
// 502 (the catchable parse-error path) does not.
func TestIsElevatedStale_StatusError(t *testing.T) {
	if !IsElevatedStale(&httpclient.StatusError{Backend: "ica-adapter", StatusCode: http.StatusUnauthorized, Path: "/shopping-lists"}) {
		t.Error("401 StatusError should be detected as elevated-stale")
	}
	if !IsElevatedStale(&httpclient.StatusError{Backend: "ica-adapter", StatusCode: http.StatusForbidden, Path: "/shopping-lists"}) {
		t.Error("403 StatusError should be detected as elevated-stale")
	}
	if IsElevatedStale(&httpclient.StatusError{Backend: "ica-adapter", StatusCode: http.StatusBadGateway, Path: "/shopping-lists"}) {
		t.Error("502 StatusError should NOT be detected as elevated-stale")
	}
	if IsElevatedStale(nil) {
		t.Error("nil error should not be detected as elevated-stale")
	}
}

// TestIsElevatedStale_Sentinel verifies a caller-wrapped ErrElevatedStale is
// detected, and that an unrelated wrapped error is not.
func TestIsElevatedStale_Sentinel(t *testing.T) {
	wrapped := fmt.Errorf("push failed: %w", ErrElevatedStale)
	if !IsElevatedStale(wrapped) {
		t.Error("wrapped ErrElevatedStale should be detected")
	}
	unrelated := fmt.Errorf("push failed: %w", errors.New("connection reset"))
	if IsElevatedStale(unrelated) {
		t.Error("unrelated wrapped error should NOT be detected as elevated-stale")
	}
}

// TestIsElevatedStale_EndToEnd verifies the full path: an elevated-tier call
// (wishlist push) against an adapter that returns 401 surfaces as a detected
// stale-credential error, while a 502 does not.
func TestIsElevatedStale_EndToEnd(t *testing.T) {
	mk := func(status int) *Client {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		t.Cleanup(srv.Close)
		return NewICA(srv.URL)
	}

	staleErr, err := mk(http.StatusUnauthorized).CreateShoppingList(context.Background(), "Vecka 30", nil)
	if err == nil {
		t.Fatal("expected an error from 401 response")
	}
	if !IsElevatedStale(err) {
		t.Errorf("401 from CreateShoppingList should be detected as elevated-stale, got: %v", err)
	}
	_ = staleErr

	notStaleErr, err := mk(http.StatusBadGateway).CreateShoppingList(context.Background(), "Vecka 30", nil)
	if err == nil {
		t.Fatal("expected an error from 502 response")
	}
	if IsElevatedStale(err) {
		t.Errorf("502 from CreateShoppingList should NOT be detected as elevated-stale, got: %v", err)
	}
	_ = notStaleErr
}

// TestWithAuthFile verifies the elevated-credential file path is wired onto the
// client by the composition root (from config) and read back, rather than the
// client reading the environment itself.
func TestWithAuthFile(t *testing.T) {
	const path = "/home/andreas/.config/spisordning/ica-auth.json"
	c := NewICA("http://localhost:8403")
	if got := c.AuthFile(); got != "" {
		t.Fatalf("new ICA client AuthFile = %q, want empty", got)
	}
	c.WithAuthFile(path)
	if got := c.AuthFile(); got != path {
		t.Errorf("AuthFile after WithAuthFile = %q, want %q", got, path)
	}
}

// TestBasicTierProceedsWithoutCredential verifies a basic-tier call (resolve)
// succeeds with no elevated credential present — the anonymous ecom surface
// never needs (and never goes stale on) the elevated session.
func TestBasicTierProceedsWithoutCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"resolutions":[]}`))
	}))
	t.Cleanup(srv.Close)

	c := NewICA(srv.URL) // no credential wired
	if c.AuthFile() != "" {
		t.Fatalf("AuthFile = %q, want empty (no credential present)", c.AuthFile())
	}
	if _, err := c.ResolveRequirements(context.Background(), nil, nil); err != nil {
		t.Errorf("basic-tier ResolveRequirements without credential = %v, want nil", err)
	}
}
