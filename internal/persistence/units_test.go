package persistence

import (
	"context"
	"testing"
)

// TestInvariant11_RegisterProductCreatesNoConversions reproduces the exact
// Grocy scenario from docs/research/grocy-units-and-planning.md: a product whose
// purchase unit differs from its stock unit. Grocy's trigger silently
// auto-inserted a wrong 1:1 conversion, which then collided when a correct
// factor was set afterward. design.md invariant 11 forbids that: RegisterProduct
// (CreateProduct) has NO side effect on the conversion tables; only
// DefineUnitConversion / DefineIngredientUnitConversion ever write a row.
func TestInvariant11_RegisterProductCreatesNoConversions(t *testing.T) {
	s := skipWithoutDB(t)
	ctx := context.Background()
	truncateTables(t, ctx, s, "ingredient_unit_conversion", "product", "ingredient")

	// Baseline: the migration's seeded universal conversions are present and
	// untouched by what follows.
	baseline, err := s.CountUnitConversions(ctx)
	if err != nil {
		t.Fatalf("CountUnitConversions (baseline): %v", err)
	}

	// An ingredient with a product bought by the package but weighed in stock —
	// the differing purchase/stock units that trigger Grocy's bug.
	if err := s.UpsertIngredient(ctx, Ingredient{ID: "flour", Display: "Flour"}); err != nil {
		t.Fatalf("UpsertIngredient: %v", err)
	}
	p := Product{ID: "p-flour-1kg", Name: "Flour 1kg", Brand: "Generic", PackageSize: "1kg"}
	if err := s.CreateProduct(ctx, p); err != nil {
		t.Fatalf("CreateProduct: %v", err)
	}

	// Invariant 11: no universal conversion row was auto-created on the side.
	if got, err := s.CountUnitConversions(ctx); err != nil {
		t.Fatalf("CountUnitConversions (post-register): %v", err)
	} else if got != baseline {
		t.Errorf("RegisterProduct changed universal conversion count %d -> %d, want unchanged", baseline, got)
	}

	// ...and no ingredient-specific conversion for the product's ingredient.
	if got, err := s.CountIngredientUnitConversions(ctx, "flour"); err != nil {
		t.Fatalf("CountIngredientUnitConversions: %v", err)
	} else if got != 0 {
		t.Errorf("RegisterProduct auto-created %d ingredient conversions for flour, want 0", got)
	}

	// The absence of a conversion is a valid, queryable state — not a silent
	// 1:1 default.
	if _, found, err := s.GetIngredientUnitConversion(ctx, "flour", "", "dl", "g"); err != nil {
		t.Fatalf("GetIngredientUnitConversion (absent): %v", err)
	} else if found {
		t.Error("found a dl->g conversion before any was explicitly defined")
	}

	// Explicitly defining a conversion is the only write path, and it stores the
	// real factor with no collision (Grocy's auto 1:1 row collided here).
	if err := s.DefineIngredientUnitConversion(ctx, "flour", "", "dl", "g", 60); err != nil {
		t.Fatalf("DefineIngredientUnitConversion: %v", err)
	}
	factor, found, err := s.GetIngredientUnitConversion(ctx, "flour", "", "dl", "g")
	if err != nil {
		t.Fatalf("GetIngredientUnitConversion (defined): %v", err)
	}
	if !found || factor != 60 {
		t.Errorf("dl->g = (%v, %v), want (60, true)", factor, found)
	}
}
