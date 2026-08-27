package retailer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateShoppingList_401IsElevatedAuthStale(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_token"})
	}))
	defer srv.Close()

	c := NewICA(srv.URL)
	_, err := c.CreateShoppingList(context.Background(), "Vecka 1", nil)
	if err == nil {
		t.Fatal("expected an error for a 401 response")
	}
	if !errors.Is(err, ErrElevatedAuthStale) {
		t.Errorf("expected errors.Is(err, ErrElevatedAuthStale) to be true, got: %v", err)
	}
}

func TestCreateShoppingList_403IsElevatedAuthStale(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := NewICA(srv.URL)
	_, err := c.CreateShoppingList(context.Background(), "Vecka 1", nil)
	if !errors.Is(err, ErrElevatedAuthStale) {
		t.Errorf("expected errors.Is(err, ErrElevatedAuthStale) to be true for 403, got: %v", err)
	}
}

func TestCreateShoppingList_502IsNotElevatedAuthStale(t *testing.T) {
	// The "catchable" ICA failure shape is an opaque 502 wrapping a JSON-parse
	// error (per expose-shopping-price-and-notes-bridge's task 1.2 research) —
	// distinct from the 401/403 this change can reliably detect. It should
	// surface as a normal error, not be mistaken for a stale-auth signal.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unexpected token < in JSON"})
	}))
	defer srv.Close()

	c := NewICA(srv.URL)
	_, err := c.CreateShoppingList(context.Background(), "Vecka 1", nil)
	if err == nil {
		t.Fatal("expected an error for a 502 response")
	}
	if errors.Is(err, ErrElevatedAuthStale) {
		t.Errorf("502 should not be classified as ErrElevatedAuthStale (that's a distinct, undetectable-from-here failure shape): %v", err)
	}
}

func TestResolveRequirements_401IsNotWrappedAsElevatedAuthStale(t *testing.T) {
	// ResolveRequirements is AuthBasic — it should not go through
	// wrapElevatedAuthError at all, so even a hypothetical 401 here stays a
	// plain error rather than being misclassified as an elevated-auth issue.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewICA(srv.URL)
	_, err := c.ResolveRequirements(context.Background(), nil, nil)
	if err == nil {
		t.Fatal("expected an error for a 401 response")
	}
	if errors.Is(err, ErrElevatedAuthStale) {
		t.Error("ResolveRequirements (AuthBasic) should never surface ErrElevatedAuthStale")
	}
}

func TestWrapElevatedAuthError_NilPassesThrough(t *testing.T) {
	if wrapElevatedAuthError(nil) != nil {
		t.Error("wrapElevatedAuthError(nil) should return nil")
	}
}

func TestWrapElevatedAuthError_NonStatusErrorPassesThrough(t *testing.T) {
	plain := errors.New("some transport error")
	got := wrapElevatedAuthError(plain)
	if got != plain {
		t.Errorf("expected the original error to pass through unchanged, got: %v", got)
	}
}
