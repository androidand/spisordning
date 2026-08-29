package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/androidand/spisordning/internal/domain"
)

// RecipeFamily mirrors recipe_family (migrations/000003_recipe_family.sql).
type RecipeFamily struct {
	ID               string
	Name             string
	Description      string
	DefaultVariantID string // "" when unset
	Archived         bool
	CreatedAt        time.Time
}

// RecipeVariant mirrors recipe_variant.
type RecipeVariant struct {
	ID                string
	FamilyID          string
	Title             string
	SourceAttribution string
	Archived          bool
	CreatedAt         time.Time
}

// RecipeRevision mirrors recipe_revision. Ingredients is the JSONB array of
// domain.Ingredient; Steps is the JSONB array of strings.
type RecipeRevision struct {
	ID          int64
	VariantID   string
	Servings    int
	Description string
	Ingredients []domain.Ingredient
	Steps       []string
	CreatedAt   time.Time
}

// CreateRecipeFamily inserts a family.
func (s *Store) CreateRecipeFamily(ctx context.Context, f RecipeFamily) error {
	const q = `INSERT INTO recipe_family (id, name, description, archived)
		VALUES ($1, $2, $3, $4)`
	if _, err := s.db.Exec(ctx, q, f.ID, f.Name, f.Description, f.Archived); err != nil {
		return fmt.Errorf("persistence: create recipe family: %w", err)
	}
	return nil
}

// GetRecipeFamily fetches one family by id.
func (s *Store) GetRecipeFamily(ctx context.Context, id string) (RecipeFamily, error) {
	const q = `SELECT id, name, description,
		COALESCE(default_variant_id, ''), archived, created_at
		FROM recipe_family WHERE id = $1`
	var f RecipeFamily
	err := s.db.QueryRow(ctx, q, id).Scan(
		&f.ID, &f.Name, &f.Description, &f.DefaultVariantID, &f.Archived, &f.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RecipeFamily{}, fmt.Errorf("persistence: get recipe family %q: %w", id, pgx.ErrNoRows)
		}
		return RecipeFamily{}, fmt.Errorf("persistence: get recipe family: %w", err)
	}
	return f, nil
}

