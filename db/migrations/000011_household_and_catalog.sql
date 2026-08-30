-- +goose Up
-- Full household/catalog schema for establish-household-and-catalog.
--
-- This migration adds the tables that implement-household-and-catalog designs but that
-- migrations 0008 (household, product, product_identifier, product_ingredient_mapping) and
-- 0010 (household_membership, default-household backfill) already created or deferred.
--
-- Idempotent: uses CREATE TABLE IF NOT EXISTS and ALTER TABLE ... ADD COLUMN IF NOT EXISTS
-- so it can be run against a database that already has some of these tables/columns.


-- ── 1. Account ↔ Person shape (design.md Step 5½) ───────────────────────────

CREATE TABLE IF NOT EXISTS account (
    id            UUID PRIMARY KEY,
    -- Reserved slots for future auth columns (no real auth logic in this change).
    username      TEXT UNIQUE,            -- nullable: OIDC-only accounts may have none
    email         TEXT UNIQUE,
    password_hash TEXT,                   -- nullable: OIDC-only accounts may have none
    auth_method   TEXT NOT NULL DEFAULT 'NONE'
        CHECK (auth_method IN ('NONE', 'LOCAL', 'OIDC')),
    -- Optional reference to a Person; 0..1 per Person, N per Account (multi-household
    -- support deferred — see design.md §Step 5½).
    person_id     UUID UNIQUE REFERENCES person(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ
);

-- Add account_id to person if it does not already exist (0010 may have added it).
ALTER TABLE person
    ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES account(id) ON DELETE SET NULL;

-- ── 1½. Add the deferred FK on household_membership.ended_by ─────────────────
--     Migration 0010 created the column as UUID (the re-baselined type) because
--     `account` did not exist yet. Now that account is created above, add the
--     FK constraint so referential integrity is enforced. Uses a DO block with
--     exception handling for idempotency (PostgreSQL has no ADD CONSTRAINT
--     IF NOT EXISTS).
-- +goose StatementBegin
DO $$ BEGIN
    ALTER TABLE household_membership
        ADD CONSTRAINT household_membership_ended_by_fkey
            FOREIGN KEY (ended_by) REFERENCES account(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
-- +goose StatementEnd

-- ── 2. Person restrictions (design.md Step 5½½) ──────────────────────────────

CREATE TABLE IF NOT EXISTS person_restriction (
    person_id   UUID NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    tag         TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('ALLERGY', 'HARD_RESTRICTION')),
    note        TEXT,
    recorded_by UUID REFERENCES account(id),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    cleared_at  TIMESTAMPTZ,
    cleared_by  UUID REFERENCES account(id),
    PRIMARY KEY (person_id, tag, kind)
);
CREATE INDEX IF NOT EXISTS idx_person_restriction_person_cleared
    ON person_restriction (person_id, cleared_at);

-- ── 3. Ingredient canonicalization (design.md Step 3.3) ─────────────────────

ALTER TABLE ingredient
    ADD COLUMN IF NOT EXISTS merged_into_id UUID REFERENCES ingredient(id);
CREATE INDEX IF NOT EXISTS idx_ingredient_merged_into
    ON ingredient (merged_into_id);

-- ── 4. Ingredient forms (design.md Step 5½½) ────────────────────────────────

CREATE TABLE IF NOT EXISTS ingredient_form (
    ingredient_id UUID NOT NULL REFERENCES ingredient(id) ON DELETE CASCADE,
    form          TEXT NOT NULL,
    notes         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ingredient_id, form)
);

-- ── 5. Ingredient substitution (design.md Step 5½½½) ────────────────────────

CREATE TABLE IF NOT EXISTS ingredient_substitution (
    id                   UUID PRIMARY KEY,
    from_ingredient_id   UUID NOT NULL REFERENCES ingredient(id),
    from_form            TEXT,
    to_ingredient_id     UUID NOT NULL REFERENCES ingredient(id),
    to_form              TEXT,
    category             TEXT NOT NULL CHECK (category IN (
        'EQUIVALENT', 'GOOD', 'ACCEPTABLE', 'FORM', 'DIETARY', 'EMERGENCY'
    )),
    ratio                numeric(12,3) NOT NULL DEFAULT 1.0,
    retired_at           TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (from_ingredient_id, from_form, to_ingredient_id, to_form, category)
);
CREATE INDEX IF NOT EXISTS idx_ingredient_substitution_from
    ON ingredient_substitution (from_ingredient_id, from_form);
