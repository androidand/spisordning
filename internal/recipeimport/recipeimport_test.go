package recipeimport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// loadFixture reads a captured-recipe fixture from testdata/.
func loadFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// parseFixture runs the full extract+parse pipeline on a fixture HTML document.
func parseFixture(t *testing.T, name string) (ParsedRecipe, json.RawMessage) {
	t.Helper()
	node, err := ExtractRecipeJSONLD(loadFixture(t, name))
	if err != nil {
		t.Fatalf("ExtractRecipeJSONLD(%s): %v", name, err)
	}
	parsed, err := ParseRecipe(node)
	if err != nil {
		t.Fatalf("ParseRecipe(%s): %v", name, err)
	}
	return parsed, node
}

// TestExtractAndParseFixtures runs the generic pipeline against captured, real
// recipe pages (no live network). Each fixture exercises a distinct JSON-LD
// branch:
//   - ica.html:  single top-level Recipe node, reviewCount, string category.
//   - koket.html: [Corporation, Recipe] node array, array category/cuisine, and
//     a "Till ..." note line that must be filtered out of the ingredients.
//   - arls.html: string-typed rating values and nested HowToSection/HowToStep.
func TestExtractAndParseFixtures(t *testing.T) {
	tests := []struct {
		file        string
		title       string
		author      string
		servings    int
		totalSec    int
		prepSec     int
		cookSec     int
		rating      float64
		ratingCount int
		category    string
		cuisine     string
		nIngr       int
		nSteps      int
	}{
		{file: "ica.html", title: "Potatisgratäng (grundrecept)", author: "ICA Köket",
			servings: 6, totalSec: 5400, rating: 4.6, ratingCount: 422,
			category: "Buffé", nIngr: 7, nSteps: 5},
		{file: "koket.html", title: "Krämig potatisgratäng på gräddkokt potatis", author: "Nigella Lawson",
			servings: 10, totalSec: 2700, rating: 3.4, ratingCount: 671,
			category: "Fest", cuisine: "Sverige/Norden", nIngr: 7, nSteps: 5},
		{file: "arls.html", title: "Chokladkaka", author: "Carolina Soudah",
			servings: 12, totalSec: 2700, prepSec: 900, cookSec: 0,
			rating: 4.2, ratingCount: 266, category: "Dessert, Efterrätt", nIngr: 11, nSteps: 9},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			parsed, _ := parseFixture(t, tc.file)
			if parsed.Title != tc.title {
				t.Errorf("Title = %q, want %q", parsed.Title, tc.title)
			}
			if parsed.Attribution != tc.author {
				t.Errorf("Attribution = %q, want %q", parsed.Attribution, tc.author)
			}
			if parsed.Servings != tc.servings {
				t.Errorf("Servings = %d, want %d", parsed.Servings, tc.servings)
			}
			if parsed.TotalSec != tc.totalSec {
				t.Errorf("TotalSec = %d, want %d", parsed.TotalSec, tc.totalSec)
			}
			if parsed.PrepSec != tc.prepSec {
				t.Errorf("PrepSec = %d, want %d", parsed.PrepSec, tc.prepSec)
			}
			if parsed.CookSec != tc.cookSec {
				t.Errorf("CookSec = %d, want %d", parsed.CookSec, tc.cookSec)
			}
			if parsed.Rating != tc.rating {
				t.Errorf("Rating = %v, want %v", parsed.Rating, tc.rating)
			}
			if parsed.RatingCount != tc.ratingCount {
				t.Errorf("RatingCount = %d, want %d", parsed.RatingCount, tc.ratingCount)
			}
			if parsed.Category != tc.category {
				t.Errorf("Category = %q, want %q", parsed.Category, tc.category)
			}
			if parsed.Cuisine != tc.cuisine {
				t.Errorf("Cuisine = %q, want %q", parsed.Cuisine, tc.cuisine)
			}
			if len(parsed.Ingredients) != tc.nIngr {
				t.Errorf("len(Ingredients) = %d, want %d (note lines must be filtered)", len(parsed.Ingredients), tc.nIngr)
			}
			if len(parsed.Instructions) != tc.nSteps {
				t.Errorf("len(Instructions) = %d, want %d", len(parsed.Instructions), tc.nSteps)
			}
		})
	}
}

// TestExtractRecipeJSONLD_NoRecipe confirms the fallback trigger (2.3): an HTML
// doc with no Recipe node (or with no/invalid JSON-LD) yields an error, which is
// what routes a source to the per-site-parser path.
func TestExtractRecipeJSONLD_NoRecipe(t *testing.T) {
	cases := map[string]string{
		"no-ldjson":     `<html><head></head><body><h1>Recipe</h1></body></html>`,
		"non-recipe-ld": `<html><head><script type="application/ld+json">{"@context":"https://schema.org","@type":"Organization","name":"X"}</script></head><body></body></html>`,
		"invalid-json":  `<html><head><script type="application/ld+json">{not json</script></head><body></body></html>`,
	}
	for name, html := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ExtractRecipeJSONLD(html); err == nil {
				t.Fatalf("expected error for %s, got nil", name)
			}
		})
	}
}

