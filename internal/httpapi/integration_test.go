// Package httpapi contains API integration tests that exercise the HTTP layer
// against a real handler wired to a real Postgres Store (task 3.5). They skip
// cleanly when no DATABASE_URL/POSTGRES_PASSWORD is configured, matching the
// pattern used by internal/persistence/*_test.go.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/persistence"
)

// skipWithoutDB skips the test when no Postgres is reachable, matching the
// persistence package's convention.
func skipWithoutDB(t *testing.T) *persistence.Store {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" && os.Getenv("POSTGRES_PASSWORD") == "" {
		t.Skip("no DATABASE_URL/POSTGRES_PASSWORD in env; skipping HTTP integration test")
	}
	cfg, err := persistence.FromEnv(os.Getenv)
	if err != nil {
		t.Skipf("no usable postgres config: %v", err)
	}
	ctx := context.Background()
	store, err := persistence.New(ctx, cfg)
	if err != nil {
		t.Skipf("cannot connect to postgres: %v", err)
	}
	return store
}

// newTestServer builds an http.ServeMux wired to a real *persistence.Store and
// returns a httptest.Server bound to it. The caller owns the returned server.
func newTestServer(t *testing.T, store *persistence.Store) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	adapter := dbAdapter{store: store}
	RegisterHandlers(mux, Dependencies{
		People:      adapter,
		Preferences: adapter,
		Recipes:     adapter,
		Meals:       adapter,
	})
	return httptest.NewServer(mux)
}

// dbAdapter bridges *persistence.Store to the httpapi service interfaces. It
// lives in this test file so httpapi stays dependency-free of persistence.
type dbAdapter struct {
	store *persistence.Store
}

func (a dbAdapter) ListPeople(ctx context.Context) ([]PersonResponse, error) {
	people, err := a.store.ListPeople(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PersonResponse, 0, len(people))
	for _, p := range people {
		out = append(out, PersonResponse{ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt})
	}
	return out, nil
}

func (a dbAdapter) GetPerson(ctx context.Context, id string) (PersonResponse, error) {
	p, err := a.store.GetPerson(ctx, id)
	if err != nil {
		return PersonResponse{}, ErrNotFound
	}
	return PersonResponse{ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt}, nil
}

func (a dbAdapter) CreatePerson(ctx context.Context, in PersonInput) (PersonResponse, error) {
	if in.Weight <= 0 {
		in.Weight = 1.0
	}
	id := "itest-" + strings.ReplaceAll(time.Now().Format("20060102150405.000"), ".", "")
	p := persistence.Person{ID: id, Name: in.Name, Weight: in.Weight, CreatedAt: time.Now()}
	if err := a.store.CreatePerson(ctx, p); err != nil {
		return PersonResponse{}, err
	}
	return PersonResponse{ID: p.ID, Name: p.Name, Weight: p.Weight, CreatedAt: p.CreatedAt}, nil
}

func (a dbAdapter) ListPreferences(ctx context.Context, personID string) ([]PersonPreferenceResponse, error) {
	prefs, err := a.store.ListPreferences(ctx, personID)
	if err != nil {
		return nil, err
	}
	out := make([]PersonPreferenceResponse, 0, len(prefs))
	for _, p := range prefs {
		out = append(out, PersonPreferenceResponse{
			PersonID: p.PersonID, Tag: p.Tag, Sentiment: int(p.Sentiment),
			Confidence: p.Confidence, UpdatedAt: p.UpdatedAt,
		})
	}
	return out, nil
}

func (a dbAdapter) ListRecipes(ctx context.Context) ([]RecipeRefResponse, error) {
	refs, err := a.store.ListRecipeRefs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RecipeRefResponse, 0, len(refs))
	for _, r := range refs {
		out = append(out, RecipeRefResponse{
			MealieRecipeID: r.MealieRecipeID, Title: r.Title, Tags: r.Tags,
			Effort: r.Effort, LastSyncedAt: r.LastSyncedAt,
		})
	}
	return out, nil
}

func (a dbAdapter) GetMeal(ctx context.Context, id int64) (MealEventResponse, error) {
	event, err := a.store.GetMealEvent(ctx, id)
	if err != nil {
		return MealEventResponse{}, err
	}
	rxns, err := a.store.ListMealReactions(ctx, event.ID)
	if err != nil {
		return MealEventResponse{}, err
	}
	out := MealEventResponse{
		ID: event.ID, MealieRecipeID: event.MealieRecipeID,
		ServedOn:  event.ServedOn.Format("2006-01-02"),
		CreatedAt: event.CreatedAt,
		Reactions: make([]MealReactionResponse, 0, len(rxns)),
	}
	for _, r := range rxns {
		out.Reactions = append(out.Reactions, MealReactionResponse{PersonID: r.PersonID, Sentiment: r.Sentiment})
	}
	return out, nil
}

