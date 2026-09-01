package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/androidand/spisordning/internal/domain"
)

// RecipeSourceRef mirrors recipe_source_ref (migrations/000020_recipe_source_ref.sql).
// It maps a native recipe_family to an external source recipe (Mealie slug,
// structured-text import, discovery promotion, etc.).
type RecipeSourceRef struct {
	ID             domain.RecipeSourceRefID
	RecipeFamilyID domain.RecipeFamilyID
	Source         string
	SourceRecipeID string
	ImportedAt     time.Time
	ImportedBy     string
}

// GetRecipeSourceRefByFamily fetches the source ref for a family, if any.
func (s *Store) GetRecipeSourceRefByFamily(ctx context.Context, familyID domain.RecipeFamilyID) (RecipeSourceRef, error) {
	const q = `SELECT id, recipe_family_id, source, source_recipe_id, imported_at,
		COALESCE(imported_by, '') FROM recipe_source_ref WHERE recipe_family_id = $1`
	var r RecipeSourceRef
	err := s.db.QueryRow(ctx, q, familyID).Scan(
		&r.ID, &r.RecipeFamilyID, &r.Source, &r.SourceRecipeID, &r.ImportedAt, &r.ImportedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RecipeSourceRef{}, fmt.Errorf("persistence: get recipe source ref by family %q: %w", familyID, pgx.ErrNoRows)
		}
		return RecipeSourceRef{}, fmt.Errorf("persistence: get recipe source ref by family: %w", err)
	}
	return r, nil
}

// GetRecipeSourceRefBySource fetches the source ref for an external recipe id.
func (s *Store) GetRecipeSourceRefBySource(ctx context.Context, source, sourceRecipeID string) (RecipeSourceRef, error) {
	const q = `SELECT id, recipe_family_id, source, source_recipe_id, imported_at,
		COALESCE(imported_by, '') FROM recipe_source_ref WHERE source = $1 AND source_recipe_id = $2`
	var r RecipeSourceRef
	err := s.db.QueryRow(ctx, q, source, sourceRecipeID).Scan(
		&r.ID, &r.RecipeFamilyID, &r.Source, &r.SourceRecipeID, &r.ImportedAt, &r.ImportedBy,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RecipeSourceRef{}, fmt.Errorf("persistence: get recipe source ref %s/%q: %w", source, sourceRecipeID, pgx.ErrNoRows)
		}
		return RecipeSourceRef{}, fmt.Errorf("persistence: get recipe source ref by source: %w", err)
	}
	return r, nil
}

// UpsertRecipeSourceRef inserts or updates a source ref. The upsert is keyed on
// (source, source_recipe_id); if a family already has a different source ref,
// the old one is replaced (the UNIQUE(recipe_family_id) constraint is enforced
// by deleting the existing row first).
func (s *Store) UpsertRecipeSourceRef(ctx context.Context, r RecipeSourceRef) error {
	if r.ID == (domain.RecipeSourceRefID{}) {
		r.ID = domain.NewRecipeSourceRefID()
	}
	// Delete any existing ref for this family to satisfy UNIQUE(recipe_family_id).
	if _, err := s.db.Exec(ctx, `DELETE FROM recipe_source_ref WHERE recipe_family_id = $1`, r.RecipeFamilyID); err != nil {
		return fmt.Errorf("persistence: delete existing recipe source ref: %w", err)
	}
	const q = `INSERT INTO recipe_source_ref (id, recipe_family_id, source, source_recipe_id, imported_by)
		VALUES ($1, $2, $3, $4, $5)`
	importedBy := pgtype.Text{String: r.ImportedBy, Valid: r.ImportedBy != ""}
	if _, err := s.db.Exec(ctx, q, r.ID, r.RecipeFamilyID, r.Source, r.SourceRecipeID, importedBy); err != nil {
		return fmt.Errorf("persistence: upsert recipe source ref: %w", err)
	}
	return nil
}

// ListUnmappedMealieRecipes returns Mealie recipe slugs that have no
// recipe_source_ref row yet (for import progress tracking).
func (s *Store) ListUnmappedMealieRecipes(ctx context.Context) ([]string, error) {
	const q = `SELECT rr.mealie_recipe_id FROM recipe_ref rr
		LEFT JOIN recipe_source_ref rsr ON rsr.source = 'mealie'
			AND rsr.source_recipe_id = rr.mealie_recipe_id
		WHERE rsr.id IS NULL ORDER BY rr.mealie_recipe_id`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: list unmapped mealie recipes: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var slug string
		if err := rows.Scan(&slug); err != nil {
			return nil, err
		}
		out = append(out, slug)
	}
	return out, rows.Err()
}
