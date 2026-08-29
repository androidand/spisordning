package dto

import "context"

// InspirationSuggestion is one recipe ranked by how much of it is already in
// the pantry. MatchedIngredientIDs are the recipe's ingredients that are on
// hand; MissingIngredientIDs are those that are not (so the UI can show what
// is still needed to cook it).
type InspirationSuggestion struct {
	MealieRecipeID       string   `json:"mealie_recipe_id"`
	Title                string   `json:"title"`
	Tags                 []string `json:"tags"`
	Effort               int      `json:"effort"`
	TotalIngredients     int      `json:"total_ingredients"`
	MatchedIngredientIDs []string `json:"matched_ingredient_ids"`
	MissingIngredientIDs []string `json:"missing_ingredient_ids"`
	MatchRatio           float64  `json:"match_ratio"` // [0,1]; 1 = fully cookable from pantry
}

// InspirationService is the surface the /inspiration handler needs.
type InspirationService interface {
	// Suggest returns recipes ranked by pantry coverage (most-matched first).
	Suggest(ctx context.Context) ([]InspirationSuggestion, error)
}
