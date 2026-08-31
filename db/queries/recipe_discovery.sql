-- name: GetExternalRecipeSource :one
SELECT id, name, kind, base_url, license_note, decision, enabled, created_at
FROM external_recipe_source WHERE id = $1;

-- name: UpsertExternalRecipeSource :exec
INSERT INTO external_recipe_source (id, name, kind, base_url, license_note, decision, enabled, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE
SET name = EXCLUDED.name,
    kind = EXCLUDED.kind,
    base_url = EXCLUDED.base_url,
    license_note = EXCLUDED.license_note,
    decision = EXCLUDED.decision,
    enabled = EXCLUDED.enabled;

-- name: SaveImportCandidate :one
INSERT INTO recipe_import_candidate (id, source_id, source_url, external_id, title, description, image_url,
    servings, prep_time_sec, cook_time_sec, total_time_sec, category, cuisine, attribution,
    rating, rating_count, license_note, imported_at, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
RETURNING id;

-- name: SaveCandidateIngredient :exec
INSERT INTO recipe_import_candidate_ingredient (candidate_id, line_no, raw_text, ingredient_id, quantity, unit, needs_review)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (candidate_id, line_no) DO UPDATE
SET raw_text = EXCLUDED.raw_text,
    ingredient_id = EXCLUDED.ingredient_id,
    quantity = EXCLUDED.quantity,
    unit = EXCLUDED.unit,
    needs_review = EXCLUDED.needs_review;

-- name: GetImportCandidate :one
SELECT id, source_id, source_url, external_id, title, description, image_url,
    servings, prep_time_sec, cook_time_sec, total_time_sec, category, cuisine, attribution,
    rating, rating_count, license_note, imported_at, status, promoted_variant_id
FROM recipe_import_candidate WHERE id = $1;

-- name: ListImportCandidates :many
SELECT id, source_id, source_url, external_id, title, description, image_url,
    servings, prep_time_sec, cook_time_sec, total_time_sec, category, cuisine, attribution,
    rating, rating_count, license_note, imported_at, status, promoted_variant_id
FROM recipe_import_candidate
WHERE ($1::text IS NULL OR status = $1)
ORDER BY imported_at DESC;

-- name: ListCandidateIngredients :many
SELECT candidate_id, line_no, raw_text, ingredient_id, quantity, unit, needs_review
FROM recipe_import_candidate_ingredient
WHERE candidate_id = $1
ORDER BY line_no;

-- name: SetCandidateStatus :exec
UPDATE recipe_import_candidate SET status = $2 WHERE id = $1;

-- name: SetCandidatePromoted :exec
UPDATE recipe_import_candidate
SET status = 'promoted', promoted_variant_id = $2
WHERE id = $1;
