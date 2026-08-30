package service

import (
	"context"
	"fmt"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/persistence"
)

// Favorites implements dto.FavoritesService. Favorites are explicit, person- or
// household-scoped markers on a recipe — created only by an explicit action,
// never derived from ratings or reactions.
type Favorites struct {
	db Store
}

// NewFavorites returns a Favorites service backed by db.
func NewFavorites(db Store) *Favorites { return &Favorites{db: db} }

func (s *Favorites) ListFavoritesForRecipe(ctx context.Context, mealieRecipeID string) ([]dto.FavoriteResponse, error) {
	ref, err := s.db.GetRecipeRefByMealieID(ctx, mealieRecipeID)
	if err != nil {
		return nil, fmt.Errorf("service: list favorites: %w", err)
	}
	favs, err := s.db.ListFavoritesForRecipe(ctx, ref.ID)
	if err != nil {
		return nil, fmt.Errorf("service: list favorites: %w", err)
	}
	out := make([]dto.FavoriteResponse, 0, len(favs))
	for _, f := range favs {
		out = append(out, toFavoriteDTO(f, ref.MealieRecipeID))
	}
	return out, nil
}

func (s *Favorites) GetRecipeRating(ctx context.Context, mealieRecipeID string) (dto.RecipeRatingResponse, error) {
	ref, err := s.db.GetRecipeRefByMealieID(ctx, mealieRecipeID)
	if err != nil {
		return dto.RecipeRatingResponse{}, fmt.Errorf("service: get recipe rating: %w", err)
	}
	r, err := s.db.GetRecipeRating(ctx, ref.ID)
	if err != nil {
		return dto.RecipeRatingResponse{}, fmt.Errorf("service: get recipe rating: %w", err)
	}
	return dto.RecipeRatingResponse{
		MealieRecipeID: ref.MealieRecipeID,
		Average:        r.Average,
		ReviewCount:    r.ReviewCount,
	}, nil
}

func (s *Favorites) SetFavorite(ctx context.Context, mealieRecipeID string, in dto.SetFavoriteInput) error {
	scopeType, scopeID, err := favoriteScope(in)
	if err != nil {
		return fmt.Errorf("service: set favorite: %w", err)
	}
	ref, err := s.db.GetRecipeRefByMealieID(ctx, mealieRecipeID)
	if err != nil {
		return fmt.Errorf("service: set favorite: %w", err)
	}
	if err := s.db.UpsertFavorite(ctx, scopeType, scopeID, ref.ID); err != nil {
		return fmt.Errorf("service: set favorite: %w", err)
	}
	return nil
}

func (s *Favorites) UnsetFavorite(ctx context.Context, mealieRecipeID string, in dto.SetFavoriteInput) error {
	scopeType, scopeID, err := favoriteScope(in)
	if err != nil {
		return fmt.Errorf("service: unset favorite: %w", err)
	}
	ref, err := s.db.GetRecipeRefByMealieID(ctx, mealieRecipeID)
	if err != nil {
		return fmt.Errorf("service: unset favorite: %w", err)
	}
	if err := s.db.DeleteFavorite(ctx, scopeType, scopeID, ref.ID); err != nil {
		return fmt.Errorf("service: unset favorite: %w", err)
	}
	return nil
}

// favoriteScope resolves a SetFavoriteInput to a (scope_type, scope_id) pair
// per D7: exactly one of PersonID or HouseholdID must be set.
func favoriteScope(in dto.SetFavoriteInput) (string, string, error) {
	if in.PersonID != "" && in.HouseholdID != "" {
		return "", "", fmt.Errorf("exactly one of person_id or household_id must be set")
	}
	if in.PersonID != "" {
		return "person", in.PersonID, nil
	}
	if in.HouseholdID != "" {
		return "household", in.HouseholdID, nil
	}
	return "", "", fmt.Errorf("exactly one of person_id or household_id must be set")
}

// toFavoriteDTO projects a persistence.Favorite to its wire DTO.
func toFavoriteDTO(f persistence.Favorite, mealieRecipeID string) dto.FavoriteResponse {
	out := dto.FavoriteResponse{
		MealieRecipeID: mealieRecipeID,
		CreatedAt:      f.CreatedAt,
	}
	if f.ScopeType == "person" {
		out.PersonID = f.ScopeID
	} else if f.ScopeType == "household" {
		out.HouseholdID = f.ScopeID
	}
	return out
}
