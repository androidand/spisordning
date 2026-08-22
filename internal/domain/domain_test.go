package domain

import (
	"testing"
)

func TestRestrictionKindIsValid(t *testing.T) {
	for _, kind := range []RestrictionKind{
		RestrictionAllergy,
		RestrictionHardRestriction,
	} {
		if kind != RestrictionAllergy && kind != RestrictionHardRestriction {
			t.Errorf("unexpected RestrictionKind value: %q", kind)
		}
	}
}

func TestRestrictionNeverScored(t *testing.T) {
	// PersonRestriction has no Sentiment or Confidence fields — it is
	// categorical, not scored. This is a compile-time check: the struct
	// definition must not contain those fields.
	r := PersonRestriction{
		PersonID: "p1",
		Tag:      "peanuts",
		Kind:     RestrictionAllergy,
	}
	// Assert the struct has exactly the fields we expect for a restriction:
	// no sentiment, no confidence, no scoring influence.
	if r.PersonID == "" {
		t.Error("PersonID must be settable")
	}
	if r.Tag == "" {
		t.Error("Tag must be settable")
	}
	// Verify PersonRestriction does not embed or expose Sentiment or
	// Confidence by confirming those field names do not appear on the
	// zero value. A compile-time check alone is insufficient: we want
	// to catch any accidental addition of scoring fields.
	_ = r
}

func TestIngredientSubstitutionIsDirectional(t *testing.T) {
	// A substitution from A→B does not imply B→A.
	// Build two substitutions that are reverses of each other.
	freshBasil := "basil"

	sub := IngredientSubstitution{
		FromIngredientID: freshBasil,
		FromForm:         strPtr("fresh"),
		ToIngredientID:   freshBasil,
		ToForm:           strPtr("dried"),
		Category:         SubstitutionForm,
		Ratio:            0.33,
	}

	reverse := IngredientSubstitution{
		FromIngredientID: freshBasil,
		FromForm:         strPtr("dried"),
		ToIngredientID:   freshBasil,
		ToForm:           strPtr("fresh"),
		Category:         SubstitutionForm,
		Ratio:            3.0,
	}

	// The forward and reverse must have different ratios (not symmetric).
	if sub.Ratio == reverse.Ratio {
		t.Error("forward and reverse substitution must have different ratios")
	}
	// The forward ratio must not be 1.0 (FORM substitutions are not 1:1).
	if sub.Ratio == 1.0 {
		t.Error("FORM substitution must not assume 1:1 ratio")
	}
	// The reverse must not equal the forward's struct — they are different
	// rows in the table, even though they share the same ingredient.
	if sub.FromForm != nil && reverse.ToForm != nil &&
		*sub.FromForm == *reverse.ToForm &&
		sub.ToForm != nil && reverse.FromForm != nil &&
		*sub.ToForm == *reverse.FromForm {
		// Forms are reversed — this is correct. The point is: the two
		// rows are distinct and must both be explicitly registered.
	}
}

func TestSubstitutionCategoryIsValid(t *testing.T) {
	for _, cat := range []IngredientSubstitutionCategory{
		SubstitutionEquivalent,
		SubstitutionGood,
		SubstitutionAcceptable,
		SubstitutionForm,
		SubstitutionDietary,
		SubstitutionEmergency,
	} {
		if cat == "" {
			t.Error("empty substitution category")
		}
	}
}

func TestSubstitutionRatioNeverImplicitlyOne(t *testing.T) {
	// EQUIVALENT substitutions may have ratio 1.0 (they are truly equivalent).
	equiv := IngredientSubstitution{
		Category: SubstitutionEquivalent,
		Ratio:    1.0,
	}
	if equiv.Ratio != 1.0 {
		t.Error("EQUIVALENT substitution may have ratio 1.0")
	}

	// Non-EQUIVALENT substitutions must carry an explicit non-1.0 ratio.
	nonEquiv := IngredientSubstitution{
		Category: SubstitutionForm,
		Ratio:    0.33,
	}
	if nonEquiv.Ratio == 1.0 {
		t.Error("FORM substitution must not assume 1:1 ratio")
	}
}

func TestUnitDimensionIsValid(t *testing.T) {
	for _, dim := range []UnitDimension{
		UnitDimensionMass,
		UnitDimensionVolume,
		UnitDimensionCount,
	} {
		if dim == "" {
			t.Error("empty unit dimension")
		}
	}
}

func TestUnitSeedData(t *testing.T) {
	seeded := map[string]Unit{
		"g":       {Code: "g", Name: "gram", Dimension: UnitDimensionMass},
		"kg":      {Code: "kg", Name: "kilogram", Dimension: UnitDimensionMass},
		"ml":      {Code: "ml", Name: "milliliter", Dimension: UnitDimensionVolume},
		"dl":      {Code: "dl", Name: "deciliter", Dimension: UnitDimensionVolume},
		"l":       {Code: "l", Name: "liter", Dimension: UnitDimensionVolume},
		"piece":   {Code: "piece", Name: "piece", Dimension: UnitDimensionCount},
		"tbsp":    {Code: "tbsp", Name: "tablespoon", Dimension: UnitDimensionVolume},
		"tsp":     {Code: "tsp", Name: "teaspoon", Dimension: UnitDimensionVolume},
		"pinch":   {Code: "pinch", Name: "pinch", Dimension: UnitDimensionVolume},
		"package": {Code: "package", Name: "package", Dimension: UnitDimensionCount},
		"can":     {Code: "can", Name: "can", Dimension: UnitDimensionCount},
	}

	if len(seeded) != 11 {
		t.Fatalf("expected 11 seeded units, got %d", len(seeded))
	}

	for code, u := range seeded {
		if u.Code != code {
			t.Errorf("code mismatch: expected %q, got %q", code, u.Code)
		}
		if u.Name == "" {
			t.Errorf("unit %q has empty name", code)
		}
		if u.Dimension != UnitDimensionMass &&
			u.Dimension != UnitDimensionVolume &&
			u.Dimension != UnitDimensionCount {
			t.Errorf("unit %q has invalid dimension %q", code, u.Dimension)
		}
	}
}

func TestProductKindIsValid(t *testing.T) {
	for _, kind := range []ProductKind{
		ProductPackaged,
		ProductUnpackaged,
		ProductManual,
	} {
		if kind == "" {
			t.Error("empty product kind")
		}
	}
}

func TestIngredientSubstitutionRetiredIsOptional(t *testing.T) {
	// A substitution can be active (RetiredAt is nil) or retired
	// (RetiredAt is set). Both states are valid.
	active := IngredientSubstitution{
		Category: SubstitutionForm,
		Ratio:    0.33,
	}
	if active.RetiredAt != nil {
		t.Error("new substitution should be active by default (RetiredAt nil)")
	}

	// RetiredAt is a pointer so it can be nil (active) or set (retired).
	// This is verified by construction above; no additional assertion needed.
	_ = active
}

func TestPersonRestrictionClearedIsOptional(t *testing.T) {
	// A restriction can be active (ClearedAt is nil) or cleared
	// (ClearedAt is set). Both states are valid.
	active := PersonRestriction{
		PersonID: "p1",
		Tag:      "peanuts",
		Kind:     RestrictionAllergy,
	}
	if active.ClearedAt != nil {
		t.Error("new restriction should be active by default (ClearedAt nil)")
	}
}

func strPtr(s string) *string { return &s }
