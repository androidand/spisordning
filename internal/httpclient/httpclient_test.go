package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGetJSON_RoundTrip(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/things" || r.Method != http.MethodGet {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"name": "x"})
	}))
	defer srv.Close()

	var out struct {
		Name string `json:"name"`
	}
	headers := func(req *http.Request) { req.Header.Set("Authorization", "Bearer tok") }
	if err := New(srv.URL, "test", time.Second).GetJSON(context.Background(), "/things", &out, headers); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if out.Name != "x" || gotAuth != "Bearer tok" {
		t.Errorf("round trip failed: out=%+v auth=%q", out, gotAuth)
	}
}

func TestPostJSON_RoundTripAccepts2xx(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/submit" || r.Method != http.MethodPost {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %q, want application/json", ct)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated) // 201 must be accepted, not just 200
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "42"})
	}))
	defer srv.Close()

	var out struct {
		ID string `json:"id"`
	}
	err := New(srv.URL, "test", time.Second).PostJSON(context.Background(), "/submit",
		map[string]any{"name": "y"}, &out, nil)
	if err != nil {
		t.Fatalf("PostJSON: %v", err)
	}
	if out.ID != "42" || body["name"] != "y" {
		t.Errorf("round trip failed: out=%+v body=%v", out, body)
	}
}

func TestPatchJSON_RoundTrip(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/recipes/pasta" || r.Method != http.MethodPatch {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %q, want application/json", ct)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]string{"slug": "pasta"})
	}))
	defer srv.Close()

	var out struct {
		Slug string `json:"slug"`
	}
	err := New(srv.URL, "test", time.Second).PatchJSON(context.Background(), "/recipes/pasta",
		map[string]any{"recipeIngredient": []string{"x"}}, &out, nil)
	if err != nil {
		t.Fatalf("PatchJSON: %v", err)
	}
	if out.Slug != "pasta" {
		t.Errorf("round trip failed: out=%+v", out)
	}
	if ings, _ := body["recipeIngredient"].([]any); len(ings) != 1 {
		t.Errorf("unexpected patched body: %v", body)
	}
}

func TestNon2xx_SurfacesBackendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "session expired"})
	}))
	defer srv.Close()

	err := New(srv.URL, "backend", time.Second).GetJSON(context.Background(), "/x", &struct{}{}, nil)
	if err == nil {
		t.Fatal("expected an error for non-2xx")
	}
	if !strings.Contains(err.Error(), "backend") || !strings.Contains(err.Error(), "session expired") {
		t.Errorf("error should be attributed and carry the backend message: %v", err)
	}

	var statusErr *StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("expected a *StatusError, got %T", err)
	}
	if statusErr.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %d, want %d", statusErr.StatusCode, http.StatusBadGateway)
	}
	if statusErr.Detail != "session expired" {
		t.Errorf("Detail = %q, want %q", statusErr.Detail, "session expired")
	}
}

func Test2xx_UndecodableBodyErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	err := New(srv.URL, "backend", time.Second).GetJSON(context.Background(), "/x", &struct{}{}, nil)
	if err == nil {
		t.Fatal("expected a decode error for an undecodable 2xx body")
	}
}
