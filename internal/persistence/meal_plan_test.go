package persistence

import (
	"context"
	"os"
	"testing"
	"time"
)

// skipWithoutDB skips the test when no Postgres is reachable from this host.
// CI provides one (see .github/workflows/ci.yml persistence-test job); local
// dev without `docker compose up -d` skips cleanly rather than failing red.
func skipWithoutDB(t *testing.T) *Store {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" && os.Getenv("POSTGRES_PASSWORD") == "" {
		t.Skip("no DATABASE_URL/POSTGRES_PASSWORD in env; skipping Postgres integration test")
	}
	cfg, err := FromEnv(os.Getenv)
	if err != nil {
		t.Skipf("no usable postgres config: %v", err)
	}
	ctx := context.Background()
	store, err := New(ctx, cfg)
	if err != nil {
		t.Skipf("cannot connect to postgres (expected without `docker compose up`): %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("parseDate %q: %v", s, err)
	}
	return d
}

func sameDate(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func TestMealPlan_CreateGetByWeek(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	weekStart := date(t, "2043-01-18") // first Monday of 2043
	id, err := s.CreateMealPlan(ctx, weekStart)
	if err != nil {
		t.Fatalf("CreateMealPlan: %v", err)
	}
	got, err := s.GetMealPlanByWeek(ctx, weekStart)
	if err != nil {
		t.Fatalf("GetMealPlanByWeek: %v", err)
	}
	if got.ID != id {
		t.Errorf("id %d, want %d", got.ID, id)
	}
	if got.Status != "draft" {
		t.Errorf("status %q, want draft", got.Status)
	}
	if !sameDate(got.WeekStart, weekStart) {
		t.Errorf("week_start %v, want %v", got.WeekStart, weekStart)
	}
	if err := s.SetMealPlanStatus(ctx, id, "approved"); err != nil {
		t.Fatalf("SetMealPlanStatus: %v", err)
	}
	got, _ = s.GetMealPlan(ctx, id)
	if got.Status != "approved" {
		t.Errorf("after approve: status %q, want approved", got.Status)
	}
}

func TestMealPlan_CandidatesAndDecisionsRoundTrip(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	weekStart := date(t, "2043-01-25")
	pid, err := s.CreateMealPlan(ctx, weekStart)
	if err != nil {
		t.Fatalf("CreateMealPlan: %v", err)
	}
	cand := MealPlanCandidate{
		PlanID:         pid,
		SlotDate:       weekStart,
		MealieRecipeID: "r-1",
		Score:          0.8,
		Breakdown:      map[string]float64{"pref": 0.5, "effort": 0.3},
		Feasible:       true,
		Rank:           0,
	}
	if err := s.InsertCandidate(ctx, cand); err != nil {
		t.Fatalf("InsertCandidate: %v", err)
	}
	got, err := s.ListCandidates(ctx, pid)
	if err != nil {
		t.Fatalf("ListCandidates: %v", err)
	}
	if len(got) != 1 || got[0].MealieRecipeID != "r-1" || !got[0].Feasible {
		t.Errorf("candidates = %+v", got)
	}

	dec := MealPlanDecision{PlanID: pid, SlotDate: weekStart, MealieRecipeID: "r-1"}
	if err := s.SetDecision(ctx, dec); err != nil {
		t.Fatalf("SetDecision: %v", err)
	}
	decs, err := s.ListDecisions(ctx, pid)
	if err != nil {
		t.Fatalf("ListDecisions: %v", err)
	}
	if len(decs) != 1 || decs[0].MealieRecipeID != "r-1" {
		t.Errorf("decisions = %+v", decs)
	}
}

func TestShoppingRequirements_RoundTrip(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	weekStart := date(t, "2043-02-01")
	pid, err := s.CreateMealPlan(ctx, weekStart)
	if err != nil {
		t.Fatalf("CreateMealPlan: %v", err)
	}
	pref := "g"
	req := ShoppingRequirement{
		PlanID:          pid,
		IngredientID:    "köttfärs",
		Quantity:        400,
		Unit:            "g",
		AcceptableForms: []string{"400 g", "500 g"},
		PreferredForm:   &pref,
	}
	if err := s.InsertShoppingRequirement(ctx, req); err != nil {
		t.Fatalf("InsertShoppingRequirement: %v", err)
	}
	// Idempotent merge on duplicate (plan_id, ingredient_id) sums quantity.
	if err := s.InsertShoppingRequirement(ctx, req); err != nil {
		t.Fatalf("second InsertShoppingRequirement: %v", err)
	}
	got, err := s.ListShoppingRequirements(ctx, pid)
	if err != nil {
		t.Fatalf("ListShoppingRequirements: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 requirement, got %d", len(got))
	}
	if got[0].Quantity != 800 {
		t.Errorf("merged quantity %v, want 800", got[0].Quantity)
	}
	if got[0].PreferredForm == nil || *got[0].PreferredForm != "g" {
		t.Errorf("preferred_form = %v", got[0].PreferredForm)
	}
}
