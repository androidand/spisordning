package availability

import (
	"fmt"
	"sort"

	"github.com/androidand/spisordning/internal/domain"
)

// subTierOrder defines the preference ordering for substitution tiers.
// EQUIVALENT is best (0); EMERGENCY is worst (5).
var subTierOrder = map[domain.IngredientSubstitutionCategory]int{
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
// this capability's own verdict.
//
// UNKNOWN-confidence lots are treated as satisfied but flagged (IsUncertain).
// The recipe-level verdict elevates to feasible-with-substitution when any
// line is uncertain, so the user sees that not everything is confidently on
// hand.
func ComputeRecipeAvailability(recipeID string, lines []RecipeIngredientLine, lots []InventoryLotInput, subs []Substitution) RecipeVerdict {
	v := RecipeVerdict{
		RecipeID: recipeID,
		Lines:    make([]LineVerdict, len(lines)),
	}

	subByIngredient := groupSubstitutions(subs)

	// Greedy consumption tracker: lotID → remaining quantity.
	remaining := make(map[int64]float64)
	for i := range lots {
		remaining[lots[i].ID] = lots[i].Quantity
	}

	for i := range lines {
		v.Lines[i] = computeLine(lines[i], lots, subByIngredient, remaining)
	}

	v.Verdict = aggregateVerdict(v.Lines)
	return v
}

func computeLine(line RecipeIngredientLine, lots []InventoryLotInput, subByIngredient map[string][]Substitution, remaining map[int64]float64) LineVerdict {
	v := LineVerdict{
		IngredientID: line.IngredientID,
		Quantity:     line.Quantity,
		Unit:         line.Unit,
	}

	cands := matchingLots(lots, remaining, line.IngredientID, line.Unit)

	if len(cands) == 0 {
		v.Status = StatusMissing
		v.Shortfall = line.Quantity
		v.Reason = fmt.Sprintf("no on-hand match for ingredient %q (unit=%q)", line.IngredientID, line.Unit)
		return trySubstitution(v, line, subByIngredient, lots, remaining)
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

	// Try confident lots first (greedy, lowest ID).
	ok, shortfall, consumed := consumeFromLots(confident, line.Quantity, remaining)
	if ok {
		v.Status = StatusOnHand
		v.Reason = fmt.Sprintf("on-hand: %s %.2f %s", line.IngredientID, line.Quantity, line.Unit)
		v.ConsumedLotIDs = consumed
		return v
	}

	if len(confident) > 0 {
		v.Shortfall = shortfall
		v.Reason = fmt.Sprintf("partial on-hand: have %.2f %s, need %.2f %s (short %.2f)",
			line.Quantity-shortfall, line.Unit, line.Quantity, line.Unit, shortfall)
		v.Status = StatusMissing
		v.ConsumedLotIDs = consumed
		return trySubstitution(v, line, subByIngredient, lots, remaining)
	}

	// No confident lot — fall back to unknown-confidence lot if available.
	if len(unknown) > 0 {
		ok, shortfall, consumed = consumeFromLots(unknown, line.Quantity, remaining)
		if ok {
			v.Status = StatusUnknown
			v.Reason = fmt.Sprintf("on-hand (uncertain confidence): %s %.2f %s", line.IngredientID, line.Quantity, line.Unit)
			v.IsUncertain = true
			v.ConsumedLotIDs = consumed
			return v
		}
		v.Shortfall = shortfall
		v.Reason = fmt.Sprintf("only unknown-confidence lot for %q has insufficient quantity (have %.2f, need %.2f %s)",
			line.IngredientID, line.Quantity-shortfall, line.Quantity, line.Unit)
		v.Status = StatusMissing
		v.ConsumedLotIDs = consumed
		return trySubstitution(v, line, subByIngredient, lots, remaining)
	}

	v.Reason = fmt.Sprintf("no on-hand match for ingredient %q (unit=%q)", line.IngredientID, line.Unit)
	return trySubstitution(v, line, subByIngredient, lots, remaining)
}

type lotCandidate struct {
	l         InventoryLotInput
	remaining float64
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

func trySubstitution(v LineVerdict, line RecipeIngredientLine, subByIngredient map[string][]Substitution, lots []InventoryLotInput, remaining map[int64]float64) LineVerdict {
	subs := subByIngredient[line.IngredientID]
	if len(subs) == 0 {
		v.Reason = fmt.Sprintf("no substitute for ingredient %q (unit=%q)", line.IngredientID, line.Unit)
		return v
	}

	// Sort by tier preference: EQUIVALENT (0) first, EMERGENCY (5) last.
	sort.Slice(subs, func(i, j int) bool {
		return subTierOrder[subs[i].Category] < subTierOrder[subs[j].Category]
	})

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

		cands := matchingLots(lots, remaining, sub.ToIngredientID, line.Unit)
		ok, shortfall, consumed := consumeFromLots(cands, needed, remaining)
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
		return v
	}

	v.Reason = fmt.Sprintf("no viable substitute for ingredient %q (unit=%q)", line.IngredientID, line.Unit)
	return v
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
