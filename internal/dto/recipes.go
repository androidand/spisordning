package dto

import (
	"context"
	"time"
)

// RecipeRefResponse mirrors api/openapi.yaml components/schemas/RecipeRef.
type RecipeRefResponse struct {
	MealieRecipeID string    `json:"mealie_recipe_id"`
	Title          string    `json:"title"`
	Tags           []string  `json:"tags"`
	Effort         int       `json:"effort"` // 1..3
	LastSyncedAt   time.Time `json:"last_synced_at"`
}

// RecipesService is the read surface the /recipes handler needs.
type RecipesService interface {
	ListRecipes(ctx context.Context) ([]RecipeRefResponse, error)
	GetRecipe(ctx context.Context, id string) (RecipeRefResponse, error)
}
