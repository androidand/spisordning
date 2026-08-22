package httpapi

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/oapi-codegen/runtime/types"
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

// ---- Tonight fakes + tests ----

type fakeTonightSvc struct {
	view TonightView
	err  error
}

func (f *fakeTonightSvc) GetTonight(ctx context.Context) (TonightView, error) {
	if f.err != nil {
		return TonightView{}, f.err
	}
	return f.view, nil
}

func TestGetTonight_HappyPath(t *testing.T) {
	svc := &fakeTonightSvc{view: TonightView{
		ServedOn: "2026-08-21",
		Recipe:   RecipeRefResponse{MealieRecipeID: "r-1", Title: "Pasta Bolognese", Tags: []string{"pasta"}, Effort: 2},
		Reactions: []MealReactionResponse{{PersonID: "p1", Sentiment: 2}},
	}}
	mux := newMux(t, Dependencies{Tonight: svc})

	rec := doGet(t, mux, "/tonight")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	var got TonightView
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.ServedOn != "2026-08-21" || got.Recipe.Title != "Pasta Bolognese" {
		t.Fatalf("unexpected tonight view: %+v", got)
	}
	if len(got.Reactions) != 1 || got.Reactions[0].Sentiment != 2 {
		t.Fatalf("unexpected reactions: %+v", got.Reactions)
	}
}

func TestGetTonight_NotFound(t *testing.T) {
	svc := &fakeTonightSvc{err: ErrNoMealTonight}
	mux := newMux(t, Dependencies{Tonight: svc})

	rec := doGet(t, mux, "/tonight")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body errorBody
	mustJSON(t, rec.Body.Bytes(), &body)
	if body.Message == "" {
		t.Fatal("expected error message")
	}
}

func TestGetTonight_ServiceError(t *testing.T) {
	svc := &fakeTonightSvc{err: errSentinel("db error")}
	mux := newMux(t, Dependencies{Tonight: svc})

	rec := doGet(t, mux, "/tonight")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ---- Reactions fakes + tests ----

type fakeReactionsSvc struct {
	created MealReactionResponse
	err     error
}

func (f *fakeReactionsSvc) CreateReaction(ctx context.Context, in ReactionNew) (MealReactionResponse, error) {
	if f.err != nil {
		return MealReactionResponse{}, f.err
	}
	return f.created, nil
}

func TestCreateReaction_HappyPath(t *testing.T) {
	svc := &fakeReactionsSvc{created: MealReactionResponse{PersonID: "p1", Sentiment: 2}}
	mux := newMux(t, Dependencies{Reactions: svc})

	rec := doPost(t, mux, "/reactions", `{"person_id":"p1","sentiment":2}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	var got MealReactionResponse
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.PersonID != "p1" || got.Sentiment != 2 {
		t.Fatalf("unexpected reaction: %+v", got)
	}
}

func TestCreateReaction_Validation(t *testing.T) {
	mux := newMux(t, Dependencies{Reactions: &fakeReactionsSvc{}})

	cases := []struct {
		name, body string
		wantCode   int
	}{
		{"missing_person", `{"sentiment":1}`, http.StatusBadRequest},
		{"bad_sentiment_high", `{"person_id":"p1","sentiment":3}`, http.StatusBadRequest},
		{"bad_sentiment_low", `{"person_id":"p1","sentiment":-3}`, http.StatusBadRequest},
		{"bad_json", `not-json`, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := doPost(t, mux, "/reactions", c.body)
			if rec.Code != c.wantCode {
				t.Errorf("%s: status = %d, want %d", c.name, rec.Code, c.wantCode)
			}
		})
	}
}

func TestCreateReaction_ServiceError(t *testing.T) {
	svc := &fakeReactionsSvc{err: errSentinel("db error")}
	mux := newMux(t, Dependencies{Reactions: svc})

	rec := doPost(t, mux, "/reactions", `{"person_id":"p1","sentiment":1}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ---- Plans fakes + tests ----

type fakePlansSvc struct {
	result PlanRunResult
	err    error
	// Plan stubs
	plans           []PlanResponse
	planView        PlanView
	planErr         error
	candidates      []PlanCandidateResponse
	candidatesErr   error
	shoppingReqs    []ShoppingRequirementResponse
	shoppingErr     error
	decisions       []PlanDecisionInput
	decisionsErr    error
}

func (f *fakePlansSvc) RunPlan(ctx context.Context, in PlanRunInput) (PlanRunResult, error) {
	if f.err != nil {
		return PlanRunResult{}, f.err
	}
	return f.result, nil
}

func (f *fakePlansSvc) ListPlans(ctx context.Context) ([]PlanResponse, error) {
	if f.planErr != nil {
		return nil, f.planErr
	}
	return f.plans, nil
}

func (f *fakePlansSvc) CreatePlan(ctx context.Context, weekStart time.Time) (PlanResponse, error) {
	if f.planErr != nil {
		return PlanResponse{}, f.planErr
	}
	return PlanResponse{ID: 1, WeekStart: types.Date{Time: weekStart}, Status: "draft"}, nil
}

func (f *fakePlansSvc) GetPlan(ctx context.Context, planID int64) (PlanView, error) {
	if f.planErr != nil {
		return PlanView{}, f.planErr
	}
	return f.planView, nil
}

func (f *fakePlansSvc) UpdatePlan(ctx context.Context, planID int64, status string) (PlanResponse, error) {
	if f.planErr != nil {
		return PlanResponse{}, f.planErr
	}
	return PlanResponse{ID: int(planID), Status: status}, nil
}

func (f *fakePlansSvc) SetDecisions(ctx context.Context, planID int64, decisions []PlanDecisionInput) error {
	return f.decisionsErr
}

func (f *fakePlansSvc) ListCandidates(ctx context.Context, planID int64) ([]PlanCandidateResponse, error) {
	if f.candidatesErr != nil {
		return nil, f.candidatesErr
	}
	return f.candidates, nil
}

func (f *fakePlansSvc) InsertCandidates(ctx context.Context, candidates []PlanCandidateInput) error {
	return nil
}

func (f *fakePlansSvc) ListShoppingRequirements(ctx context.Context, planID int64) ([]ShoppingRequirementResponse, error) {
	if f.shoppingErr != nil {
		return nil, f.shoppingErr
	}
	return f.shoppingReqs, nil
}

func TestRunPlan_HappyPath(t *testing.T) {
	svc := &fakePlansSvc{result: PlanRunResult{
		Status:  "accepted",
		Message: "planned 7 dinners",
	}}
	mux := newMux(t, Dependencies{Plans: svc})

	rec := doPost(t, mux, "/plans/run", `{}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rec.Code, rec.Body)
	}
	var got PlanRunResult
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Status != "accepted" || got.Message != "planned 7 dinners" {
		t.Fatalf("unexpected result: %+v", got)
	}
}

func TestRunPlan_EmptyBody(t *testing.T) {
	svc := &fakePlansSvc{result: PlanRunResult{Status: "accepted", Message: "ok"}}
	mux := newMux(t, Dependencies{Plans: svc})

	rec := doPost(t, mux, "/plans/run", "")
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
}

func TestRunPlan_ServiceError(t *testing.T) {
	svc := &fakePlansSvc{err: errSentinel("mealie unavailable")}
	mux := newMux(t, Dependencies{Plans: svc})

	rec := doPost(t, mux, "/plans/run", `{}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	var got PlanRunResult
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Status != "failed" {
		t.Fatalf("status = %q, want failed", got.Status)
	}
}
