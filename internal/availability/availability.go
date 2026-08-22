// Package availability determines whether a recipe can be made from a
// household's current inventory, accounting for substitutions and ingredient
// forms. It is a read-only consumer of pantry-inventory and ingredient-catalog
// data — it never mutates inventory or substitutions.
//
// The core computation is pure: EvaluateRecipe takes pre-fetched domain data
// and returns explainable per-line verdicts plus a recipe-level verdict. No
// database access lives in this package (design.md Step 7, task 7.1).
package availability

import (
	"slices"
	"sort"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

// RecipeLine is one ingredient line from a recipe, enriched with form
// preferences. IngredientID is the canonical ingredient id; PreferredForm and
// AcceptableForms are the recipe's form hints (may be empty if the recipe
// does not specify form preferences).
type RecipeLine struct {
	IngredientID    string
	Quantity        float64
	Unit            string
	PreferredForm   *domain.IngredientForm
	AcceptableForms []domain.IngredientForm
}

// LotInfo is the domain view of an inventory lot needed for availability
// computation. Form is nil when unknown (no ingredient_mapping.default_form
// or product-level form data was available). Confidence controls whether a
// lot can silently satisfy a requirement (task 3.6).
type LotInfo struct {
	ID           int64
	IngredientID string
	ProductID    string
	Quantity     float64
	Unit         string
	Confidence   domain.Confidence
	BestBefore   *time.Time // nil = no date
	Form         *domain.IngredientForm
}

// IngredientVerdict is the result for a single recipe ingredient line.
type IngredientVerdict struct {
	IngredientID       string
	Status             IngredientStatus
	OnHandQuantity     float64 // quantity available (0 if missing)
	RequiredQuantity   float64
	Shortfall          float64 // required - available, 0 if satisfied
	SubstitutionTier   *domain.SubstitutionTier
	SubstitutedFromID  string // original ingredient id (empty if on-hand)
	Confidence         domain.Confidence
	Reason             string // machine-readable explanation
	ConsumedLotIDs     []int64
	NearExpiryLotIDs   []int64
}

// IngredientStatus classifies one line's outcome.
type IngredientStatus string

const (
	StatusOnHand            IngredientStatus = "on-hand"
	StatusOnHandUncertain   IngredientStatus = "on-hand-uncertain"
	StatusSubstituted       IngredientStatus = "substituted"
	StatusMissing           IngredientStatus = "missing"
)

// RecipeVerdict is the aggregate result for a recipe.
type RecipeVerdict struct {
	RecipeID         string
	Status           RecipeStatus
	Lines            []IngredientVerdict
	// ConsumedLotIDs are the lot ids the verdict would consume, ordered by
	// line. NearExpiryLotIDs is a subset of those lots whose best-before
	// date is within nearExpiryDays of now (set by the caller; 0 means
	// "do not surface expiry").
	ConsumedLotIDs   []int64
	NearExpiryLotIDs []int64
}

// RecipeStatus is the recipe-level tri-state verdict.
type RecipeStatus string

const (
	RecipeFeasible          RecipeStatus = "feasible"
	RecipeFeasibleWithSub   RecipeStatus = "feasible-with-substitution"
	RecipeInfeasible        RecipeStatus = "infeasible"
)

// EvaluateInputs bundles the data EvaluateRecipe needs. The caller is
// responsible for fetching lots and substitutions scoped to the household
// and for enriching lots with form data (if available).
type EvaluateInputs struct {
	RecipeID      string
	Lines         []RecipeLine
	Lots          []LotInfo
	Substitutions []domain.IngredientSubstitution
	Now           time.Time // for expiry checks; zero means "do not surface expiry"
}

// EvaluateRecipe computes per-ingredient and recipe-level feasibility. It is
// pure: no I/O, no side effects.
func EvaluateRecipe(inputs EvaluateInputs) RecipeVerdict {
	var lines []IngredientVerdict
	hasSubstitution := false
	hasUncertain := false
	hasMissing := false

	for _, line := range inputs.Lines {
		v := evaluateLine(line, inputs.Lots, inputs.Substitutions, inputs.Now)
		lines = append(lines, v)
		switch v.Status {
		case StatusSubstituted:
			hasSubstitution = true
		case StatusOnHandUncertain:
			hasUncertain = true
		case StatusMissing:
			hasMissing = true
		}
	}

	status := computeRecipeStatus(hasMissing, hasSubstitution, hasUncertain)

	var consumed, nearExpiry []int64
	for _, v := range lines {
		consumed = append(consumed, v.ConsumedLotIDs...)
		nearExpiry = append(nearExpiry, v.NearExpiryLotIDs...)
	}

	return RecipeVerdict{
		RecipeID:         inputs.RecipeID,
		Status:           status,
		Lines:            lines,
		ConsumedLotIDs:   consumed,
		NearExpiryLotIDs: nearExpiry,
	}
}

func computeRecipeStatus(hasMissing, hasSubstitution, hasUncertain bool) RecipeStatus {
	if hasMissing {
		return RecipeInfeasible
	}
	if hasSubstitution || hasUncertain {
		return RecipeFeasibleWithSub
	}
	return RecipeFeasible
}

func evaluateLine(line RecipeLine, lots []LotInfo, subs []domain.IngredientSubstitution, now time.Time) IngredientVerdict {
	v := IngredientVerdict{
		IngredientID:   line.IngredientID,
		RequiredQuantity: line.Quantity,
	}

	// Step 1: try direct on-hand match, preferring matching form.
	bestLot := findBestDirectLot(line, lots)
	if bestLot != nil {
		if bestLot.Confidence == domain.ConfidenceUnknown {
			v.Status = StatusOnHandUncertain
			v.Confidence = domain.ConfidenceUnknown
			v.Reason = "on-hand-uncertain"
		} else {
			v.Status = StatusOnHand
			v.Confidence = bestLot.Confidence
			v.Reason = "on-hand"
		}
		v.OnHandQuantity = bestLot.Quantity
		if bestLot.BestBefore != nil && !now.IsZero() {
			days := bestLot.BestBefore.Sub(now).Hours() / 24
			if days >= 0 && days <= 7 {
				v.NearExpiryLotIDs = []int64{bestLot.ID}
			}
		}
		if v.OnHandQuantity < line.Quantity {
			v.Shortfall = line.Quantity - v.OnHandQuantity
			v.Status = StatusMissing
			v.Reason = "missing-shortfall"
			return v
		}
		return v
	}

	// Step 2: walk substitutions in decreasing preference order.
	for _, tier := range domain.SubstitutionTierOrder() {
		for _, sub := range subs {
			if sub.Retired {
				continue
			}
			if sub.FromIngredientID != line.IngredientID {
				continue
			}
			if sub.Category != tier {
				continue
			}
			// Check form constraints on the source side.
			if sub.FromForm != nil && line.PreferredForm != nil && *sub.FromForm != *line.PreferredForm {
				continue
			}
			// Find lots of the target ingredient.
			subLot := findBestDirectLot(
				RecipeLine{IngredientID: sub.ToIngredientID, Quantity: line.Quantity * sub.Ratio, Unit: line.Unit},
				lots,
			)
			if subLot == nil {
				continue
			}
			v.Status = StatusSubstituted
			v.SubstitutionTier = &tier
			v.SubstitutedFromID = line.IngredientID
			v.OnHandQuantity = subLot.Quantity
			v.Reason = "substituted-" + string(tier)
			if subLot.Confidence == domain.ConfidenceUnknown {
				v.Confidence = domain.ConfidenceUnknown
				v.Reason += "-uncertain"
			} else {
				v.Confidence = subLot.Confidence
			}
			if subLot.BestBefore != nil && !now.IsZero() {
				days := subLot.BestBefore.Sub(now).Hours() / 24
				if days >= 0 && days <= 7 {
					v.NearExpiryLotIDs = []int64{subLot.ID}
				}
			}
			if v.OnHandQuantity < line.Quantity*sub.Ratio {
				v.Shortfall = line.Quantity*sub.Ratio - v.OnHandQuantity
				v.Status = StatusMissing
				v.Reason = "missing-shortfall"
				return v
			}
			return v
		}
	}

	// Step 3: no match, no substitution.
	v.Status = StatusMissing
	v.Reason = "missing"
	return v
}

// findBestDirectLot returns the best matching lot for a recipe line from the
// given lots, or nil if none match. "Best" means: matching ingredient,
// preferred form first, then highest confidence, then earliest best-before
// (to consume near-expiry lots first). Quantity sufficiency is checked by
// the caller (task 3.5).
func findBestDirectLot(line RecipeLine, lots []LotInfo) *LotInfo {
	var candidates []int
	for i, lot := range lots {
		if lot.IngredientID != line.IngredientID {
			continue
		}
		if lot.Quantity <= 0 {
			continue
		}
		// Form check: if the recipe has a preferred form and the lot has a
		// different known form, this lot is not a direct match (task 3.2).
		if line.PreferredForm != nil && lot.Form != nil && *lot.Form != *line.PreferredForm {
			// Still a candidate if the form is in the acceptable list.
			if !slices.Contains(line.AcceptableForms, *lot.Form) {
				continue
			}
		}
		candidates = append(candidates, i)
	}
	if len(candidates) == 0 {
		return nil
	}

	// Sort candidates: preferred-form match first, then by confidence
	// (EXACT > LIKELY > ESTIMATED > UNKNOWN), then by best-before (soonest first).
	sort.SliceStable(candidates, func(a, b int) bool {
		la, lb := lots[candidates[a]], lots[candidates[b]]
		// Preferred form match wins.
		laPref, lbPref := formMatchesPreferred(la.Form, line.PreferredForm), formMatchesPreferred(lb.Form, line.PreferredForm)
		if laPref != lbPref {
			return laPref
		}
		// Higher confidence wins.
		if la.Confidence != lb.Confidence {
			return confidenceRank(la.Confidence) > confidenceRank(lb.Confidence)
		}
		// Soonest best-before wins (nil = no date, treated as farthest).
		if la.BestBefore != nil && lb.BestBefore != nil {
			return la.BestBefore.Before(*lb.BestBefore)
		}
		if la.BestBefore != nil {
			return true
		}
		if lb.BestBefore != nil {
			return false
		}
		return false
	})

	return &lots[candidates[0]]
}

func formMatchesPreferred(lotForm *domain.IngredientForm, preferred *domain.IngredientForm) bool {
	if preferred == nil || lotForm == nil {
		return true // no preference or no lot form → no mismatch
	}
	return *lotForm == *preferred
}

func confidenceRank(c domain.Confidence) int {
	switch c {
	case domain.ConfidenceExact:
		return 4
	case domain.ConfidenceLikely:
		return 3
	case domain.ConfidenceEstimated:
		return 2
	case domain.ConfidenceUnknown:
		return 1
	default:
		return 0
	}
}
