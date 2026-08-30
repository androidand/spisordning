package dto

import (
	"context"
	"time"
)

// FavoriteResponse is one explicit favorite marker on a recipe, person- or
// household-scoped. Exactly one of PersonID or HouseholdID is non-empty.
type FavoriteResponse struct {
	PersonID       string    `json:"person_id,omitempty"`
	HouseholdID    string    `json:"household_id,omitempty"`
	MealieRecipeID string    `json:"mealie_recipe_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// RecipeRatingResponse is the read-side aggregate of a recipe's meal reviews:
// the weighted average (1..5) and the number of reviews.
type RecipeRatingResponse struct {
	MealieRecipeID string  `json:"mealie_recipe_id"`
	Average        float64 `json:"average"`
	ReviewCount    int     `json:"review_count"`
}

// SetFavoriteInput is the body for POST /recipes/{id}/favorites. Exactly one of
// PersonID or HouseholdID must be set.
type SetFavoriteInput struct {
	PersonID    string `json:"person_id"`
	HouseholdID string `json:"household_id"`
}

// FavoritesService is the read/write surface the /recipes/{id}/favorites
// handlers need. Favorites are explicit markers — never derived from ratings or
// reactions.
type FavoritesService interface {
	ListFavoritesForRecipe(ctx context.Context, mealieRecipeID string) ([]FavoriteResponse, error)
	GetRecipeRating(ctx context.Context, mealieRecipeID string) (RecipeRatingResponse, error)
	SetFavorite(ctx context.Context, mealieRecipeID string, in SetFavoriteInput) error
	UnsetFavorite(ctx context.Context, mealieRecipeID string, in SetFavoriteInput) error
}
