// Package domain holds the core value types for the Food Brain: the family,
// their preferences, and the meal candidates the planner scores. These types
// are deliberately free of persistence and transport concerns so the scorer
// can be exercised as pure functions.
package domain

import (
	"strings"
	"time"
)

// Sentiment is a person's directional feeling about a food or dish, on a small
// signed scale. It is combined with a Confidence in [0,1] before it influences
// a score, so a strong-but-unproven opinion never outweighs a well-observed one.
type Sentiment int

const (
	Hates    Sentiment = -2
	Dislikes Sentiment = -1
	Neutral  Sentiment = 0
	Likes    Sentiment = 1
	Loves    Sentiment = 2
)

// Person is a family member whose preferences shape the plan.
type Person struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Weight lets some people count for more in the aggregate (e.g. a picky
	// child whose buy-in matters most). Defaults to 1.0 when zero.
	Weight float64 `json:"weight"`
}

// EffectiveWeight returns Weight, defaulting to 1.0 when unset.
func (p Person) EffectiveWeight() float64 {
	if p.Weight <= 0 {
		return 1.0
	}
	return p.Weight
}

// Preference is one person's confidence-weighted sentiment toward a tag
// (an ingredient, cuisine, or dish trait such as "spicy" or "fish").
type Preference struct {
	PersonID   string   `json:"personId"`
	Tag        string   `json:"tag"`
	Sentiment  Sentiment `json:"sentiment"`
	Confidence float64  `json:"confidence"` // [0,1]; freshly-guessed = low, well-observed = high.
}

// Effort classifies how much active work a meal costs the cook.
type Effort int

const (
	EffortLow    Effort = 1 // assemble / reheat / sheet-pan
	EffortMedium Effort = 2 // a normal weeknight cook
	EffortHigh   Effort = 3 // project cooking
)

// Candidate is a recipe considered for a slot, carrying the denormalized
// metadata the scorer needs. Recipes remain owned by Mealie; MealieRecipeID is
// the reference back to the source of truth.
type Candidate struct {
	MealieRecipeID string
	Title          string
	// Tags are the same vocabulary preferences are expressed in.
	Tags []string
	// Ingredients are canonical ingredient ids (not retailer product ids).
	Ingredients []string
	Effort      Effort
}

// Ingredient is one structured line of a recipe's ingredient list in canonical
// form — the single canonical ingredient-line type shared by the shopping
// planner and the recipe hierarchy. IngredientID is the canonical ingredient id
// (see CanonicalIngredientID), not a source or retailer id; RawText keeps the
// human-readable line. AcceptableForms / PreferredForm are optional household
// recipe knowledge ("prefer fresh, accept frozen") that carries through to the
// shopping requirement so the retailer adapter can negotiate the form; most
// lines leave them empty.
type Ingredient struct {
	IngredientID    string
	Quantity        float64
	Unit            string
	RawText         string
	AcceptableForms []string
	PreferredForm   string
}

// CanonicalIngredientID derives the canonical ingredient id for a food name:
// lowercased and trimmed. Until the ingredient_mapping table refines ids, the
// canonical id of an unmapped food is its own normalized name.
func CanonicalIngredientID(foodName string) string {
	return strings.ToLower(strings.TrimSpace(foodName))
}

// PlanContext is everything about a given day/slot that is not the candidate
// itself: who is eating, what they prefer, how much energy the cook has, what
// was eaten recently, what the kids already had at school, and what is on
// campaign at the family's store this week.
type PlanContext struct {
	Day                 time.Time
	People              []Person
	Preferences         []Preference
	KitchenEnergy       Effort // the most effort the cook can spend today
	RecentMealIDs       []RecentMeal
	SchoolLunchTags     []string        // tags of dishes served at school today
	CampaignIngredients map[string]bool // canonical ingredient id -> on campaign
	// PantryAvailability maps a recipe's MealieRecipeID to its current
	// pantry availability status. Populated by the meal planner from the
	// recipe-availability capability. Empty map = no pantry data available
	// (the pantry dimension scores 0).
	PantryAvailability map[string]PantryStatus
	// AvailabilityVerdicts maps a recipe's mealie_recipe_id to the
	// availability verdict string ("feasible", "feasible-with-substitution",
	// or "infeasible"). Populated by the caller once the availability
	// capability has been evaluated; empty means no availability data is
	// available and the scorer falls back to effort-only feasibility.
	// Distinct from PantryAvailability (domain.PantryStatus-keyed): this is
	// the internal/availability package's own string-verdict shape, used by
	// scoring's hard-constraint feasibility() check. Reconciled 2026-08-25 -
	// both are kept since they're populated and consumed by different call
	// sites (see scoring.go: pantryScore reads PantryAvailability, feasibility
	// reads AvailabilityVerdicts).
	AvailabilityVerdicts map[string]string
}

// PantryStatus classifies whether a recipe can be made from the household's
// current inventory, accounting for substitutions. Owned by
// implement-recipe-availability; consumed read-only by the scorer.
type PantryStatus string

const (
	PantryFeasible          PantryStatus = "feasible"
	PantryFeasibleWithSub   PantryStatus = "feasible-with-substitution"
	PantryInfeasible        PantryStatus = "infeasible"
)

// RecentMeal records that a recipe was served on a day, used for the
// repetition penalty. More recent repeats are penalized harder.
type RecentMeal struct {
	MealieRecipeID string
	Served         time.Time
}

// ShoppingRequirement is the retailer-independent output of planning: a
// canonical ingredient plus an amount. It intentionally carries no retailer
// product id — resolving it to an actual product is the retailer adapter's
// job. It lives in domain because both the planner (application) and the
// retailer client (infrastructure) exchange it without either depending on
// the other's layer.
type ShoppingRequirement struct {
	IngredientID    string
	Quantity        float64
	Unit            string
	AcceptableForms []string
	PreferredForm   string
}

// Note: Household, Account, PersonRestriction, RestrictionKind, IngredientForm,
// IngredientSubstitution(Category), Unit(Dimension/Conversion), Product(Kind),
// and ProductIngredientMapping are NOT defined here. A branch written before
// establish-household-and-catalog landed re-declared all of these locally in
// this file; that owner has since landed richer, real versions in catalog.go
// (IngredientForm/IngredientSubstitution/Unit*/Product*) and household.go
// (Household/Account/PersonRestriction/RestrictionKind, both *string-typed
// for optional actor fields) - reconciled 2026-08-25 by dropping the
// duplicate block in favor of those real types rather than keeping two
// incompatible definitions of the same concepts.
