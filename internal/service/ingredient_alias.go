package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/androidand/spisordning/internal/dto"
	"github.com/androidand/spisordning/internal/persistence"
)

// IngredientAlias implements dto.IngredientAliasService. It manages household
// nicknames for ingredients (the "configurable nickname matching" use case).
type IngredientAlias struct {
	db Store
}

// NewIngredientAlias returns an IngredientAlias service backed by db.
func NewIngredientAlias(db Store) *IngredientAlias { return &IngredientAlias{db: db} }

func (s *IngredientAlias) List(ctx context.Context, householdID string) ([]dto.IngredientAlias, error) {
	rows, err := s.db.ListIngredientAliases(ctx, householdID)
	if err != nil {
		return nil, fmt.Errorf("service: list ingredient aliases: %w", err)
	}
	out := make([]dto.IngredientAlias, 0, len(rows))
	for _, a := range rows {
		out = append(out, toAliasDTO(a))
	}
	return out, nil
}

func (s *IngredientAlias) Create(ctx context.Context, in dto.IngredientAliasNew) (dto.IngredientAlias, error) {
	alias := normalizeAlias(in.Alias)
	if alias == "" {
		return dto.IngredientAlias{}, fmt.Errorf("%w: alias is required", dto.ErrInvalidAlias)
	}
	if in.IngredientID == "" {
		return dto.IngredientAlias{}, fmt.Errorf("%w: ingredient_id is required", dto.ErrInvalidAlias)
	}
	pa := persistence.IngredientAlias{
		HouseholdID:  in.HouseholdID,
		Alias:        alias,
		IngredientID: in.IngredientID,
	}
	if err := s.db.UpsertIngredientAlias(ctx, pa); err != nil {
		return dto.IngredientAlias{}, fmt.Errorf("service: create ingredient alias: %w", err)
	}
	created, err := s.db.GetIngredientAlias(ctx, in.HouseholdID, alias)
	if err != nil {
		return dto.IngredientAlias{}, fmt.Errorf("service: create ingredient alias: %w", err)
	}
	return toAliasDTO(created), nil
}

func (s *IngredientAlias) Delete(ctx context.Context, householdID, alias string) error {
	if err := s.db.DeleteIngredientAlias(ctx, householdID, normalizeAlias(alias)); err != nil {
		return fmt.Errorf("service: delete ingredient alias: %w", err)
	}
	return nil
}

func (s *IngredientAlias) Resolve(ctx context.Context, householdID, alias string) (string, error) {
	id, err := s.db.ResolveIngredientAlias(ctx, householdID, normalizeAlias(alias))
	if err != nil {
		return "", fmt.Errorf("service: resolve ingredient alias: %w", err)
	}
	return id, nil
}

// normalizeAlias lowercases and trims a nickname so lookups are case- and
// whitespace-insensitive.
func normalizeAlias(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

func toAliasDTO(a persistence.IngredientAlias) dto.IngredientAlias {
	return dto.IngredientAlias{
		ID:           a.ID,
		HouseholdID:  a.HouseholdID,
		Alias:        a.Alias,
		IngredientID: a.IngredientID,
		CreatedAt:    a.CreatedAt,
	}
}
