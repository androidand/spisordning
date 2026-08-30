-- +goose Up
-- Food Brain initial schema.
--
-- One owner per domain: this database owns the FAMILY (people, preferences,
-- reactions, effort, plan decisions) and the CANONICAL shopping requirements.
-- Recipes are owned by Mealie and referenced by id + snapshot, never copied as
-- the source of truth. Retailer product ids never live on a recipe.


-- ── People & preferences ────────────────────────────────────────────────────

CREATE TABLE person (
    id          UUID PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    -- Some people count for more in the aggregate (a picky child's buy-in).
    weight      numeric(6,3) NOT NULL DEFAULT 1.0 CHECK (weight > 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Current, confidence-weighted sentiment toward a tag (ingredient/cuisine/trait).
-- This is the derived "belief"; the raw evidence lives in preference_observation.
CREATE TABLE person_preference (
    person_id   UUID NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    tag         TEXT NOT NULL,
    sentiment   SMALLINT NOT NULL CHECK (sentiment BETWEEN -2 AND 2),
    confidence  numeric(5,4) NOT NULL DEFAULT 0 CHECK (confidence >= 0 AND confidence <= 1),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (person_id, tag)
);

-- Append-only evidence that feeds preference confidence over time.
CREATE TABLE preference_observation (
    id          UUID PRIMARY KEY,
    person_id   UUID NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    tag         TEXT NOT NULL,
    sentiment   SMALLINT NOT NULL CHECK (sentiment BETWEEN -2 AND 2),
    source      TEXT NOT NULL,          -- 'reaction' | 'manual' | 'import'
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON preference_observation (person_id, tag);

-- ── Recipe references (Mealie is the source of truth) ───────────────────────

CREATE TABLE recipe_ref (
    id               UUID PRIMARY KEY,
    mealie_recipe_id TEXT NOT NULL UNIQUE,
    title            TEXT NOT NULL,
    tags             TEXT[] NOT NULL DEFAULT '{}',
    effort           SMALLINT NOT NULL DEFAULT 2 CHECK (effort BETWEEN 1 AND 3),
    last_synced_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw_snapshot     JSONB                    -- cached Mealie payload
);

-- Canonical ingredients referenced by recipes and requirements. NOT retailer
-- products — the mapping to a Willys article lives in the adapter layer.
CREATE TABLE ingredient (
    id          UUID PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    display     TEXT NOT NULL
);

CREATE TABLE recipe_ingredient (
    recipe_ref_id    UUID NOT NULL REFERENCES recipe_ref(id) ON DELETE CASCADE,
    ingredient_id    UUID NOT NULL REFERENCES ingredient(id),
    quantity         numeric(12,3),
    unit             TEXT,
    PRIMARY KEY (recipe_ref_id, ingredient_id)
);

-- Mealie food id/unit → canonical ingredient + typical form/quantity.
-- First-class per design D6: Swedish dl/msk/tsk/förp → grams → package sizes.
CREATE TABLE ingredient_external_ref (
    provider        TEXT NOT NULL,
    external_id     TEXT NOT NULL,
    ingredient_id   UUID NOT NULL REFERENCES ingredient(id) ON DELETE CASCADE,
    grams_per_unit  numeric(12,3),           -- for volume/count → mass
    default_form    TEXT,                    -- 'fresh' | 'frozen' | ...
    needs_review    BOOLEAN NOT NULL DEFAULT false,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, external_id)
);

-- ── Meal history & reactions ────────────────────────────────────────────────

CREATE TABLE meal_event (
    id               UUID PRIMARY KEY,
    recipe_ref_id    UUID NOT NULL REFERENCES recipe_ref(id),
    served_on        DATE NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON meal_event (served_on);
CREATE INDEX ON meal_event (recipe_ref_id);

CREATE TABLE meal_reaction (
    meal_event_id UUID NOT NULL REFERENCES meal_event(id) ON DELETE CASCADE,
    person_id     UUID NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    sentiment     SMALLINT NOT NULL CHECK (sentiment BETWEEN -2 AND 2),
    note          TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (meal_event_id, person_id)
);

-- ── Effort & planning ───────────────────────────────────────────────────────

-- Expected kitchen energy per weekday (0=Sun..6=Sat); the plan won't propose a
-- meal costing more effort than the day allows.
CREATE TABLE effort_profile (
    weekday       SMALLINT PRIMARY KEY CHECK (weekday BETWEEN 0 AND 6),
    kitchen_energy SMALLINT NOT NULL CHECK (kitchen_energy BETWEEN 1 AND 3)
);

CREATE TABLE planning_constraint (
    id          UUID PRIMARY KEY,
    kind        TEXT NOT NULL,           -- 'avoid_tag' | 'max_repeats' | ...
    value       TEXT NOT NULL,
    active      BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE meal_plan (
    id          UUID PRIMARY KEY,
    week_start  DATE NOT NULL UNIQUE,    -- Monday of the planned week
    status      TEXT NOT NULL DEFAULT 'draft', -- 'draft'|'approved'|'archived'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ranked candidates considered for a slot, with the score breakdown retained
-- for explanation.
CREATE TABLE meal_plan_candidate (
    id               UUID PRIMARY KEY,
    plan_id          UUID NOT NULL REFERENCES meal_plan(id) ON DELETE CASCADE,
    slot_date        DATE NOT NULL,
    recipe_ref_id    UUID NOT NULL REFERENCES recipe_ref(id),
    score            DOUBLE PRECISION NOT NULL,
    breakdown        JSONB NOT NULL,
    feasible         BOOLEAN NOT NULL,
    rank             INT NOT NULL
);
CREATE INDEX ON meal_plan_candidate (plan_id, slot_date, rank);

-- The chosen meal per slot (the human's decision).
CREATE TABLE meal_plan_decision (
    plan_id          UUID NOT NULL REFERENCES meal_plan(id) ON DELETE CASCADE,
    slot_date        DATE NOT NULL,
    recipe_ref_id    UUID NOT NULL REFERENCES recipe_ref(id),
    decided_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (plan_id, slot_date)
);

-- ── Canonical shopping requirements (retailer-independent) ───────────────────

CREATE TABLE shopping_requirement (
    id             UUID PRIMARY KEY,
    plan_id        UUID NOT NULL REFERENCES meal_plan(id) ON DELETE CASCADE,
    ingredient_id  UUID NOT NULL REFERENCES ingredient(id),
    quantity       numeric(12,3) NOT NULL,
    unit           TEXT NOT NULL,
    acceptable_forms TEXT[] NOT NULL DEFAULT '{}',
    preferred_form   TEXT,
    -- Deliberately NO retailer product id column: resolution is the adapter's job.
    UNIQUE (plan_id, ingredient_id)
);

-- +goose Down
DROP TABLE IF EXISTS shopping_requirement CASCADE;
DROP TABLE IF EXISTS meal_plan_decision CASCADE;
DROP TABLE IF EXISTS meal_plan_candidate CASCADE;
DROP TABLE IF EXISTS meal_plan CASCADE;
DROP TABLE IF EXISTS planning_constraint CASCADE;
DROP TABLE IF EXISTS effort_profile CASCADE;
DROP TABLE IF EXISTS meal_reaction CASCADE;
DROP TABLE IF EXISTS meal_event CASCADE;
DROP TABLE IF EXISTS ingredient_external_ref CASCADE;
DROP TABLE IF EXISTS recipe_ingredient CASCADE;
DROP TABLE IF EXISTS ingredient CASCADE;
DROP TABLE IF EXISTS recipe_ref CASCADE;
DROP TABLE IF EXISTS preference_observation CASCADE;
DROP TABLE IF EXISTS person_preference CASCADE;
DROP TABLE IF EXISTS person CASCADE;
