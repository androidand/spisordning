package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakePersonSvc is an in-memory PersonService for handler unit tests — no DB needed.
type fakePersonSvc struct {
	people  []PersonResponse
	created int
	err     error // returned by every method when non-nil
}

func newFake() *fakePersonSvc {
	return &fakePersonSvc{
		people: []PersonResponse{
			{ID: "p1", Name: "Ada", Weight: 1.0, CreatedAt: time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)},
			{ID: "p2", Name: "Blaise", Weight: 1.2, CreatedAt: time.Date(2026, 8, 19, 9, 5, 0, 0, time.UTC)},
		},
	}
}

func (f *fakePersonSvc) ListPeople(ctx context.Context) ([]PersonResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.people, nil
}

func (f *fakePersonSvc) GetPerson(ctx context.Context, id string) (PersonResponse, error) {
	if f.err != nil {
		return PersonResponse{}, f.err
	}
	for _, p := range f.people {
		if p.ID == id {
			return p, nil
		}
	}
	return PersonResponse{}, ErrNotFound
}

func (f *fakePersonSvc) CreatePerson(ctx context.Context, in PersonInput) (PersonResponse, error) {
	if f.err != nil {
		return PersonResponse{}, f.err
	}
	p := PersonResponse{
		ID: "new", Name: in.Name, Weight: in.Weight, CreatedAt: time.Now(),
	}
	f.people = append(f.people, p)
	f.created++
	return p, nil
}

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

func newMux(t *testing.T, svc PersonService) *http.ServeMux {
	t.Helper()
	mux := http.NewServeMux()
	RegisterHandlers(mux, Dependencies{People: svc})
	return mux
}

func doGet(t *testing.T, mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func doPost(t *testing.T, mux *http.ServeMux, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	return rec
}

func mustJSON(t *testing.T, data []byte, out any) {
	t.Helper()
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal %q: %v", data, err)
	}
}

func TestListPeople_HappyPath(t *testing.T) {
	mux := newMux(t, newFake())

	rec := doGet(t, mux, "/people")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}

	var got []PersonResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 2 {
		t.Fatalf("got %d people, want 2", len(got))
	}
	if got[0].ID != "p1" || got[0].Name != "Ada" || got[0].Weight != 1.0 {
		t.Fatalf("unexpected first person: %+v", got[0])
	}
}

func TestListPeople_Error(t *testing.T) {
	svc := newFake()
	svc.err = errSentinel("boom")
	mux := newMux(t, svc)

	rec := doGet(t, mux, "/people")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var body errorBody
	mustJSON(t, rec.Body.Bytes(), &body)
	if body.Message == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestGetPerson_HappyPath(t *testing.T) {
	mux := newMux(t, newFake())

	rec := doGet(t, mux, "/people/p2")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}

	var got PersonResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ID != "p2" || got.Name != "Blaise" || got.Weight != 1.2 {
		t.Fatalf("unexpected person: %+v", got)
	}
}

func TestGetPerson_NotFound(t *testing.T) {
	mux := newMux(t, newFake())

	rec := doGet(t, mux, "/people/does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body errorBody
	mustJSON(t, rec.Body.Bytes(), &body)
	if body.Message == "" {
		t.Fatal("expected error message in 404 body")
	}
}

func TestCreatePerson_HappyPath(t *testing.T) {
	svc := newFake()
	mux := newMux(t, svc)

	rec := doPost(t, mux, "/people", `{"name":"Grace","weight":1.1}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}

	var got PersonResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Name != "Grace" || got.Weight != 1.1 || got.ID == "" || got.CreatedAt.IsZero() {
		t.Fatalf("unexpected created person: %+v", got)
	}
	if svc.created != 1 {
		t.Fatalf("CreatePerson called %d times, want 1", svc.created)
	}
}

func TestCreatePerson_EmptyName(t *testing.T) {
	mux := newMux(t, newFake())

	rec := doPost(t, mux, "/people", `{"name":"  ","weight":1.0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	var body errorBody
	mustJSON(t, rec.Body.Bytes(), &body)
	if body.Message == "" {
		t.Fatal("expected error message")
	}
}

func TestCreatePerson_BadJSON(t *testing.T) {
	mux := newMux(t, newFake())

	rec := doPost(t, mux, "/people", `not-json`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHealthStillWorks(t *testing.T) {
	mux := newMux(t, newFake())

	rec := doGet(t, mux, "/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var h Health
	mustJSON(t, rec.Body.Bytes(), &h)
	if h.Status != "ok" {
		t.Fatalf(`status = %q, want "ok"`, h.Status)
	}
}

func TestNilServicesStillServesHealth(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHandlers(mux, Dependencies{}) // no services injected

	rec := doGet(t, mux, "/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	// /people is unregistered when the service is nil -> 404 from the mux.
	rec2 := doGet(t, mux, "/people")
	if rec2.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 for /people with no svc", rec2.Code)
	}
}
