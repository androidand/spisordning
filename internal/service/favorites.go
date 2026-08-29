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
	favs, err := s.db.ListFavoritesForRecipe(ctx, mealieRecipeID)
	if err != nil {
		return nil, fmt.Errorf("service: list favorites: %w", err)
	}
	out := make([]dto.FavoriteResponse, 0, len(favs))
	for _, f := range favs {
		out = append(out, toFavoriteDTO(f))
	}
	return out, nil
}

func (s *Favorites) GetRecipeRating(ctx context.Context, mealieRecipeID string) (dto.RecipeRatingResponse, error) {
	r, err := s.db.GetRecipeRating(ctx, mealieRecipeID)
	if err != nil {
		return dto.RecipeRatingResponse{}, fmt.Errorf("service: get recipe rating: %w", err)
	}
	return dto.RecipeRatingResponse{
		MealieRecipeID: r.MealieRecipeID,
		Average:        r.Average,
		ReviewCount:    r.ReviewCount,
	}, nil
}

func (s *Favorites) SetFavorite(ctx context.Context, mealieRecipeID string, in dto.SetFavoriteInput) error {
	if err := s.db.UpsertFavorite(ctx, in.PersonID, in.HouseholdID, mealieRecipeID); err != nil {
		return fmt.Errorf("service: set favorite: %w", err)
	}
	return nil
}

func (s *Favorites) UnsetFavorite(ctx context.Context, mealieRecipeID string, in dto.SetFavoriteInput) error {
	if err := s.db.DeleteFavorite(ctx, in.PersonID, in.HouseholdID, mealieRecipeID); err != nil {
		return fmt.Errorf("service: unset favorite: %w", err)
	}
	return nil
}

// toFavoriteDTO projects a persistence.Favorite to its wire DTO.
func toFavoriteDTO(f persistence.Favorite) dto.FavoriteResponse {
	out := dto.FavoriteResponse{
		ID:             f.ID,
		MealieRecipeID: f.MealieRecipeID,
		CreatedAt:      f.CreatedAt,
	}
	if f.PersonID != nil {
		out.PersonID = *f.PersonID
	}
	if f.HouseholdID != nil {
		out.HouseholdID = *f.HouseholdID
	}
	return out
}
