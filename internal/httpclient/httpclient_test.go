package httpclient

import (
	"context"
	"encoding/json"
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
