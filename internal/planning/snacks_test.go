package planning

import (
	"context"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

func TestHasSnackTag(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want bool
	}{
		{"mellanmål tag", []string{"mellanmål"}, true},
		{"mellanmal tag", []string{"mellanmal"}, true},
		{"snack tag", []string{"snack"}, true},
		{"SNACK uppercase", []string{"SNACK"}, true},
		{"mixed case", []string{"Snack"}, true},
		{"with spaces", []string{" mellanmål "}, true},
		{"dinner tag", []string{"middag"}, false},
		{"empty tags", []string{}, false},
		{"nil tags", nil, false},
		{"multiple tags with snack", []string{"vegetarisk", "snack"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasSnackTag(tt.tags); got != tt.want {
				t.Errorf("HasSnackTag(%v) = %v, want %v", tt.tags, got, tt.want)
			}
		})
	}
}

func TestHasBreakfastTag(t *testing.T) {
	tests := []struct {
		name string
		tags []string
		want bool
	}{
		{"frukost tag", []string{"frukost"}, true},
		{"breakfast tag", []string{"breakfast"}, true},
		{"BREAKFAST uppercase", []string{"BREAKFAST"}, true},
		{"dinner tag", []string{"middag"}, false},
		{"empty tags", []string{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasBreakfastTag(tt.tags); got != tt.want {
				t.Errorf("HasBreakfastTag(%v) = %v, want %v", tt.tags, got, tt.want)
			}
		})
	}
}

func TestFilterSnackCandidates(t *testing.T) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "yogurt", Title: "Yoghurt", Tags: []string{"mellanmål"}},
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}},
		{MealieRecipeID: "kex", Title: "Kex", Tags: []string{"snack", "bröd"}},
	}
	filtered := FilterSnackCandidates(candidates)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 snack candidates, got %d", len(filtered))
	}
	if filtered[0].MealieRecipeID != "yogurt" {
		t.Errorf("first = %q, want yogurt", filtered[0].MealieRecipeID)
	}
	if filtered[1].MealieRecipeID != "kex" {
		t.Errorf("second = %q, want kex", filtered[1].MealieRecipeID)
	}
}

func TestFilterBreakfastCandidates(t *testing.T) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "gröt", Title: "Gröt", Tags: []string{"frukost"}},
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}},
		{MealieRecipeID: "egg", Title: "Egg", Tags: []string{"breakfast"}},
	}
	filtered := FilterBreakfastCandidates(candidates)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 breakfast candidates, got %d", len(filtered))
	}
	if filtered[0].MealieRecipeID != "gröt" {
		t.Errorf("first = %q, want gröt", filtered[0].MealieRecipeID)
	}
	if filtered[1].MealieRecipeID != "egg" {
		t.Errorf("second = %q, want egg", filtered[1].MealieRecipeID)
	}
}

func TestSnackCandidates_FallbackWhenNoTagged(t *testing.T) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}},
	}
	result := SnackCandidates(candidates)
	if len(result) != len(FallbackSnacks) {
		t.Fatalf("expected %d fallback snacks, got %d", len(FallbackSnacks), len(result))
	}
}

func TestSnackCandidates_ReturnsTaggedWhenAvailable(t *testing.T) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "yogurt", Title: "Yoghurt", Tags: []string{"mellanmål"}},
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}},
	}
	result := SnackCandidates(candidates)
	if len(result) != 1 {
		t.Fatalf("expected 1 tagged snack, got %d", len(result))
	}
	if result[0].MealieRecipeID != "yogurt" {
		t.Errorf("got %q, want yogurt", result[0].MealieRecipeID)
	}
}

func TestBreakfastCandidates_WeekdayFallback(t *testing.T) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}},
	}
	// Monday 2026-08-17 is a weekday.
	date := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	result := BreakfastCandidates(candidates, date)
	if len(result) != len(FallbackBreakfastsWeekday) {
		t.Fatalf("expected %d weekday fallback breakfasts, got %d", len(FallbackBreakfastsWeekday), len(result))
	}
}

func TestBreakfastCandidates_WeekendFallback(t *testing.T) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}},
	}
	// Saturday 2026-08-22 is a weekend.
	date := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	result := BreakfastCandidates(candidates, date)
	if len(result) != len(FallbackBreakfastsWeekend) {
		t.Fatalf("expected %d weekend fallback breakfasts, got %d", len(FallbackBreakfastsWeekend), len(result))
	}
}

func TestBreakfastCandidates_PrefersTaggedOverFallback(t *testing.T) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "english", Title: "English breakfast", Tags: []string{"frukost", "ovrigt"}},
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}},
	}
	// Monday 2026-08-17 is a weekday.
	date := time.Date(2026, 8, 17, 0, 0, 0, 0, time.UTC)
	result := BreakfastCandidates(candidates, date)
	if len(result) != 1 {
		t.Fatalf("expected 1 tagged breakfast candidate, got %d", len(result))
	}
	if result[0].MealieRecipeID != "english" {
		t.Fatalf("expected 'english' recipe, got %q", result[0].MealieRecipeID)
	}
}

