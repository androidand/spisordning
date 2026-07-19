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
	Preference float64
	Effort     float64
	Repetition float64
	SchoolDedup float64
	Campaign   float64
}

// DefaultWeights is the baseline tuning. Preference dominates; the others nudge.
func DefaultWeights() Weights {
	return Weights{
		Preference:  1.0,
		Effort:      0.6,
		Repetition:  0.8,
		SchoolDedup: 0.7,
		Campaign:    0.4,
	}
}

// Breakdown is the per-signal contribution to a candidate's score, retained so
// the plan can be explained (by code or, later, by the LLM in prose).
type Breakdown struct {
	Preference float64
	Effort     float64
	Repetition float64
	SchoolDedup float64
	Campaign   float64
}

// Total sums the breakdown.
func (b Breakdown) Total() float64 {
	return b.Preference + b.Effort + b.Repetition + b.SchoolDedup + b.Campaign
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

func score(c domain.Candidate, ctx domain.PlanContext, w Weights) ScoredCandidate {
	b := Breakdown{
		Preference:  w.Preference * preferenceScore(c, ctx),
		Effort:      w.Effort * effortScore(c, ctx),
		Repetition:  w.Repetition * repetitionPenalty(c, ctx),
		SchoolDedup: w.SchoolDedup * schoolDedupPenalty(c, ctx),
		Campaign:    w.Campaign * campaignBonus(c, ctx),
	}

	feasible, reason := feasibility(c, ctx)
	return ScoredCandidate{
		Candidate: c,
		Score:     b.Total(),
		Breakdown: b,
		Feasible:  feasible,
		Reason:    reason,
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
