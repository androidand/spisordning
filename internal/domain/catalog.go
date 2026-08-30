package domain

import "fmt"

// UnitDimension is the physical dimension a Unit measures (invariant 9). A
// universal UnitConversion may only ever link units of the same dimension; any
// cross-dimension conversion is ingredient-specific, never a global density.
type UnitDimension string

const (
	DimensionMass   UnitDimension = "mass"
	DimensionVolume UnitDimension = "volume"
	DimensionCount  UnitDimension = "count"
)

// Valid reports whether the dimension is one of the three.
func (d UnitDimension) Valid() bool {
	return d == DimensionMass || d == DimensionVolume || d == DimensionCount
}

// Unit is a universal, dimensioned measure (g, kg, ml, dl, l, piece, tbsp, tsp,
// pinch, package, can). Effectively immutable/seeded — the units ship in a
// migration, not created via the app (Step 4).
type Unit struct {
	ID        string
	Name      string
	Dimension UnitDimension
}

// NewUnit validates a unit: it must have an id and an explicit dimension
// (invariant 9 — every unit carries a dimension).
func NewUnit(id, name string, dimension UnitDimension) (Unit, error) {
	if id == "" {
		return Unit{}, fmt.Errorf("domain: unit requires an id")
	}
	if !dimension.Valid() {
		return Unit{}, fmt.Errorf("domain: unit %q has invalid dimension %q", id, dimension)
	}
	return Unit{ID: id, Name: name, Dimension: dimension}, nil
}

// UnitConversion is a universal conversion between two units of the SAME
// dimension (invariant 9). Factor is to = from * Factor. Cross-dimension
// conversions are never here — they are ingredient-specific
// (IngredientUnitConversion). A universal conversion is written only via
// DefineUnitConversion (invariant 11), never auto-created.
type UnitConversion struct {
	FromUnit string
	ToUnit   string
	Factor   float64
}

// NewUnitConversion validates a universal conversion: the two units must be
// distinct and share a dimension (invariant 9), and the factor must be positive.
// This is the domain enforcement of "conversions never cross dimensions
// globally" — the same-dimension rule a cross-table CHECK cannot express.
func NewUnitConversion(from, to Unit, factor float64) (UnitConversion, error) {
	if from.ID == to.ID {
		return UnitConversion{}, fmt.Errorf("domain: unit conversion from %q to itself is meaningless", from.ID)
	}
	if from.Dimension != to.Dimension {
		return UnitConversion{}, fmt.Errorf("domain: universal conversion %q→%q crosses dimensions (%s→%s); cross-dimension conversions are ingredient-specific", from.ID, to.ID, from.Dimension, to.Dimension)
	}
	if factor <= 0 {
		return UnitConversion{}, fmt.Errorf("domain: conversion factor must be positive, got %g", factor)
	}
	return UnitConversion{FromUnit: from.ID, ToUnit: to.ID, Factor: factor}, nil
}

// IngredientUnitConversion is an ingredient-specific conversion, used for
// cross-dimension (volume→mass) where no universal density exists (invariant 9).
// Scoped to one Ingredient and, optionally, one of its forms. Written only via
// DefineIngredientUnitConversion (invariant 11).
type IngredientUnitConversion struct {
	IngredientID string
	// Form is optional ("" = form-agnostic).
	Form     string
	FromUnit string
	ToUnit   string
	Factor   float64
}

// NewIngredientUnitConversion validates an ingredient-specific conversion: it
// must name an ingredient and carry a positive factor. Unlike NewUnitConversion,
// it MAY cross dimensions — that is its purpose (e.g. 1 dl flour = 60 g).
func NewIngredientUnitConversion(ingredientID, form, fromUnit, toUnit string, factor float64) (IngredientUnitConversion, error) {
	if ingredientID == "" {
		return IngredientUnitConversion{}, fmt.Errorf("domain: ingredient unit conversion requires an ingredient id")
	}
	if factor <= 0 {
		return IngredientUnitConversion{}, fmt.Errorf("domain: conversion factor must be positive, got %g", factor)
	}
	return IngredientUnitConversion{IngredientID: ingredientID, Form: form, FromUnit: fromUnit, ToUnit: toUnit, Factor: factor}, nil
}

// IngredientForm is a preparation/preservation state of an Ingredient
// (fresh/dried/canned/frozen) that changes how it is used and measured. Belongs
// to exactly one Ingredient (invariant 6).
type IngredientForm struct {
	IngredientID string
	Form         string
	Notes        string
}

// NewIngredientForm validates that a form names both its ingredient and the form
// itself (invariant 6 — a form belongs to exactly one ingredient).
func NewIngredientForm(ingredientID, form, notes string) (IngredientForm, error) {
	if ingredientID == "" {
		return IngredientForm{}, fmt.Errorf("domain: ingredient form requires an ingredient id")
	}
	if form == "" {
		return IngredientForm{}, fmt.Errorf("domain: ingredient form requires a form name")
	}
	return IngredientForm{IngredientID: ingredientID, Form: form, Notes: notes}, nil
}

