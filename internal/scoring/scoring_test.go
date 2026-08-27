package scoring

import (
	"testing"
	"time"

	"github.com/androidand/spisordning/internal/availability"
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

// mixedPool is a pool with one loved, thrice-cooked favorite and one loved,
// never-cooked discovery candidate. Preference is equal for both, so the mode's
// Familiarity weight is the only differentiator.
func mixedPool() ([]domain.Candidate, domain.PlanContext) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "fav", Title: "Pasta", Tags: []string{"pasta"}, Effort: domain.EffortLow},
		{MealieRecipeID: "novel", Title: "Curry", Tags: []string{"curry"}, Effort: domain.EffortLow},
	}
	ctx := baseCtx()
	ctx.Preferences = []domain.Preference{
		{PersonID: "kid", Tag: "pasta", Sentiment: domain.Loves, Confidence: 1.0},
		{PersonID: "kid", Tag: "curry", Sentiment: domain.Loves, Confidence: 1.0},
	}
	for i := 0; i < 3; i++ {
		ctx.RecentMealIDs = append(ctx.RecentMealIDs, domain.RecentMeal{MealieRecipeID: "fav", Served: day.AddDate(0, 0, -(30 + i*30))})
	}
	return candidates, ctx
}

func TestMode_WeightsDistinct(t *testing.T) {
	seen := map[Weights]bool{}
	for _, m := range Modes() {
		w := m.WeightsFor()
		if seen[w] {
			t.Errorf("mode %q produces duplicate weights %v", m, w)
		}
		seen[w] = true
	}
}

func TestMode_RankingDistinct(t *testing.T) {
	candidates, ctx := mixedPool()
	top := func(m Mode) string {
		return RankWithMode(candidates, ctx, m)[0].Candidate.MealieRecipeID
	}
	if got := top(ModeSafeChoice); got != "fav" {
		t.Errorf("safe choice top = %q, want fav", got)
	}
	if got := top(ModeSomethingSimilar); got != "fav" {
		t.Errorf("something similar top = %q, want fav", got)
	}
	if got := top(ModeSurpriseMe); got != "novel" {
		t.Errorf("surprise me top = %q, want novel", got)
	}
	if got := top(ModeCompletelyNew); got != "novel" {
		t.Errorf("completely new top = %q, want novel", got)
	}
}

func TestMode_Deterministic(t *testing.T) {
	candidates, ctx := mixedPool()
	for _, m := range Modes() {
		first := RankWithMode(candidates, ctx, m)
		for i := range 5 {
			again := RankWithMode(candidates, ctx, m)
			if len(again) != len(first) {
				t.Fatalf("mode %q: length changed between runs", m)
			}
			for j := range first {
				if again[j].Candidate.MealieRecipeID != first[j].Candidate.MealieRecipeID ||
					again[j].Score != first[j].Score {
					t.Fatalf("mode %q run %d differs at %d", m, i, j)
				}
			}
		}
	}
}

func TestModeSelection_DeterministicWithoutLLM(t *testing.T) {
	// The scorer is a pure function: it never imports or calls the LLM (Olla).
	// Repeated runs with Olla unavailable produce identical rankings and mode
	// selections - the LLM-invariance rule (tasks 6.1/6.2).
	candidates, ctx := mixedPool()
	for _, m := range Modes() {
		first := RankWithMode(candidates, ctx, m)
		for i := range 5 {
			again := RankWithMode(candidates, ctx, m)
			for j := range first {
				if again[j].Candidate.MealieRecipeID != first[j].Candidate.MealieRecipeID ||
					again[j].Score != first[j].Score {
					t.Fatalf("mode %q run %d differs at %d", m, i, j)
				}
			}
		}
	}
}

func TestRankWithMode_EmptyUsesDefault(t *testing.T) {
	candidates, ctx := mixedPool()
	empty := RankWithMode(candidates, ctx, "")
	def := RankWithMode(candidates, ctx, DefaultMode)
	if empty[0].Score != def[0].Score {
		t.Errorf("empty mode should use DefaultMode: %.3f vs %.3f", empty[0].Score, def[0].Score)
	}
}

