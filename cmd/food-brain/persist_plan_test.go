package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/persistence"
	"github.com/androidand/spisordning/internal/planning"
	"github.com/androidand/spisordning/internal/scoring"
)

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

// fakePlanStore records every call persistPlan makes against a planStore.
type fakePlanStore struct {
	planID         int64
	getOrCreateErr error
	insCandErr     error
	setDecErr      error
	insReqErr      error

	candidates   []persistence.MealPlanCandidate
	decisions    []persistence.MealPlanDecision
	requirements []persistence.ShoppingRequirement
}

func (f *fakePlanStore) GetOrCreateMealPlan(ctx context.Context, weekStart time.Time) (persistence.MealPlan, error) {
	if f.getOrCreateErr != nil {
		return persistence.MealPlan{}, f.getOrCreateErr
	}
	return persistence.MealPlan{ID: f.planID, WeekStart: weekStart, Status: "draft"}, nil
}
func (f *fakePlanStore) InsertCandidate(ctx context.Context, c persistence.MealPlanCandidate) error {
	if f.insCandErr != nil {
		return f.insCandErr
	}
	f.candidates = append(f.candidates, c)
	return nil
}
func (f *fakePlanStore) SetDecision(ctx context.Context, d persistence.MealPlanDecision) error {
	if f.setDecErr != nil {
		return f.setDecErr
	}
	f.decisions = append(f.decisions, d)
	return nil
}
func (f *fakePlanStore) InsertShoppingRequirement(ctx context.Context, r persistence.ShoppingRequirement) error {
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

func TestPersistPlan_HappyPath(t *testing.T) {
	ctx := context.Background()
	store := &fakePlanStore{planID: 7}
	sl := testSlots()
	reqs := []domain.ShoppingRequirement{
		{IngredientID: "pasta", Quantity: 400, Unit: "g"},
		{IngredientID: "ost", Quantity: 200, Unit: "g", PreferredForm: "riven"},
	}

	if err := persistPlan(ctx, store, sl[0].Date, sl, reqs); err != nil {
		t.Fatalf("persistPlan: %v", err)
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

func TestPersistPlan_ErrorsPropagate(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		text  string
		field func(*fakePlanStore)
	}{
		{"get_or_create", "lock", func(s *fakePlanStore) { s.getOrCreateErr = errSentinel("lock") }},
		{"insert_candidate", "dup", func(s *fakePlanStore) { s.insCandErr = errSentinel("dup") }},
		{"set_decision", "conflict", func(s *fakePlanStore) { s.setDecErr = errSentinel("conflict") }},
		{"insert_requirement", "bad", func(s *fakePlanStore) { s.insReqErr = errSentinel("bad") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := &fakePlanStore{planID: 1}
			c.field(store)
			err := persistPlan(ctx, store, time.Now(), testSlots(), []domain.ShoppingRequirement{{IngredientID: "x"}})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), c.text) {
				t.Fatalf("err = %q, want it to contain %q", err.Error(), c.text)
			}
		})
	}
}

func TestPersistPlan_EmptyPlannedStillCreatesPlan(t *testing.T) {
	ctx := context.Background()
	store := &fakePlanStore{planID: 3}
	// No slots, no requirements — the meal_plan row should still be created.
	if err := persistPlan(ctx, store, time.Now(), nil, nil); err != nil {
		t.Fatalf("persistPlan: %v", err)
	}
	if len(store.candidates) != 0 || len(store.decisions) != 0 || len(store.requirements) != 0 {
		t.Errorf("expected no child rows, got cand=%d dec=%d req=%d",
			len(store.candidates), len(store.decisions), len(store.requirements))
	}
}
