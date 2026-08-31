-- name: UpsertIngredientAlias :exec
INSERT INTO ingredient_alias (id, household_id, alias, ingredient_id, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (household_id, alias) DO UPDATE
SET ingredient_id = EXCLUDED.ingredient_id;

-- name: GetIngredientAlias :one
SELECT id, household_id, alias, ingredient_id, created_at
FROM ingredient_alias
WHERE household_id = $1 AND alias = $2;

-- name: ListIngredientAliases :many
SELECT id, household_id, alias, ingredient_id, created_at
FROM ingredient_alias
WHERE household_id = $1
ORDER BY alias;

-- name: DeleteIngredientAlias :exec
DELETE FROM ingredient_alias
WHERE household_id = $1 AND alias = $2;

-- name: ResolveIngredientAlias :one
SELECT ingredient_id
FROM ingredient_alias
WHERE household_id = $1 AND alias = $2;