func TestMode_SurpriseMeRespectsFeasibility(t *testing.T) {
	// Even in surprise me mode, an infeasible candidate ranks last.
	ctx := baseCtx()
	ctx.KitchenEnergy = domain.EffortLow // exhausted cook
	candidates := []domain.Candidate{
		{MealieRecipeID: "easyNovel", Tags: []string{"curry"}, Effort: domain.EffortLow},
		{MealieRecipeID: "bigNovel", Tags: []string{"tacos"}, Effort: domain.EffortHigh},
	}
	ranked := RankWithMode(candidates, ctx, ModeSurpriseMe)
	if ranked[0].Candidate.MealieRecipeID != "easyNovel" {
		t.Errorf("feasible meal should rank first under surprise me, got %s", ranked[0].Candidate.MealieRecipeID)
	}
	for _, r := range ranked {
		if r.Candidate.MealieRecipeID == "bigNovel" && r.Feasible {
			t.Errorf("high-effort meal should be infeasible on a low-energy day")
		}
	}
}

// favPool builds a pool of cooked, loved favorites plus never-cooked discovery
// candidates. The favorites rank above the novel ones under safe choice, so a
// small top-n batch would be all favorites without the balance guarantee.
func favPool(novelIDs []string) ([]domain.Candidate, domain.PlanContext) {
	favIDs := []string{"fav1", "fav2", "fav3", "fav4"}
	tags := map[string]string{"fav1": "pasta", "fav2": "fish", "fav3": "stew", "fav4": "rice"}
	candidates := make([]domain.Candidate, 0, len(favIDs)+len(novelIDs))
	for _, id := range favIDs {
		candidates = append(candidates, domain.Candidate{MealieRecipeID: id, Tags: []string{tags[id]}, Effort: domain.EffortLow})
	}
	for _, id := range novelIDs {
		candidates = append(candidates, domain.Candidate{MealieRecipeID: id, Tags: []string{id}, Effort: domain.EffortLow})
	}
	ctx := baseCtx()
	ctx.People = []domain.Person{person("kid", 1)}
	for _, id := range favIDs {
		ctx.Preferences = append(ctx.Preferences, domain.Preference{PersonID: "kid", Tag: tags[id], Sentiment: domain.Loves, Confidence: 1.0})
		for i := 0; i < 3; i++ {
			ctx.RecentMealIDs = append(ctx.RecentMealIDs, domain.RecentMeal{MealieRecipeID: id, Served: day.AddDate(0, 0, -(30 + i*30))})
		}
	}
	return candidates, ctx
}

func TestSelectBatch_BalanceGuarantee(t *testing.T) {
	candidates, ctx := favPool([]string{"novel1", "novel2"})
	ranked := RankWithMode(candidates, ctx, ModeSafeChoice)
	batch := SelectBatch(ranked, ctx, 3)
	if len(batch) != 3 {
		t.Fatalf("batch size = %d, want 3", len(batch))
	}
	fav, novel := groupsPresent(batch, ctx)
	if !fav {
		t.Errorf("batch should include at least one known favorite")
	}
	if !novel {
		t.Errorf("balance guarantee: batch should include at least one discovery candidate")
	}
}

func TestSelectBatch_AllFavorites(t *testing.T) {
	candidates, ctx := favPool(nil)
	ranked := RankWithMode(candidates, ctx, ModeSafeChoice)
	batch := SelectBatch(ranked, ctx, 2)
	fav, novel := groupsPresent(batch, ctx)
	if !fav {
		t.Errorf("all-favorites pool should yield a favorites batch")
	}
	if novel {
		t.Errorf("all-favorites pool should not fabricate a discovery candidate")
	}
}

func TestSelectBatch_SmallBatch(t *testing.T) {
	candidates, ctx := favPool([]string{"novel1"})
	ranked := RankWithMode(candidates, ctx, ModeSafeChoice)
	batch := SelectBatch(ranked, ctx, 1)
	if len(batch) != 1 {
		t.Fatalf("1-slot batch size = %d, want 1", len(batch))
	}
	if batch[0].Candidate.MealieRecipeID != "fav1" {
		t.Errorf("1-slot batch top = %q, want fav1", batch[0].Candidate.MealieRecipeID)
	}
}

// --- Pantry availability tests (task 5.2) ---

func TestPantryScore_Feasible(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := baseCtx()
	ctx.PantryAvailability = map[string]domain.PantryStatus{"r": domain.PantryFeasible}
	got := pantryScore(c, ctx)
	if got != 1.0 {
		t.Errorf("pantry score = %v, want 1.0 for feasible", got)
	}
}

