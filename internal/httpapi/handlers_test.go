package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"
)

// ---- Preferences fakes + tests ----

type fakePrefsSvc struct {
	prefs []PersonPreferenceResponse
	call  string
	err   error
}

func (f *fakePrefsSvc) ListPreferences(ctx context.Context, personID string) ([]PersonPreferenceResponse, error) {
	f.call = personID
	if f.err != nil {
		return nil, f.err
	}
	return f.prefs, nil
}

func TestListPreferences_HappyPath(t *testing.T) {
	svc := &fakePrefsSvc{prefs: []PersonPreferenceResponse{
		{PersonID: "p1", Tag: "pasta", Sentiment: 2, Confidence: 0.9, UpdatedAt: time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)},
	}}
	mux := newMux(t, Dependencies{Preferences: svc})

	rec := doGet(t, mux, "/preferences?personId=p1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	if svc.call != "p1" {
		t.Errorf("filter = %q, want p1", svc.call)
	}
	var got []PersonPreferenceResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].Tag != "pasta" || got[0].Sentiment != 2 {
		t.Fatalf("unexpected preferences: %+v", got)
	}
}

func TestListPreferences_NoFilter(t *testing.T) {
	svc := &fakePrefsSvc{prefs: []PersonPreferenceResponse{{PersonID: "p1", Tag: "x"}}}
	mux := newMux(t, Dependencies{Preferences: svc})
	rec := doGet(t, mux, "/preferences")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if svc.call != "" {
		t.Errorf("filter = %q, want empty", svc.call)
	}
}

// ---- Recipes fakes + tests ----

type fakeRecipesSvc struct {
	refs []RecipeRefResponse
	err  error
}

func (f *fakeRecipesSvc) ListRecipes(ctx context.Context) ([]RecipeRefResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.refs, nil
}

func TestListRecipes_HappyPath(t *testing.T) {
	svc := &fakeRecipesSvc{refs: []RecipeRefResponse{
		{MealieRecipeID: "r-1", Title: "Pasta", Tags: []string{"pasta"}, Effort: 2, LastSyncedAt: time.Date(2026, 8, 19, 8, 0, 0, 0, time.UTC)},
	}}
	mux := newMux(t, Dependencies{Recipes: svc})

	rec := doGet(t, mux, "/recipes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got []RecipeRefResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if len(got) != 1 || got[0].MealieRecipeID != "r-1" || got[0].Title != "Pasta" || len(got[0].Tags) != 1 {
		t.Fatalf("unexpected recipes: %+v", got)
	}
}

// ---- Meals fakes + tests ----

type fakeMealsSvc struct {
	created MealEventResponse
	err     error
}

func (f *fakeMealsSvc) CreateMealEvent(ctx context.Context, in MealEventNew) (MealEventResponse, error) {
	if f.err != nil {
		return MealEventResponse{}, f.err
	}
	return f.created, nil
}

func TestCreateMealEvent_HappyPath(t *testing.T) {
	svc := &fakeMealsSvc{created: MealEventResponse{
		ID: 42, MealieRecipeID: "r-1", ServedOn: "2026-08-19",
		CreatedAt: time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC),
		Reactions: []MealReactionResponse{{PersonID: "p1", Sentiment: 2}},
	}}
	mux := newMux(t, Dependencies{Meals: svc})

	body := `{"mealie_recipe_id":"r-1","served_on":"2026-08-19","reactions":[{"person_id":"p1","sentiment":2}]}`
	rec := doPost(t, mux, "/meals", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	var got MealEventResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ID != 42 || got.MealieRecipeID != "r-1" || got.ServedOn != "2026-08-19" {
		t.Fatalf("unexpected meal event: %+v", got)
	}
	if len(got.Reactions) != 1 || got.Reactions[0].PersonID != "p1" {
		t.Fatalf("unexpected reactions: %+v", got.Reactions)
	}
}

func TestCreateMealEvent_Validation(t *testing.T) {
	mux := newMux(t, Dependencies{Meals: &fakeMealsSvc{}})

	cases := []struct {
		name, body string
		wantCode   int
	}{
		{"missing_recipe", `{"served_on":"2026-08-19"}`, http.StatusBadRequest},
		{"missing_date", `{"mealie_recipe_id":"r-1"}`, http.StatusBadRequest},
		{"bad_date", `{"mealie_recipe_id":"r-1","served_on":"not-a-date"}`, http.StatusBadRequest},
		{"bad_sentiment", `{"mealie_recipe_id":"r-1","served_on":"2026-08-19","reactions":[{"person_id":"p1","sentiment":9}]}`, http.StatusBadRequest},
		{"bad_json", `not-json`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := doPost(t, mux, "/meals", c.body)
			if rec.Code != c.wantCode {
				t.Errorf("%s: status = %d, want %d; body: %s", c.name, rec.Code, c.wantCode, rec.Body)
			}
		})
	}
}

func TestCreateMealEvent_ServiceError(t *testing.T) {
	svc := &fakeMealsSvc{err: errSentinel("dupe")}
	mux := newMux(t, Dependencies{Meals: svc})
	rec := doPost(t, mux, "/meals", `{"mealie_recipe_id":"r-1","served_on":"2026-08-19"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
