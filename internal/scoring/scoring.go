// Package scoring implements the deterministic meal scorer. It is a pure
// function of a candidate and a PlanContext: the same inputs always produce the
// same ranking, and it never calls out to an LLM or any network service. The
// LLM's role (varying candidates, writing explanations) happens elsewhere and
// can never change feasibility or the numeric score computed here.
package scoring

import (
	"math"
	"sort"

	"github.com/androidand/spisordning/internal/domain"
)

// Weights tune the relative pull of each signal. They live in a struct so a
// caller (or a test) can hold them fixed; DefaultWeights is the baseline.
type Weights struct {
	Preference  float64
	Effort      float64
	Repetition  float64
	SchoolDedup float64
	Campaign    float64
	// Familiarity weights the novelty/familiarity axis. Positive values pull
	// toward known, frequently-cooked favorites; negative values pull toward
	// discovery (recipes the household has rarely or never cooked). A mode
	// (safe choice / surprise me) is a deterministic re-weighting of this field.
	Familiarity float64
}

// DefaultWeights is the baseline tuning. Preference dominates; the others nudge.
// Familiarity is a moderate positive pull so the default leans slightly toward
// known favorites while still leaving room for discovery.
func DefaultWeights() Weights {
	return Weights{
		Preference:  1.0,
		Effort:      0.6,
		Repetition:  0.8,
		SchoolDedup: 0.7,
		Campaign:    0.4,
		Familiarity: 0.5,
	}
}

// Mode is a user-facing recommendation control mode. Each mode is a
// deterministic transformation of the scorer's Weights over the same underlying
// scorer - never a separate scoring algorithm and never an LLM decision. The
// four modes are the user controls PLAN.md's "Recommendation Inspiration"
// section names.
type Mode string

const (
	// ModeSafeChoice leans hard toward known, low-effort favorites and accepts
	// repeats.
	ModeSafeChoice Mode = "safe choice"
	// ModeSomethingSimilar leans toward what the household already likes and
	// cooks, with room for discovery. It is the default.
	ModeSomethingSimilar Mode = "something similar"
	// ModeSurpriseMe pulls toward novelty while keeping preference and
	// feasibility in play.
	ModeSurpriseMe Mode = "surprise me"
	// ModeCompletelyNew maximizes novelty, pulling hard toward candidates the
	// household has not cooked.
	ModeCompletelyNew Mode = "something completely new"
)

// DefaultMode is the mode used when the caller does not specify one. It is the
// "something similar" mode: a gentle lean toward known favorites that still
// leaves room for discovery.
const DefaultMode Mode = ModeSomethingSimilar

// Modes lists every supported mode in a stable order.
func Modes() []Mode {
	return []Mode{ModeSafeChoice, ModeSomethingSimilar, ModeSurpriseMe, ModeCompletelyNew}
}

// WeightsFor returns the deterministic Weights a mode applies, derived from
// DefaultWeights. Each mode is a pure re-weighting of the same signals; the
// underlying scorer and its feasibility rule are unchanged. An unknown mode
// falls back to DefaultWeights.
func (m Mode) WeightsFor() Weights {
	w := DefaultWeights()
	switch m {
	case ModeSafeChoice:
		w.Preference = 1.2
		w.Effort = 1.0
		w.Repetition = 0.3
		w.Familiarity = 1.0
	case ModeSomethingSimilar:
		w.Familiarity = 0.8
	case ModeSurpriseMe:
		w.Preference = 0.7
		w.Familiarity = -0.6
	case ModeCompletelyNew:
		w.Preference = 0.5
		w.Familiarity = -1.0
	}
	return w
}

// Breakdown is the per-signal contribution to a candidate's score, retained so
// the plan can be explained (by code or, later, by the LLM in prose).
type Breakdown struct {
	Preference  float64
	Effort      float64
	Repetition  float64
	SchoolDedup float64
	Campaign    float64
	Familiarity float64
}

// Total sums the breakdown.
func (b Breakdown) Total() float64 {
	return b.Preference + b.Effort + b.Repetition + b.SchoolDedup + b.Campaign + b.Familiarity
}

// ScoredCandidate is a candidate with its computed score and feasibility.
type ScoredCandidate struct {
	Candidate domain.Candidate
	Score     float64
	Breakdown Breakdown
	// Feasible is false when a hard constraint rules the candidate out (e.g. it
	// costs more effort than the cook has today). Infeasible candidates are
	// ranked last regardless of score.
	Feasible bool
	// Reason is a short machine-generated note; the LLM may replace it with prose.
	Reason string
}

