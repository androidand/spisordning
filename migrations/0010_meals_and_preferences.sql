-- implement-meals-and-preferences: actual-meal-side entities.
--
-- Extends migrations/0001_init.sql (meal_event/meal_reaction) and
-- migrations/0008_household_catalog_minimal.sql (household) with:
--   - meal_participant   who actually ate at a meal (attendance)
--   - meal_review        per-person considered rating (1-5)
--   - favorite           person/household-scoped explicit recipe preference
--   - meal_event.meal_plan_id  explicit link from a planned decision to the actual meal
--
-- meal_reaction (existing) stays untouched: it captures the quick directional
-- reaction (-2..2) that seeds preference_observation and drives the scoring
-- repetition penalty. meal_review is a sibling table for the richer post-meal
-- considered rating. Two scales, two concerns.
--
-- person_id FKs target the flat person table from 0001_init.sql; household_id
-- FKs target the household table from 0008. The full household_membership
-- lifecycle (joined_at/ended_at) is establish-household-and-catalog's deferred
-- scope — this migration works with what exists today.

BEGIN;

-- Link a cooked meal back to the specific plan decision (plan_id + slot_date)
-- that produced it. Nullable: ad-hoc meals (never planned) have no link.
-- Composite FK targets meal_plan_decision's PK so the link is to the exact
-- decision row, not just the week's plan. PostgreSQL skips the FK check when
-- any column of a composite FK is NULL, so (NULL, NULL) is valid for unplanned
-- meals.
ALTER TABLE meal_event
    ADD COLUMN meal_plan_id     BIGINT,
    ADD COLUMN meal_plan_slot_date DATE;
CREATE INDEX ON meal_event (meal_plan_id, meal_plan_slot_date);
ALTER TABLE meal_event
    ADD CONSTRAINT meal_event_plan_decision_fk
        FOREIGN KEY (meal_plan_id, meal_plan_slot_date)
        REFERENCES meal_plan_decision (plan_id, slot_date)
        ON DELETE SET NULL;

-- Who was actually present/ate at a meal. Distinct from meal_reaction (who
-- reacted and how). A person can attend without reacting; a reaction can
-- exist without a recorded participant row (e.g. someone reviews a meal
-- they watched someone else cook).
CREATE TABLE meal_participant (
    id            BIGSERIAL PRIMARY KEY,
    meal_event_id BIGINT NOT NULL REFERENCES meal_event(id) ON DELETE CASCADE,
    person_id     TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (meal_event_id, person_id)
);
CREATE INDEX ON meal_participant (person_id);

-- Per-person considered rating of a specific meal instance (1-5). Sibling of
-- meal_reaction, not a replacement: reaction is quick/directional (-2..2),
-- review is considered/post-meal (1..5). Recipe-level rating is an aggregate
-- computed read-side from these rows, never stored as a denormalized column.
CREATE TABLE meal_review (
    id             BIGSERIAL PRIMARY KEY,
    meal_event_id  BIGINT NOT NULL REFERENCES meal_event(id) ON DELETE CASCADE,
    person_id      TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    rating         SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    note           TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (meal_event_id, person_id)
);
CREATE INDEX ON meal_review (person_id);
CREATE INDEX ON meal_review (meal_event_id);

-- Explicit person- or household-scoped preference for a recipe. Never
-- derived automatically from ratings or reactions — always created by an
-- explicit action. Exactly one of person_id / household_id must be non-NULL.
CREATE TABLE favorite (
    id               BIGSERIAL PRIMARY KEY,
    person_id        TEXT REFERENCES person(id) ON DELETE CASCADE,
    household_id     TEXT REFERENCES household(id) ON DELETE CASCADE,
    mealie_recipe_id TEXT NOT NULL REFERENCES recipe_ref(mealie_recipe_id),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (person_id, mealie_recipe_id),
    UNIQUE (household_id, mealie_recipe_id),
    CHECK (
        (person_id IS NOT NULL AND household_id IS NULL) OR
        (household_id IS NOT NULL AND person_id IS NULL)
    )
);
CREATE INDEX ON favorite (mealie_recipe_id);
CREATE INDEX ON favorite (person_id);
CREATE INDEX ON favorite (household_id);

COMMIT;
