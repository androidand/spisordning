// Package availability determines whether a recipe can be made from a
// household's current inventory, accounting for substitutions and ingredient
// forms. It is a read-only consumer of pantry-inventory and ingredient-catalog
// data — it never mutates inventory or substitutions.
//
// The core computation is pure: EvaluateRecipe takes pre-fetched domain data
// and returns explainable per-line verdicts plus a recipe-level verdict. No
// database access lives in this package (task 7.1).
package availability

import (
	"slices"
	"sort"
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

// IngredientForm, SubstitutionTier, and IngredientSubstitution are
// intentionally package-local rather than living in internal/domain: this
// package predates and is not yet wired to any caller (reconciled 2026-08-25
// alongside establish-household-and-catalog, which owns the real, differently
// -shaped domain.IngredientForm/IngredientSubstitution - a persistence-row
// struct and a string-keyed substitution, respectively, vs. the simple
// comparable enum + ordered-tier walk this package's matching logic needs).
// When this package gets a real caller, that caller is responsible for
// converting catalog.go's persisted rows into these shapes - do not assume
// the two are drop-in compatible.

// IngredientForm is the simple preservation/preparation state used for
// availability matching (fresh/dried/canned/frozen).
type IngredientForm string

const (
	FormFresh  IngredientForm = "fresh"
	FormDried  IngredientForm = "dried"
	FormCanned IngredientForm = "canned"
	FormFrozen IngredientForm = "frozen"
)

// SubstitutionTier is the preference category of an IngredientSubstitution,
// walked in decreasing order: EQUIVALENT first, EMERGENCY last.
type SubstitutionTier string

const (
	TierEquivalent SubstitutionTier = "EQUIVALENT"
	TierGood       SubstitutionTier = "GOOD"
	TierAcceptable SubstitutionTier = "ACCEPTABLE"
	TierForm       SubstitutionTier = "FORM"
	TierDietary    SubstitutionTier = "DIETARY"
	TierEmergency  SubstitutionTier = "EMERGENCY"
)

// SubstitutionTierOrder returns the tiers in decreasing preference order,
// for walking substitutions from best to worst.
func SubstitutionTierOrder() []SubstitutionTier {
	return []SubstitutionTier{
		TierEquivalent, TierGood, TierAcceptable, TierForm, TierDietary, TierEmergency,
	}
}

// IngredientSubstitution is a directional substitution from one ingredient to
// another, with an explicit quantity ratio (to's quantity per from's
// quantity). A nil FromForm/ToForm means "applies to any form." Retired rows
// are kept so past recommendations remain explainable.
type IngredientSubstitution struct {
	ID               string
	FromIngredientID string
	FromForm         *IngredientForm
	ToIngredientID   string
	ToForm           *IngredientForm
	Category         SubstitutionTier
	Ratio            float64 // to_qty per from_qty
	Retired          bool
}

// RecipeLine is one ingredient line from a recipe, enriched with form
// preferences. IngredientID is the canonical ingredient id; PreferredForm and
// AcceptableForms are the recipe's form hints (may be empty if the recipe
// does not specify form preferences).
type RecipeLine struct {
	IngredientID    string
	Quantity        float64
	Unit            string
	PreferredForm   *IngredientForm
	AcceptableForms []IngredientForm
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
	Form         *IngredientForm
}

// LotAgg is the result of aggregating all matching lots for a single
// ingredient line.
type LotAgg struct {
	TotalQuantity float64
	LotIDs        []int64
	WorstConf     domain.Confidence
	NearExpiryIDs []int64
}

// IngredientVerdict is the result for a single recipe ingredient line.
type IngredientVerdict struct {
	IngredientID       string
	Status             IngredientStatus
	OnHandQuantity     float64 // quantity available (0 if missing)
	RequiredQuantity   float64
	Shortfall          float64 // required - available, 0 if satisfied
	SubstitutionTier   *SubstitutionTier
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
	// ConsumedLotIDs are the unique lot ids the verdict would consume,
	// ordered by first appearance. NearExpiryLotIDs is a deduplicated
	// subset of those lots whose best-before date is within 7 days of now
	// (set by the caller; zero Now means "do not surface expiry").
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
	Substitutions []IngredientSubstitution
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

	// Deduplicate consumed and near-expiry lot ids at recipe level.
	consumed := dedupInt64s(flattenInt64s(lines, func(v IngredientVerdict) []int64 { return v.ConsumedLotIDs }))
	nearExpiry := dedupInt64s(flattenInt64s(lines, func(v IngredientVerdict) []int64 { return v.NearExpiryLotIDs }))

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

// evaluateLine computes the verdict for a single recipe ingredient line.
// It first aggregates all on-hand lots (task 3.1 + multi-lot aggregation),
// then falls through to the substitution walk for any residual shortfall
// (task 3.3).
func evaluateLine(line RecipeLine, lots []LotInfo, subs []IngredientSubstitution, now time.Time) IngredientVerdict {
	v := IngredientVerdict{
		IngredientID:     line.IngredientID,
		RequiredQuantity: line.Quantity,
	}

	// Step 1: aggregate all on-hand lots matching this ingredient.
	agg := aggregateDirectLots(line, lots, now)
	if agg.TotalQuantity > 0 {
		if agg.WorstConf == domain.ConfidenceUnknown {
			v.Status = StatusOnHandUncertain
			v.Confidence = domain.ConfidenceUnknown
			v.Reason = "on-hand-uncertain"
		} else {
			v.Status = StatusOnHand
			v.Confidence = agg.WorstConf
			v.Reason = "on-hand"
		}
		v.OnHandQuantity = agg.TotalQuantity
		v.ConsumedLotIDs = agg.LotIDs
		v.NearExpiryLotIDs = agg.NearExpiryIDs
		if agg.TotalQuantity < line.Quantity {
			// Direct lots exist but are insufficient. Walk substitutions
			// for the residual before declaring missing (minor fix from
			// reviewer round 3).
			residual := line.Quantity - agg.TotalQuantity
			subResult, _, _ := walkSubstitutions(line, subs, lots, now, residual)
			if subResult != nil {
				// Substitution covers the residual.
				v.Status = StatusSubstituted
				v.SubstitutionTier = subResult.tier
				v.SubstitutedFromID = line.IngredientID
				v.OnHandQuantity = agg.TotalQuantity + subResult.available
				v.Shortfall = 0
				v.Reason = "on-hand-plus-substituted-" + string(*subResult.tier)
				v.ConsumedLotIDs = append(v.ConsumedLotIDs, subResult.lotIDs...)
				v.NearExpiryLotIDs = append(v.NearExpiryLotIDs, subResult.nearExpiry...)
				if subResult.confidence == domain.ConfidenceUnknown {
					v.Confidence = domain.ConfidenceUnknown
					v.Reason += "-uncertain"
				} else if v.Confidence != domain.ConfidenceUnknown {
					v.Confidence = subResult.confidence
				}
				return v
			}
			// No substitution covers the residual.
			v.Shortfall = line.Quantity - agg.TotalQuantity
			v.Status = StatusMissing
			v.Reason = "missing-shortfall"
			return v
		}
		return v
	}

	// Step 2: no on-hand lots at all. Walk substitutions.
	subResult, bestSubAvailable, bestSubNeeded := walkSubstitutions(line, subs, lots, now, line.Quantity)
	if subResult != nil {
		v.Status = StatusSubstituted
		v.SubstitutionTier = subResult.tier
		v.SubstitutedFromID = line.IngredientID
		v.OnHandQuantity = subResult.available
		v.Reason = "substituted-" + string(*subResult.tier)
		v.ConsumedLotIDs = subResult.lotIDs
		v.NearExpiryLotIDs = subResult.nearExpiry
		v.Confidence = subResult.confidence
		if subResult.confidence == domain.ConfidenceUnknown {
			v.Reason += "-uncertain"
		}
		return v
	}

	// Step 3: no match, no substitution.
	v.Status = StatusMissing
	v.Reason = "missing"
		if bestSubAvailable > 0 {
			v.Shortfall = bestSubNeeded - bestSubAvailable
			v.OnHandQuantity = bestSubAvailable
			v.Reason = "missing-shortfall"
		}
	return v
}

// subResult holds the outcome of a substitution attempt.
type subResult struct {
	tier       *SubstitutionTier
	available  float64
	lotIDs     []int64
	nearExpiry []int64
	confidence domain.Confidence
}

// walkSubstitutions walks all substitutions for the given line and returns
// the first one that fully covers `needed`, or (nil, bestAvailable) if
// none do. bestAvailable holds the max quantity found across all failed
// attempts so the caller can surface a shortfall.
func walkSubstitutions(line RecipeLine, subs []IngredientSubstitution, lots []LotInfo, now time.Time, needed float64) (*subResult, float64, float64) {
	var bestAvailable float64
	var bestNeeded float64
	for _, tier := range SubstitutionTierOrder() {
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
			// Find and aggregate lots of the target ingredient, enforcing
			// the target form if the substitution specifies one.
			// Convert needed to target units using the substitution ratio.
			neededInTarget := needed * sub.Ratio
			subLine := RecipeLine{
				IngredientID:  sub.ToIngredientID,
				Quantity:      neededInTarget,
				Unit:          line.Unit,
				PreferredForm: sub.ToForm,
			}
			subAgg := aggregateDirectLots(subLine, lots, now)
			if subAgg.TotalQuantity == 0 {
				continue
			}
			if subAgg.TotalQuantity >= neededInTarget {
				// Full coverage at this tier — take it.
				r := &subResult{
					tier:       &tier,
					available:  subAgg.TotalQuantity,
					lotIDs:     subAgg.LotIDs,
					nearExpiry: subAgg.NearExpiryIDs,
					confidence: subAgg.WorstConf,
				}
				return r, bestAvailable, bestNeeded
			}
			// Insufficient at this tier — track the best available and
			// keep walking.
			if subAgg.TotalQuantity > bestAvailable {
				bestAvailable = subAgg.TotalQuantity
				bestNeeded = neededInTarget
			}
		}
	}
	return nil, bestAvailable, bestNeeded
}

// aggregateDirectLots returns all lots matching the given line's ingredient
// and form constraints, summed into a LotAgg. Matching is by ingredient_id
// (task 3.1); form constraints from PreferredForm and AcceptableForms are
// applied (task 3.2); lots with quantity <= 0 are excluded.
func aggregateDirectLots(line RecipeLine, lots []LotInfo, now time.Time) LotAgg {
	var total float64
	var ids []int64
	var nearExpiry []int64
	// Start at highest confidence so any real lot will update it.
	var worstConf domain.Confidence = domain.ConfidenceExact

	for _, lot := range lots {
		if lot.IngredientID != line.IngredientID {
			continue
		}
		if lot.Quantity <= 0 {
			continue
		}
		// Form check: if the recipe has a preferred form and the lot has a
		// different known form, this lot is not a direct match (task 3.2).
		if line.PreferredForm != nil && lot.Form != nil && *lot.Form != *line.PreferredForm {
			if !slices.Contains(line.AcceptableForms, *lot.Form) {
				continue
			}
		}
		total += lot.Quantity
		ids = append(ids, lot.ID)
		if confidenceRank(lot.Confidence) < confidenceRank(worstConf) {
			worstConf = lot.Confidence
		}
		if !now.IsZero() && lot.BestBefore != nil {
			days := lot.BestBefore.Sub(now).Hours() / 24
			if days >= 0 && days <= 7 {
				nearExpiry = append(nearExpiry, lot.ID)
			}
		}
	}

	return LotAgg{
		TotalQuantity: total,
		LotIDs:        ids,
		WorstConf:     worstConf,
		NearExpiryIDs: nearExpiry,
	}
}

// findBestDirectLot returns the single best matching lot for a recipe line.
// "Best" means: matching ingredient, preferred form first, then highest
// confidence, then earliest best-before. Quantity sufficiency is checked
// by the caller (task 3.5). Kept for potential future single-lot use cases.
func findBestDirectLot(line RecipeLine, lots []LotInfo) *LotInfo {
	var candidates []int
	for i, lot := range lots {
		if lot.IngredientID != line.IngredientID {
			continue
		}
		if lot.Quantity <= 0 {
			continue
		}
		if line.PreferredForm != nil && lot.Form != nil && *lot.Form != *line.PreferredForm {
			if !slices.Contains(line.AcceptableForms, *lot.Form) {
				continue
			}
		}
		candidates = append(candidates, i)
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.SliceStable(candidates, func(a, b int) bool {
		la, lb := lots[candidates[a]], lots[candidates[b]]
		laPref, lbPref := formMatchesPreferred(la.Form, line.PreferredForm), formMatchesPreferred(lb.Form, line.PreferredForm)
		if laPref != lbPref {
			return laPref
		}
		if la.Confidence != lb.Confidence {
			return confidenceRank(la.Confidence) > confidenceRank(lb.Confidence)
		}
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

func formMatchesPreferred(lotForm *IngredientForm, preferred *IngredientForm) bool {
	if preferred == nil || lotForm == nil {
		return true
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

func dedupInt64s(ids []int64) []int64 {
	seen := make(map[int64]struct{})
	var out []int64
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func flattenInt64s[T any](items []T, fn func(T) []int64) []int64 {
	var out []int64
	for _, item := range items {
		out = append(out, fn(item)...)
	}
	return out
}
