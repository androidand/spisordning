package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/androidand/spisordning/internal/ingredients"
)

// TestIngredientsSeed verifies the curated seed for task 2.3: the small set of
// Swedish-unit → grams → package-size mappings, and the ByIngredientID lookup.
func TestIngredientsSeed(t *testing.T) {
	if len(ingredients.Seed) == 0 {
		t.Fatal("seed is empty — task 2.3 requires a curated set")
	}

	// Every row must carry the full Swedish-unit → grams → package-size chain.
	for _, m := range ingredients.Seed {
		if m.IngredientID == "" || m.Display == "" || m.Unit == "" {
			t.Errorf("incomplete mapping: %+v", m)
		}
		if m.GramsPerUnit <= 0 {
			t.Errorf("%s: grams_per_unit must be > 0, got %v", m.IngredientID, m.GramsPerUnit)
		}
		if m.PackageSizeGrams <= 0 {
			t.Errorf("%s: package size must be > 0, got %v", m.IngredientID, m.PackageSizeGrams)
		}
	}

	// Spot-check the dl → grams conversion (1 dl vetemjöl ≈ 60 g).
	m, ok := ingredients.ByIngredientID("vetemjol")
	if !ok {
		t.Fatal("vetemjol not found in seed")
	}
	if m.Unit != "dl" || m.GramsPerUnit != 60 || m.PackageSizeGrams != 1000 {
		t.Errorf("unexpected vetemjol mapping: %+v", m)
	}

	if _, ok := ingredients.ByIngredientID("does-not-exist"); ok {
		t.Error("ByIngredientID should report ok=false for unknown ids")
	}
}

// TestRunIngredientsOutput drives the review-surface command and asserts it
// renders the curated mappings and flags the ones that need review.
func TestRunIngredientsOutput(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	runIngredients()

	_ = w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)

	got := string(out)
	for _, want := range []string{"Vetemjöl", "dl", "Falukorv", "förp", "need(s) review"} {
		if !strings.Contains(got, want) {
			t.Errorf("output missing %q:\n%s", want, got)
		}
	}
}
