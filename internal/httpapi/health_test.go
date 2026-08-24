package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/androidand/spisordning/internal/dto"
)

func TestHealthHandler(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHandlers(mux, Dependencies{})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type %q, want application/json", ct)
	}
	var h dto.Health
	if err := json.NewDecoder(rec.Body).Decode(&h); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if h.Status != "ok" {
		t.Errorf("status %q, want ok", h.Status)
	}
}

func TestHealthUnknownRoute404(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHandlers(mux, Dependencies{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d, want 404", rec.Code)
	}
}
