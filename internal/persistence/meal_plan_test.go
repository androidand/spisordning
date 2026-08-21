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
	// Clear any leftover plans from a prior run so the create-then-approve
	// assertions below start from a clean state (this test runs first, so it
	// also resets state for the other meal-plan tests).
	truncateTables(t, ctx, s, "meal_plan")
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

func TestMealPlan_GetOrCreateMealPlan(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	weekStart := date(t, "2043-03-01") // a fresh week
	// Seed the FK targets the anchored artifacts reference below.
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-1", Title: "Test Recipe", Effort: 1}); err != nil {
		t.Fatalf("UpsertRecipeRef (fixture): %v", err)
	}
	if err := s.UpsertIngredient(ctx, Ingredient{ID: "köttfärs", Display: "Köttfärs"}); err != nil {
		t.Fatalf("UpsertIngredient (fixture): %v", err)
	}
	// First call creates the 'draft' plan row.
	p1, err := s.GetOrCreateMealPlan(ctx, weekStart)
	if err != nil {
		t.Fatalf("GetOrCreateMealPlan create: %v", err)
	}
	if p1.ID == 0 || p1.Status != "draft" {
		t.Fatalf("first call = %+v", p1)
	}
	// Second call returns the SAME row (not a duplicate).
	p2, err := s.GetOrCreateMealPlan(ctx, weekStart)
	if err != nil {
		t.Fatalf("GetOrCreateMealPlan get: %v", err)
	}
	if p2.ID != p1.ID {
		t.Fatalf("expected same plan id, got %d and %d", p1.ID, p2.ID)
	}
	// Anchor a full plan artifact set against the single plan (2.3 wiring surface).
	if err := s.InsertCandidate(ctx, MealPlanCandidate{
		PlanID: p1.ID, SlotDate: weekStart, MealieRecipeID: "r-1",
		Score: 0.85, Breakdown: map[string]float64{"pref": 0.5, "effort": 0.35},
		Feasible: true, Rank: 0,
	}); err != nil {
		t.Fatalf("InsertCandidate: %v", err)
	}
	if err := s.SetDecision(ctx, MealPlanDecision{
		PlanID: p1.ID, SlotDate: weekStart, MealieRecipeID: "r-1",
	}); err != nil {
		t.Fatalf("SetDecision: %v", err)
	}
	if err := s.InsertShoppingRequirement(ctx, ShoppingRequirement{
		PlanID: p1.ID, IngredientID: "köttfärs", Quantity: 400, Unit: "g",
	}); err != nil {
		t.Fatalf("InsertShoppingRequirement: %v", err)
	}
	cands, _ := s.ListCandidates(ctx, p1.ID)
	decs, _ := s.ListDecisions(ctx, p1.ID)
	reqs, _ := s.ListShoppingRequirements(ctx, p1.ID)
	if len(cands) != 1 || len(decs) != 1 || len(reqs) != 1 {
		t.Fatalf("anchored artifacts: %d cand %d dec %d req", len(cands), len(decs), len(reqs))
	}
}

func TestMealPlan_GetOrCreateMealPlan_NotDuplicatedAcrossWeeks(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	a := date(t, "2043-03-08")
	b := date(t, "2043-03-15")
	pa, _ := s.GetOrCreateMealPlan(ctx, a)
	pb, _ := s.GetOrCreateMealPlan(ctx, b)
	if pa.ID == pb.ID {
		t.Fatalf("different weeks must not share a plan row")
	}
}

func TestMealPlan_CandidatesAndDecisionsRoundTrip(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	weekStart := date(t, "2043-01-25")
	// Seed the recipe_ref the candidate/decision reference.
	if err := s.UpsertRecipeRef(ctx, RecipeRef{MealieRecipeID: "r-1", Title: "Test Recipe", Effort: 1}); err != nil {
		t.Fatalf("UpsertRecipeRef (fixture): %v", err)
	}
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
	// Seed the ingredient the shopping requirement references.
	if err := s.UpsertIngredient(ctx, Ingredient{ID: "köttfärs", Display: "Köttfärs"}); err != nil {
		t.Fatalf("UpsertIngredient (fixture): %v", err)
	}
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
