package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/persistence"
)

// Inspiration suggests recipes based on what is already in the pantry.
type Inspiration struct{ db Store }

// NewInspiration returns an Inspiration service backed by db.
func NewInspiration(db Store) *Inspiration { return &Inspiration{db: db} }

// Suggest ranks recipes by how much of their ingredients are already on hand.
// Recipes with at least one matched ingredient come first, ordered by match
// ratio (desc) then title (asc). Recipes with no matched ingredients are
// omitted — they are not "inspired" by the pantry.
func (s *Inspiration) Suggest(ctx context.Context) ([]dto.InspirationSuggestion, error) {
	pantryIDs, err := s.db.ListPantryIngredientIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: inspiration: %w", err)
	}
	pantrySet := make(map[string]bool, len(pantryIDs))
	for _, id := range pantryIDs {
		pantrySet[id] = true
	}

	recipeRefs, err := s.db.ListRecipeRefs(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: inspiration: %w", err)
	}
	refByID := make(map[string]persistence.RecipeRef, len(recipeRefs))
	for _, r := range recipeRefs {
		refByID[r.MealieRecipeID] = r
	}

	allLines, err := s.db.ListAllRecipeIngredients(ctx)
	if err != nil {
		return nil, fmt.Errorf("service: inspiration: %w", err)
	}

	// Group recipe ingredient lines by recipe.
	byRecipe := make(map[string][]string)
	for _, line := range allLines {
		byRecipe[line.MealieRecipeID] = append(byRecipe[line.MealieRecipeID], line.IngredientID)
	}

	var out []dto.InspirationSuggestion
	for recipeID, ingredientIDs := range byRecipe {
		ref, ok := refByID[recipeID]
		if !ok {
			continue // recipe ref not cached; skip
		}
		matched := make([]string, 0, len(ingredientIDs))
		missing := make([]string, 0, len(ingredientIDs))
		for _, id := range ingredientIDs {
			if pantrySet[id] {
				matched = append(matched, id)
			} else {
				missing = append(missing, id)
			}
		}
		if len(matched) == 0 {
			continue // nothing in common with the pantry
		}
		ratio := 0.0
		if len(ingredientIDs) > 0 {
			ratio = float64(len(matched)) / float64(len(ingredientIDs))
		}
		out = append(out, dto.InspirationSuggestion{
			MealieRecipeID:       recipeID,
			Title:                ref.Title,
			Tags:                 ref.Tags,
			Effort:               int(ref.Effort),
			TotalIngredients:     len(ingredientIDs),
			MatchedIngredientIDs: matched,
			MissingIngredientIDs: missing,
			MatchRatio:           ratio,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].MatchRatio != out[j].MatchRatio {
			return out[i].MatchRatio > out[j].MatchRatio
		}
		return out[i].Title < out[j].Title
	})
	return out, nil
}