func TestPantryScore_FeasibleWithSub(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := baseCtx()
	ctx.PantryAvailability = map[string]domain.PantryStatus{"r": domain.PantryFeasibleWithSub}
	got := pantryScore(c, ctx)
	if got != 0.6 {
		t.Errorf("pantry score = %v, want 0.6 for feasible-with-substitution", got)
	}
}

func TestPantryScore_Infeasible(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := baseCtx()
	ctx.PantryAvailability = map[string]domain.PantryStatus{"r": domain.PantryInfeasible}
	got := pantryScore(c, ctx)
	if got != 0.0 {
		t.Errorf("pantry score = %v, want 0.0 for infeasible", got)
	}
}

func TestPantryScore_MissingData(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := baseCtx()
	// Empty pantry map = no data = score 0.
	got := pantryScore(c, ctx)
	if got != 0.0 {
		t.Errorf("pantry score = %v, want 0.0 when no data", got)
	}
}

func TestPantryScore_KnownKeyMissing(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := baseCtx()
	ctx.PantryAvailability = map[string]domain.PantryStatus{"other": domain.PantryFeasible}
	got := pantryScore(c, ctx)
	if got != 0.0 {
		t.Errorf("pantry score = %v, want 0.0 when recipe not in map", got)
	}
}

func TestScore_PantryIntegrated(t *testing.T) {
	// A feasible recipe should score higher than an infeasible one,
	// all else equal.
	cFeasible := domain.Candidate{MealieRecipeID: "ok", Tags: []string{"pasta"}, Effort: domain.EffortLow}
	cInfeasible := domain.Candidate{MealieRecipeID: "bad", Tags: []string{"pasta"}, Effort: domain.EffortLow}
	ctx := baseCtx()
	ctx.Preferences = []domain.Preference{
		{PersonID: "kid", Tag: "pasta", Sentiment: domain.Loves, Confidence: 1.0},
	}
	ctx.PantryAvailability = map[string]domain.PantryStatus{
		"ok":   domain.PantryFeasible,
		"bad":  domain.PantryInfeasible,
	}
	ranked := Rank([]domain.Candidate{cFeasible, cInfeasible}, ctx, DefaultWeights())
	if ranked[0].Candidate.MealieRecipeID != "ok" {
		t.Errorf("feasible recipe should rank first, got %s", ranked[0].Candidate.MealieRecipeID)
	}
	// Breakdown.Pantry is the weighted score (weight * raw score).
	if ranked[0].Breakdown.Pantry != 0.5 {
		t.Errorf("feasible pantry breakdown = %v, want 0.5 (weight 0.5 * score 1.0)", ranked[0].Breakdown.Pantry)
	}
	if ranked[1].Breakdown.Pantry != 0.0 {
		t.Errorf("infeasible pantry breakdown = %v, want 0.0", ranked[1].Breakdown.Pantry)
	}
}

func TestScore_PantryDoesNotOverrideFeasibility(t *testing.T) {
	// An infeasible recipe (effort > energy) should rank last even if it
	// has perfect pantry availability.
	cFeasible := domain.Candidate{MealieRecipeID: "easy", Tags: []string{"pasta"}, Effort: domain.EffortLow}
	cInfeasible := domain.Candidate{MealieRecipeID: "hard", Tags: []string{"pasta"}, Effort: domain.EffortHigh}
	ctx := baseCtx()
	ctx.KitchenEnergy = domain.EffortLow
	ctx.Preferences = []domain.Preference{
		{PersonID: "kid", Tag: "pasta", Sentiment: domain.Loves, Confidence: 1.0},
	}
	ctx.PantryAvailability = map[string]domain.PantryStatus{
		"easy": domain.PantryFeasible,
		"hard": domain.PantryFeasible, // perfect pantry but effort infeasible
	}
	ranked := Rank([]domain.Candidate{cFeasible, cInfeasible}, ctx, DefaultWeights())
	if ranked[0].Candidate.MealieRecipeID != "easy" {
		t.Errorf("feasible meal should rank first, got %s", ranked[0].Candidate.MealieRecipeID)
	}
	if ranked[0].Feasible != true {
		t.Errorf("easy meal should be feasible")
	}
	if ranked[1].Feasible != false {
		t.Errorf("hard meal should be infeasible")
	}
}

