package dto

import (
	"context"
	"time"
)

// Ingredient is an HTTP-side view of a food from SLV or Dabas.
type Ingredient struct {
	ID        string `json:"id"`
	Display   string `json:"display"`
	Source    string `json:"source"`
	SlvNummer int    `json:"slv_nummer,omitempty"`
	GTIN      string `json:"gtin,omitempty"`
	Brand     string `json:"brand,omitempty"`
}

// IngredientNutrient is an HTTP-side view of a nutrient value.
type IngredientNutrient struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// IngredientProduct is an HTTP-side view of a product from Matpriskollen.
type IngredientProduct struct {
	Key         string `json:"key"`
	GTIN        string `json:"gtin"`
	Name        string `json:"name"`
	Brand       string `json:"brand"`
	Description string `json:"description"`
	Amount      string `json:"amount"`
	ImageURL    string `json:"image_url,omitempty"`
}

// IngredientMapping mirrors api/openapi.yaml components/schemas/IngredientMapping.
type IngredientMapping struct {
	MealieFoodID string    `json:"mealie_food_id"`
	IngredientID string    `json:"ingredient_id"`
	SourceName   string    `json:"source_name"`
	ExternalID   *string   `json:"external_id,omitempty"`
	NeedsReview  bool      `json:"needs_review"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// IngredientMappingResolve is the PATCH /ingredient-mappings/{ingredient} request body.
type IngredientMappingResolve struct {
	IngredientID    string   `json:"ingredient_id"`
	AcceptableForms []string `json:"acceptable_forms,omitempty"`
	PreferredForm   *string  `json:"preferred_form,omitempty"`
}

// IngredientsService is the surface the /ingredients handlers need.
type IngredientsService interface {
	SearchFood(ctx context.Context, query string, limit int) ([]Ingredient, error)
	LookupNutrition(ctx context.Context, nummer int) ([]IngredientNutrient, error)
	SearchDabas(ctx context.Context, query string) ([]IngredientProduct, error)
	SearchMatpriskollen(ctx context.Context, query string) ([]IngredientProduct, error)
	ResolveMapping(ctx context.Context, mealieFoodID string, in IngredientMappingResolve) (IngredientMapping, error)
}
