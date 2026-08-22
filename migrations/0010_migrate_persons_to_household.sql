-- Migrate existing flat-person data into the household model.
--
-- establish-household-and-catalog (openspec/changes/establish-household-and-catalog)
-- introduced `household` and `household_membership` as new tables. This migration
-- backfills the one household that already exists in practice — the family that
-- has been using the flat `person` table since `0001_init.sql` — so that every
-- existing person has a valid `HouseholdMembership` row without touching any
-- preference, observation, or meal-history data.
--
-- Idempotent: safe to run multiple times. If the default household already
-- exists and memberships are already in place, this is a no-op.

BEGIN;

-- ── 1. Create the household table if it does not already exist.
--     (0008_household_catalog_minimal.sql may have created it already.)
CREATE TABLE IF NOT EXISTS household (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── 2. Create the membership table if it does not already exist.
--     (This is the full-shape table from design.md §Step 5, not the minimal
--     version — joined_at and ended_at are required so the schema is usable
--     by downstream changes immediately.)
CREATE TABLE IF NOT EXISTS household_membership (
    household_id  TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    person_id     TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    joined_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at      TIMESTAMPTZ,
    ended_by      TEXT,                    -- nullable; account() is created in migration 0011,
                                          -- so the FK is deferred to avoid a fresh-DB ordering
                                          -- failure. See 0011_household_and_catalog.sql for the
                                          -- account table that later satisfies this column.
    PRIMARY KEY (household_id, person_id)
);
CREATE INDEX IF NOT EXISTS idx_household_membership_person_ended
    ON household_membership (person_id, ended_at);

-- ── 3. Create the default household if none exists.
--     The id 'default' is deliberately simple — it is not a user-facing name,
--     just the bootstrap anchor. The application layer will rename it later
--     (Household is mutable per design.md Step 4).
INSERT INTO household (id, name)
    SELECT 'default', 'Default Household'
    WHERE NOT EXISTS (SELECT 1 FROM household LIMIT 1);

-- ── 4. Assign every existing person to the default household.
--     Skip persons that already have a membership (idempotent).
--     Does NOT touch person_preference, preference_observation, or any other
--     table — those rows survive unchanged.
INSERT INTO household_membership (household_id, person_id, joined_at)
    SELECT 'default', p.id, p.created_at
    FROM person p
    LEFT JOIN household_membership hm ON hm.person_id = p.id
    WHERE hm.person_id IS NULL;

COMMIT;