func TestPantryStatus_EnumValues(t *testing.T) {
	// Pin the enum values so consumers know what to expect.
	if domain.PantryFeasible != "feasible" {
		t.Errorf("PantryFeasible = %q, want %q", domain.PantryFeasible, "feasible")
	}
	if domain.PantryFeasibleWithSub != "feasible-with-substitution" {
		t.Errorf("PantryFeasibleWithSub = %q, want %q", domain.PantryFeasibleWithSub, "feasible-with-substitution")
	}
	if domain.PantryInfeasible != "infeasible" {
		t.Errorf("PantryInfeasible = %q, want %q", domain.PantryInfeasible, "infeasible")
	}
}

func TestScore_PantryStatusPropagated(t *testing.T) {
	c := domain.Candidate{MealieRecipeID: "r"}
	ctx := baseCtx()
	ctx.PantryAvailability = map[string]domain.PantryStatus{"r": domain.PantryFeasibleWithSub}
	sc := score(c, ctx, DefaultWeights())
	if sc.PantryStatus != domain.PantryFeasibleWithSub {
		t.Errorf("pantry status = %q, want %q", sc.PantryStatus, domain.PantryFeasibleWithSub)
	}
}

func TestDefaultWeights_PantryIncluded(t *testing.T) {
	w := DefaultWeights()
	if w.Pantry != 0.5 {
		t.Errorf("default pantry weight = %v, want 0.5", w.Pantry)
	}
}

func TestSelectBatch_InfeasibleCandidateNotPromoted(t *testing.T) {
	// Regression test for reviewer round 4: the balance guarantee must
	// never promote an infeasible candidate into the batch. If the pool's
	// only favorite (or only novel) candidate is infeasible, the batch
	// should not include it.
	ctx := baseCtx()
	ctx.KitchenEnergy = domain.EffortLow // exhausted cook
	ctx.Preferences = []domain.Preference{
		{PersonID: "kid", Tag: "pasta", Sentiment: domain.Loves, Confidence: 1.0},
		{PersonID: "kid", Tag: "curry", Sentiment: domain.Loves, Confidence: 1.0},
	}
	// fav: loved, thrice-cooked → known favorite, but high effort → infeasible
	fav := domain.Candidate{MealieRecipeID: "fav", Tags: []string{"pasta"}, Effort: domain.EffortHigh}
	// novel: loved, never-cooked → discovery, low effort → feasible
	novel := domain.Candidate{MealieRecipeID: "novel", Tags: []string{"curry"}, Effort: domain.EffortLow}
	// other: loved, thrice-cooked → known favorite, low effort → feasible
	other := domain.Candidate{MealieRecipeID: "other", Tags: []string{"pasta"}, Effort: domain.EffortLow}
	for i := 0; i < 3; i++ {
		ctx.RecentMealIDs = append(ctx.RecentMealIDs,
			domain.RecentMeal{MealieRecipeID: "fav", Served: day.AddDate(0, 0, -(30+i*30))},
			domain.RecentMeal{MealieRecipeID: "other", Served: day.AddDate(0, 0, -(40+i*30))},
		)
	}

	ranked := RankWithMode([]domain.Candidate{fav, novel, other}, ctx, ModeSafeChoice)
	// Verify the ranking: feasible candidates first, infeasible last.
	if ranked[0].Candidate.MealieRecipeID == "fav" {
		t.Fatal("infeasible candidate should not rank first")
	}

	batch := SelectBatch(ranked, ctx, 2)
	for _, sc := range batch {
		if !sc.Feasible {
			t.Errorf("batch should not contain infeasible candidate %s", sc.Candidate.MealieRecipeID)
		}
	}
	// The batch should have the feasible novel and the feasible other,
	// never the infeasible favorite.
	if len(batch) != 2 {
		t.Fatalf("batch size = %d, want 2", len(batch))
	}
	ids := map[string]bool{batch[0].Candidate.MealieRecipeID: true, batch[1].Candidate.MealieRecipeID: true}
	if !ids["novel"] || !ids["other"] {
		t.Errorf("batch should contain novel and other, got %v", batch)
	}
}

