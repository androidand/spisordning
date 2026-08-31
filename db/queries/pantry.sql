-- name: CreateInventoryLocation :exec
INSERT INTO inventory_location (id, slug, household_id, name, location_type, parent_location_id, archived_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetInventoryLocation :one
SELECT id, slug, household_id, name, location_type, parent_location_id, archived_at
FROM inventory_location WHERE id = $1;

-- name: ListInventoryLocations :many
SELECT id, slug, household_id, name, location_type, parent_location_id, archived_at
FROM inventory_location
WHERE location_type = $1 AND archived_at IS NULL
ORDER BY name;

-- name: ListLotsUnderLocation :many
SELECT id, ingredient_id, product_id, location_id, quantity, unit, confidence, best_before, opened_at, created_at, updated_at
FROM inventory_lot
WHERE location_id = $1
ORDER BY best_before ASC NULLS LAST;

-- name: GetInventoryLot :one
SELECT id, ingredient_id, product_id, location_id, quantity, unit, confidence, best_before, opened_at, created_at, updated_at
FROM inventory_lot WHERE id = $1;

-- name: ListExpiringLots :many
SELECT id, ingredient_id, product_id, location_id, quantity, unit, confidence, best_before, opened_at, created_at, updated_at
FROM inventory_lot
WHERE best_before IS NOT NULL
  AND best_before <= CURRENT_DATE + $1::int
ORDER BY best_before ASC;

-- name: ListPantryIngredientIDs :many
SELECT DISTINCT ingredient_id
FROM inventory_lot
WHERE quantity > 0
ORDER BY ingredient_id;

-- name: RecordConsume :exec
UPDATE inventory_lot
SET quantity = quantity - $2,
    updated_at = NOW()
WHERE id = $1;

-- name: RecordDiscard :exec
UPDATE inventory_lot
SET quantity = quantity - $2,
    updated_at = NOW()
WHERE id = $1;

-- name: RecordAdjust :exec
UPDATE inventory_lot
SET quantity = $2,
    updated_at = NOW()
WHERE id = $1;

-- name: RecordMarkEmpty :exec
UPDATE inventory_lot
SET quantity = 0,
    updated_at = NOW()
WHERE id = $1;

-- name: RecordOpen :exec
UPDATE inventory_lot
SET opened_at = NOW(),
    updated_at = NOW()
WHERE id = $1;
