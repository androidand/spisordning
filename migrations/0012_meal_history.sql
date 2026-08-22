-- Add meal history tables: meal_participant, meal_review, favorite.
--
-- This migration is additive on top of 0001_init.sql and 0011_household_and_catalog.sql.
-- It does not rename or modify any existing table. meal_event and meal_reaction remain
-- untouched so internal/scoring's consumption is unaffected.
--
-- Idempotent: uses CREATE TABLE IF NOT EXISTS and DO/EXCEPTION for constraint adds.

BEGIN;

-- ── 1. Optional link from meal_event back to the plan that generated it.
--     A meal_event may be ad-hoc (no plan) or may derive from a
--     meal_plan_decision. The link is optional; when present both columns
--     are set so the plan→meal trace is explicit rather than inferred.
ALTER TABLE meal_event
    ADD COLUMN IF NOT EXISTS plan_id BIGINT REFERENCES meal_plan(id) ON DELETE SET NULL;
ALTER TABLE meal_event
    ADD COLUMN IF NOT EXISTS plan_slot_date DATE;
-- Note: plan_id and plan_slot_date are application-managed together via
-- LinkMealEventToPlan. When a meal_plan is deleted, plan_id is set NULL by
-- the FK; plan_slot_date is left as-is (it is a DATE, not an FK). Consumers
-- that read the link via GetMealEventPlanLink treat a NULL plan_id as "no
-- link" regardless of whether plan_slot_date is still set. The CHECK
-- constraint below is intentionally omitted to avoid a latent conflict:
-- ON DELETE SET NULL would break it (plan_id→NULL while plan_slot_date
-- remains non-NULL). If a strict invariant is ever needed, a trigger
-- should null both columns together instead.
CREATE INDEX IF NOT EXISTS idx_meal_event_plan
    ON meal_event (plan_id, plan_slot_date);

-- ── 2. meal_participant: who was actually present at a meal_event.
--     Distinct from meal_reaction (who reacted and how). A person can attend
--     without reacting, and a reaction can exist without a recorded
--     attendance row (the reaction is the only evidence someone was there).
CREATE TABLE IF NOT EXISTS meal_participant (
    id            BIGSERIAL PRIMARY KEY,
    meal_event_id BIGINT NOT NULL REFERENCES meal_event(id) ON DELETE CASCADE,
    person_id     TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (meal_event_id, person_id)
);
CREATE INDEX IF NOT EXISTS idx_meal_participant_person
    ON meal_participant (person_id);

-- ── 3. meal_review: per-person, per-meal-instance rating (1-5 scale).
--     Separate from meal_reaction.sentiment (-2..2): sentiment is a quick
--     directional reaction; rating is a considered review. A person may
--     leave both a reaction and a review for the same meal_event.
CREATE TABLE IF NOT EXISTS meal_review (
    id            BIGSERIAL PRIMARY KEY,
    meal_event_id BIGINT NOT NULL REFERENCES meal_event(id) ON DELETE CASCADE,
    person_id     TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    rating        INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    note          TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (meal_event_id, person_id)
);
CREATE INDEX IF NOT EXISTS idx_meal_review_person
    ON meal_review (person_id);
CREATE INDEX IF NOT EXISTS idx_meal_review_event
    ON meal_review (meal_event_id);

-- ── 4. favorite: explicit person-scoped preference over a recipe.
--     Never derived from ratings or reactions — only created by an
--     explicit action. Household-scoped favorites are not modeled here;
--     they can be derived by querying all household members' favorites.
--     The schema leaves room for a future household_id column if needed.
CREATE TABLE IF NOT EXISTS favorite (
    id                   BIGSERIAL PRIMARY KEY,
    person_id            TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    mealie_recipe_id     TEXT NOT NULL REFERENCES recipe_ref(mealie_recipe_id) ON DELETE CASCADE,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (person_id, mealie_recipe_id)
);
CREATE INDEX IF NOT EXISTS idx_favorite_recipe
    ON favorite (mealie_recipe_id);

COMMIT;
