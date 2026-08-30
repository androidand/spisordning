package dto

import (
	"context"
	"time"
)

// MealReactionResponse mirrors api/openapi.yaml components/schemas/MealReaction.
type MealReactionResponse struct {
	PersonID  string `json:"person_id"`
	Sentiment int    `json:"sentiment"` // -2..2
}

// MealEventResponse mirrors api/openapi.yaml components/schemas/MealEvent.
type MealEventResponse struct {
	ID             string                 `json:"id"`
	MealieRecipeID string                 `json:"mealie_recipe_id"`
	ServedOn       string                 `json:"served_on"`  // date (date-only)
	CreatedAt      time.Time              `json:"created_at"` // rendered as RFC3339
	Reactions      []MealReactionResponse `json:"reactions"`
}

// MealEventNew is the POST /meals request body (api/openapi.yaml MealEventNew).
type MealEventNew struct {
	MealieRecipeID string              `json:"mealie_recipe_id"`
	ServedOn       string              `json:"served_on"` // date
	Reactions      []MealReactionInput `json:"reactions"`
}

// MealReactionInput is the request-side view of a reaction (person_id + sentiment).
type MealReactionInput struct {
	PersonID  string `json:"person_id"`
	Sentiment int    `json:"sentiment"` // -2..2
}

// MealsService is the surface the /meals handlers need.
type MealsService interface {
	CreateMealEvent(ctx context.Context, in MealEventNew) (MealEventResponse, error)
	GetMeal(ctx context.Context, id string) (MealEventResponse, error)
	ListMeals(ctx context.Context, mealieRecipeID, servedOn string) ([]MealEventResponse, error)
}
