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
	ID   string
	Name string
	// Weight lets some people count for more in the aggregate (e.g. a picky
	// child whose buy-in matters most). Defaults to 1.0 when zero.
	Weight float64
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
	PersonID   string
	Tag        string
	Sentiment  Sentiment
	Confidence float64 // [0,1]; freshly-guessed = low, well-observed = high.
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
}

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

// IngredientForm is one of the preservation/preparation states an ingredient
// can be in. Owned by establish-household-and-catalog (design.md Step 4) and
// consumed read-only by downstream capabilities such as recipe availability
// (task 3.2 of implement-recipe-availability).
type IngredientForm string

const (
	FormFresh  IngredientForm = "fresh"
	FormDried  IngredientForm = "dried"
	FormCanned IngredientForm = "canned"
	FormFrozen IngredientForm = "frozen"
)

// SubstitutionTier is the preference category of an IngredientSubstitution.
// Walked in decreasing order: EQUIVALENT first, EMERGENCY last. Owned by
// establish-household-and-catalog (design.md Step 3) and consumed read-only
// here; this capability does not define a parallel taxonomy.
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
// quantity). Owned by establish-household-and-catalog; consumed read-only by
// recipe availability. A nil FromForm/ToForm means "applies to any form."
// Retired rows are kept so past recommendations remain explainable.
type IngredientSubstitution struct {
	ID                 string
	FromIngredientID   string
	FromForm           *IngredientForm
	ToIngredientID     string
	ToForm             *IngredientForm
	Category           SubstitutionTier
	Ratio              float64 // to_qty per from_qty
	Retired            bool
}
