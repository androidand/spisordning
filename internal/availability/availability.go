package availability

import (
	"fmt"
	"sort"

	"github.com/androidand/spisordning/internal/domain"
)

// subTierOrder defines the preference ordering for substitution tiers.
// EQUIVALENT is best (0); EMERGENCY is worst (5).
var subTierOrder = map[domain.SubstitutionCategory]int{
	domain.SubstitutionEquivalent: 0,
	domain.SubstitutionGood:       1,
	domain.SubstitutionAcceptable: 2,
	domain.SubstitutionForm:       3,
	domain.SubstitutionDietary:    4,
	domain.SubstitutionEmergency:  5,
}

// ComputeRecipeAvailability determines feasibility for every ingredient line
// of a recipe against the provided on-hand lots and substitution rules.
//
// It never mutates any input. Lots are consumed greedily in ID order (lowest
// id first) so the same inputs always produce the same result.
//
// Partial-quantity policy: an on-hand quantity smaller than required counts as
// unmet — the shortfall is recorded but the line is not partially satisfied.
// This feeds implement-shopping-and-commerce' gap computation rather than
// this capability's own verdict. When a substitution is found after partial
// confident lots were considered, the partial lots are NOT consumed (the
// substitution replaces the line); when no substitution is found, the partial
// lots ARE consumed so they don't double-count for other lines.
//
// UNKNOWN-confidence lots are treated as satisfied but flagged (IsUncertain).
// The recipe-level verdict elevates to feasible-with-substitution when any
// line is uncertain, so the user sees that not everything is confidently on
// hand. The same flagging applies when a substitution is backed by an
// UNKNOWN-confidence lot.
//
// Confident and unknown lots are tried independently: if confident lots are
// insufficient, the algorithm falls through to unknown lots (not straight to
// missing), and substitution is attempted between both phases.
func ComputeRecipeAvailability(recipeID string, lines []RecipeIngredientLine, lots []InventoryLotInput, subs []Substitution) RecipeVerdict {
	v := RecipeVerdict{
		RecipeID: recipeID,
		Lines:    make([]LineVerdict, len(lines)),
	}

	subByIngredient := groupSubstitutions(subs)

	remaining := make(map[int64]float64)
	for i := range lots {
		remaining[lots[i].ID] = lots[i].Quantity
	}

	lotConf := make(map[int64]domain.Confidence, len(lots))
	for i := range lots {
		lotConf[lots[i].ID] = lots[i].Confidence
	}

	for i := range lines {
		v.Lines[i] = computeLine(lines[i], lots, subByIngredient, lotConf, remaining)
	}

	v.Verdict = aggregateVerdict(v.Lines)
	return v
}

func computeLine(line RecipeIngredientLine, lots []InventoryLotInput, subByIngredient map[string][]Substitution, lotConf map[int64]domain.Confidence, remaining map[int64]float64) LineVerdict {
	v := LineVerdict{
		IngredientID: line.IngredientID,
		Quantity:     line.Quantity,
		Unit:         line.Unit,
	}

	cands := matchingLots(lots, remaining, line.IngredientID, line.Unit)

	if len(cands) == 0 {
		v.Shortfall = line.Quantity
		v.Reason = fmt.Sprintf("no on-hand match for ingredient %q (unit=%q)", line.IngredientID, line.Unit)
		v.Status = StatusMissing
		return trySubstitutionCommit(v, line, subByIngredient, lots, lotConf, remaining)
	}

	// Separate confident from unknown-confidence candidates.
	var confident, unknown []lotCandidate
	for i := range cands {
		c := &cands[i]
		if c.l.Confidence == domain.ConfidenceUnknown {
			unknown = append(unknown, *c)
		} else {
			confident = append(confident, *c)
		}
	}

	confidentAvail := sumRemaining(confident, remaining)
	unknownAvail := sumRemaining(unknown, remaining)
	totalAvail := confidentAvail + unknownAvail

	// Phase 1: Confident sufficient → on-hand.
	if confidentAvail >= line.Quantity {
		_, _, consumed := consumeFromLots(confident, line.Quantity, remaining)
		v.Status = StatusOnHand
		v.Reason = fmt.Sprintf("on-hand: %s %.2f %s", line.IngredientID, line.Quantity, line.Unit)
		v.ConsumedLotIDs = consumed
		return v
	}

	// Phase 2: Confident insufficient — try combining with unknown lots.
	// Unknown lots are flagged (IsUncertain) so the user sees they're not
	// confidently on hand. Prefer consuming from unknown lots first to
	// preserve confident lots for other lines.
	if totalAvail >= line.Quantity {
		_, _, consumed := consumeFromCombined(confident, unknown, line.Quantity, remaining)
		v.Status = StatusUnknown
		v.Reason = fmt.Sprintf("on-hand (uncertain confidence): %s %.2f %s", line.IngredientID, line.Quantity, line.Unit)
		v.IsUncertain = true
		v.ConsumedLotIDs = consumed
		return v
	}

	// Phase 3: Confident + unknown insufficient. Try substitution without
	// consuming partial lots (substitution replaces the line).
	if result := trySubstitutionSnapshot(v, line, subByIngredient, lots, lotConf, remaining); result.succeeded {
		// Re-apply the consumption on the real remaining (snapshot was read-only).
		subCands := matchingLots(lots, remaining, result.toIngredient, line.Unit)
		_, _, consumed := consumeFromLots(subCands, result.needed, remaining)
		result.verdict.ConsumedLotIDs = consumed
		return result.verdict
	}

	// Phase 4: Nothing works. Consume partial lots so they don't double-count
	// for subsequent lines.
	consumeFromLots(confident, line.Quantity, remaining)
	consumeFromLots(unknown, line.Quantity, remaining)
	v.Shortfall = line.Quantity - totalAvail
	v.Reason = fmt.Sprintf("insufficient on-hand: have %.2f %s, need %.2f %s (short %.2f) — no viable substitute",
		totalAvail, line.Unit, line.Quantity, line.Unit, v.Shortfall)
	v.Status = StatusMissing
	return v
}

