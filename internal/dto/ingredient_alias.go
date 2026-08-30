package dto

import (
	"context"
	"errors"
	"time"
)

// ErrInvalidAlias is returned when an alias or ingredient id is missing.
// Handlers map this to HTTP 400.
var ErrInvalidAlias = errors.New("invalid ingredient alias")

// IngredientAlias is the HTTP-side view of a household nickname → canonical
// ingredient mapping. HouseholdID "" means a global alias.
type IngredientAlias struct {
	ID           string    `json:"id"`
	HouseholdID  string    `json:"household_id,omitempty"`
	Alias        string    `json:"alias"`
	IngredientID string    `json:"ingredient_id"`
	CreatedAt    time.Time `json:"created_at"`
}

// IngredientAliasNew is the POST /ingredient-aliases request body. Alias is
// normalized (lowercased, trimmed) server-side. HouseholdID "" creates a global
// alias.
type IngredientAliasNew struct {
	HouseholdID  string `json:"household_id,omitempty"`
	Alias        string `json:"alias"`
	IngredientID string `json:"ingredient_id"`
}

// IngredientAliasService is the surface for managing household ingredient
// nicknames (the "configurable nickname matching" use case).
type IngredientAliasService interface {
	// List returns every alias for the household, including global aliases.
	List(ctx context.Context, householdID string) ([]IngredientAlias, error)
	// Create adds (or re-asserts) a nickname → ingredient mapping.
	Create(ctx context.Context, in IngredientAliasNew) (IngredientAlias, error)
	// Delete removes a nickname mapping.
	Delete(ctx context.Context, householdID, alias string) error
	// Resolve returns the canonical ingredient id for a nickname, or "" when
	// none matches.
	Resolve(ctx context.Context, householdID, alias string) (string, error)
}
