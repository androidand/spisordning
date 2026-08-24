-- Pantry inventory: locations, lots, and the inventory event ledger.
-- See openspec/changes/implement-pantry-inventory/design.md (Step 7 persistence sketch,
-- amended by D8 graduated item specificity and D9 location taxonomy/hierarchy).

BEGIN;

-- unaccent is a built-in PG extension (part of the standard distribution, not
-- a third-party module) used for accent-insensitive product name matching in
-- ListCandidateProductsForIngredient. Available in postgres:16-alpine (CI).
CREATE EXTENSION IF NOT EXISTS unaccent;

-- A named place physical inventory sits, scoped to a household. location_type is an optional,
-- non-authoritative hint (D9, invariant 8) — never identity: two FRIDGE-typed rows are still
-- distinct locations. parent_location_id supports arbitrary-depth nesting; most locations are
-- expected to stay flat (NULL parent) — nesting is opt-in, not imposed (D9).
CREATE TABLE inventory_location (
    id                  TEXT PRIMARY KEY,
    household_id        TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    location_type       TEXT CHECK (location_type IN
                          ('CUPBOARD','DRAWER','FRIDGE','FREEZER','BASEMENT','BALCONY','BREADBOX','OTHER')),
    parent_location_id  TEXT REFERENCES inventory_location(id),
    archived_at         TIMESTAMPTZ
);
CREATE INDEX ON inventory_location (household_id);
CREATE INDEX ON inventory_location (parent_location_id);

-- The unit of physical inventory — never a products.current_quantity-style single field
-- (invariant 1). Always anchored to an Ingredient; product_id is nullable — a lot may exist at
-- ingredient-only specificity, refined to a specific Product later or never (D8, invariant 7).
-- This is a mutable, transactionally-maintained PROJECTION: every write happens in the same
-- transaction as the inventory_event causing it (D2, invariant 3) — application code is the
-- only thing enforcing that, not a trigger, matching this schema's existing style.
CREATE TABLE inventory_lot (
    id            BIGSERIAL PRIMARY KEY,
    ingredient_id TEXT NOT NULL REFERENCES ingredient(id),
    product_id    TEXT REFERENCES product(id),
    location_id   TEXT NOT NULL REFERENCES inventory_location(id),
    quantity      DOUBLE PRECISION NOT NULL,
    unit          TEXT NOT NULL,
    confidence    TEXT NOT NULL CHECK (confidence IN ('EXACT','LIKELY','ESTIMATED','UNKNOWN')),
    best_before   DATE,
    opened_at     TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON inventory_lot (location_id);
CREATE INDEX ON inventory_lot (ingredient_id);
-- Fast "show me all UNKNOWN lots" (design.md D3's stated query shape).
CREATE INDEX ON inventory_lot (confidence) WHERE confidence = 'UNKNOWN';

-- Append-only ledger (invariant 2: rows are never updated or deleted). Six kinds, not seven —
-- MARK_EMPTY collapses into CONSUME with source='mark_empty' (D7). Concrete typed FK columns
-- per kind's actual references, never a generic entity_type/entity_id/value shape (D5,
-- invariant 6). ingredient_id/product_id are only populated on PURCHASE, where they establish a
-- new lot's identity (product_id nullable per D8); other kinds reference the lot they mutate.
CREATE TABLE inventory_event (
    id                BIGSERIAL PRIMARY KEY,
    kind              TEXT NOT NULL CHECK (kind IN
                        ('PURCHASE','CONSUME','DISCARD','ADJUST','TRANSFER','OPEN')),
    lot_id            BIGINT REFERENCES inventory_lot(id),
    ingredient_id     TEXT REFERENCES ingredient(id),
    product_id        TEXT REFERENCES product(id),
    from_location_id  TEXT REFERENCES inventory_location(id),
    to_location_id    TEXT REFERENCES inventory_location(id),
    quantity_delta    DOUBLE PRECISION NOT NULL,
    reason            TEXT,
    source            TEXT NOT NULL,
    recorded_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON inventory_event (lot_id, recorded_at);

COMMIT;