// consumeFromCombined greedily consumes from unknown lots first, then confident
// lots, to satisfy needed. Unknown lots are consumed first because they're
// uncertain anyway; preserving confident lots maximises options for other lines.
func consumeFromCombined(confident, unknown []lotCandidate, needed float64, remaining map[int64]float64) (bool, float64, []int64) {
	if needed <= 0 {
		return true, 0, nil
	}
	// Try unknown first (preserve confident lots for other lines).
	ok, shortfall, consumed := consumeFromLots(unknown, needed, remaining)
	if ok {
		return true, 0, consumed
	}
	// Unknown insufficient — try confident for the remainder.
	_, _, confidentConsumed := consumeFromLots(confident, shortfall, remaining)
	return false, shortfall, append(consumed, confidentConsumed...)
}

// subAttempt holds the result of a read-only substitution attempt.
type subAttempt struct {
	verdict      LineVerdict
	succeeded    bool
	toIngredient string
	ratio        float64
	needed       float64
	shortfall    float64
}

// trySubstitutionSnapshot tries all substitutions without mutating the
// remaining map. Returns a subAttempt indicating whether any substitution
// succeeded. The caller must re-apply consumption on the real remaining on
// success (since the snapshot was read-only).
func trySubstitutionSnapshot(v LineVerdict, line RecipeIngredientLine, subByIngredient map[string][]Substitution, lots []InventoryLotInput, lotConf map[int64]domain.Confidence, remaining map[int64]float64) subAttempt {
	subs := subByIngredient[line.IngredientID]
	if len(subs) == 0 {
		v.Reason = fmt.Sprintf("no substitute for ingredient %q (unit=%q)", line.IngredientID, line.Unit)
		return subAttempt{verdict: v}
	}

	// Sort by tier preference: EQUIVALENT (0) first, EMERGENCY (5) last.
	// Secondary sort by ToIngredientID for deterministic tie-breaking within
	// the same tier.
	sort.Slice(subs, func(i, j int) bool {
		oi, oj := subTierOrder[subs[i].Category], subTierOrder[subs[j].Category]
		if oi != oj {
			return oi < oj
		}
		return subs[i].ToIngredientID < subs[j].ToIngredientID
	})

	// Work on a copy so the original remaining is never mutated.
	snap := make(map[int64]float64, len(remaining))
	for k, val := range remaining {
		snap[k] = val
	}

	for _, sub := range subs {
		// Form filter: if the recipe line specifies a default_form, only
		// consider substitutions whose from_form matches (or is nil = any form).
		if line.DefaultForm != "" && sub.FromForm != nil && *sub.FromForm != line.DefaultForm {
			continue
		}

		needed := line.Quantity * sub.Ratio
		if needed <= 0 {
			continue
		}

		cands := matchingLots(lots, snap, sub.ToIngredientID, line.Unit)
		ok, shortfall, consumed := consumeFromLots(cands, needed, snap)
		if !ok {
			v.Shortfall = shortfall
			continue
		}

		v.Status = StatusSubstituted
		v.Reason = fmt.Sprintf("substituted %s→%s via %s (ratio %.2f): %s %.2f %s",
			line.IngredientID, sub.ToIngredientID, sub.Category, sub.Ratio,
			sub.ToIngredientID, line.Quantity, line.Unit)
		v.SubstitutedFromIngredient = line.IngredientID
		v.SubstitutedToIngredient = sub.ToIngredientID
		v.SubstitutionTier = string(sub.Category)
		v.Shortfall = 0
		v.ConsumedLotIDs = consumed

		// Flag uncertainty when the substitute is backed by an UNKNOWN-confidence
		// lot. The spec requires that UNKNOWN confidence is never silently trusted.
		for _, lid := range consumed {
			if lotConf[lid] == domain.ConfidenceUnknown {
				v.IsUncertain = true
				break
			}
		}

		return subAttempt{
			verdict:      v,
			succeeded:    true,
			toIngredient: sub.ToIngredientID,
			ratio:        sub.Ratio,
			needed:       needed,
		}
	}

	v.Reason = fmt.Sprintf("no viable substitute for ingredient %q (unit=%q)", line.IngredientID, line.Unit)
	return subAttempt{verdict: v}
}