// Rank scores every candidate against the context and returns them sorted best
// first. Feasible candidates always precede infeasible ones; within each group
// ordering is by descending score, with MealieRecipeID as a deterministic
// tie-breaker so the result is stable across runs.
func Rank(candidates []domain.Candidate, ctx domain.PlanContext, w Weights) []ScoredCandidate {
	out := make([]ScoredCandidate, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, score(c, ctx, w))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Feasible != out[j].Feasible {
			return out[i].Feasible // feasible first
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Candidate.MealieRecipeID < out[j].Candidate.MealieRecipeID
	})
	return out
}

// RankWithMode ranks candidates under the given control mode. It is the same
// deterministic Rank with the mode's weight transformation applied - the mode
// is a pure re-weighting, so feasibility and the underlying scorer are
// unchanged. An empty mode uses DefaultMode. The meal-planning API threads the
// user-selected mode here (see implement-meal-planning); until that lands the
// CLI passes DefaultMode.
func RankWithMode(candidates []domain.Candidate, ctx domain.PlanContext, m Mode) []ScoredCandidate {
	if m == "" {
		m = DefaultMode
	}
	return Rank(candidates, ctx, m.WeightsFor())
}

// SelectBatch returns the top n candidates from a mode-ranked list, enforcing
// the balance guarantee: when the pool contains both known favorites and
// discovery candidates, the batch includes at least one of each. It reorders
// (never re-scores) the ranked candidates, so the deterministic ranking is
// preserved and only the batch composition is adjusted. n <= 0 returns nil;
// n >= len(ranked) returns the full ranked list; a 1-slot batch cannot hold a
// mix and is returned as-is.
func SelectBatch(ranked []ScoredCandidate, ctx domain.PlanContext, n int) []ScoredCandidate {
	if n <= 0 || len(ranked) == 0 {
		return nil
	}
	if n >= len(ranked) {
		return ranked
	}
	batch := append([]ScoredCandidate(nil), ranked[:n]...)
	if n < 2 {
		return batch
	}
	rest := ranked[n:]

	batchFav, batchNovel := groupsPresent(batch, ctx)
	poolFav, poolNovel := groupsPresent(ranked, ctx)

	// slot is the next lowest-ranked batch slot to replace.
	slot := len(batch) - 1
	swapIn := func(pred func(ScoredCandidate) bool) {
		for _, sc := range rest {
			if pred(sc) {
				batch[slot] = sc
				slot--
				return
			}
		}
	}
	if poolFav && !batchFav {
		swapIn(func(sc ScoredCandidate) bool { return IsKnownFavorite(sc.Candidate, ctx) })
	}
	if poolNovel && !batchNovel {
		swapIn(func(sc ScoredCandidate) bool { return IsDiscovery(sc.Candidate, ctx) })
	}
	return batch
}

// groupsPresent reports whether the given candidates include at least one known
// favorite and/or at least one discovery candidate.
func groupsPresent(cands []ScoredCandidate, ctx domain.PlanContext) (fav, novel bool) {
	for _, sc := range cands {
		if !fav && IsKnownFavorite(sc.Candidate, ctx) {
			fav = true
		}
		if !novel && IsDiscovery(sc.Candidate, ctx) {
			novel = true
		}
		if fav && novel {
			return true, true
		}
	}
	return fav, novel
}

func score(c domain.Candidate, ctx domain.PlanContext, w Weights) ScoredCandidate {
	// The meal-history count is scanned once per candidate and shared by every
	// familiarity signal, rather than each helper re-scanning RecentMealIDs.
	cooks := cookCount(c, ctx)
	b := Breakdown{
		Preference:  w.Preference * preferenceScore(c, ctx),
		Effort:      w.Effort * effortScore(c, ctx),
		Repetition:  w.Repetition * repetitionPenalty(c, ctx),
		SchoolDedup: w.SchoolDedup * schoolDedupPenalty(c, ctx),
		Campaign:    w.Campaign * campaignBonus(c, ctx),
		Familiarity: w.Familiarity * familiarityScore(c, ctx, cooks),
	}

	feasible, reason := feasibility(c, ctx)
	// The Reason states the candidate's novelty/familiarity classification (the
	// explainable face of the Familiarity dimension); when infeasible it is
	// prefixed with the hard-constraint note so both are visible.
	fullReason := familiarityReason(c, ctx, cooks)
	if !feasible {
		fullReason = reason + "; " + fullReason
	}
	return ScoredCandidate{
		Candidate: c,
		Score:     b.Total(),
		Breakdown: b,
		Feasible:  feasible,
		Reason:    fullReason,
	}
}

