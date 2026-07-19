package scoring

import (
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

// day is a fixed reference date so tests never depend on the wall clock.
var day = time.Date(2026, 7, 20, 17, 0, 0, 0, time.UTC)

func person(id string, weight float64) domain.Person {
	return domain.Person{ID: id, Name: id, Weight: weight}
}

// baseCtx is a minimal two-person context with no recent meals, no school
// lunch, and no campaigns — signals are added per test.
func baseCtx() domain.PlanContext {
	return domain.PlanContext{
		Day:           day,
		People:        []domain.Person{person("mum", 1), person("kid", 2)},
		KitchenEnergy: domain.EffortHigh,
	}
}

func TestRank_DeterministicAndReproducible(t *testing.T) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "r1", Title: "Fisk", Tags: []string{"fish"}, Effort: domain.EffortLow},
		{MealieRecipeID: "r2", Title: "Pasta", Tags: []string{"pasta"}, Effort: domain.EffortLow},
		{MealieRecipeID: "r3", Title: "Gryta", Tags: []string{"stew"}, Effort: domain.EffortMedium},
	}
	ctx := baseCtx()
	ctx.Preferences = []domain.Preference{
		{PersonID: "kid", Tag: "pasta", Sentiment: domain.Loves, Confidence: 0.9},
		{PersonID: "kid", Tag: "fish", Sentiment: domain.Hates, Confidence: 0.9},
	}
	w := DefaultWeights()

	first := Rank(candidates, ctx, w)
	// Reproducibility: identical inputs → identical ordering and scores.
	for i := range 5 {
		again := Rank(candidates, ctx, w)
		if len(again) != len(first) {
			t.Fatalf("length changed between runs")
		}
		for j := range first {
			if again[j].Candidate.MealieRecipeID != first[j].Candidate.MealieRecipeID {
				t.Fatalf("run %d order differs at %d: %s vs %s", i, j,
					again[j].Candidate.MealieRecipeID, first[j].Candidate.MealieRecipeID)
			}
			if again[j].Score != first[j].Score {
				t.Fatalf("run %d score differs for %s", i, first[j].Candidate.MealieRecipeID)
			}
		}
	}

	// The kid loves pasta and hates fish (and counts double), so pasta wins and
	// fish loses.
	if first[0].Candidate.MealieRecipeID != "r2" {
		t.Errorf("expected pasta (r2) first, got %s", first[0].Candidate.MealieRecipeID)
	}
	if last := first[len(first)-1]; last.Candidate.MealieRecipeID != "r1" {
		t.Errorf("expected fish (r1) last, got %s", last.Candidate.MealieRecipeID)
	}
}

func TestFeasibility_EffortIsHardConstraint(t *testing.T) {
	ctx := baseCtx()
	ctx.KitchenEnergy = domain.EffortLow // exhausted cook

	high := domain.Candidate{MealieRecipeID: "big", Title: "Projekt", Effort: domain.EffortHigh}
	low := domain.Candidate{MealieRecipeID: "easy", Title: "Mackor", Effort: domain.EffortLow}

	ranked := Rank([]domain.Candidate{high, low}, ctx, DefaultWeights())

	// The infeasible high-effort meal must rank last regardless of any score.
	if ranked[0].Candidate.MealieRecipeID != "easy" {
		t.Errorf("feasible meal should rank first, got %s", ranked[0].Candidate.MealieRecipeID)
	}
	for _, r := range ranked {
		if r.Candidate.MealieRecipeID == "big" && r.Feasible {
			t.Errorf("high-effort meal should be infeasible on a low-energy day")
		}
	}
}

func TestPreference_ConfidenceScales(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r", Tags: []string{"spicy"}}
	high := baseCtx()
	high.People = []domain.Person{person("a", 1)}
	high.Preferences = []domain.Preference{{PersonID: "a", Tag: "spicy", Sentiment: domain.Loves, Confidence: 1.0}}

	low := baseCtx()
	low.People = []domain.Person{person("a", 1)}
	low.Preferences = []domain.Preference{{PersonID: "a", Tag: "spicy", Sentiment: domain.Loves, Confidence: 0.1}}

	hs := Rank([]domain.Candidate{c}, high, DefaultWeights())[0].Breakdown.Preference
	ls := Rank([]domain.Candidate{c}, low, DefaultWeights())[0].Breakdown.Preference
	if !(hs > ls) {
		t.Errorf("high-confidence liking (%.3f) should outscore low-confidence (%.3f)", hs, ls)
	}
}

