-- +goose Up
-- complete-live-meal-planning: add slot_kind to meal plan tables.
--
-- Extends meal_plan_candidate and meal_plan_decision (from 000001_init.sql)
-- with a slot_kind column so a single plan can hold dinner, breakfast, and
-- snack candidates/decisions per date. Default 'dinner' keeps every existing
-- row and query valid — this is additive, not a breaking migration.
--
-- meal_plan_decision's primary key widens from (plan_id, slot_date) to
-- (plan_id, slot_date, slot_kind). The composite FK in meal_event
-- (from 000012_meals_and_preferences.sql) is updated to include slot_kind.

-- Add slot_kind to meal_plan_candidate.
ALTER TABLE meal_plan_candidate
    ADD COLUMN slot_kind TEXT NOT NULL DEFAULT 'dinner'
        CHECK (slot_kind IN ('dinner', 'breakfast', 'snack'));
CREATE INDEX ON meal_plan_candidate (plan_id, slot_date, slot_kind);

-- Add slot_kind to meal_plan_decision and widen its primary key.
-- Must drop meal_event's FK first since it depends on the old PK.
ALTER TABLE meal_event DROP CONSTRAINT IF EXISTS meal_event_plan_decision_fk;
ALTER TABLE meal_plan_decision
    ADD COLUMN slot_kind TEXT NOT NULL DEFAULT 'dinner'
        CHECK (slot_kind IN ('dinner', 'breakfast', 'snack'));
ALTER TABLE meal_plan_decision DROP CONSTRAINT meal_plan_decision_pkey;
ALTER TABLE meal_plan_decision
    ADD PRIMARY KEY (plan_id, slot_date, slot_kind);

-- Re-add meal_event's composite FK with slot_kind.
ALTER TABLE meal_event
    ADD COLUMN meal_plan_slot_kind TEXT;
ALTER TABLE meal_event
    ADD CONSTRAINT meal_event_plan_decision_fk
        FOREIGN KEY (meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind)
        REFERENCES meal_plan_decision (plan_id, slot_date, slot_kind)
        ON DELETE SET NULL;
CREATE INDEX ON meal_event (meal_plan_id, meal_plan_slot_date, meal_plan_slot_kind);

-- +goose Down
ALTER TABLE meal_event DROP CONSTRAINT IF EXISTS meal_event_plan_decision_fk;
ALTER TABLE meal_event DROP COLUMN IF EXISTS meal_plan_slot_kind;
ALTER TABLE meal_plan_decision DROP CONSTRAINT meal_plan_decision_pkey;
ALTER TABLE meal_plan_decision
    ADD PRIMARY KEY (plan_id, slot_date);
ALTER TABLE meal_plan_decision DROP COLUMN IF EXISTS slot_kind;
-- Re-add the old FK without slot_kind.
ALTER TABLE meal_event
    ADD CONSTRAINT meal_event_plan_decision_fk
        FOREIGN KEY (meal_plan_id, meal_plan_slot_date)
        REFERENCES meal_plan_decision (plan_id, slot_date)
        ON DELETE SET NULL;
DROP INDEX IF EXISTS meal_plan_candidate_plan_id_slot_date_slot_kind_idx;
ALTER TABLE meal_plan_candidate DROP COLUMN IF EXISTS slot_kind;
