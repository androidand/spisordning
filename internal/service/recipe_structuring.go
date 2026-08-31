package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/mealie"
	"github.com/androidand/spisordning/internal/persistence"
)

// lowConfidenceThreshold: a line whose Mealie brute-parser confidence is at
// or below this (or that never resolved a food name at all) gets surfaced to
// the caller as low_confidence rather than presented as a fact. Verified
// against the live parser (task group 5): a bare single-word line with no
// quantity/unit (salt, svartpeppar, the "(eller något annat?)" aside)
// consistently scores exactly 0.5 average confidence, while a line with real
// quantity+unit structure (e.g. "500g kokt pasta") scores 0.75+ — so the
// threshold must be an inclusive <=, not <, or the bare-word case (the exact
// thing this is meant to catch) slips through uncaught.
const lowConfidenceThreshold = 0.5

// chatImportTag distinguishes recipes created via StructureFromText from the
// Middagsbank imports and manually-imported recipes already in Mealie.
const chatImportTag = "chat-import"

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
// this change's proposal.md) into a native recipe_family: sections the text,
// creates family + variant + revision, and registers a recipe_source_ref row.
// Best-effort on structuring quality — a line that can't be confidently
// parsed is still written (as raw text) and reported via LowConfidence.
//
// When the RECIPE_SOURCE flag is "mealie" (pre-migration default), this
// falls back to the legacy Mealie write path for backward compatibility.
func (s *Recipes) StructureFromText(ctx context.Context, rawText string) (StructuredRecipe, error) {
	sec := sectionRecipeText(rawText)
	if sec.Title == "" {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: no title found (expected a non-blank first line)")
	}
	if len(sec.IngredientLines) == 0 {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: no ingredient lines found between the title and the instructions marker")
	}

	mode := RecipeSourceModeFromEnv()
	if mode == SourceMealie {
		return s.structureFromTextMealie(ctx, sec)
	}
	return s.structureFromTextNative(ctx, sec)
}

// structureFromTextNative creates the recipe in recipe_family (P3 write cutover,
// design.md D4). No Mealie write occurs.
func (s *Recipes) structureFromTextNative(ctx context.Context, sec structuredText) (StructuredRecipe, error) {
	slug := slugify(sec.Title)

	// Conflict policy: link to existing family if slug already exists.
	family, err := s.db.GetRecipeFamilyBySlug(ctx, slug)
	if err != nil && !isNoRows(err) {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: lookup family: %w", err)
	}
	if isNoRows(err) {
		family = persistence.RecipeFamily{
			ID:   domain.NewRecipeFamilyID(),
			Slug: slug,
			Name: sec.Title,
		}
		if err := s.db.CreateRecipeFamily(ctx, family); err != nil {
			return StructuredRecipe{}, fmt.Errorf("service: structure recipe: create family: %w", err)
		}
	}

	variant := persistence.RecipeVariant{
		ID:                domain.NewRecipeVariantID(),
		Slug:              slug + "-default",
		FamilyID:          family.ID,
		Title:             sec.Title,
		SourceAttribution: "structured",
	}
	if err := s.db.CreateRecipeVariant(ctx, variant); err != nil {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: create variant: %w", err)
	}

	ingredients := make([]domain.Ingredient, 0, len(sec.IngredientLines))
	var lowConfidence []string
	for _, note := range sec.IngredientLines {
		name := domain.CanonicalIngredientID(note)
		ingredients = append(ingredients, domain.Ingredient{
			IngredientID: name,
			RawText:      note,
		})
		// Simple heuristic: bare single-word lines with no quantity are low-confidence.
		if len(strings.Fields(note)) <= 1 {
			lowConfidence = append(lowConfidence, note)
		}
	}

	revision := persistence.RecipeRevision{
		ID:          domain.NewRecipeRevisionID(),
		VariantID:   variant.ID,
		Ingredients: ingredients,
		Steps:       sec.InstructionSteps,
	}
	if _, err := s.db.CreateRecipeRevision(ctx, revision); err != nil {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: create revision: %w", err)
	}

	if err := s.db.SetRecipeFamilyDefaultVariant(ctx, family.ID, variant.ID); err != nil {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: set default variant: %w", err)
	}

	// Register source ref (source="structured").
	ref := persistence.RecipeSourceRef{
		RecipeFamilyID: family.ID,
		Source:         "structured",
		SourceRecipeID: slug,
		ImportedBy:     "structure-from-text",
	}
	if err := s.db.UpsertRecipeSourceRef(ctx, ref); err != nil {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: upsert source ref: %w", err)
	}

	out := StructuredRecipe{
		RecipeID:      slug,
		Title:         sec.Title,
		Instructions:  sec.InstructionSteps,
		LowConfidence: lowConfidence,
	}
	for _, ing := range ingredients {
		out.Ingredients = append(out.Ingredients, StructuredIngredient{
			Note:     ing.RawText,
			FoodName: ing.IngredientID,
		})
	}
	return out, nil
}

// structureFromTextMealie is the legacy Mealie write path, retained for
// backward compatibility when RECIPE_SOURCE=mealie.
func (s *Recipes) structureFromTextMealie(ctx context.Context, sec structuredText) (StructuredRecipe, error) {
	if s.mealie == nil {
		return StructuredRecipe{}, fmt.Errorf("service: structure recipe: no Mealie client configured")
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

	_ = s.mealie.SetTags(ctx, slug, []string{chatImportTag})

	out := StructuredRecipe{
		RecipeID:     slug,
		Title:        sec.Title,
		Instructions: sec.InstructionSteps,
	}
	for _, l := range lines {
		out.Ingredients = append(out.Ingredients, StructuredIngredient{
			Note: l.Note, FoodName: l.FoodName, Quantity: l.Quantity, Unit: l.Unit,
		})
		if l.FoodName == "" || l.Confidence <= lowConfidenceThreshold {
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
