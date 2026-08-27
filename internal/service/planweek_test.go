package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/planning"
	"github.com/androidand/spisordning/internal/scoring"
)

type persistErrSentinel string

func (e persistErrSentinel) Error() string { return string(e) }

// fakePersistStore records every call persistWeek makes against a Store. It
// embeds Store (nil) so only the four plan-persistence methods are implemented;
// persistWeek never calls anything else.
type fakePersistStore struct {
	Store
	planID         int64
	getOrCreateErr error
	insCandErr     error
	setDecErr      error
	insReqErr      error

	candidates   []persistence.MealPlanCandidate
	decisions    []persistence.MealPlanDecision
	requirements []persistence.ShoppingRequirement
}

func (f *fakePersistStore) GetOrCreateMealPlan(ctx context.Context, weekStart time.Time) (persistence.MealPlan, error) {
	if f.getOrCreateErr != nil {
		return persistence.MealPlan{}, f.getOrCreateErr
	}
	return persistence.MealPlan{ID: f.planID, WeekStart: weekStart, Status: "draft"}, nil
}
func (f *fakePersistStore) InsertCandidate(ctx context.Context, c persistence.MealPlanCandidate) error {
	if f.insCandErr != nil {
		return f.insCandErr
	}
	f.candidates = append(f.candidates, c)
	return nil
}
func (f *fakePersistStore) SetDecision(ctx context.Context, d persistence.MealPlanDecision) error {
	if f.setDecErr != nil {
		return f.setDecErr
	}
	f.decisions = append(f.decisions, d)
	return nil
}
func (f *fakePersistStore) InsertShoppingRequirement(ctx context.Context, r persistence.ShoppingRequirement) error {
	if f.insReqErr != nil {
		return f.insReqErr
	}
	f.requirements = append(f.requirements, r)
	return nil
}

func testSlots() []planning.PlannedSlot {
	monday := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC) // Mon
	return []planning.PlannedSlot{
		{Date: monday, Winner: scoring.ScoredCandidate{
			Candidate: domain.Candidate{MealieRecipeID: "r-1", Title: "Pasta"},
			Score:     0.9,
			Breakdown: scoring.Breakdown{Preference: 0.5, Effort: 0.4},
			Feasible:  true,
		}},
		{Date: monday.AddDate(0, 0, 1), Winner: scoring.ScoredCandidate{
			Candidate: domain.Candidate{MealieRecipeID: "r-2", Title: "Tacos"},
			Score:     0.8,
			Feasible:  true,
		}},
	}
}

func TestPersistWeek_HappyPath(t *testing.T) {
	ctx := context.Background()
	store := &fakePersistStore{planID: 7}
	sl := testSlots()
	reqs := []domain.ShoppingRequirement{
		{IngredientID: "pasta", Quantity: 400, Unit: "g"},
		{IngredientID: "ost", Quantity: 200, Unit: "g", PreferredForm: "riven"},
	}

	if err := persistWeek(ctx, store, sl[0].Date, sl, reqs); err != nil {
		t.Fatalf("persistWeek: %v", err)
	}

	if len(store.candidates) != 2 {
		t.Errorf("candidates = %d, want 2", len(store.candidates))
	}
	for _, c := range store.candidates {
		if c.PlanID != 7 || c.MealieRecipeID == "" || c.Score == 0 {
			t.Errorf("unexpected candidate: %+v", c)
		}
	}
	if len(store.decisions) != 2 {
		t.Errorf("decisions = %d, want 2", len(store.decisions))
	}
	if len(store.requirements) != 2 {
		t.Errorf("requirements = %d, want 2", len(store.requirements))
	}
	// PreferredForm must become a non-nil *string; empty stays nil.
	var ost *persistence.ShoppingRequirement
	for i := range store.requirements {
		if store.requirements[i].IngredientID == "ost" {
			ost = &store.requirements[i]
		}
	}
	if ost == nil || ost.PreferredForm == nil || *ost.PreferredForm != "riven" {
		t.Errorf("ost preferred_form = %+v", ost)
	}
}

