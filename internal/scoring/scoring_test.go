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

// history builds a PlanContext whose RecentMealIDs contain the given recipe ids.
func history(ids ...string) domain.PlanContext {
	ctx := baseCtx()
	for _, id := range ids {
		ctx.RecentMealIDs = append(ctx.RecentMealIDs, domain.RecentMeal{MealieRecipeID: id, Served: day.AddDate(0, 0, -30)})
	}
	return ctx
}

func TestFamiliarity_FrequentCookIsFamiliar(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	// Cooked 9 times → strongly familiar (positive signal).
	ctx := history("r", "r", "r", "r", "r", "r", "r", "r", "r")
	got := Rank([]domain.Candidate{c}, ctx, DefaultWeights())[0].Breakdown.Familiarity
	if got <= 0 {
		t.Errorf("a frequently-cooked recipe should carry a positive familiarity contribution, got %.3f", got)
	}
}

func TestFamiliarity_NeverCookedIsNovel(t *testing.T) {
	// The household has a meal history, but this recipe was never in it → novel.
	other := domain.Candidate{MealieRecipeID: "other"}
	novel := domain.Candidate{MealieRecipeID: "novel"}
	ctx := history("other", "other", "other", "other", "other", "other")

	ranked := Rank([]domain.Candidate{other, novel}, ctx, DefaultWeights())
	byID := map[string]float64{}
	for _, r := range ranked {
		byID[r.Candidate.MealieRecipeID] = r.Breakdown.Familiarity
	}
	// The known recipe is familiar (positive); the untried one is novel (negative).
	if byID["other"] <= 0 {
		t.Errorf("a cooked recipe should be familiar (positive), got %.3f", byID["other"])
	}
	if byID["novel"] >= 0 {
		t.Errorf("a never-cooked recipe should be novel (negative), got %.3f", byID["novel"])
	}
}

func TestFamiliarity_NoHistoryIsNeutral(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	// No meal history at all → no basis for the signal → 0.
	ctx := baseCtx()
	got := Rank([]domain.Candidate{c}, ctx, DefaultWeights())[0].Breakdown.Familiarity
	if got != 0 {
		t.Errorf("with no meal history the familiarity signal should be 0, got %.3f", got)
	}
}

func TestFamiliarity_Deterministic(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := history("r", "r", "other")
	w := DefaultWeights()
	first := Rank([]domain.Candidate{c}, ctx, w)[0].Breakdown.Familiarity
	for i := range 5 {
		again := Rank([]domain.Candidate{c}, ctx, w)[0].Breakdown.Familiarity
		if again != first {
			t.Fatalf("run %d familiarity differs: %.3f vs %.3f", i, again, first)
		}
	}
}

// favCtx builds a single-person context where the person has the given sentiment
// toward tag (full confidence) and recipe "r" has been cooked cookTimes times,
// spaced 30+ days apart so the repetition penalty stays out of the picture.
func favCtx(tag string, sentiment domain.Sentiment, cookTimes int) domain.PlanContext {
	ctx := baseCtx()
	ctx.People = []domain.Person{person("kid", 1)}
	ctx.Preferences = []domain.Preference{{PersonID: "kid", Tag: tag, Sentiment: sentiment, Confidence: 1.0}}
	for i := 0; i < cookTimes; i++ {
		ctx.RecentMealIDs = append(ctx.RecentMealIDs, domain.RecentMeal{MealieRecipeID: "r", Served: day.AddDate(0, 0, -(30 + i*30))})
	}
	return ctx
}

func TestIsKnownFavorite_LikedAndCooked(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r", Tags: []string{"pasta"}}
	if !IsKnownFavorite(c, favCtx("pasta", domain.Loves, 2)) {
		t.Errorf("a well-liked, twice-cooked recipe should be a known favorite")
	}
}

func TestIsKnownFavorite_LikedButNeverCooked(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r", Tags: []string{"pasta"}}
	if IsKnownFavorite(c, favCtx("pasta", domain.Loves, 0)) {
		t.Errorf("a liked recipe with no meal history should not be a known favorite")
	}
}

func TestIsKnownFavorite_CookedButDisliked(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r", Tags: []string{"broccoli"}}
	if IsKnownFavorite(c, favCtx("broccoli", domain.Hates, 3)) {
		t.Errorf("a frequently-cooked recipe the family hates should not be a known favorite")
	}
}