// TestParseIngredientLine checks the conservative quantity/unit/food split.
// Every result is flagged NeedsReview; ambiguous lines keep their text whole.
func TestParseIngredientLine(t *testing.T) {
	tests := []struct {
		in       string
		quantity float64
		unit     string
		food     string
	}{
		{in: "200 g fast potatis", quantity: 200, unit: "g", food: "fast potatis"},
		{in: "5 dl vispgrädde", quantity: 5, unit: "dl", food: "vispgrädde"},
		{in: "2 1/2 dl vispgrädde", quantity: 2, unit: "", food: "1/2 dl vispgrädde"},
		{in: "3 vitlöksklyftor", quantity: 3, unit: "", food: "vitlöksklyftor"},
		{in: "1 gul lök, skalad", quantity: 1, unit: "", food: "gul lök, skalad"},
		{in: "salt och peppar", quantity: 0, unit: "", food: "salt och peppar"},
		{in: "50 g osaltat smör", quantity: 50, unit: "g", food: "osaltat smör"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			il := ParseIngredientLine(tc.in)
			if il.Quantity != tc.quantity {
				t.Errorf("Quantity = %v, want %v", il.Quantity, tc.quantity)
			}
			if il.Unit != tc.unit {
				t.Errorf("Unit = %q, want %q", il.Unit, tc.unit)
			}
			if il.Food != tc.food {
				t.Errorf("Food = %q, want %q", il.Food, tc.food)
			}
			if !il.NeedsReview {
				t.Errorf("NeedsReview = false, want true (import always flags for review)")
			}
			if il.RawText != tc.in {
				t.Errorf("RawText = %q, want %q", il.RawText, tc.in)
			}
		})
	}
}

// TestParseDuration checks ISO-8601 duration -> seconds conversion.
func TestParseDuration(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{in: "PT45M", want: 2700},
		{in: "PT90M", want: 5400},
		{in: "PT15M", want: 900},
		{in: "PT00M", want: 0},
		{in: "PT1H30M", want: 5400},
		{in: "P1D", want: 86400},
		{in: "PT1H", want: 3600},
		{in: "PT30S", want: 30},
		{in: "", want: 0},
		{in: "not-a-duration", want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := ParseDuration(tc.in); got != tc.want {
				t.Errorf("ParseDuration(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestParseYield checks leading-digit extraction from recipeYield values.
func TestParseYield(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{in: "6", want: 6},
		{in: "10 portioner", want: 10},
		{in: "12 bitar", want: 12},
		{in: "4-6 personer", want: 4},
		{in: "", want: 0},
		{in: "en stor form", want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := ParseYield(tc.in); got != tc.want {
				t.Errorf("ParseYield(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

// TestTrailingID checks the generic source-id heuristic.
func TestTrailingID(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "https://www.ica.se/recept/potatisgratang-grundrecept-721833/", want: "721833"},
		{in: "https://www.koket.se/nigella-lawson/gratanger/x/kramig-potatisgratang", want: ""},
		{in: "https://example.com/recipe/3967260", want: "3967260"},
		{in: "https://example.com/", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := TrailingID(tc.in); got != tc.want {
				t.Errorf("TrailingID(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestCandidateFromParsed confirms the import stage (7) produces a review
// candidate with provenance, assigns ingredient line numbers, and never
// promotes to cookbook content.
func TestCandidateFromParsed(t *testing.T) {
	src := Source{ID: "ica", Name: "ICA", Kind: "jsonld_web", LicenseNote: "personal use"}
	parsed, _ := parseFixture(t, "ica.html")
	c := CandidateFromParsed(src, "https://www.ica.se/recept/potatisgratang-grundrecept-721833/", parsed)
	if c.Status != StatusCandidate {
		t.Errorf("Status = %q, want %q", c.Status, StatusCandidate)
	}
	if c.SourceID != "ica" {
		t.Errorf("SourceID = %q, want %q", c.SourceID, "ica")
	}
	if c.ExternalID != "721833" {
		t.Errorf("ExternalID = %q, want %q", c.ExternalID, "721833")
	}
	if c.LicenseNote != "personal use" {
		t.Errorf("LicenseNote = %q, want %q", c.LicenseNote, "personal use")
	}
	if c.PromotedVariantID != "" {
		t.Errorf("PromotedVariantID = %q, want empty at import", c.PromotedVariantID)
	}
	for i, il := range c.Parsed.Ingredients {
		if il.LineNo != i+1 {
			t.Errorf("Ingredients[%d].LineNo = %d, want %d", i, il.LineNo, i+1)
		}
		if !il.NeedsReview {
			t.Errorf("Ingredients[%d].NeedsReview = false, want true", i)
		}
	}
}

// TestImportProducesCandidateNotCookbookContent enforces the review-before-
// cookbook invariant (6.3): the package's only creation entry point,
// CandidateFromParsed, always yields a StatusCandidate with an empty
// PromotedVariantID. This package has no code path that writes a
// RecipeFamily/RecipeVariant/RecipeRevision; promotion is a separate, explicit
// review action owned by the recipe-family-and-revisions change.
func TestImportProducesCandidateNotCookbookContent(t *testing.T) {
	for _, file := range []string{"ica.html", "koket.html", "arls.html"} {
		t.Run(file, func(t *testing.T) {
			src := Source{ID: "src", Name: "Src", Kind: "jsonld_web"}
			parsed, _ := parseFixture(t, file)
			c := CandidateFromParsed(src, "https://example.com/recept/x-123/", parsed)
			if c.Status != StatusCandidate {
				t.Fatalf("import must not auto-promote: Status = %q, want %q", c.Status, StatusCandidate)
			}
			if c.PromotedVariantID != "" {
				t.Fatalf("import must not reference a cookbook variant: PromotedVariantID = %q", c.PromotedVariantID)
			}
		})
	}
}