func TestPersistWeek_ErrorsPropagate(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		text  string
		field func(*fakePersistStore)
	}{
		{"get_or_create", "lock", func(s *fakePersistStore) { s.getOrCreateErr = persistErrSentinel("lock") }},
		{"insert_candidate", "dup", func(s *fakePersistStore) { s.insCandErr = persistErrSentinel("dup") }},
		{"set_decision", "conflict", func(s *fakePersistStore) { s.setDecErr = persistErrSentinel("conflict") }},
		{"insert_requirement", "bad", func(s *fakePersistStore) { s.insReqErr = persistErrSentinel("bad") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &fakePersistStore{planID: 1}
			c.field(store)
			err := persistWeek(ctx, store, time.Now(), testSlots(), []domain.ShoppingRequirement{{IngredientID: "x"}})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), c.text) {
				t.Fatalf("err = %q, want it to contain %q", err.Error(), c.text)
			}
		})
	}
}

func TestPersistWeek_EmptyPlannedStillCreatesPlan(t *testing.T) {
	ctx := context.Background()
	store := &fakePersistStore{planID: 3}
	// No slots, no requirements — the meal_plan row should still be created.
	if err := persistWeek(ctx, store, time.Now(), nil, nil); err != nil {
		t.Fatalf("persistWeek: %v", err)
	}
	if len(store.candidates) != 0 || len(store.decisions) != 0 || len(store.requirements) != 0 {
		t.Errorf("expected no child rows, got cand=%d dec=%d req=%d",
			len(store.candidates), len(store.decisions), len(store.requirements))
	}
}

func TestPlanWeek_Orchestrates(t *testing.T) {
	fakeMealie := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/recipes":
			w.Write([]byte(`{"page":1,"perPage":50,"total":2,"items":[
				{"id":"r-pasta","slug":"pasta","name":"Pasta Bolognese"},
				{"id":"r-tacos","slug":"tacos","name":"Tacos"}]}`))
		case "/api/recipes/pasta":
			w.Write([]byte(`{"id":"r-pasta","slug":"pasta","name":"Pasta Bolognese","totalTime":"20 min",
				"tags":[{"name":"pasta"}],
				"recipeIngredient":[{"quantity":400,"unit":{"name":"g"},"food":{"id":"f1","name":"köttfärs"}}]}`))
		case "/api/recipes/tacos":
			w.Write([]byte(`{"id":"r-tacos","slug":"tacos","name":"Tacos","totalTime":"30 min",
				"tags":[{"name":"mexican"}],
				"recipeIngredient":[{"quantity":200,"unit":{"name":"g"},"food":{"id":"f2","name":"ost"}}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(fakeMealie.Close)

	store := &fakePersistStore{planID: 42}
	svc := NewPlanning(store, mealie.New(fakeMealie.URL, "tok"))

	res, err := svc.PlanWeek(context.Background(), PlanWeekInput{
		WeekStart: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
		Days:      7,
		People:    []domain.Person{{ID: "p1", Name: "Andreas", Weight: 1.0}},
	})
	if err != nil {
		t.Fatalf("PlanWeek: %v", err)
	}
	if len(res.Planned) == 0 {
		t.Fatal("expected planned slots, got none")
	}
	if !res.Persisted || res.PersistError != nil {
		t.Errorf("persisted = %v, err = %v; want persisted with no error", res.Persisted, res.PersistError)
	}
	// One candidate + decision persisted per planned slot.
	if len(store.candidates) != len(res.Planned) || len(store.decisions) != len(res.Planned) {
		t.Errorf("persisted cand=%d dec=%d, want %d of each",
			len(store.candidates), len(store.decisions), len(res.Planned))
	}
	if len(res.Reqs) == 0 {
		t.Error("expected shopping requirements for planned meals")
	}
}

func TestPlanWeek_NoMealieClient(t *testing.T) {
	svc := NewPlanning(nil, nil)
	if _, err := svc.PlanWeek(context.Background(), PlanWeekInput{
		WeekStart: time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC),
	}); err == nil {
		t.Fatal("expected error when no Mealie client is configured")
	}
}