func (a dbAdapter) ListMeals(ctx context.Context, mealieRecipeID, servedOn string) ([]MealEventResponse, error) {
	events, err := a.store.ListMealEvents(ctx, mealieRecipeID, servedOn)
	if err != nil {
		return nil, err
	}
	out := make([]MealEventResponse, 0, len(events))
	for _, event := range events {
		rxns, err := a.store.ListMealReactions(ctx, event.ID)
		if err != nil {
			return nil, err
		}
		resp := MealEventResponse{
			ID: event.ID, MealieRecipeID: event.MealieRecipeID,
			ServedOn:  event.ServedOn.Format("2006-01-02"),
			CreatedAt: event.CreatedAt,
			Reactions: make([]MealReactionResponse, 0, len(rxns)),
		}
		for _, r := range rxns {
			resp.Reactions = append(resp.Reactions, MealReactionResponse{PersonID: r.PersonID, Sentiment: r.Sentiment})
		}
		out = append(out, resp)
	}
	return out, nil
}

func (a dbAdapter) CreateMealEvent(ctx context.Context, in MealEventNew) (MealEventResponse, error) {
	servedOn, err := time.Parse("2006-01-02", in.ServedOn)
	if err != nil {
		return MealEventResponse{}, err
	}
	eventID, err := a.store.CreateMealEvent(ctx, in.MealieRecipeID, servedOn, nil, nil)
	if err != nil {
		return MealEventResponse{}, err
	}
	for _, rx := range in.Reactions {
		if err := a.store.AddMealReaction(ctx, persistence.MealReaction{
			MealEventID: eventID, PersonID: rx.PersonID, Sentiment: rx.Sentiment,
		}); err != nil {
			return MealEventResponse{}, err
		}
	}
	rxns, err := a.store.ListMealReactions(ctx, eventID)
	if err != nil {
		return MealEventResponse{}, err
	}
	out := MealEventResponse{
		ID: eventID, MealieRecipeID: in.MealieRecipeID, ServedOn: in.ServedOn,
		CreatedAt: time.Now(), Reactions: make([]MealReactionResponse, 0, len(rxns)),
	}
	for _, r := range rxns {
		out.Reactions = append(out.Reactions, MealReactionResponse{
			PersonID: r.PersonID, Sentiment: r.Sentiment,
		})
	}
	return out, nil
}

// TestAPI_Health verifies GET /health returns 200 with {"status":"ok"}
// regardless of whether a database is configured.
func TestAPI_Health(t *testing.T) {
	mux := http.NewServeMux()
	RegisterHandlers(mux, Dependencies{})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /health: status = %d, want 200", rec.Code)
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body %q: %v", string(rec.Body.String()), err)
	}
	if body.Status != "ok" {
		t.Errorf("GET /health: status = %q, want ok", body.Status)
	}
}

// TestAPI_PeopleRoundTrip exercises the full /people lifecycle against a real
// Postgres-backed store: list, create, list (one), get by id, get 404.
func TestAPI_PeopleRoundTrip(t *testing.T) {
	store := skipWithoutDB(t)
	server := newTestServer(t, store)
	defer server.Close()

	// List people.
	rec := serverDoGet(server, "/people")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /people: status = %d", rec.Code)
	}
	var people []PersonResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &people); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Create one.
	body := `{"name":"Ada"}`
	rec = serverDoPost(server, "/people", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /people: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created PersonResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if created.Name != "Ada" || created.ID == "" {
		t.Errorf("POST /people: unexpected response %+v", created)
	}

	// Get by id.
	rec = serverDoGet(server, "/people/"+created.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /people/%s: status = %d", created.ID, rec.Code)
	}
	var got PersonResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got.ID != created.ID || got.Name != "Ada" {
		t.Errorf("GET /people/%s: got %+v", created.ID, got)
	}

	// Get non-existent → 404.
	rec = serverDoGet(server, "/people/does-not-exist")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /people/does-not-exist: status = %d, want 404", rec.Code)
	}
}

// TestAPI_MealsRoundTrip exercises POST /meals with reactions against a real
// Postgres-backed store, verifying the event is persisted and reactions are
// returned in the response.
func TestAPI_MealsRoundTrip(t *testing.T) {
	store := skipWithoutDB(t)
	ctx := context.Background()

	// meal_event has a FK to recipe_ref, so seed one.
	if err := store.UpsertRecipeRef(ctx, persistence.RecipeRef{
		MealieRecipeID: "r-integ-pasta", Title: "Pasta Bolognese", Tags: []string{"pasta"}, Effort: 2,
	}); err != nil {
		t.Fatalf("UpsertRecipeRef: %v", err)
	}

	server := newTestServer(t, store)
	defer server.Close()

	body := `{"mealie_recipe_id":"r-integ-pasta","served_on":"2026-08-18","reactions":[{"person_id":"p-kid","sentiment":2}]}`
	rec := serverDoPost(server, "/meals", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /meals: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var event MealEventResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &event); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if event.MealieRecipeID != "r-integ-pasta" || event.ServedOn != "2026-08-18" {
		t.Errorf("POST /meals: unexpected response %+v", event)
	}
	if len(event.Reactions) != 1 || event.Reactions[0].PersonID != "p-kid" || event.Reactions[0].Sentiment != 2 {
		t.Errorf("POST /meals: unexpected reactions %+v", event.Reactions)
	}
}

