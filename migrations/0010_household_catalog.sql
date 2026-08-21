-- establish-household-and-catalog: the full household/catalog domain.
--
-- Builds on 0008_household_catalog_minimal.sql (which shipped household, product,
-- product_identifier, product_ingredient_mapping as FK targets for
-- implement-pantry-inventory). This migration adds the rest of that change's scope:
-- the Household/Person membership + Account split, the PersonRestriction model
-- (separate from person_preference), the IngredientForm / IngredientSubstitution
-- vocabulary, the Unit system (universal + ingredient-specific conversions), and
-- ingredient canonicalization (merged_into_id). See
-- openspec/changes/establish-household-and-catalog/design.md for the full
-- vocabulary → aggregates → relationships → lifecycle → commands → invariants
-- sequence this schema bridges.
--
-- Additive only: no existing table or column is renamed or dropped. `person` gains
-- an optional account_id FK (the Account/Person split, invariant 1) and `ingredient`
-- gains a nullable merged_into_id self-FK (canonicalization, Step 4). Existing
-- person rows are assigned to a default household + membership (task 1.4) without
-- touching person_preference / preference_observation.

BEGIN;

-- ── Household / Person / Account ────────────────────────────────────────────

-- The join between a Household and a Person, with a lifecycle (joined/left).
-- Append + close: a row is created on join and gets an ended_at on leaving; it is
-- never deleted (invariant 10 — "who was in the household when this meal was rated"
-- must stay answerable). At most one ACTIVE membership per person per household; a
-- person may re-join after leaving (a new row).
CREATE TABLE household_membership (
    id           BIGSERIAL PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    person_id    TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at     TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX ON household_membership (household_id, person_id) WHERE ended_at IS NULL;
CREATE INDEX ON household_membership (person_id);

-- A login identity (credential, email, auth), entirely outside the food domain
-- (invariant 1). Referenced by a Person (optional FK) but never owned by a
-- Household. Minimal — real auth is a separate future change; this only reserves
-- the shape.
CREATE TABLE account (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The Account/Person split (invariant 1): a Person may exist with no Account (a
-- child). Optional FK; deleting an Account never deletes a Person or their history
-- (ON DELETE SET NULL).
ALTER TABLE person ADD COLUMN account_id TEXT REFERENCES account(id) ON DELETE SET NULL;

-- ── Restrictions (separate from preferences) ─────────────────────────────────

-- An ALLERGY or HARD_RESTRICTION a Person holds against a tag. Categorical, never
-- scored, safety-critical (invariants 2 and 3). Deliberately a separate table from
-- person_preference: no sentiment/confidence column exists, so a restriction can
-- never be fed into preference scoring. Set and cleared only by explicit command;
-- each change is attributed (recorded_by/cleared_by) because it is safety-critical.
-- Cleared rows are kept (cleared_at set) for the audit trail, never deleted.
CREATE TABLE person_restriction (
    id          BIGSERIAL PRIMARY KEY,
    person_id   TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    tag         TEXT NOT NULL,
    kind        TEXT NOT NULL CHECK (kind IN ('ALLERGY', 'HARD_RESTRICTION')),
    note        TEXT,
    recorded_by TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    cleared_by  TEXT,
    cleared_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- At most one ACTIVE restriction per (person, tag, kind); a cleared one may be
-- re-set later (a new active row).
CREATE UNIQUE INDEX ON person_restriction (person_id, tag, kind) WHERE cleared_at IS NULL;
CREATE INDEX ON person_restriction (person_id);

-- ── Unit system ──────────────────────────────────────────────────────────────

-- A universal, dimensioned measure (invariant 9). Effectively immutable/seeded —
-- the 11 units PLAN.md lists ship here, not created via the app (Step 4).
CREATE TABLE unit (
    id         TEXT PRIMARY KEY,   -- 'g' | 'kg' | 'ml' | 'dl' | 'l' | 'piece' | 'tbsp' | 'tsp' | 'pinch' | 'package' | 'can'
    name       TEXT NOT NULL,
    dimension  TEXT NOT NULL CHECK (dimension IN ('mass', 'volume', 'count')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A universal conversion between two units of the SAME dimension (invariant 9:
-- kg↔g, l↔dl↔ml). factor is to = from * factor. Cross-dimension conversions
-- (volume→mass, e.g. "1 dl flour = 60 g") are NEVER here — they are
-- ingredient-specific and live in ingredient_unit_conversion. A universal
-- conversion row is written only via DefineUnitConversion (invariant 11); the
-- same-dimension rule is enforced in the domain layer (NewUnitConversion), not a
-- cross-table CHECK.
CREATE TABLE unit_conversion (
    id         BIGSERIAL PRIMARY KEY,
    from_unit  TEXT NOT NULL REFERENCES unit(id),
    to_unit    TEXT NOT NULL REFERENCES unit(id),
    factor     DOUBLE PRECISION NOT NULL CHECK (factor > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (from_unit, to_unit),
    CHECK (from_unit <> to_unit)
);

-- An ingredient-specific conversion, used for cross-dimension (volume→mass) where
-- no universal density exists (invariant 9). Scoped to one Ingredient and,
-- optionally, one of its forms (e.g. fresh vs. dried flour). Written only via
-- DefineIngredientUnitConversion (invariant 11).
CREATE TABLE ingredient_unit_conversion (
    id            BIGSERIAL PRIMARY KEY,
    ingredient_id TEXT NOT NULL REFERENCES ingredient(id) ON DELETE CASCADE,
    form          TEXT,
    from_unit     TEXT NOT NULL REFERENCES unit(id),
    to_unit       TEXT NOT NULL REFERENCES unit(id),
    factor        DOUBLE PRECISION NOT NULL CHECK (factor > 0),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- form is optional (NULL = form-agnostic); COALESCE keeps the unique key total.
CREATE UNIQUE INDEX ON ingredient_unit_conversion (ingredient_id, from_unit, to_unit, COALESCE(form, ''));

-- ── Ingredient forms & substitution ─────────────────────────────────────────

-- A preparation/preservation state of an Ingredient (fresh/dried/canned/frozen).
-- Belongs to exactly one Ingredient (invariant 6).
CREATE TABLE ingredient_form (
    id            BIGSERIAL PRIMARY KEY,
    ingredient_id TEXT NOT NULL REFERENCES ingredient(id) ON DELETE CASCADE,
    form          TEXT NOT NULL,
    notes         TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (ingredient_id, form)
);

-- A directed, categorized substitution from one Ingredient(+Form) to another,
-- with an explicit quantity ratio (invariants 7 and 8). A→B does NOT imply B→A;
-- the reverse, if valid, is a separate row. ratio is to's quantity per from's
-- quantity (never assumed 1:1). Retired rows are kept (retired = true) so past
-- recommendations stay explainable.
CREATE TABLE ingredient_substitution (
    id                 BIGSERIAL PRIMARY KEY,
    from_ingredient_id TEXT NOT NULL REFERENCES ingredient(id) ON DELETE CASCADE,
    from_form          TEXT,
    to_ingredient_id   TEXT NOT NULL REFERENCES ingredient(id) ON DELETE CASCADE,
    to_form            TEXT,
    category           TEXT NOT NULL CHECK (category IN ('EQUIVALENT', 'GOOD', 'ACCEPTABLE', 'FORM', 'DIETARY', 'EMERGENCY')),
    ratio              DOUBLE PRECISION NOT NULL CHECK (ratio > 0),
    retired            BOOLEAN NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- One edge per (from, from_form, to, to_form, category); COALESCE keeps form NULLs total.
CREATE UNIQUE INDEX ON ingredient_substitution (from_ingredient_id, COALESCE(from_form, ''), to_ingredient_id, COALESCE(to_form, ''), category);
CREATE INDEX ON ingredient_substitution (to_ingredient_id);

-- ── Ingredient canonicalization ─────────────────────────────────────────────

-- Marks a duplicate ingredient as merged into a canonical one (Step 4). A
-- nullable self-FK, not a delete: history and FKs from recipe_ingredient /
-- ingredient_mapping / new tables survive the merge. Canonicalization is an
-- explicit merge (MergeIngredients), never a drop.
ALTER TABLE ingredient ADD COLUMN merged_into_id TEXT REFERENCES ingredient(id) ON DELETE SET NULL;
CREATE INDEX ON ingredient (merged_into_id);

-- ── Seed data ────────────────────────────────────────────────────────────────

-- The 11 universal units PLAN.md lists, with an explicit dimension each (task 6.2).
INSERT INTO unit (id, name, dimension) VALUES
    ('g',       'gram',       'mass'),
    ('kg',      'kilogram',   'mass'),
    ('ml',      'milliliter', 'volume'),
    ('dl',      'deciliter',  'volume'),
    ('l',       'liter',      'volume'),
    ('piece',   'piece',      'count'),
    ('tbsp',    'tablespoon', 'count'),
    ('tsp',     'teaspoon',   'count'),
    ('pinch',   'pinch',      'count'),
    ('package', 'package',    'count'),
    ('can',     'can',        'count')
ON CONFLICT (id) DO NOTHING;

-- Universal same-dimension conversions (task 6.3). Only mass↔mass and
-- volume↔volume; no count-unit conversions (not universally convertible) and no
-- cross-dimension density (invariant 9). Explicit, curated seed data — not an
-- auto-created default (invariant 11).
INSERT INTO unit_conversion (from_unit, to_unit, factor) VALUES
    ('kg', 'g',  1000),
    ('g',  'kg', 0.001),
    ('l',  'dl', 10),
    ('dl', 'l',  0.1),
    ('l',  'ml', 1000),
    ('ml', 'l',  0.001),
    ('dl', 'ml', 100),
    ('ml', 'dl', 0.01)
ON CONFLICT (from_unit, to_unit) DO NOTHING;

-- ── Migrate existing person rows (task 1.4) ─────────────────────────────────

-- Assign every existing person to a default household + active membership,
-- without touching person_preference / preference_observation. Idempotent: a
-- person already in the default household is not re-added.
INSERT INTO household (id, name) VALUES ('default', 'Default Household')
ON CONFLICT (id) DO NOTHING;

INSERT INTO household_membership (household_id, person_id)
SELECT 'default', p.id
FROM person p
WHERE NOT EXISTS (
    SELECT 1 FROM household_membership hm
    WHERE hm.household_id = 'default' AND hm.person_id = p.id
);

COMMIT;