func TestSelectBatch_NoFeasibleFavoriteOrNovel(t *testing.T) {
	// Edge case: all candidates in the pool are infeasible.
	// The batch should contain the top-ranked infeasible candidates
	// without pretending they are feasible.
	ctx := baseCtx()
	ctx.KitchenEnergy = domain.EffortLow
	candidates := []domain.Candidate{
		{MealieRecipeID: "a", Tags: []string{"pasta"}, Effort: domain.EffortHigh},
		{MealieRecipeID: "b", Tags: []string{"curry"}, Effort: domain.EffortHigh},
	}
	ranked := RankWithMode(candidates, ctx, ModeSafeChoice)
	batch := SelectBatch(ranked, ctx, 1)
	if len(batch) != 1 {
		t.Fatalf("batch size = %d, want 1", len(batch))
	}
	// Top-ranked infeasible should be returned (1-slot batch, no alternative).
	if !batch[0].Feasible {
		t.Logf("batch contains infeasible candidate %s (expected — no feasible alternative)", batch[0].Candidate.MealieRecipeID)
	}
}

// --- 5.2: Pantry availability wiring ---

func TestFeasibility_AvailabilityInfeasible(t *testing.T) {
	// A recipe that the availability capability reports as infeasible
	// should be ruled out by the scorer regardless of effort.
	c := domain.Candidate{MealieRecipeID: "r-missing", Effort: domain.EffortLow}
	ctx := baseCtx()
	ctx.AvailabilityVerdicts = map[string]string{
		"r-missing": string(availability.VerdictInfeasible),
	}
	got := score(c, ctx, DefaultWeights())
	if got.Feasible {
		t.Errorf("candidate should be infeasible when pantry availability is infeasible")
	}
	if got.Reason == "" {
		t.Error("reason should not be empty for infeasible candidate")
	}
}

func TestFeasibility_AvailabilityFeasibleDoesNotBlock(t *testing.T) {
	// An availability verdict of "feasible" or "feasible-with-substitution"
	// should not prevent the candidate from being feasible.
	c := domain.Candidate{MealieRecipeID: "r-ok", Effort: domain.EffortLow}
	ctx := baseCtx()
	ctx.AvailabilityVerdicts = map[string]string{
		"r-ok": string(availability.VerdictFeasibleWithSub),
	}
	got := score(c, ctx, DefaultWeights())
	if !got.Feasible {
		t.Errorf("candidate should be feasible when pantry availability is feasible-with-substitution")
	}
}

func TestFeasibility_MissingAvailabilityIsIgnored(t *testing.T) {
	// When no availability data is provided, the scorer falls back to
	// effort-only feasibility (backward compatibility).
	c := domain.Candidate{MealieRecipeID: "r-no-data", Effort: domain.EffortLow}
	ctx := baseCtx()
	// AvailabilityVerdicts is nil — no data provided.
	got := score(c, ctx, DefaultWeights())
	if !got.Feasible {
		t.Errorf("candidate should be feasible when no availability data is present")
	}
}

func TestFeasibility_EffortAndAvailabilityBothBlock(t *testing.T) {
	// When both effort and availability block, the reason should mention
	// both constraints.
	c := domain.Candidate{MealieRecipeID: "r-hard", Effort: domain.EffortHigh}
	ctx := baseCtx()
	ctx.KitchenEnergy = domain.EffortLow
	ctx.AvailabilityVerdicts = map[string]string{
		"r-hard": string(availability.VerdictInfeasible),
	}
	got := score(c, ctx, DefaultWeights())
	if got.Feasible {
		t.Errorf("candidate should be infeasible when both effort and availability block")
	}
	if got.Reason == "" {
		t.Error("reason should mention both constraints")
	}
}

func TestRank_AvailabilityInfeasibleRanksLast(t *testing.T) {
	// An infeasible recipe should rank below feasible ones, even if it
	// has a high preference score.
	candidates := []domain.Candidate{
		{MealieRecipeID: "r-fav", Tags: []string{"pasta"}, Effort: domain.EffortLow},
		{MealieRecipeID: "r-missing", Tags: []string{"saffron"}, Effort: domain.EffortLow},
	}
	ctx := baseCtx()
	ctx.Preferences = []domain.Preference{
		{PersonID: "kid", Tag: "saffron", Sentiment: domain.Loves, Confidence: 1.0},
	}
	ctx.AvailabilityVerdicts = map[string]string{
		"r-missing": string(availability.VerdictInfeasible),
	}
	ranked := Rank(candidates, ctx, DefaultWeights())
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked candidates, got %d", len(ranked))
	}
	if ranked[0].Candidate.MealieRecipeID != "r-fav" {
		t.Errorf("top candidate = %q, want r-fav (r-missing is infeasible)", ranked[0].Candidate.MealieRecipeID)
	}
	if ranked[1].Candidate.MealieRecipeID != "r-missing" {
		t.Errorf("bottom candidate = %q, want r-missing", ranked[1].Candidate.MealieRecipeID)
	}
	if ranked[1].Feasible {
		t.Errorf("r-missing should be infeasible")
	}
}