// TestAPI_Validation verifies that bad input is rejected with 400 before
// reaching the persistence layer.
func TestAPI_Validation(t *testing.T) {
	store := skipWithoutDB(t)
	server := newTestServer(t, store)
	defer server.Close()

	cases := []struct {
		name, method, path, body string
		wantCode                 int
	}{
		{"bad_json_post_people", http.MethodPost, "/people", `not-json`, http.StatusBadRequest},
		{"empty_name", http.MethodPost, "/people", `{"name":""}`, http.StatusBadRequest},
		{"missing_recipe_meals", http.MethodPost, "/meals", `{"served_on":"2026-08-18"}`, http.StatusBadRequest},
		{"missing_date_meals", http.MethodPost, "/meals", `{"mealie_recipe_id":"r-1"}`, http.StatusBadRequest},
		{"bad_date_meals", http.MethodPost, "/meals", `{"mealie_recipe_id":"r-1","served_on":"not-a-date"}`, http.StatusBadRequest},
		{"bad_sentiment_meals", http.MethodPost, "/meals", `{"mealie_recipe_id":"r-1","served_on":"2026-08-18","reactions":[{"person_id":"p1","sentiment":9}]}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var req *http.Request
			if c.body != "" {
				req = httptest.NewRequest(c.method, c.path, bytes.NewBufferString(c.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(c.method, c.path, nil)
			}
			rec := httptest.NewRecorder()
			server.Config.Handler.ServeHTTP(rec, req)
			if rec.Code != c.wantCode {
				t.Errorf("%s %s: status = %d, want %d; body = %s", c.method, c.path, rec.Code, c.wantCode, rec.Body.String())
			}
		})
	}
}

// serverDoGet and serverDoPost make requests against a httptest.Server.
// They avoid colliding with the mux-level doGet/doPost in people_test.go.
func serverDoGet(server *httptest.Server, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, server.URL+path, nil)
	rec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(rec, req)
	return rec
}

func serverDoPost(server *httptest.Server, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, server.URL+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(rec, req)
	return rec
}

func serverDoPatch(server *httptest.Server, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPatch, server.URL+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	server.Config.Handler.ServeHTTP(rec, req)
	return rec
}

// TestAPI_PlanRoundTrip exercises the /plans lifecycle against a real Postgres store.
func TestAPI_PlanRoundTrip(t *testing.T) {
	store := skipWithoutDB(t)
	server := newTestServer(t, store)
	defer server.Close()

	// Create a plan.
	rec := serverDoPost(server, "/plans", `{"week_start":"2026-01-13"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /plans: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var plan MealPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if plan.Status != "draft" {
		t.Errorf("expected draft, got %s", plan.Status)
	}

	// List plans.
	rec = serverDoGet(server, "/plans")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /plans: status = %d", rec.Code)
	}
	var plans []MealPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plans); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(plans) != 1 {
		t.Fatalf("expected 1 plan, got %d", len(plans))
	}

	// Get plan.
	rec = serverDoGet(server, "/plans/"+strconv.FormatInt(plan.ID, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /plans/{id}: status = %d", rec.Code)
	}
}

// TestAPI_PantryRoundTrip exercises the /pantry/locations lifecycle against a real Postgres store.
func TestAPI_PantryRoundTrip(t *testing.T) {
	store := skipWithoutDB(t)
	server := newTestServer(t, store)
	defer server.Close()

	// Create a location.
	rec := serverDoPost(server, "/pantry/locations", `{"name":"Kitchen","household_id":"h1"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /pantry/locations: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var loc PantryLocation
	if err := json.Unmarshal(rec.Body.Bytes(), &loc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if loc.Name != "Kitchen" {
		t.Errorf("expected Kitchen, got %s", loc.Name)
	}

	// List locations.
	rec = serverDoGet(server, "/pantry/locations")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /pantry/locations: status = %d", rec.Code)
	}
	var locs []PantryLocation
	if err := json.Unmarshal(rec.Body.Bytes(), &locs); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(locs) != 1 || locs[0].Name != "Kitchen" {
		t.Errorf("unexpected locations: %+v", locs)
	}
}

// TestAPI_MealsList verifies GET /meals returns an array against a real Postgres store.
func TestAPI_MealsList(t *testing.T) {
	store := skipWithoutDB(t)
	server := newTestServer(t, store)
	defer server.Close()

	rec := serverDoGet(server, "/meals")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /meals: status = %d", rec.Code)
	}
	var events []MealEventResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}