// feasibility applies hard constraints. Today the only hard rule is that a meal
// may not cost more active effort than the cook has budgeted. Feasibility is
// separate from score so the LLM layer can be asserted never to override it.
func feasibility(c domain.Candidate, ctx domain.PlanContext) (bool, string) {
	if ctx.KitchenEnergy > 0 && c.Effort > ctx.KitchenEnergy {
		return false, "needs more effort than the cook has today"
	}
	return true, "ok"
}

// preferenceScore is the family-aggregate sentiment toward the candidate's
// tags, weighted by each person's confidence and weight, normalized so it does
// not scale with family size or tag count.
func preferenceScore(c domain.Candidate, ctx domain.PlanContext) float64 {
	tagSet := toSet(c.Tags)
	if len(tagSet) == 0 || len(ctx.People) == 0 {
		return 0
	}

	// Index preferences by person then tag for O(1) lookup.
	byPerson := map[string]map[string]domain.Preference{}
	for _, p := range ctx.Preferences {
		if _, ok := byPerson[p.PersonID]; !ok {
			byPerson[p.PersonID] = map[string]domain.Preference{}
		}
		byPerson[p.PersonID][p.Tag] = p
	}

	var weightedSum, weightTotal float64
	for _, person := range ctx.People {
		prefs := byPerson[person.ID]
		var personScore float64
		var matched int
		for tag := range tagSet {
			if pref, ok := prefs[tag]; ok {
				personScore += float64(pref.Sentiment) * clamp01(pref.Confidence)
				matched++
			}
		}
		if matched > 0 {
			personScore /= float64(matched) // average over matched tags
		}
		pw := person.EffectiveWeight()
		weightedSum += personScore * pw
		weightTotal += pw
	}
	if weightTotal == 0 {
		return 0
	}
	return weightedSum / weightTotal
}

// effortScore rewards leaving effort headroom: a low-effort meal on a
// low-energy day scores best. Returns 0 when energy is unspecified.
func effortScore(c domain.Candidate, ctx domain.PlanContext) float64 {
	if ctx.KitchenEnergy <= 0 {
		return 0
	}
	// Headroom in [0, 2]; scale to [0, 1].
	headroom := float64(ctx.KitchenEnergy - c.Effort)
	return headroom / 2.0
}

// repetitionPenalty returns a non-positive value that grows toward zero as the
// last time this recipe was served recedes. A repeat within the last week is
// penalized; older repeats fade out linearly to zero at 14 days.
func repetitionPenalty(c domain.Candidate, ctx domain.PlanContext) float64 {
	const window = 14.0
	var worst float64
	for _, m := range ctx.RecentMealIDs {
		if m.MealieRecipeID != c.MealieRecipeID {
			continue
		}
		days := ctx.Day.Sub(m.Served).Hours() / 24.0
		if days < 0 {
			days = 0
		}
		if days >= window {
			continue
		}
		penalty := -(window - days) / window // -1 (today) .. 0 (14d ago)
		if penalty < worst {
			worst = penalty
		}
	}
	return worst
}

// schoolDedupPenalty penalizes a dinner sharing tags with today's school lunch,
// scaled by the fraction of the candidate's tags that overlap.
func schoolDedupPenalty(c domain.Candidate, ctx domain.PlanContext) float64 {
	if len(ctx.SchoolLunchTags) == 0 || len(c.Tags) == 0 {
		return 0
	}
	school := toSet(ctx.SchoolLunchTags)
	var overlap int
	for _, tag := range c.Tags {
		if school[tag] {
			overlap++
		}
	}
	if overlap == 0 {
		return 0
	}
	return -float64(overlap) / float64(len(c.Tags))
}

// campaignBonus rewards candidates whose ingredients are on campaign at the
// family's store this week, scaled by the fraction on sale.
func campaignBonus(c domain.Candidate, ctx domain.PlanContext) float64 {
	if len(ctx.CampaignIngredients) == 0 || len(c.Ingredients) == 0 {
		return 0
	}
	var onSale int
	for _, ing := range c.Ingredients {
		if ctx.CampaignIngredients[ing] {
			onSale++
		}
	}
	return float64(onSale) / float64(len(c.Ingredients))
}

// cookCount is how many times the recipe appears in the household's meal history.
func cookCount(c domain.Candidate, ctx domain.PlanContext) int {
	n := 0
	for _, m := range ctx.RecentMealIDs {
		if m.MealieRecipeID == c.MealieRecipeID {
			n++
		}
	}
	return n
}

