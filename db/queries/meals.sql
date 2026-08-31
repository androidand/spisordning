-- name: CreateMealEvent :one
INSERT INTO meal_event (id, recipe_ref_id, served_on, meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: GetMealEvent :one
SELECT id, recipe_ref_id, served_on, meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind
FROM meal_event WHERE id = $1;

-- name: ListMealEvents :many
SELECT id, recipe_ref_id, served_on, meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind
FROM meal_event
WHERE recipe_ref_id = $1
ORDER BY served_on DESC;

-- name: AddMealReaction :exec
INSERT INTO meal_reaction (meal_event_id, person_id, sentiment)
VALUES ($1, $2, $3)
ON CONFLICT (meal_event_id, person_id) DO UPDATE
SET sentiment = EXCLUDED.sentiment;

-- name: ListMealReactions :many
SELECT meal_event_id, person_id, sentiment
FROM meal_reaction
WHERE meal_event_id = $1
ORDER BY person_id;

-- name: UpsertFavorite :exec
INSERT INTO favorite (scope_type, scope_id, recipe_ref_id, created_at)
VALUES ('person', $1, $2, $3)
ON CONFLICT (scope_type, scope_id, recipe_ref_id) DO NOTHING;

-- name: DeleteFavorite :exec
DELETE FROM favorite
WHERE scope_type = 'person' AND scope_id = $1 AND recipe_ref_id = $2;

-- name: ListFavoritesForRecipe :many
SELECT scope_type, scope_id, recipe_ref_id, created_at
FROM favorite
WHERE recipe_ref_id = $1
ORDER BY scope_id;

-- name: GetRecipeRating :one
SELECT
    COALESCE(AVG(sentiment), 0) AS avg_sentiment,
    COUNT(*) AS count
FROM meal_reaction mr
JOIN meal_event me ON mr.meal_event_id = me.id
WHERE me.recipe_ref_id = $1;
