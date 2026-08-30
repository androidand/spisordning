-- +goose Up
-- Shopping list (retailer-independent, durable, human-editable).
--
-- This migration adds the two tables that back spisordning's own shopping list
-- (see openspec/changes/implement-shopping-and-commerce/design.md, "Concrete
-- schema (v1)"):
--   shopping_list         a durable list owned by spisordning (not a retailer's)
--   shopping_list_item    one line on a list: a requirement, an ingredient, or a
--                         free-text label, with quantity/unit and check-off state
--
-- This builds on (does not replace) the existing shopping_requirement table in
-- migrations/0001_init.sql: a list item MAY reference the shopping_requirement
-- it was seeded from, but the requirement itself is the planner's ephemeral,
-- per-plan output and is never mutated or duplicated by this migration (design
-- D1). Later migrations add the retailer-facing stages (retailer_list_binding,
-- shopping_cart, order) — see tasks 2.5, 3.4, 4.4.
--
-- Invariants enforced here (schema-level):
--   * A list item belongs to exactly one list (shopping_list_id FK, CASCADE).
--   * A list item's owner-person / requirement / ingredient refs are real or NULL.
--   * A list has no fully-empty item (CHECK: requirement OR ingredient OR label).
--   * A list's status is 'active' or 'archived' (CHECK).
--   * A requirement is seeded into a given list at most once (partial unique
--     index), so re-seeding is idempotent (design, "Seeding").
-- Invariants enforced in the application layer, NOT here:
--   * Seeding never mutates or duplicates shopping_requirement (design, 1.3).
--   * Manual add/remove/check-off semantics (design, 1.4).


-- ── Shopping lists (retailer-independent, durable) ──────────────────────────

-- A durable shopping list owned by spisordning, distinct from any retailer's own
-- list. `owner_person_id` attributes the list to a person; NULL means a shared
-- household-level list. When the household table lands
-- (establish-household-and-catalog) this becomes a household_id reference.
CREATE TABLE shopping_list (
    id              UUID PRIMARY KEY,
    owner_person_id UUID REFERENCES person(id) ON DELETE SET NULL,
    name            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Shopping list items ─────────────────────────────────────────────────────

-- One line on a list. At most one of the three identifiers below is the "what
-- is this" for the row (design D1): the shopping_requirement it was seeded
-- from, a direct ingredient, or a free-text label for non-ingredient items
-- (e.g. "paper towels"). No retailer product id — resolution is the adapter's
-- job.
CREATE TABLE shopping_list_item (
    id                      UUID PRIMARY KEY,
    shopping_list_id        UUID NOT NULL REFERENCES shopping_list(id) ON DELETE CASCADE,
    shopping_requirement_id UUID REFERENCES shopping_requirement(id) ON DELETE SET NULL,
    ingredient_id           UUID REFERENCES ingredient(id) ON DELETE SET NULL,
    label                   TEXT,
    quantity                numeric(12,3) NOT NULL,
    unit                    TEXT NOT NULL,
    checked                 BOOLEAN NOT NULL DEFAULT false,
    added_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- No fully-empty item: at least one identifier must be present (task 1.2).
    CHECK (shopping_requirement_id IS NOT NULL
           OR ingredient_id IS NOT NULL
           OR label IS NOT NULL)
);
CREATE INDEX ON shopping_list_item (shopping_list_id);

-- Idempotent seeding (design, "Seeding"): a shopping_requirement is seeded into
-- a given list at most once, so re-seeding the same requirement is a no-op.
-- Manual items (no shopping_requirement_id) are exempt from this index.
CREATE UNIQUE INDEX ON shopping_list_item (shopping_list_id, shopping_requirement_id)
    WHERE shopping_requirement_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS shopping_list_item CASCADE;
DROP TABLE IF EXISTS shopping_list CASCADE;