func TestIsKnownFavorite_Deterministic(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r", Tags: []string{"pasta"}}
	ctx := favCtx("pasta", domain.Likes, 2)
	first := IsKnownFavorite(c, ctx)
	for i := range 5 {
		if again := IsKnownFavorite(c, ctx); again != first {
			t.Fatalf("run %d differs: %v vs %v", i, again, first)
		}
	}
}

func TestIsDiscovery_NeverCooked(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	// No meal history at all → never cooked → a discovery candidate.
	if !IsDiscovery(c, baseCtx()) {
		t.Errorf("a never-cooked recipe should be a discovery candidate")
	}
}

func TestIsDiscovery_CookedOnceIsStillDiscovery(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := history("r") // cooked once → still minimal history
	if !IsDiscovery(c, ctx) {
		t.Errorf("a once-cooked recipe should still be a discovery candidate")
	}
}

func TestIsDiscovery_RegularlyCookedIsNotDiscovery(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := history("r", "r", "r") // cooked 3 times → familiar, not novel
	if IsDiscovery(c, ctx) {
		t.Errorf("a regularly-cooked recipe should not be a discovery candidate")
	}
}

func TestIsDiscovery_IgnoresPreference(t *testing.T) {
	// Novelty is preference-agnostic: a strongly disliked recipe the family has
	// never cooked is still a discovery candidate.
	c := domain.Candidate{MealieRecipeID: "r", Tags: []string{"pasta"}}
	ctx := favCtx("pasta", domain.Dislikes, 0)
	if !IsDiscovery(c, ctx) {
		t.Errorf("discovery should not depend on preference sentiment")
	}
}

func TestIsDiscovery_Deterministic(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := history("r")
	first := IsDiscovery(c, ctx)
	for i := range 5 {
		if again := IsDiscovery(c, ctx); again != first {
			t.Fatalf("run %d differs: %v vs %v", i, again, first)
		}
	}
}

func TestFamiliarityReason_NeverCooked(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	if got := familiarityReason(c, baseCtx(), 0); got != "novel (never cooked)" {
		t.Errorf("never-cooked reason = %q, want %q", got, "novel (never cooked)")
	}
}

func TestFamiliarityReason_RarelyCooked(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	if got := familiarityReason(c, history("r"), 1); got != "novel (rarely cooked)" {
		t.Errorf("rarely-cooked reason = %q, want %q", got, "novel (rarely cooked)")
	}
}

func TestFamiliarityReason_KnownFavorite(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r", Tags: []string{"pasta"}}
	if got := familiarityReason(c, favCtx("pasta", domain.Loves, 3), 3); got != "known favorite" {
		t.Errorf("favorite reason = %q, want %q", got, "known favorite")
	}
}

func TestFamiliarityReason_FamiliarNotFavorite(t *testing.T) {
	// Cooked regularly but disliked → familiar, not a known favorite.
	c := domain.Candidate{MealieRecipeID: "r", Tags: []string{"broccoli"}}
	if got := familiarityReason(c, favCtx("broccoli", domain.Hates, 3), 3); got != "familiar" {
		t.Errorf("familiar reason = %q, want %q", got, "familiar")
	}
}

func TestScore_ReasonStatesNoveltyWhenFeasible(t *testing.T) {
	// A feasible, never-cooked candidate's Reason states its novelty.
	c := domain.Candidate{MealieRecipeID: "r"}
	if got := score(c, baseCtx(), DefaultWeights()).Reason; got != "novel (never cooked)" {
		t.Errorf("feasible reason = %q, want %q", got, "novel (never cooked)")
	}
}

func TestScore_ReasonKeepsFeasibilityWhenInfeasible(t *testing.T) {
	// An infeasible candidate's Reason keeps the hard-constraint note AND the
	// novelty/familiarity classification.
	c := domain.Candidate{MealieRecipeID: "big", Effort: domain.EffortHigh}
	ctx := baseCtx()
	ctx.KitchenEnergy = domain.EffortLow // force infeasible
	got := score(c, ctx, DefaultWeights()).Reason
	want := "needs more effort than the cook has today; novel (never cooked)"
	if got != want {
		t.Errorf("infeasible reason = %q, want %q", got, want)
	}
}