// familiarityScore is the novelty/familiarity axis, derived purely from the
// household's meal history (how often this recipe has been cooked). It returns
// a value in [-1, 1]:
//
//	+1  heavily cooked — a known, familiar favorite
//	 0  neutral / no basis (no meal history at all)
//	-1  never cooked — a discovery candidate
//
// cooks is the precomputed cookCount for the candidate. It is separate from
// the Preference dimension, which captures *sentiment*; this dimension captures
// *frequency*. A positive Familiarity weight therefore pulls toward known
// favorites, while a negative weight pulls toward discovery. It returns 0 when
// there is no meal history to base the signal on, so a candidate in an
// otherwise-empty context still scores 0 (see TestRank_EmptyInputs).
func familiarityScore(c domain.Candidate, ctx domain.PlanContext, cooks int) float64 {
	if len(ctx.RecentMealIDs) == 0 {
		return 0
	}
	// Saturating familiarity in [0,1]: 0 when never cooked, approaching 1 as the
	// recipe is cooked more. k=3 makes a thrice-cooked recipe feel "familiar".
	const k = 3.0
	fam := float64(cooks) / (float64(cooks) + k)
	// Center on 0 → [-1, 1].
	return 2*fam - 1
}

// favoriteMinCooks is the minimum number of times a recipe must appear in the
// household's meal history before it can count as a "known" favorite. A recipe
// the family loves but has never (or barely) cooked is a prospect, not a known
// favorite — "known" means the household has actually cooked it.
const favoriteMinCooks = 2

// IsKnownFavorite reports whether the recipe is a known household favorite. It is
// a deterministic rule over the two signals PLAN.md's Recommendation Domain names
// for "known favorites": the family's aggregate confidence-weighted preference
// (sentiment) and the recipe's meal-history frequency. Both must hold:
//
//   - the family's net sentiment toward the recipe's tags is positive, and
//   - the recipe has been cooked at least favoriteMinCooks times.
//
// A frequently-cooked recipe the family dislikes is a habit, not a favorite; a
// well-liked recipe that has never been cooked is a prospect, not yet known.
func IsKnownFavorite(c domain.Candidate, ctx domain.PlanContext) bool {
	return isKnownFavorite(c, ctx, cookCount(c, ctx))
}

func isKnownFavorite(c domain.Candidate, ctx domain.PlanContext, cooks int) bool {
	return cooks >= favoriteMinCooks && preferenceScore(c, ctx) > 0
}

// discoveryMaxCooks is the maximum number of times a recipe may appear in the
// household's meal history before it stops counting as a discovery/novel
// candidate. A recipe the family has never (or barely) cooked is novel — a
// candidate for discovery; one it has cooked regularly is familiar, not new.
// It is the mirror of favoriteMinCooks: a recipe is either still novel
// (cooked <= discoveryMaxCooks times) or no longer is (cooked >=
// favoriteMinCooks times), with the two thresholds meeting cleanly.
const discoveryMaxCooks = 1

// IsDiscovery reports whether the recipe is a discovery/novel candidate: one
// the household has no or minimal meal history for (cooked at most
// discoveryMaxCooks times). This is the novelty side of PLAN.md's
// Recommendation Inspiration balance — the counterpart to IsKnownFavorite.
//
// The rule is deliberately preference-agnostic: a recipe is novel because the
// family has not yet cooked it, regardless of how much they like its tags. A
// well-liked recipe that has never been cooked is a discovery prospect, not a
// known favorite. (A future recipe-discovery capability may also flag freshly
// added external recipes as discovery; that is out of scope here and would OR
// into this rule.)
func IsDiscovery(c domain.Candidate, ctx domain.PlanContext) bool {
	return isDiscovery(cookCount(c, ctx))
}

func isDiscovery(cooks int) bool {
	return cooks <= discoveryMaxCooks
}

// familiarityReason returns a short, deterministic phrase explaining the
// candidate's position on the novelty/familiarity axis, in the same
// machine-generated-note shape as the feasibility reason. It is derived purely
// from meal history and preference data — never the LLM — so the novelty
// dimension is explainable in the same Breakdown/Reason shape as every other
// signal the scorer already produces. cooks is the precomputed cookCount.
func familiarityReason(c domain.Candidate, ctx domain.PlanContext, cooks int) string {
	switch {
	case isDiscovery(cooks):
		if cooks == 0 {
			return "novel (never cooked)"
		}
		return "novel (rarely cooked)"
	case isKnownFavorite(c, ctx, cooks):
		return "known favorite"
	default:
		return "familiar"
	}
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, it := range items {
		s[it] = true
	}
	return s
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}