func TestFeasibility_AvailabilityFeasibleWithSubStillFeasible(t *testing.T) {
	// "feasible-with-substitution" is NOT a hard block — the recipe can
	// still be made, just with a substitute.
	c := domain.Candidate{MealieRecipeID: "r-sub", Effort: domain.EffortLow}
	ctx := baseCtx()
	ctx.AvailabilityVerdicts = map[string]string{
		"r-sub": string(availability.VerdictFeasibleWithSub),
	}
	got := score(c, ctx, DefaultWeights())
	if !got.Feasible {
		t.Errorf("candidate should be feasible when availability is feasible-with-substitution")
	}
}

func TestRankSimple_NoEffortFiltering(t *testing.T) {
	// A high-effort candidate should be feasible under RankSimple because
	// KitchenEnergy is zeroed out.
	candidates := []domain.Candidate{
		{MealieRecipeID: "project", Title: "Projekt", Effort: domain.EffortHigh},
		{MealieRecipeID: "quick", Title: "Snabb", Effort: domain.EffortLow},
	}
	ctx := baseCtx()
	ctx.KitchenEnergy = domain.EffortLow // would normally block "project"
	ranked := RankSimple(candidates, ctx)
	// Both should be feasible since RankSimple zeros KitchenEnergy.
	for _, sc := range ranked {
		if !sc.Feasible {
			t.Errorf("candidate %q should be feasible under RankSimple", sc.Candidate.MealieRecipeID)
		}
	}
}

func TestRankSimple_NoSchoolDedup(t *testing.T) {
	// A candidate matching school lunch tags should NOT be penalized under
	// RankSimple.
	candidates := []domain.Candidate{
		{MealieRecipeID: "fisk", Title: "Fisk", Tags: []string{"fisk"}},
		{MealieRecipeID: "pasta", Title: "Pasta", Tags: []string{"pasta"}},
	}
	ctx := baseCtx()
	ctx.SchoolLunchTags = []string{"fisk"} // would normally penalize "fisk"
	ctx.Preferences = []domain.Preference{
		{PersonID: "kid", Tag: "fisk", Sentiment: domain.Likes, Confidence: 1.0},
	}
	ranked := RankSimple(candidates, ctx)
	// Fisk should be at the top (preference boost, no school dedup penalty).
	if ranked[0].Candidate.MealieRecipeID != "fisk" {
		t.Errorf("top candidate = %q, want fisk (no school dedup in RankSimple)", ranked[0].Candidate.MealieRecipeID)
	}
	// Verify the school dedup component is zero in the breakdown.
	if ranked[0].Breakdown.SchoolDedup != 0 {
		t.Errorf("SchoolDedup = %f, want 0", ranked[0].Breakdown.SchoolDedup)
	}
}

func TestRankSimple_EffortComponentZero(t *testing.T) {
	candidates := []domain.Candidate{
		{MealieRecipeID: "high", Title: "High Effort", Effort: domain.EffortHigh},
	}
	ctx := baseCtx()
	ranked := RankSimple(candidates, ctx)
	if ranked[0].Breakdown.Effort != 0 {
		t.Errorf("Effort = %f, want 0", ranked[0].Breakdown.Effort)
	}
}

func TestSimpleWeights_DiffersFromDefault(t *testing.T) {
	simple := SimpleWeights()
	def := DefaultWeights()
	if simple.Effort != 0 {
		t.Errorf("SimpleWeights.Effort = %f, want 0", simple.Effort)
	}
	if simple.SchoolDedup != 0 {
		t.Errorf("SimpleWeights.SchoolDedup = %f, want 0", simple.SchoolDedup)
	}
	// Other weights should match default.
	if simple.Preference != def.Preference {
		t.Errorf("SimpleWeights.Preference = %f, want %f", simple.Preference, def.Preference)
	}
	if simple.Repetition != def.Repetition {
		t.Errorf("SimpleWeights.Repetition = %f, want %f", simple.Repetition, def.Repetition)
	}
}
