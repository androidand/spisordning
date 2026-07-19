-- Food Brain initial schema.
--
-- One owner per domain: this database owns the FAMILY (people, preferences,
-- reactions, effort, plan decisions) and the CANONICAL shopping requirements.
-- Recipes are owned by Mealie and referenced by id + snapshot, never copied as
-- the source of truth. Retailer product ids never live on a recipe.

BEGIN;

-- ── People & preferences ────────────────────────────────────────────────────

CREATE TABLE person (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    -- Some people count for more in the aggregate (a picky child's buy-in).
    weight      DOUBLE PRECISION NOT NULL DEFAULT 1.0 CHECK (weight > 0),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Current, confidence-weighted sentiment toward a tag (ingredient/cuisine/trait).
-- This is the derived "belief"; the raw evidence lives in preference_observation.
CREATE TABLE person_preference (
    person_id   TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    tag         TEXT NOT NULL,
    sentiment   SMALLINT NOT NULL CHECK (sentiment BETWEEN -2 AND 2),
    confidence  DOUBLE PRECISION NOT NULL DEFAULT 0 CHECK (confidence BETWEEN 0 AND 1),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (person_id, tag)
);

-- Append-only evidence that feeds preference confidence over time.
CREATE TABLE preference_observation (
    id          BIGSERIAL PRIMARY KEY,
    person_id   TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    tag         TEXT NOT NULL,
    sentiment   SMALLINT NOT NULL CHECK (sentiment BETWEEN -2 AND 2),
    source      TEXT NOT NULL,          -- 'reaction' | 'manual' | 'import'
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON preference_observation (person_id, tag);

-- ── Recipe references (Mealie is the source of truth) ───────────────────────

CREATE TABLE recipe_ref (
    mealie_recipe_id TEXT PRIMARY KEY,
    title            TEXT NOT NULL,
    tags             TEXT[] NOT NULL DEFAULT '{}',
    effort           SMALLINT NOT NULL DEFAULT 2 CHECK (effort BETWEEN 1 AND 3),
    last_synced_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    raw_snapshot     JSONB                    -- cached Mealie payload
);

-- Canonical ingredients referenced by recipes and requirements. NOT retailer
-- products — the mapping to a Willys article lives in the adapter layer.
CREATE TABLE ingredient (
    id          TEXT PRIMARY KEY,        -- canonical id, e.g. 'cauliflower'
    display     TEXT NOT NULL
);

CREATE TABLE recipe_ingredient (
    mealie_recipe_id TEXT NOT NULL REFERENCES recipe_ref(mealie_recipe_id) ON DELETE CASCADE,
    ingredient_id    TEXT NOT NULL REFERENCES ingredient(id),
    quantity         DOUBLE PRECISION,
    unit             TEXT,
    PRIMARY KEY (mealie_recipe_id, ingredient_id)
);

-- Mealie food id/unit → canonical ingredient + typical form/quantity.
-- First-class per design D6: Swedish dl/msk/tsk/förp → grams → package sizes.
CREATE TABLE ingredient_mapping (
    mealie_food_id  TEXT PRIMARY KEY,
    ingredient_id   TEXT NOT NULL REFERENCES ingredient(id),
    grams_per_unit  DOUBLE PRECISION,        -- for volume/count → mass
    default_form    TEXT,                    -- 'fresh' | 'frozen' | ...
    needs_review    BOOLEAN NOT NULL DEFAULT true,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Meal history & reactions ────────────────────────────────────────────────

CREATE TABLE meal_event (
    id               BIGSERIAL PRIMARY KEY,
    mealie_recipe_id TEXT NOT NULL REFERENCES recipe_ref(mealie_recipe_id),
    served_on        DATE NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON meal_event (served_on);
CREATE INDEX ON meal_event (mealie_recipe_id);

CREATE TABLE meal_reaction (
    id            BIGSERIAL PRIMARY KEY,
    meal_event_id BIGINT NOT NULL REFERENCES meal_event(id) ON DELETE CASCADE,
    person_id     TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    sentiment     SMALLINT NOT NULL CHECK (sentiment BETWEEN -2 AND 2),
    note          TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (meal_event_id, person_id)
);

-- ── Effort & planning ───────────────────────────────────────────────────────

-- Expected kitchen energy per weekday (0=Sun..6=Sat); the plan won't propose a
-- meal costing more effort than the day allows.
CREATE TABLE effort_profile (
    weekday       SMALLINT PRIMARY KEY CHECK (weekday BETWEEN 0 AND 6),
    kitchen_energy SMALLINT NOT NULL CHECK (kitchen_energy BETWEEN 1 AND 3)
);

CREATE TABLE planning_constraint (
    id          BIGSERIAL PRIMARY KEY,
    kind        TEXT NOT NULL,           -- 'avoid_tag' | 'max_repeats' | ...
    value       TEXT NOT NULL,
    active      BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE meal_plan (
    id          BIGSERIAL PRIMARY KEY,
    week_start  DATE NOT NULL UNIQUE,    -- Monday of the planned week
    status      TEXT NOT NULL DEFAULT 'draft', -- 'draft'|'approved'|'archived'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Ranked candidates considered for a slot, with the score breakdown retained
-- for explanation.
CREATE TABLE meal_plan_candidate (
    id               BIGSERIAL PRIMARY KEY,
    plan_id          BIGINT NOT NULL REFERENCES meal_plan(id) ON DELETE CASCADE,
    slot_date        DATE NOT NULL,
    mealie_recipe_id TEXT NOT NULL REFERENCES recipe_ref(mealie_recipe_id),
    score            DOUBLE PRECISION NOT NULL,
    breakdown        JSONB NOT NULL,
    feasible         BOOLEAN NOT NULL,
    rank             INT NOT NULL
);
CREATE INDEX ON meal_plan_candidate (plan_id, slot_date, rank);

-- The chosen meal per slot (the human's decision).
CREATE TABLE meal_plan_decision (
    plan_id          BIGINT NOT NULL REFERENCES meal_plan(id) ON DELETE CASCADE,
    slot_date        DATE NOT NULL,
    mealie_recipe_id TEXT NOT NULL REFERENCES recipe_ref(mealie_recipe_id),
    decided_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (plan_id, slot_date)
);

-- ── Canonical shopping requirements (retailer-independent) ───────────────────

CREATE TABLE shopping_requirement (
    id             BIGSERIAL PRIMARY KEY,
    plan_id        BIGINT NOT NULL REFERENCES meal_plan(id) ON DELETE CASCADE,
    ingredient_id  TEXT NOT NULL REFERENCES ingredient(id),
    quantity       DOUBLE PRECISION NOT NULL,
    unit           TEXT NOT NULL,
    acceptable_forms TEXT[] NOT NULL DEFAULT '{}',
    preferred_form   TEXT,
    -- Deliberately NO retailer product id column: resolution is the adapter's job.
    UNIQUE (plan_id, ingredient_id)
);

COMMIT;
