-- name: GetRecipeSourceRefBySource :one
SELECT id, recipe_family_id, source, source_recipe_id, imported_at, imported_by
FROM recipe_source_ref
WHERE source = $1 AND source_recipe_id = $2;

-- name: GetRecipeSourceRefByFamily :one
SELECT id, recipe_family_id, source, source_recipe_id, imported_at, imported_by
FROM recipe_source_ref
WHERE recipe_family_id = $1;

-- name: UpsertRecipeSourceRef :exec
INSERT INTO recipe_source_ref (id, recipe_family_id, source, source_recipe_id, imported_at, imported_by)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (source, source_recipe_id) DO UPDATE
SET recipe_family_id = EXCLUDED.recipe_family_id,
    imported_by = EXCLUDED.imported_by;

-- name: ListUnmappedMealieRecipes :many
SELECT rr.mealie_recipe_id
FROM recipe_ref rr
WHERE NOT EXISTS (
    SELECT 1 FROM recipe_source_ref rsr
    WHERE rsr.source = 'mealie' AND rsr.source_recipe_id = rr.mealie_recipe_id
)
ORDER BY rr.mealie_recipe_id;