// ListRecipeFamilies returns all non-archived families ordered by name.
func (s *Store) ListRecipeFamilies(ctx context.Context) ([]RecipeFamily, error) {
	const q = `SELECT id, name, description,
		COALESCE(default_variant_id, ''), archived, created_at
		FROM recipe_family WHERE NOT archived ORDER BY name`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("persistence: list recipe families: %w", err)
	}
	defer rows.Close()
	var out []RecipeFamily
	for rows.Next() {
		var f RecipeFamily
		if err := rows.Scan(&f.ID, &f.Name, &f.Description, &f.DefaultVariantID, &f.Archived, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// SetRecipeFamilyDefaultVariant pins a family's default (expanded) variant. The
// variant must belong to the family (enforced in the application layer, not here).
func (s *Store) SetRecipeFamilyDefaultVariant(ctx context.Context, familyID, variantID string) error {
	const q = `UPDATE recipe_family SET default_variant_id = $2 WHERE id = $1`
	if _, err := s.db.Exec(ctx, q, familyID, variantID); err != nil {
		return fmt.Errorf("persistence: set recipe family default variant: %w", err)
	}
	return nil
}

// CreateRecipeVariant inserts a variant.
func (s *Store) CreateRecipeVariant(ctx context.Context, v RecipeVariant) error {
	const q = `INSERT INTO recipe_variant (id, family_id, title, source_attribution, archived)
		VALUES ($1, $2, $3, $4, $5)`
	if _, err := s.db.Exec(ctx, q, v.ID, v.FamilyID, v.Title, v.SourceAttribution, v.Archived); err != nil {
		return fmt.Errorf("persistence: create recipe variant: %w", err)
	}
	return nil
}

// GetRecipeVariant fetches one variant by id.
func (s *Store) GetRecipeVariant(ctx context.Context, id string) (RecipeVariant, error) {
	const q = `SELECT id, family_id, title, COALESCE(source_attribution, ''), archived, created_at
		FROM recipe_variant WHERE id = $1`
	var v RecipeVariant
	err := s.db.QueryRow(ctx, q, id).Scan(
		&v.ID, &v.FamilyID, &v.Title, &v.SourceAttribution, &v.Archived, &v.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RecipeVariant{}, fmt.Errorf("persistence: get recipe variant %q: %w", id, pgx.ErrNoRows)
		}
		return RecipeVariant{}, fmt.Errorf("persistence: get recipe variant: %w", err)
	}
	return v, nil
}

// ListRecipeVariants returns the variants of one family ordered by title.
func (s *Store) ListRecipeVariants(ctx context.Context, familyID string) ([]RecipeVariant, error) {
	const q = `SELECT id, family_id, title, COALESCE(source_attribution, ''), archived, created_at
		FROM recipe_variant WHERE family_id = $1 ORDER BY title`
	rows, err := s.db.Query(ctx, q, familyID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list recipe variants: %w", err)
	}
	defer rows.Close()
	var out []RecipeVariant
	for rows.Next() {
		var v RecipeVariant
		if err := rows.Scan(&v.ID, &v.FamilyID, &v.Title, &v.SourceAttribution, &v.Archived, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// CreateRecipeRevision inserts an immutable revision snapshot and returns its id.
// Ingredients and Steps are serialized to JSONB.
func (s *Store) CreateRecipeRevision(ctx context.Context, r RecipeRevision) (int64, error) {
	ingJSON, err := json.Marshal(r.Ingredients)
	if err != nil {
		return 0, fmt.Errorf("persistence: marshal revision ingredients: %w", err)
	}
	stepsJSON, err := json.Marshal(r.Steps)
	if err != nil {
		return 0, fmt.Errorf("persistence: marshal revision steps: %w", err)
	}
	const q = `INSERT INTO recipe_revision (variant_id, servings, description, ingredients, steps)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var id int64
	if err := s.db.QueryRow(ctx, q, r.VariantID, r.Servings, r.Description, ingJSON, stepsJSON).Scan(&id); err != nil {
		return 0, fmt.Errorf("persistence: create recipe revision: %w", err)
	}
	return id, nil
}

// GetRecipeRevision fetches one revision by id, decoding its JSONB content.
func (s *Store) GetRecipeRevision(ctx context.Context, id int64) (RecipeRevision, error) {
	const q = `SELECT id, variant_id, servings, COALESCE(description, ''), ingredients, steps, created_at
		FROM recipe_revision WHERE id = $1`
	var r RecipeRevision
	var ingJSON, stepsJSON []byte
	err := s.db.QueryRow(ctx, q, id).Scan(
		&r.ID, &r.VariantID, &r.Servings, &r.Description, &ingJSON, &stepsJSON, &r.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RecipeRevision{}, fmt.Errorf("persistence: get recipe revision %d: %w", id, pgx.ErrNoRows)
		}
		return RecipeRevision{}, fmt.Errorf("persistence: get recipe revision: %w", err)
	}
	if err := decodeRevisionContent(&r, ingJSON, stepsJSON); err != nil {
		return RecipeRevision{}, err
	}
	return r, nil
}

// ListRecipeRevisions returns every revision of one variant, newest first.
func (s *Store) ListRecipeRevisions(ctx context.Context, variantID string) ([]RecipeRevision, error) {
	const q = `SELECT id, variant_id, servings, COALESCE(description, ''), ingredients, steps, created_at
		FROM recipe_revision WHERE variant_id = $1 ORDER BY id DESC`
	rows, err := s.db.Query(ctx, q, variantID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list recipe revisions: %w", err)
	}
	defer rows.Close()
	var out []RecipeRevision
	for rows.Next() {
		var r RecipeRevision
		var ingJSON, stepsJSON []byte
		if err := rows.Scan(&r.ID, &r.VariantID, &r.Servings, &r.Description, &ingJSON, &stepsJSON, &r.CreatedAt); err != nil {
			return nil, err
		}
		if err := decodeRevisionContent(&r, ingJSON, stepsJSON); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AddRecipeRevisionParent records that child was derived from parent. The
// application layer (recipefamily.Graph) is responsible for the acyclicity check
// before calling this; the schema only enforces referential integrity.
func (s *Store) AddRecipeRevisionParent(ctx context.Context, child, parent int64) error {
	const q = `INSERT INTO recipe_revision_parent (revision_id, parent_revision_id)
		VALUES ($1, $2) ON CONFLICT DO NOTHING`
	if _, err := s.db.Exec(ctx, q, child, parent); err != nil {
		return fmt.Errorf("persistence: add recipe revision parent: %w", err)
	}
	return nil
}

// ListRecipeRevisionParents returns the direct parent revision ids of a revision.
func (s *Store) ListRecipeRevisionParents(ctx context.Context, revisionID int64) ([]int64, error) {
	const q = `SELECT parent_revision_id FROM recipe_revision_parent
		WHERE revision_id = $1 ORDER BY parent_revision_id`
	rows, err := s.db.Query(ctx, q, revisionID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list recipe revision parents: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var p int64
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// decodeRevisionContent decodes a revision's JSONB ingredients and steps.
func decodeRevisionContent(r *RecipeRevision, ingJSON, stepsJSON []byte) error {
	if len(ingJSON) > 0 {
		if err := json.Unmarshal(ingJSON, &r.Ingredients); err != nil {
			return fmt.Errorf("persistence: decode revision %d ingredients: %w", r.ID, err)
		}
	}
	if len(stepsJSON) > 0 {
		if err := json.Unmarshal(stepsJSON, &r.Steps); err != nil {
			return fmt.Errorf("persistence: decode revision %d steps: %w", r.ID, err)
		}
	}
	return nil
}
