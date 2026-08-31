-- name: GetOrCreateMealPlan :one
INSERT INTO meal_plan (id, week_start, status, created_at)
VALUES ($1, $2, 'draft', $3)
ON CONFLICT (week_start) DO UPDATE SET week_start = EXCLUDED.week_start
RETURNING id, week_start, status, created_at;

-- name: GetMealPlan :one
SELECT id, week_start, status, created_at
FROM meal_plan WHERE id = $1;

-- name: ListMealPlans :many
SELECT id, week_start, status, created_at
FROM meal_plan
ORDER BY week_start DESC;

-- name: SetMealPlanStatus :exec
UPDATE meal_plan SET status = $2 WHERE id = $1;

-- name: InsertCandidate :exec
INSERT INTO meal_plan_candidate (id, plan_id, slot_date, slot_kind, recipe_ref_id, score, breakdown, feasible, rank)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListCandidates :many
SELECT id, plan_id, slot_date, slot_kind, recipe_ref_id, score, breakdown, feasible, rank
FROM meal_plan_candidate
WHERE plan_id = $1
ORDER BY slot_date, rank;

-- name: SetDecision :exec
INSERT INTO meal_plan_decision (plan_id, slot_date, slot_kind, recipe_ref_id, decided_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (plan_id, slot_date, slot_kind) DO UPDATE
SET recipe_ref_id = EXCLUDED.recipe_ref_id,
    decided_at = EXCLUDED.decided_at;

-- name: ListDecisions :many
SELECT plan_id, slot_date, slot_kind, recipe_ref_id, decided_at
FROM meal_plan_decision
WHERE plan_id = $1
ORDER BY slot_date, slot_kind;

-- name: InsertShoppingRequirement :exec
INSERT INTO shopping_requirement (plan_id, ingredient_id, quantity, unit, acceptable_forms, preferred_form)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (plan_id, ingredient_id) DO UPDATE
SET quantity = EXCLUDED.quantity,
    unit = EXCLUDED.unit,
    acceptable_forms = EXCLUDED.acceptable_forms,
    preferred_form = EXCLUDED.preferred_form;

-- name: ListShoppingRequirements :many
SELECT plan_id, ingredient_id, quantity, unit, acceptable_forms, preferred_form
FROM shopping_requirement
WHERE plan_id = $1
ORDER BY ingredient_id;
