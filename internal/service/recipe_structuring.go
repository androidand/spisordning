package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/androidand/spisordning/internal/mealie"
)

// lowConfidenceThreshold: a line whose Mealie brute-parser confidence falls
// below this (or that never resolved a food name at all) gets surfaced to the
// caller as low_confidence rather than presented as a fact. 0.5 is a
// deliberately permissive cut — the goal is catching genuinely bad guesses
// (a bare "salt" with no quantity, "eller något annat?" asides), not
// second-guessing every imperfect parse.
const lowConfidenceThreshold = 0.5

// StructuredIngredient is one ingredient line as actually written to Mealie,
// reported back to the caller.
type StructuredIngredient struct {
	Note     string
	FoodName string
	Quantity float64
	Unit     string
}

// StructuredRecipe is the result of turning freeform recipe text into a real
// Mealie recipe.
type StructuredRecipe struct {
	RecipeID      string // Mealie slug
	Title         string
	Ingredients   []StructuredIngredient
	Instructions  []string
	LowConfidence []string // original ingredient notes the parser wasn't confident about
}

// StructureFromText turns a household member's freeform pasted recipe (title,
// loose ingredient list, "Gör så här"-style steps — see the worked example in
// this change's proposal.md) into a real Mealie recipe: sections the text,
// creates the recipe, and writes ingredients/instructions using the
// corruption-safe shapes internal/mealie.Client's write path already
// enforces. Best-effort on structuring quality — a line the parser can't
// confidently handle is still written (with whatever the parser recovered,
// or as a clean unstructured note) and reported via LowConfidence rather than
// silently presented as a correct guess.
func (s *Recipes) StructureFromText(ctx context.Context, rawText string) (StructuredRecipe, error) {
	if s.mealie == nil {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: no Mealie client configured")
	}

	sec := sectionRecipeText(rawText)
	if sec.Title == "" {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: no title found (expected a non-blank first line)")
	}
	if len(sec.IngredientLines) == 0 {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: no ingredient lines found between the title and the instructions marker")
	}

	slug, err := s.mealie.CreateRecipe(ctx, sec.Title)
	if err != nil {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: create %q: %w", sec.Title, err)
	}

	lines := make([]mealie.IngredientLine, len(sec.IngredientLines))
	for i, note := range sec.IngredientLines {
		lines[i] = mealie.IngredientLine{Note: note}
	}
	if err := s.mealie.SetIngredients(ctx, slug, lines); err != nil {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: set ingredients for %q: %w", slug, err)
	}

	if len(sec.InstructionSteps) > 0 {
		if err := s.mealie.SetInstructions(ctx, slug, sec.InstructionSteps); err != nil {
			return StructuredRecipe{}, fmt.Errorf("service: structure recipe: set instructions for %q: %w", slug, err)
		}
	}

	out := StructuredRecipe{
		RecipeID:     slug,
		Title:        sec.Title,
		Instructions: sec.InstructionSteps,
	}
	for _, l := range lines {
		out.Ingredients = append(out.Ingredients, StructuredIngredient{
			Note: l.Note, FoodName: l.FoodName, Quantity: l.Quantity, Unit: l.Unit,
		})
		if l.FoodName == "" || l.Confidence < lowConfidenceThreshold {
			out.LowConfidence = append(out.LowConfidence, l.Note)
		}
	}
	return out, nil
}

// sectionMarkers are the recognized "here come the steps" lines, matched
// case-insensitively against a trimmed line (with or without a trailing
// colon). Real household recipes are Swedish; a couple of common English
// variants are included since Andreas's own notes sometimes mix languages.
var sectionMarkers = []string{
	"gör så här",
	"gör såhär",
	"instruktioner",
	"tillagning",
	"instructions",
	"directions",
	"method",
}

// structuredText is a freeform recipe pasted as plain text, split into its
// three natural parts. It carries raw strings only — parsing ingredient
// quantities/units and creating the Mealie recipe are separate steps (see
// Recipes.StructureFromText).
type structuredText struct {
	Title            string
	IngredientLines  []string
	InstructionSteps []string
}

// sectionRecipeText splits raw freeform recipe text into title, ingredient
// lines, and instruction steps.
//
// Title is the first non-blank line. Ingredients are the non-blank lines
// between the title and a recognized marker line (sectionMarkers, e.g.
// "Gör så här:"). Instructions are every non-blank line after the marker —
// one step per line; a blank line is a pure separator, never merged into or
// splitting a step (a recipe author's blank line between paragraphs is a
// stylistic break, not a signal that two adjacent lines are actually one
// step).
//
// When no marker line is found, everything after the title is treated as
// ingredients (best-effort: a caller gets a recipe with no instructions
// rather than a hard failure — the same "never block on unstructured input"
// philosophy already used for ingredient parsing).
func sectionRecipeText(raw string) structuredText {
	lines := strings.Split(raw, "\n")

	var out structuredText
	i := 0
	// Title: first non-blank line.
	for ; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			out.Title = line
			i++
			break
		}
	}

	// Ingredients: non-blank lines up to (not including) a marker line.
	markerIdx := -1
	for j := i; j < len(lines); j++ {
		if isSectionMarker(lines[j]) {
			markerIdx = j
			break
		}
	}
	ingredientEnd := len(lines)
	if markerIdx >= 0 {
		ingredientEnd = markerIdx
	}
	for ; i < ingredientEnd; i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			out.IngredientLines = append(out.IngredientLines, line)
		}
	}

	// Instructions: one step per non-blank line after the marker.
	if markerIdx >= 0 {
		for j := markerIdx + 1; j < len(lines); j++ {
			line := strings.TrimSpace(lines[j])
			if line != "" {
				out.InstructionSteps = append(out.InstructionSteps, line)
			}
		}
	}

	return out
}

// isSectionMarker reports whether line (trimmed, trailing ":" stripped,
// lowercased) matches one of sectionMarkers exactly — a whole-line match, not
// a substring, so an ingredient like "1 msk metod-ost" doesn't accidentally
// trigger on "method".
func isSectionMarker(line string) bool {
	l := strings.ToLower(strings.TrimSpace(line))
	l = strings.TrimSuffix(l, ":")
	return slices.Contains(sectionMarkers, l)
}