func TestPersonWeight_PickyKidCountsMore(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r", Tags: []string{"broccoli"}}
	ctx := baseCtx()
	ctx.People = []domain.Person{person("parent", 1), person("kid", 3)}
	ctx.Preferences = []domain.Preference{
		{PersonID: "parent", Tag: "broccoli", Sentiment: domain.Loves, Confidence: 1},
		{PersonID: "kid", Tag: "broccoli", Sentiment: domain.Hates, Confidence: 1},
	}
	// Weighted aggregate: (1*2 + 3*-2) / (1+3) = (2 - 6)/4 = -1 → net negative.
	got := Rank([]domain.Candidate{c}, ctx, DefaultWeights())[0].Breakdown.Preference
	if got >= 0 {
		t.Errorf("heavily-weighted kid's hatred should make preference negative, got %.3f", got)
	}
}

func TestRepetitionPenalty_RecentServedHurtsMore(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r", Tags: []string{"tacos"}}

	yesterday := baseCtx()
	yesterday.RecentMealIDs = []domain.RecentMeal{{MealieRecipeID: "r", Served: day.AddDate(0, 0, -1)}}
	tenDaysAgo := baseCtx()
	tenDaysAgo.RecentMealIDs = []domain.RecentMeal{{MealieRecipeID: "r", Served: day.AddDate(0, 0, -10)}}

	recent := Rank([]domain.Candidate{c}, yesterday, DefaultWeights())[0].Breakdown.Repetition
	old := Rank([]domain.Candidate{c}, tenDaysAgo, DefaultWeights())[0].Breakdown.Repetition

	if !(recent < old) {
		t.Errorf("a repeat yesterday (%.3f) should be penalized more than 10 days ago (%.3f)", recent, old)
	}
	if recent >= 0 {
		t.Errorf("a recent repeat should be a penalty (negative), got %.3f", recent)
	}
}

func TestRepetitionPenalty_OldRepeatIgnored(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := baseCtx()
	ctx.RecentMealIDs = []domain.RecentMeal{{MealieRecipeID: "r", Served: day.AddDate(0, 0, -30)}}
	if p := Rank([]domain.Candidate{c}, ctx, DefaultWeights())[0].Breakdown.Repetition; p != 0 {
		t.Errorf("a repeat older than the window should carry no penalty, got %.3f", p)
	}
}

func TestSchoolDedup_PenalizesOverlap(t *testing.T) {
	fish := domain.Candidate{MealieRecipeID: "f", Tags: []string{"fish"}}
	pasta := domain.Candidate{MealieRecipeID: "p", Tags: []string{"pasta"}}
	ctx := baseCtx()
	ctx.SchoolLunchTags = []string{"fish"}

	ranked := Rank([]domain.Candidate{fish, pasta}, ctx, DefaultWeights())
	// Pasta (no overlap with school) should beat fish (served at school today).
	if ranked[0].Candidate.MealieRecipeID != "p" {
		t.Errorf("non-overlapping meal should rank first, got %s", ranked[0].Candidate.MealieRecipeID)
	}
	for _, r := range ranked {
		if r.Candidate.MealieRecipeID == "f" && r.Breakdown.SchoolDedup >= 0 {
			t.Errorf("fish should carry a school-dedup penalty, got %.3f", r.Breakdown.SchoolDedup)
		}
	}
}

func TestCampaignBonus_RanksUpOnSale(t *testing.T) {
	// Two otherwise-identical meals; one uses an on-campaign ingredient.
	onSale := domain.Candidate{MealieRecipeID: "sale", Ingredients: []string{"chicken"}}
	full := domain.Candidate{MealieRecipeID: "full", Ingredients: []string{"beef"}}
	ctx := baseCtx()
	ctx.CampaignIngredients = map[string]bool{"chicken": true}

	ranked := Rank([]domain.Candidate{full, onSale}, ctx, DefaultWeights())
	if ranked[0].Candidate.MealieRecipeID != "sale" {
		t.Errorf("on-campaign meal should rank first, got %s", ranked[0].Candidate.MealieRecipeID)
	}
}

func TestRank_EmptyInputs(t *testing.T) {
	if got := Rank(nil, baseCtx(), DefaultWeights()); len(got) != 0 {
		t.Errorf("nil candidates should yield empty ranking, got %d", len(got))
	}
	// A candidate with no signals in an empty context scores exactly zero.
	c := domain.Candidate{MealieRecipeID: "r"}
	empty := domain.PlanContext{Day: day}
	got := Rank([]domain.Candidate{c}, empty, DefaultWeights())
	if got[0].Score != 0 {
		t.Errorf("no-signal candidate should score 0, got %.3f", got[0].Score)
	}
	if !got[0].Feasible {
		t.Errorf("candidate should be feasible when kitchen energy is unspecified")
	}
}
