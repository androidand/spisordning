package mcptools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ── Recipe-structuring tool input/output ────────────────────────────────────

// StructureRecipeInput is the input for the structure_recipe tool.
type StructureRecipeInput struct {
	// RawText is a freeform pasted recipe: a title line, a loose ingredient
	// list, and (optionally) a "Gör så här"/"Instruktioner"-style block of
	// steps. Shorthand quantities, brand asides, and missing units are all
	// expected — this is real home-cook input, not a strict format.
	RawText string `json:"raw_text"`
}

// StructuredIngredientResult is one ingredient line as actually written to
// the new Mealie recipe.
type StructuredIngredientResult struct {
	Note     string  `json:"note"`
	FoodName string  `json:"food_name,omitempty"`
	Quantity float64 `json:"quantity,omitempty"`
	Unit     string  `json:"unit,omitempty"`
}

// StructureRecipeResult is the output of the structure_recipe tool.
type StructureRecipeResult struct {
	RecipeID     string                       `json:"recipe_id"` // Mealie slug
	Title        string                       `json:"title"`
	Ingredients  []StructuredIngredientResult `json:"ingredients"`
	Instructions []string                     `json:"instructions"`
	// LowConfidence lists the original ingredient lines Mealie's parser
	// wasn't confident about (e.g. a bare "salt" with no quantity, or a
	// parenthetical aside) — surfaced so the caller tells the user rather
	// than presenting a guess as fact.
	LowConfidence []string `json:"low_confidence,omitempty"`
}

// ── Recipe-structuring service interface ────────────────────────────────────

// RecipeStructuringService turns freeform recipe text into a real, structured
// Mealie recipe.
type RecipeStructuringService interface {
	StructureRecipe(ctx context.Context, rawText string) (StructureRecipeResult, error)
}

// ── Recipe-structuring tool handler ─────────────────────────────────────────

func structureRecipeHandler(s RecipeStructuringService) mcp.ToolHandlerFor[StructureRecipeInput, StructureRecipeResult] {
	return func(ctx context.Context, _ *mcp.CallToolRequest, in StructureRecipeInput) (*mcp.CallToolResult, StructureRecipeResult, error) {
		if strings.TrimSpace(in.RawText) == "" {
			return nil, StructureRecipeResult{}, fmt.Errorf("structure_recipe: raw_text is required")
		}
		res, err := s.StructureRecipe(ctx, in.RawText)
		if err != nil {
			return nil, StructureRecipeResult{}, err
		}
		return nil, res, nil
	}
}
