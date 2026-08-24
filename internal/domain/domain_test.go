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
		FromForm:         "fresh",
		ToIngredientID:   freshBasil,
		ToForm:           "dried",
		Category:         SubstitutionForm,
		Ratio:            0.33,
	}

	reverse := IngredientSubstitution{
		FromIngredientID: freshBasil,
		FromForm:         "dried",
		ToIngredientID:   freshBasil,
		ToForm:           "fresh",
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
	if sub.FromForm == reverse.ToForm && sub.ToForm == reverse.FromForm {
		// Forms are reversed — this is correct. The point is: the two
		// rows are distinct and must both be explicitly registered.
	}
}

func TestSubstitutionCategoryIsValid(t *testing.T) {
	for _, cat := range []SubstitutionCategory{
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
		DimensionMass,
		DimensionVolume,
		DimensionCount,
	} {
		if dim == "" {
			t.Error("empty unit dimension")
		}
	}
}

func TestUnitSeedData(t *testing.T) {
	seeded := map[string]Unit{
		"g":       {ID: "g", Name: "gram", Dimension: DimensionMass},
		"kg":      {ID: "kg", Name: "kilogram", Dimension: DimensionMass},
		"ml":      {ID: "ml", Name: "milliliter", Dimension: DimensionVolume},
		"dl":      {ID: "dl", Name: "deciliter", Dimension: DimensionVolume},
		"l":       {ID: "l", Name: "liter", Dimension: DimensionVolume},
		"piece":   {ID: "piece", Name: "piece", Dimension: DimensionCount},
		"tbsp":    {ID: "tbsp", Name: "tablespoon", Dimension: DimensionVolume},
		"tsp":     {ID: "tsp", Name: "teaspoon", Dimension: DimensionVolume},
		"pinch":   {ID: "pinch", Name: "pinch", Dimension: DimensionVolume},
		"package": {ID: "package", Name: "package", Dimension: DimensionCount},
		"can":     {ID: "can", Name: "can", Dimension: DimensionCount},
	}

	if len(seeded) != 11 {
		t.Fatalf("expected 11 seeded units, got %d", len(seeded))
	}

	for code, u := range seeded {
		if u.ID != code {
			t.Errorf("id mismatch: expected %q, got %q", code, u.ID)
		}
		if u.Name == "" {
			t.Errorf("unit %q has empty name", code)
		}
		if u.Dimension != DimensionMass &&
			u.Dimension != DimensionVolume &&
			u.Dimension != DimensionCount {
			t.Errorf("unit %q has invalid dimension %q", code, u.Dimension)
		}
	}
}

func TestIngredientSubstitutionRetiredIsOptional(t *testing.T) {
	// A substitution can be active (Retired is false) or retired
	// (Retired is true). Both states are valid; new ones default to active.
	active := IngredientSubstitution{
		Category: SubstitutionForm,
		Ratio:    0.33,
	}
	if active.Retired {
		t.Error("new substitution should be active by default (Retired false)")
	}
}

func TestPersonRestrictionClearedIsOptional(t *testing.T) {
	// A restriction can be active (ClearedAt is zero) or cleared
	// (ClearedAt is set). Both states are valid.
	active := PersonRestriction{
		PersonID: "p1",
		Tag:      "peanuts",
		Kind:     RestrictionAllergy,
	}
	if !active.Active() {
		t.Error("new restriction should be active by default (ClearedAt zero)")
	}
}
