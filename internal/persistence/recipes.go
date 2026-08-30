package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/androidand/spisordning/internal/domain"
)

// RecipeRef mirrors migrations/0001_init.sql recipe_ref (Mealie is the source
// of truth; this is a cached reference, never an authoritative copy).
type RecipeRef struct {
	ID             domain.RecipeRefID
	MealieRecipeID string
	Title          string
	Tags           []string
	Effort         int // 1..3
	LastSyncedAt   time.Time
	RawSnapshot    string // JSONB as text; empty when no snapshot stored
}

// UpsertRecipeRef inserts a new reference or refreshes an existing one.
func (s *Store) UpsertRecipeRef(ctx context.Context, r RecipeRef) error {
	if r.ID == (domain.RecipeRefID{}) {
		r.ID = domain.NewRecipeRefID()
	}
	const q = `INSERT INTO recipe_ref (id, mealie_recipe_id, title, tags, effort, last_synced_at, raw_snapshot)
		VALUES ($1, $2, $3, $4, now(), $5)
		ON CONFLICT (mealie_recipe_id) DO UPDATE SET title = EXCLUDED.title,
			tags = EXCLUDED.tags, effort = EXCLUDED.effort, last_synced_at = now(),
			raw_snapshot = EXCLUDED.raw_snapshot`
	tags := r.Tags
	if tags == nil {
		tags = []string{}
	}
	raw := pgtype.Text{String: r.RawSnapshot, Valid: r.RawSnapshot != ""}
	if _, err := s.db.Exec(ctx, q, r.ID, r.MealieRecipeID, r.Title, tags, r.Effort, raw); err != nil {
		return fmt.Errorf("persistence: upsert recipe_ref: %w", err)
	}
	return nil
}

// GetRecipeRef fetches one reference by its UUID id.
func (s *Store) GetRecipeRef(ctx context.Context, id domain.RecipeRefID) (RecipeRef, error) {
	const q = `SELECT id, mealie_recipe_id, title, tags, effort, last_synced_at, raw_snapshot
		FROM recipe_ref WHERE id = $1`
	var r RecipeRef
	var tags []string
	var raw pgtype.Text
	if err := s.db.QueryRow(ctx, q, id).Scan(&r.ID, &r.MealieRecipeID, &r.Title, &tags, &r.Effort, &r.LastSyncedAt, &raw); err != nil {
		return RecipeRef{}, fmt.Errorf("persistence: get recipe_ref: %w", err)
	}
	r.Tags = tags
	r.RawSnapshot = raw.String
	return r, nil
}

// GetRecipeRefByMealieID fetches one reference by its Mealie external id.
func (s *Store) GetRecipeRefByMealieID(ctx context.Context, mealieRecipeID string) (RecipeRef, error) {
	const q = `SELECT id, mealie_recipe_id, title, tags, effort, last_synced_at, raw_snapshot
		FROM recipe_ref WHERE mealie_recipe_id = $1`
	var r RecipeRef
	var tags []string
	var raw pgtype.Text
	if err := s.db.QueryRow(ctx, q, mealieRecipeID).Scan(&r.ID, &r.MealieRecipeID, &r.Title, &tags, &r.Effort, &r.LastSyncedAt, &raw); err != nil {
		return RecipeRef{}, fmt.Errorf("persistence: get recipe_ref by mealie id: %w", err)
	}
	r.Tags = tags
	r.RawSnapshot = raw.String
	return r, nil
}

