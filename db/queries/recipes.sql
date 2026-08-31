-- name: UpsertRecipeRef :exec
INSERT INTO recipe_ref (id, mealie_recipe_id, title, tags, effort, last_synced_at, raw_snapshot)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (mealie_recipe_id) DO UPDATE
SET title = EXCLUDED.title,
    tags = EXCLUDED.tags,
    effort = EXCLUDED.effort,
    last_synced_at = EXCLUDED.last_synced_at,
    raw_snapshot = EXCLUDED.raw_snapshot;

-- name: GetRecipeRef :one
SELECT id, mealie_recipe_id, title, tags, effort, last_synced_at, raw_snapshot
FROM recipe_ref WHERE id = $1;

-- name: GetRecipeRefByMealieID :one
SELECT id, mealie_recipe_id, title, tags, effort, last_synced_at, raw_snapshot
FROM recipe_ref WHERE mealie_recipe_id = $1;

-- name: ListRecipeRefs :many
SELECT id, mealie_recipe_id, title, tags, effort, last_synced_at, raw_snapshot
FROM recipe_ref
ORDER BY mealie_recipe_id;

-- name: AddRecipeIngredient :exec
INSERT INTO recipe_ingredient (recipe_ref_id, ingredient_id, quantity, unit)
VALUES ($1, $2, $3, $4)
ON CONFLICT (recipe_ref_id, ingredient_id) DO UPDATE
SET quantity = EXCLUDED.quantity,
    unit = EXCLUDED.unit;

-- name: ListRecipeIngredients :many
SELECT recipe_ref_id, ingredient_id, quantity, unit
FROM recipe_ingredient
WHERE recipe_ref_id = $1
ORDER BY ingredient_id;

-- name: ListAllRecipeIngredients :many
SELECT recipe_ref_id, ingredient_id, quantity, unit
FROM recipe_ingredient
ORDER BY recipe_ref_id, ingredient_id;

-- name: UpsertIngredient :exec
INSERT INTO ingredient (id, slug, display)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE
SET slug = EXCLUDED.slug,
    display = EXCLUDED.display;

-- name: UpsertIngredientMapping :exec
INSERT INTO ingredient_external_ref (provider, external_id, ingredient_id, grams_per_unit, default_form, needs_review, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (provider, external_id) DO UPDATE
SET ingredient_id = EXCLUDED.ingredient_id,
    grams_per_unit = EXCLUDED.grams_per_unit,
    default_form = EXCLUDED.default_form,
    needs_review = EXCLUDED.needs_review,
    updated_at = EXCLUDED.updated_at;

-- name: GetIngredientMapping :one
SELECT provider, external_id, ingredient_id, grams_per_unit, default_form, needs_review, updated_at
FROM ingredient_external_ref
WHERE provider = $1 AND external_id = $2;