func TestPlanSimpleSlot_NoSchoolLunchPenalty(t *testing.T) {
	// A candidate that would be penalized by school lunch dedup in a full
	// PlanWeek context should NOT be penalized in PlanSimpleSlot.
	candidates := []domain.Candidate{
		{MealieRecipeID: "fisk", Title: "Fisk", Tags: []string{"fisk", "mellanmål"}, Effort: domain.EffortLow},
		{MealieRecipeID: "yoghurt", Title: "Yoghurt", Tags: []string{"mellanmål"}, Effort: domain.EffortLow},
	}
	cfg := WeekConfig{
		People: []domain.Person{
			{ID: "kid", Name: "Kid", Weight: 1},
		},
		Preferences: []domain.Preference{
			{PersonID: "kid", Tag: "fisk", Sentiment: domain.Likes, Confidence: 1.0},
		},
	}
	date := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	result := PlanSimpleSlot(context.Background(), cfg, candidates, date, domain.SlotSnack)
	if result == nil {
		t.Fatal("expected a planned slot, got nil")
	}
	// Fisk should win because it has the preference boost and no school-lunch
	// penalty is applied (PlanSimpleSlot doesn't set SchoolLunchTags).
	if result.Winner.Candidate.MealieRecipeID != "fisk" {
		t.Errorf("winner = %q, want fisk (no school-lunch penalty in simple slot)", result.Winner.Candidate.MealieRecipeID)
	}
	if result.Slot != domain.SlotSnack {
		t.Errorf("slot = %q, want snack", result.Slot)
	}
}

func TestPlanSimpleSlot_NoEffortFiltering(t *testing.T) {
	// A high-effort candidate should still be feasible in PlanSimpleSlot
	// because KitchenEnergy is not set.
	candidates := []domain.Candidate{
		{MealieRecipeID: "project", Title: "Projekt", Tags: []string{"mellanmål"}, Effort: domain.EffortHigh},
	}
	cfg := WeekConfig{
		People: []domain.Person{
			{ID: "mum", Name: "Mum", Weight: 1},
		},
	}
	date := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	result := PlanSimpleSlot(context.Background(), cfg, candidates, date, domain.SlotSnack)
	if result == nil {
		t.Fatal("expected a planned slot, got nil")
	}
	if !result.Winner.Feasible {
		t.Error("high-effort candidate should be feasible in simple slot (no effort filtering)")
	}
}

func TestPlanWeekAllSlots_PlanAllThreeSlots(t *testing.T) {
	dinnerCandidates := []domain.Candidate{
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}, Effort: domain.EffortLow},
	}
	breakfastCandidates := []domain.Candidate{
		{MealieRecipeID: "gröt", Title: "Gröt", Tags: []string{"frukost"}, Effort: domain.EffortLow},
	}
	snackCandidates := []domain.Candidate{
		{MealieRecipeID: "yogurt", Title: "Yoghurt", Tags: []string{"mellanmål"}, Effort: domain.EffortLow},
	}
	cfg := WeekConfig{
		Candidates: dinnerCandidates,
		People: []domain.Person{
			{ID: "mum", Name: "Mum", Weight: 1},
		},
	}
	monday := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	slots := PlanWeekAllSlots(context.Background(), cfg, monday, 1, breakfastCandidates, snackCandidates)

	// Expect 3 slots: dinner, breakfast, snack.
	if len(slots) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(slots))
	}

	slotKinds := map[domain.Slot]bool{}
	for _, s := range slots {
		slotKinds[s.Slot] = true
	}
	for _, kind := range []domain.Slot{domain.SlotDinner, domain.SlotBreakfast, domain.SlotSnack} {
		if !slotKinds[kind] {
			t.Errorf("missing slot kind %q", kind)
		}
	}
}

func TestPlanWeekAllSlots_SkipsBreakfastWhenNoCandidates(t *testing.T) {
	dinnerCandidates := []domain.Candidate{
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}, Effort: domain.EffortLow},
	}
	cfg := WeekConfig{
		Candidates: dinnerCandidates,
		People: []domain.Person{
			{ID: "mum", Name: "Mum", Weight: 1},
		},
	}
	monday := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	// No breakfast or snack candidates.
	slots := PlanWeekAllSlots(context.Background(), cfg, monday, 1, nil, nil)

	if len(slots) != 1 {
		t.Fatalf("expected 1 slot (dinner only), got %d", len(slots))
	}
	if slots[0].Slot != domain.SlotDinner {
		t.Errorf("slot = %q, want dinner", slots[0].Slot)
	}
}