// ListRecipeRefs returns every cached reference, most-recently-synced first.
func (s *Store) ListRecipeRefs(ctx context.Context) ([]RecipeRef, error) {
	rows, err := s.db.Query(ctx, `SELECT id, mealie_recipe_id, title, tags, effort, last_synced_at, raw_snapshot
		FROM recipe_ref ORDER BY last_synced_at DESC, mealie_recipe_id`)
	if err != nil {
		return nil, fmt.Errorf("persistence: list recipe_refs: %w", err)
	}
	defer rows.Close()
	var out []RecipeRef
	for rows.Next() {
		var r RecipeRef
		var tags []string
		var raw pgtype.Text
		if err := rows.Scan(&r.ID, &r.MealieRecipeID, &r.Title, &tags, &r.Effort, &r.LastSyncedAt, &raw); err != nil {
			return nil, err
		}
		r.Tags = tags
		r.RawSnapshot = raw.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// Ingredient mirrors migrations/0001_init.sql ingredient (canonical id).
type Ingredient struct {
	ID      domain.IngredientID
	Display string
}

// UpsertIngredient inserts or updates a canonical ingredient.
func (s *Store) UpsertIngredient(ctx context.Context, i Ingredient) error {
	const q = `INSERT INTO ingredient (id, display) VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET display = EXCLUDED.display`
	if _, err := s.db.Exec(ctx, q, i.ID, i.Display); err != nil {
		return fmt.Errorf("persistence: upsert ingredient: %w", err)
	}
	return nil
}

// GetIngredient fetches one canonical ingredient by id.
func (s *Store) GetIngredient(ctx context.Context, id domain.IngredientID) (Ingredient, error) {
	var i Ingredient
	err := s.db.QueryRow(ctx, `SELECT id, display FROM ingredient WHERE id = $1`, id).Scan(&i.ID, &i.Display)
	if err != nil {
		return Ingredient{}, fmt.Errorf("persistence: get ingredient: %w", err)
	}
	return i, nil
}

// RecipeIngredient mirrors migrations/0001_init.sql recipe_ingredient.
type RecipeIngredient struct {
	RecipeRefID  domain.RecipeRefID
	IngredientID domain.IngredientID
	Quantity     float64
	Unit         string
}

// AddRecipeIngredient records one ingredient line of a recipe. Idempotent on
// the (recipe_ref_id, ingredient_id) primary key.
func (s *Store) AddRecipeIngredient(ctx context.Context, ri RecipeIngredient) error {
	const q = `INSERT INTO recipe_ingredient (recipe_ref_id, ingredient_id, quantity, unit)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (recipe_ref_id, ingredient_id) DO UPDATE SET quantity = EXCLUDED.quantity,
			unit = EXCLUDED.unit`
	if _, err := s.db.Exec(ctx, q, ri.RecipeRefID, ri.IngredientID, ri.Quantity, ri.Unit); err != nil {
		return fmt.Errorf("persistence: add recipe_ingredient: %w", err)
	}
	return nil
}

// ListRecipeIngredients returns a recipe's canonical ingredient lines.
func (s *Store) ListRecipeIngredients(ctx context.Context, recipeRefID domain.RecipeRefID) ([]RecipeIngredient, error) {
	rows, err := s.db.Query(ctx, `SELECT recipe_ref_id, ingredient_id, quantity, unit
		FROM recipe_ingredient WHERE recipe_ref_id = $1 ORDER BY ingredient_id`, recipeRefID)
	if err != nil {
		return nil, fmt.Errorf("persistence: list recipe_ingredients: %w", err)
	}
	defer rows.Close()
	var out []RecipeIngredient
	for rows.Next() {
		var ri RecipeIngredient
		if err := rows.Scan(&ri.RecipeRefID, &ri.IngredientID, &ri.Quantity, &ri.Unit); err != nil {
			return nil, err
		}
		out = append(out, ri)
	}
	return out, rows.Err()
}

// ListAllRecipeIngredients returns every recipe_ingredient row across all
// recipes. This is the read behind the "what can I make from my pantry"
// inspiration use case: the service joins it with the pantry's ingredient ids
// to score each recipe by how much of it is already on hand.
func (s *Store) ListAllRecipeIngredients(ctx context.Context) ([]RecipeIngredient, error) {
	rows, err := s.db.Query(ctx, `SELECT recipe_ref_id, ingredient_id, quantity, unit
		FROM recipe_ingredient ORDER BY recipe_ref_id, ingredient_id`)
	if err != nil {
		return nil, fmt.Errorf("persistence: list all recipe_ingredients: %w", err)
	}
	defer rows.Close()
	var out []RecipeIngredient
	for rows.Next() {
		var ri RecipeIngredient
		if err := rows.Scan(&ri.RecipeRefID, &ri.IngredientID, &ri.Quantity, &ri.Unit); err != nil {
			return nil, err
		}
		out = append(out, ri)
	}
	return out, rows.Err()
}

// IngredientMapping mirrors migrations/0001_init.sql ingredient_mapping.
type IngredientMapping struct {
	MealieFoodID string
	IngredientID domain.IngredientID
	GramsPerUnit float64
	DefaultForm  string
	NeedsReview  bool
	UpdatedAt    time.Time
}

// UpsertIngredientMapping inserts or refreshes a Mealie food id → canonical
// ingredient mapping. This is the review surface: a high `NeedsReview` mapping
// is what 2.6 exposes to be resolved.
func (s *Store) UpsertIngredientMapping(ctx context.Context, m IngredientMapping) error {
	const q = `INSERT INTO ingredient_mapping (mealie_food_id, ingredient_id, grams_per_unit, default_form, needs_review)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (mealie_food_id) DO UPDATE SET ingredient_id = EXCLUDED.ingredient_id,
			grams_per_unit = EXCLUDED.grams_per_unit, default_form = EXCLUDED.default_form,
			needs_review = EXCLUDED.needs_review, updated_at = now()`
	if _, err := s.db.Exec(ctx, q, m.MealieFoodID, m.IngredientID, m.GramsPerUnit, m.DefaultForm, m.NeedsReview); err != nil {
		return fmt.Errorf("persistence: upsert ingredient_mapping: %w", err)
	}
	return nil
}

// GetIngredientMapping fetches a mapping by mealie_food_id.
func (s *Store) GetIngredientMapping(ctx context.Context, mealieFoodID string) (IngredientMapping, error) {
	const q = `SELECT mealie_food_id, ingredient_id, grams_per_unit, default_form, needs_review, updated_at
		FROM ingredient_mapping WHERE mealie_food_id = $1`
	var m IngredientMapping
	if err := s.db.QueryRow(ctx, q, mealieFoodID).Scan(&m.MealieFoodID, &m.IngredientID, &m.GramsPerUnit, &m.DefaultForm, &m.NeedsReview, &m.UpdatedAt); err != nil {
		return IngredientMapping{}, fmt.Errorf("persistence: get ingredient_mapping: %w", err)
	}
	return m, nil
}

// ListNeedsReviewMappings returns mappings awaiting review — the input to the
// ingredient-mapping review surface (task 2.6).
func (s *Store) ListNeedsReviewMappings(ctx context.Context) ([]IngredientMapping, error) {
	rows, err := s.db.Query(ctx, `SELECT mealie_food_id, ingredient_id, grams_per_unit,
		default_form, needs_review, updated_at FROM ingredient_mapping WHERE needs_review = true ORDER BY mealie_food_id`)
	if err != nil {
		return nil, fmt.Errorf("persistence: list needs-review mappings: %w", err)
	}
	defer rows.Close()
	var out []IngredientMapping
	for rows.Next() {
		var m IngredientMapping
		if err := rows.Scan(&m.MealieFoodID, &m.IngredientID, &m.GramsPerUnit, &m.DefaultForm, &m.NeedsReview, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
