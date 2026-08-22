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

// Household is the unit that plans, cooks, and shops together. It owns member
// ship records (HouseholdMembership) but not the Person records themselves —
// a Person may exist without a membership (a former member, or a child without
// a login). Household is mutable (rename) and never hard-deleted while it has
// history.
type Household struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Account is a login identity. It is separate from Person — a Person may exist
// without an Account (a child), and an Account may exist without being linked
// to a Person (pending link). Real auth logic is a future change; this type
// reserves the domain shape.
type Account struct {
	ID          string
	Username    *string
	Email       *string
	AuthMethod  string // 'NONE' | 'LOCAL' | 'OIDC'
	PersonID    *string
	CreatedAt   time.Time
	LastLoginAt *time.Time
}

// PersonRestriction is a safety-critical allergy or hard restriction a Person
// holds against an ingredient or tag. It is categorical, not scored, and never
// derived from PreferenceObservation — only set and cleared by an explicit
// command with an attributed actor.
type PersonRestriction struct {
	PersonID    string
	Tag         string
	Kind        RestrictionKind
	Note        *string
	RecordedBy  *string // account id
	RecordedAt  time.Time
	ClearedAt   *time.Time
	ClearedBy   *string // account id
}

// RestrictionKind is the kind of a PersonRestriction.
type RestrictionKind string

const (
	RestrictionAllergy          RestrictionKind = "ALLERGY"
	RestrictionHardRestriction  RestrictionKind = "HARD_RESTRICTION"
)

// IngredientForm is a preparation/preservation state of an Ingredient (fresh,
// dried, canned, frozen) that changes how it's used and measured.
type IngredientForm struct {
	IngredientID string
	Form         string
	Notes        *string
	CreatedAt    time.Time
}

// IngredientSubstitutionCategory is the category of an IngredientSubstitution.
type IngredientSubstitutionCategory string

const (
	SubstitutionEquivalent IngredientSubstitutionCategory = "EQUIVALENT"
	SubstitutionGood       IngredientSubstitutionCategory = "GOOD"
	SubstitutionAcceptable IngredientSubstitutionCategory = "ACCEPTABLE"
	SubstitutionForm       IngredientSubstitutionCategory = "FORM"
	SubstitutionDietary    IngredientSubstitutionCategory = "DIETARY"
	SubstitutionEmergency  IngredientSubstitutionCategory = "EMERGENCY"
)

// IngredientSubstitution is a directed, categorized relationship from one
// Ingredient(+Form) to another, with a non-implied quantity ratio. Substitution
// is directional: A→B does not imply B→A.
type IngredientSubstitution struct {
	ID               int64
	FromIngredientID string
	FromForm         *string
	ToIngredientID   string
	ToForm           *string
	Category         IngredientSubstitutionCategory
	Ratio            float64
	RetiredAt        *time.Time
	CreatedAt        time.Time
}

// Unit is a universal, dimensioned measure (mass/volume/count).
type Unit struct {
	Code      string
	Name      string
	Dimension UnitDimension
}

// UnitDimension is the physical dimension of a Unit.
type UnitDimension string

const (
	UnitDimensionMass   UnitDimension = "mass"
	UnitDimensionVolume UnitDimension = "volume"
	UnitDimensionCount  UnitDimension = "count"
)

// UnitConversion is a universal conversion factor between two Units of the same
// dimension (e.g. kg↔g, l↔dl↔ml). It is never cross-dimension — mass↔volume
// conversions are ingredient-specific and live on IngredientUnitConversion.
type UnitConversion struct {
	FromUnit string
	ToUnit   string
	Factor   float64
}

// IngredientUnitConversion is an ingredient-specific cross-dimension conversion
// (e.g. dl flour → g). The system does not invent a universal density — if no
// row exists for an ingredient+unit pair, the conversion is undefined.
type IngredientUnitConversion struct {
	IngredientID string
	FromUnit     string
	ToUnit       string
	Factor       float64
}

// Product is a concrete, purchasable good ("Garant Kycklingfilé 900g"),
// household-facing and retailer-agnostic. Distinct from RetailerProduct /
// StoreOffer (Epic F), which attach a specific retailer SKU and price to a
// Product. A Product may be unmapped (no ProductIngredientMapping row), in
// which case it is flagged for review.
type Product struct {
	ID          string
	Name        string
	Brand       *string
	PackageSize *string
	Kind        *ProductKind
	CreatedAt   time.Time
}

// ProductKind classifies a Product.
type ProductKind string

const (
	ProductPackaged   ProductKind = "PACKAGED"
	ProductUnpackaged ProductKind = "UNPACKAGED"
	ProductManual     ProductKind = "MANUAL"
)

// ProductIngredientMapping links a Product to the canonical Ingredient(s) it
// represents. A Product may map to more than one Ingredient (e.g. a spice mix);
// a single Ingredient may be mapped from many Products (e.g. different brands
// of chicken breast).
type ProductIngredientMapping struct {
	ProductID    string
	IngredientID string
	Quantity     *float64 // how much of the ingredient one unit of the product yields
}