CREATE INDEX IF NOT EXISTS idx_ingredient_substitution_to
    ON ingredient_substitution (to_ingredient_id, to_form);

-- ── 6. Unit system (design.md Step 5½½½½) ───────────────────────────────────

CREATE TABLE IF NOT EXISTS unit (
    code      TEXT PRIMARY KEY,
    name      TEXT NOT NULL,
    dimension TEXT NOT NULL CHECK (dimension IN ('mass', 'volume', 'count'))
);

-- Seed the 11 units from PLAN.md.
INSERT INTO unit (code, name, dimension) VALUES
    ('g',      'gram',      'mass'),
    ('kg',     'kilogram',  'mass'),
    ('ml',     'milliliter','volume'),
    ('dl',     'deciliter', 'volume'),
    ('l',      'liter',     'volume'),
    ('piece',  'piece',     'count'),
    ('tbsp',   'tablespoon','volume'),
    ('tsp',    'teaspoon',  'volume'),
    ('pinch',  'pinch',     'volume'),
    ('package','package',   'count'),
    ('can',    'can',       'count')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS unit_conversion (
    from_unit  TEXT NOT NULL REFERENCES unit(code),
    to_unit    TEXT NOT NULL REFERENCES unit(code),
    factor     numeric(12,3) NOT NULL,
    PRIMARY KEY (from_unit, to_unit)
);
CREATE INDEX IF NOT EXISTS idx_unit_conversion_to
    ON unit_conversion (to_unit);

CREATE TABLE IF NOT EXISTS ingredient_unit_conversion (
    ingredient_id UUID NOT NULL REFERENCES ingredient(id),
    from_unit     TEXT NOT NULL REFERENCES unit(code),
    to_unit       TEXT NOT NULL REFERENCES unit(code),
    factor        numeric(12,3) NOT NULL,
    PRIMARY KEY (ingredient_id, from_unit, to_unit)
);

-- ── 6½. Enforce same-dimension constraint on unit_conversion ─────────────────
--     Invariant 9 forbids cross-dimension conversions on unit_conversion.
--     A CHECK constraint via a pure-SQL function enforces this at the DB level.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION same_dimension(a TEXT, b TEXT) RETURNS BOOLEAN AS $$
    SELECT u1.dimension = u2.dimension
    FROM unit u1, unit u2
    WHERE u1.code = a AND u2.code = b;
$$ LANGUAGE sql STABLE;
-- +goose StatementEnd

-- Idempotent via DO/EXCEPTION (PostgreSQL has no ADD CONSTRAINT IF NOT EXISTS).
-- +goose StatementBegin
DO $$ BEGIN
    ALTER TABLE unit_conversion
        ADD CONSTRAINT unit_conversion_same_dimension
            CHECK (same_dimension(from_unit, to_unit));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;
-- +goose StatementEnd

-- ── 7. Seed universal same-dimension conversions ────────────────────────────

-- Mass: kg ↔ g
INSERT INTO unit_conversion (from_unit, to_unit, factor) VALUES
    ('kg', 'g',  1000.0),
    ('g',  'kg', 0.001)
ON CONFLICT (from_unit, to_unit) DO NOTHING;

-- Volume: l ↔ dl ↔ ml, tbsp ↔ tsp (approximate, culinary convention)
INSERT INTO unit_conversion (from_unit, to_unit, factor) VALUES
    ('l',  'dl',  10.0),
    ('dl', 'l',   0.1),
    ('l',  'ml',  1000.0),
    ('ml', 'l',   0.001),
    ('dl', 'ml',  100.0),
    ('ml', 'dl',  0.1),
    ('tbsp','tsp', 3.0),
    ('tsp', 'tbsp', 1.0/3.0)
ON CONFLICT (from_unit, to_unit) DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS ingredient_unit_conversion CASCADE;
DROP TABLE IF EXISTS unit_conversion CASCADE;
DROP FUNCTION IF EXISTS same_dimension(TEXT, TEXT);
DROP TABLE IF EXISTS unit CASCADE;
DROP TABLE IF EXISTS ingredient_substitution CASCADE;
DROP TABLE IF EXISTS ingredient_form CASCADE;
ALTER TABLE ingredient DROP COLUMN IF EXISTS merged_into_id;
DROP TABLE IF EXISTS person_restriction CASCADE;
ALTER TABLE person DROP COLUMN IF EXISTS account_id;
DROP TABLE IF EXISTS account CASCADE;