// SubstitutionCategory classifies why one ingredient may stand in for another
// (PLAN.md's candidate categories).
type SubstitutionCategory string

const (
	SubstitutionEquivalent SubstitutionCategory = "EQUIVALENT"
	SubstitutionGood       SubstitutionCategory = "GOOD"
	SubstitutionAcceptable SubstitutionCategory = "ACCEPTABLE"
	SubstitutionForm       SubstitutionCategory = "FORM"
	SubstitutionDietary    SubstitutionCategory = "DIETARY"
	SubstitutionEmergency  SubstitutionCategory = "EMERGENCY"
)

// Valid reports whether the category is one of the six.
func (c SubstitutionCategory) Valid() bool {
	switch c {
	case SubstitutionEquivalent, SubstitutionGood, SubstitutionAcceptable, SubstitutionForm, SubstitutionDietary, SubstitutionEmergency:
		return true
	}
	return false
}

// IngredientSubstitution is a directed, categorized relationship from one
// Ingredient(+Form) to another, with an explicit quantity ratio (invariants 7
// and 8). FromIngredientID→ToIngredientID does NOT imply the reverse; the
// reverse, if valid, is a separate row. Ratio is to's quantity per from's
// quantity and is never assumed 1:1. Retired edges are kept (Retired = true) so
// past recommendations stay explainable.
type IngredientSubstitution struct {
	FromIngredientID string
	FromForm         string // optional ("" = any form)
	ToIngredientID   string
	ToForm           string // optional ("" = any form)
	Category         SubstitutionCategory
	Ratio            float64
	Retired          bool
}

// NewIngredientSubstitution validates a substitution: a valid category
// (invariant 7's vocabulary) and an explicit positive ratio (invariant 8 — never
// assumed 1:1). A FORM substitution may target the same ingredient in a
// different form (fresh basil → dried basil), so the only meaningless case is the
// identical (ingredient, form) endpoint on both sides.
func NewIngredientSubstitution(fromID, fromForm, toID, toForm string, category SubstitutionCategory, ratio float64) (IngredientSubstitution, error) {
	if fromID == "" || toID == "" {
		return IngredientSubstitution{}, fmt.Errorf("domain: substitution requires both an from and a to ingredient")
	}
	if fromID == toID && fromForm == toForm {
		return IngredientSubstitution{}, fmt.Errorf("domain: substitution from %q/%q to itself is meaningless", fromID, fromForm)
	}
	if !category.Valid() {
		return IngredientSubstitution{}, fmt.Errorf("domain: invalid substitution category %q", category)
	}
	if ratio <= 0 {
		return IngredientSubstitution{}, fmt.Errorf("domain: substitution ratio must be positive and explicit, got %g", ratio)
	}
	return IngredientSubstitution{FromIngredientID: fromID, FromForm: fromForm, ToIngredientID: toID, ToForm: toForm, Category: category, Ratio: ratio}, nil
}

// Substitutable reports whether `from` can be substituted with `to` among the
// curated, non-retired substitution edges. It is strictly directional (invariant
// 7): an edge A→B does not make B→A substitutable. Returns the matching edge and
// true, or a zero edge and false.
func Substitutable(edges []IngredientSubstitution, from, to string) (IngredientSubstitution, bool) {
	for _, e := range edges {
		if e.Retired {
			continue
		}
		if e.FromIngredientID == from && e.ToIngredientID == to {
			return e, true
		}
	}
	return IngredientSubstitution{}, false
}

// Product is a concrete, purchasable good ("Garant Kycklingfilé 900g"),
// household-facing and retailer-agnostic. Distinct from the canonical Ingredient
// it may (optionally) map to (invariant 4): no brand/package data lives on
// Ingredient. RetailerProduct/StoreOffer (Epic F) attach a specific retailer SKU
// and price to a Product; they are out of scope here.
type Product struct {
	ID          ProductID
	Name        string
	Brand       string
	PackageSize string
}

// NewProduct validates a product's identity: it must have an id and a name. A
// product may be unmapped (no ProductIngredientMapping) — that is a valid,
// queryable "flagged for review" state, not an error (invariant 5).
func NewProduct(id ProductID, name, brand, packageSize string) (Product, error) {
	if id.String() == "00000000-0000-0000-0000-000000000000" {
		return Product{}, fmt.Errorf("domain: product requires a non-zero id")
	}
	if name == "" {
		return Product{}, fmt.Errorf("domain: product requires a name")
	}
	return Product{ID: id, Name: name, Brand: brand, PackageSize: packageSize}, nil
}
