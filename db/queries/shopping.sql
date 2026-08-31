-- name: CreateShoppingList :one
INSERT INTO shopping_list (id, owner_person_id, name, status, created_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: CreateShoppingListItem :exec
INSERT INTO shopping_list_item (id, shopping_list_id, shopping_requirement_id, ingredient_id, label, quantity, unit, checked, added_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListShoppingLists :many
SELECT id, owner_person_id, name, status, created_at
FROM shopping_list
ORDER BY created_at DESC;

-- name: ListShoppingListItems :many
SELECT id, shopping_list_id, shopping_requirement_id, ingredient_id, label, quantity, unit, checked, added_at
FROM shopping_list_item
WHERE shopping_list_id = $1
ORDER BY ingredient_id;

-- name: CreateOrUpdateRetailerListBinding :exec
INSERT INTO retailer_list_binding (shopping_list_id, retailer, external_list_id, sync_direction, last_pushed_at, last_push_status)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (shopping_list_id, retailer) DO UPDATE
SET external_list_id = EXCLUDED.external_list_id,
    sync_direction = EXCLUDED.sync_direction,
    last_pushed_at = EXCLUDED.last_pushed_at,
    last_push_status = EXCLUDED.last_push_status;
