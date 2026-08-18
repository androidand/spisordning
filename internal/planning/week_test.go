package planning

import (
	"context"
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/scoring"
)

// weekMonday is a fixed reference Monday so tests never depend on the wall clock.
var weekMonday = time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

// weekCtx builds the minimal context for the PlanWeek unit tests: two
// candidates the kid likes (pasta slightly more than rice) and no school tags.
func weekCtx() ([]domain.Candidate, WeekConfig) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}, Effort: domain.EffortLow},
		{MealieRecipeID: "rice", Title: "Ris", Tags: []string{"rice"}, Effort: domain.EffortLow},
	}
	cfg := WeekConfig{
		Candidates: candidates,
		People: []domain.Person{
			{ID: "mum", Name: "Mum", Weight: 1},
			{ID: "kid", Name: "Kid", Weight: 2},
		},
		Preferences: []domain.Preference{
			{PersonID: "kid", Tag: "pasta", Sentiment: domain.Likes, Confidence: 1.0},
			{PersonID: "kid", Tag: "rice", Sentiment: domain.Likes, Confidence: 0.9},
		},
		EnergyFor: func(time.Time) domain.Effort { return domain.EffortMedium },
	}
	return candidates, cfg
}

func TestPlanWeek_FeedsForwardRepetition(t *testing.T) {
	_, cfg := weekCtx()
	planned := PlanWeek(context.Background(), cfg, weekMonday, 2)
	if len(planned) != 2 {
		t.Fatalf("expected 2 planned slots, got %d", len(planned))
	}
	// Day 1: pasta (higher preference) wins.
	if planned[0].Winner.Candidate.MealieRecipeID != "pasta" {
		t.Errorf("day 1 winner = %q, want pasta", planned[0].Winner.Candidate.MealieRecipeID)
	}
	// Day 2: pasta was just served, so the repetition penalty lets rice win.
	if planned[1].Winner.Candidate.MealieRecipeID != "rice" {
		t.Errorf("day 2 winner = %q, want rice (repetition should carry forward)", planned[1].Winner.Candidate.MealieRecipeID)
	}
	if !planned[0].Date.Equal(weekMonday) || !planned[1].Date.Equal(weekMonday.AddDate(0, 0, 1)) {
		t.Errorf("dates wrong: %v %v", planned[0].Date, planned[1].Date)
	}
}

func TestPlanWeek_InfeasibleDayYieldsNoSlot(t *testing.T) {
	_, cfg := weekCtx()
	// Only a high-effort recipe exists, and the cook is exhausted on day 1.
	cfg.Candidates = []domain.Candidate{
		{MealieRecipeID: "project", Title: "Projekt", Effort: domain.EffortHigh},
	}
	cfg.EnergyFor = func(date time.Time) domain.Effort {
		if date.Equal(weekMonday) {
			return domain.EffortLow // exhausted Monday
		}
		return domain.EffortHigh
	}

	planned := PlanWeek(context.Background(), cfg, weekMonday, 2)
	if len(planned) != 1 {
		t.Fatalf("expected exactly 1 slot (Tuesday), got %d", len(planned))
	}
	if !planned[0].Date.Equal(weekMonday.AddDate(0, 0, 1)) {
		t.Errorf("the one slot should be Tuesday, got %v", planned[0].Date)
	}
}

func TestPlanWeek_ReorderHookIsApplied(t *testing.T) {
	_, cfg := weekCtx()
	cfg.Reorder = func(_ context.Context, ranked []scoring.ScoredCandidate) []scoring.ScoredCandidate {
		// Reverse the scorer order: rice wins day 1 despite pasta scoring higher.
		out := make([]scoring.ScoredCandidate, 0, len(ranked))
		for i := len(ranked) - 1; i >= 0; i-- {
			out = append(out, ranked[i])
		}
		return out
	}
	planned := PlanWeek(context.Background(), cfg, weekMonday, 1)
	if len(planned) != 1 || planned[0].Winner.Candidate.MealieRecipeID != "rice" {
		t.Errorf("reorder hook should pick rice, got %v", planned)
	}
}

func TestPlanWeek_Deterministic(t *testing.T) {
	_, cfg := weekCtx()
	first := PlanWeek(context.Background(), cfg, weekMonday, 3)
	for i := range 5 {
		again := PlanWeek(context.Background(), cfg, weekMonday, 3)
		if len(again) != len(first) {
			t.Fatalf("run %d length differs: %d vs %d", i, len(again), len(first))
		}
		for j := range first {
			if again[j].Winner.Candidate.MealieRecipeID != first[j].Winner.Candidate.MealieRecipeID {
				t.Fatalf("run %d day %d winner differs: %s vs %s", i, j,
					again[j].Winner.Candidate.MealieRecipeID, first[j].Winner.Candidate.MealieRecipeID)
			}
		}
	}
}
