package domain

import "testing"

func TestNewUnit(t *testing.T) {
	if _, err := NewUnit("g", "gram", DimensionMass); err != nil {
		t.Errorf("NewUnit(g, mass): %v", err)
	}
	if _, err := NewUnit("dl", "deciliter", DimensionVolume); err != nil {
		t.Errorf("NewUnit(dl, volume): %v", err)
	}
	if _, err := NewUnit("piece", "piece", DimensionCount); err != nil {
		t.Errorf("NewUnit(piece, count): %v", err)
	}
	if _, err := NewUnit("", "gram", DimensionMass); err == nil {
		t.Error("expected an error for empty unit id")
	}
	if _, err := NewUnit("g", "gram", UnitDimension("temperature")); err == nil {
		t.Error("expected an error for an invalid dimension")
	}
}

func TestNewUnitConversion(t *testing.T) {
	kg, _ := NewUnit("kg", "kilogram", DimensionMass)
	g, _ := NewUnit("g", "gram", DimensionMass)
	l, _ := NewUnit("l", "liter", DimensionVolume)
	ml, _ := NewUnit("ml", "milliliter", DimensionVolume)

	// Same-dimension conversions are universal and allowed (invariant 9).
	if _, err := NewUnitConversion(kg, g, 1000); err != nil {
		t.Errorf("NewUnitConversion(kg→g): %v", err)
	}
	if _, err := NewUnitConversion(l, ml, 1000); err != nil {
		t.Errorf("NewUnitConversion(l→ml): %v", err)
	}

	// Cross-dimension conversions are rejected on the universal table — they are
	// ingredient-specific (invariant 9).
	if _, err := NewUnitConversion(g, ml, 1); err == nil {
		t.Error("expected an error for a cross-dimension universal conversion")
	}
	// A unit converting to itself is meaningless.
	if _, err := NewUnitConversion(kg, kg, 1); err == nil {
		t.Error("expected an error for a self conversion")
	}
	// A non-positive factor is invalid.
	if _, err := NewUnitConversion(kg, g, 0); err == nil {
		t.Error("expected an error for a zero factor")
	}
}

func TestNewIngredientUnitConversion(t *testing.T) {
	// Cross-dimension is exactly the point of an ingredient-specific conversion
	// (e.g. 1 dl flour = 60 g) — it must be allowed here, unlike the universal
	// table (invariant 9).
	if _, err := NewIngredientUnitConversion("flour", "fresh", "dl", "g", 60); err != nil {
		t.Errorf("NewIngredientUnitConversion(flour dl→g): %v", err)
	}
	// Form-agnostic (empty form) is valid.
	if _, err := NewIngredientUnitConversion("flour", "", "dl", "g", 60); err != nil {
		t.Errorf("NewIngredientUnitConversion(form-agnostic): %v", err)
	}
	if _, err := NewIngredientUnitConversion("", "fresh", "dl", "g", 60); err == nil {
		t.Error("expected an error for a missing ingredient id")
	}
	if _, err := NewIngredientUnitConversion("flour", "fresh", "dl", "g", 0); err == nil {
		t.Error("expected an error for a zero factor")
	}
}

func TestNewIngredientForm(t *testing.T) {
	if _, err := NewIngredientForm("basil", "fresh", ""); err != nil {
		t.Errorf("NewIngredientForm(basil, fresh): %v", err)
	}
	if _, err := NewIngredientForm("", "fresh", ""); err == nil {
		t.Error("expected an error for a missing ingredient id")
	}
	if _, err := NewIngredientForm("basil", "", ""); err == nil {
		t.Error("expected an error for a missing form name")
	}
}

func TestNewIngredientSubstitution(t *testing.T) {
	// A FORM substitution with an explicit, non-1:1 ratio (invariant 8).
	if _, err := NewIngredientSubstitution("basil", "fresh", "basil", "dried", SubstitutionForm, 0.33); err != nil {
		t.Errorf("NewIngredientSubstitution(basil fresh→dried): %v", err)
	}
	// A DIETARY substitution with no form involved.
	if _, err := NewIngredientSubstitution("chicken", "", "tofu", "", SubstitutionDietary, 1.0); err != nil {
		t.Errorf("NewIngredientSubstitution(chicken→tofu): %v", err)
	}

	if _, err := NewIngredientSubstitution("chicken", "", "chicken", "", SubstitutionEquivalent, 1.0); err == nil {
		t.Error("expected an error for a self substitution")
	}
	if _, err := NewIngredientSubstitution("chicken", "", "tofu", "", SubstitutionCategory("WEIRD"), 1.0); err == nil {
		t.Error("expected an error for an invalid category")
	}
	// A missing/zero ratio is rejected — quantity is never assumed 1:1 (invariant 8).
	if _, err := NewIngredientSubstitution("chicken", "", "tofu", "", SubstitutionDietary, 0); err == nil {
		t.Error("expected an error for a zero ratio")
	}
}

func TestSubstitutableIsDirectional(t *testing.T) {
	// A DIETARY substitution between two distinct ingredients.
	chickenToTofu, _ := NewIngredientSubstitution("chicken", "", "tofu", "", SubstitutionDietary, 1.0)
	edges := []IngredientSubstitution{chickenToTofu}

	// The curated direction is substitutable.
	if _, ok := Substitutable(edges, "chicken", "tofu"); !ok {
		t.Error("expected the curated direction to be substitutable")
	}

	// Invariant 7: an A→B edge does NOT make B→A substitutable. The reverse, if
	// valid, must be its own explicit row.
	if _, ok := Substitutable(edges, "tofu", "chicken"); ok {
		t.Error("did not expect an uncurated reverse to be substitutable")
	}

	// A retired edge is not substitutable.
	retired := chickenToTofu
	retired.Retired = true
	if _, ok := Substitutable([]IngredientSubstitution{retired}, "chicken", "tofu"); ok {
		t.Error("a retired edge should not be substitutable")
	}
}

func TestNewProduct(t *testing.T) {
	if _, err := NewProduct(NewProductID(), "Garant Kycklingfilé 900g", "Garant", "900g"); err != nil {
		t.Errorf("NewProduct: %v", err)
	}
	if _, err := NewProduct(ProductID{}, "x", "", ""); err == nil {
		t.Error("expected an error for a missing product id")
	}
	if _, err := NewProduct(NewProductID(), "", "", ""); err == nil {
		t.Error("expected an error for a missing product name")
	}
}
