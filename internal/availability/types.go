package availability

import (
	"time"

	"github.com/androidand/spisordning/internal/domain"
)

// RecipeIngredientLine is one line from a recipe's ingredient list.
type RecipeIngredientLine struct {
	IngredientID string
	Quantity     float64
	Unit         string
	DefaultForm  string // from ingredient_mapping; empty means no form preference
}

// InventoryLotInput is a read-only snapshot of an on-hand lot.
type InventoryLotInput struct {
	ID           int64
	IngredientID string
	ProductID    string
	Quantity     float64
	Unit         string
	Confidence   domain.Confidence
	BestBefore   *time.Time
}

// Substitution is a read-only snapshot of an IngredientSubstitution row.
type Substitution struct {
	FromIngredientID string
	FromForm         *string
	ToIngredientID   string
	ToForm           *string
	Category         domain.IngredientSubstitutionCategory
	Ratio            float64
}

// LineVerdict is the feasibility result for one recipe ingredient line.
type LineVerdict struct {
	IngredientID            string
	Quantity                float64
	Unit                    string
	Status                  LineStatus
	Reason                  string
	Shortfall               float64 // positive when on-hand < required; zero when satisfied
	ConsumedLotIDs          []int64
	SubstitutedFromIngredient string
	SubstitutedToIngredient   string
	SubstitutionTier          string
	IsUncertain               bool // true if satisfied by UNKNOWN-confidence lot (direct or via substitution)
}

// LineStatus is the per-ingredient verdict.
type LineStatus string

const (
	// StatusOnHand means the ingredient is on hand in sufficient quantity
	// with matching unit.
	StatusOnHand LineStatus = "on-hand"
	// StatusSubstituted means the ingredient was resolved via an
	// IngredientSubstitution at the named tier.
	StatusSubstituted LineStatus = "substituted"
	// StatusUnknown means the ingredient is on hand but only with UNKNOWN
	// confidence — satisfied, but flagged as uncertain.
	StatusUnknown LineStatus = "unknown"
	// StatusMissing means no on-hand lot and no viable substitution.
	StatusMissing LineStatus = "missing"
)

// RecipeVerdictLevel is the aggregated recipe-level verdict.
type RecipeVerdictLevel string

const (
	// VerdictFeasible means every line is on hand with sufficient quantity
	// and no uncertain lots.
	VerdictFeasible RecipeVerdictLevel = "feasible"
	// VerdictFeasibleWithSub means at least one line required a substitution
	// or was satisfied only by an uncertain lot, but no line is missing.
	VerdictFeasibleWithSub RecipeVerdictLevel = "feasible-with-substitution"
	// VerdictInfeasible means at least one line is missing with no viable
	// substitute.
	VerdictInfeasible RecipeVerdictLevel = "infeasible"
)

// RecipeVerdict is the full feasibility result for a recipe.
type RecipeVerdict struct {
	RecipeID string
	Lines    []LineVerdict
	Verdict  RecipeVerdictLevel
}