// trySubstitutionCommit applies trySubstitutionSnapshot and, on success,
// commits the consumed substitute lots to the real remaining map.
func trySubstitutionCommit(v LineVerdict, line RecipeIngredientLine, subByIngredient map[string][]Substitution, lots []InventoryLotInput, lotConf map[int64]domain.Confidence, remaining map[int64]float64) LineVerdict {
	result := trySubstitutionSnapshot(v, line, subByIngredient, lots, lotConf, remaining)
	if !result.succeeded {
		return result.verdict
	}
	subCands := matchingLots(lots, remaining, result.toIngredient, line.Unit)
	_, _, consumed := consumeFromLots(subCands, result.needed, remaining)
	result.verdict.ConsumedLotIDs = consumed
	return result.verdict
}

func matchingLots(lots []InventoryLotInput, remaining map[int64]float64, ingredientID, unit string) []lotCandidate {
	var out []lotCandidate
	for i := range lots {
		l := &lots[i]
		if l.IngredientID != ingredientID || l.Unit != unit {
			continue
		}
		r := remaining[l.ID]
		if r <= 0 {
			continue
		}
		out = append(out, lotCandidate{l: *l, remaining: r})
	}
	// Deterministic order: lowest lot ID first.
	sort.Slice(out, func(i, j int) bool { return out[i].l.ID < out[j].l.ID })
	return out
}

func sumRemaining(cands []lotCandidate, remaining map[int64]float64) float64 {
	total := 0.0
	for i := range cands {
		total += remaining[cands[i].l.ID]
	}
	return total
}

// consumeFromLots greedily consumes from the (already sorted by ID) candidates.
// It mutates remaining to reflect the consumption. Returns (fullyConsumed,
// shortfall, consumedLotIDs). shortfall > 0 means the requested amount could
// not be fully met.
func consumeFromLots(cands []lotCandidate, needed float64, remaining map[int64]float64) (bool, float64, []int64) {
	if needed <= 0 {
		return true, 0, nil
	}
	var consumed []int64
	remainingNeeded := needed
	for i := range cands {
		c := &cands[i]
		if remainingNeeded <= 0 {
			break
		}
		avail := remaining[c.l.ID]
		if avail <= 0 {
			continue
		}
		take := avail
		if take > remainingNeeded {
			take = remainingNeeded
		}
		remaining[c.l.ID] -= take
		remainingNeeded -= take
		consumed = append(consumed, c.l.ID)
	}
	if remainingNeeded <= 0 {
		return true, 0, consumed
	}
	return false, remainingNeeded, consumed
}

func groupSubstitutions(subs []Substitution) map[string][]Substitution {
	out := make(map[string][]Substitution)
	for i := range subs {
		s := &subs[i]
		out[s.FromIngredientID] = append(out[s.FromIngredientID], *s)
	}
	return out
}

func aggregateVerdict(lines []LineVerdict) RecipeVerdictLevel {
	hasSubOrUncertain := false
	for i := range lines {
		l := &lines[i]
		switch l.Status {
		case StatusMissing:
			return VerdictInfeasible
		case StatusSubstituted, StatusUnknown:
			hasSubOrUncertain = true
		}
	}
	if hasSubOrUncertain {
		return VerdictFeasibleWithSub
	}
	return VerdictFeasible
}

type lotCandidate struct {
	l         InventoryLotInput
	remaining float64
}
