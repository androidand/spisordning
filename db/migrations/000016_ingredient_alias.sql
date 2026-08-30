-- +goose Up
-- Ingredient aliases: household-configurable nicknames → canonical ingredient.
--
-- The "ingredient/product translation with configurable nickname matching" use
-- case. A household writes recipe ingredients and shopping-list items in their
-- own words ("potatis", "mjölk", "a-l-mjölk"); the alias table maps those
-- nicknames to a canonical ingredient id so planning, pantry, and shopping all
-- agree on the same thing.
--
-- Design decisions:
--   - alias is the normalized (lowercased, trimmed) household nickname; it is
--     unique per household so the lookup is a single indexed equality.
--   - ingredient_id references the canonical ingredient (migrations/0001).
--   - household_id scopes aliases per household (a household's "potatis" need
--     not collide with another's). NULL household_id is allowed for global
--     aliases that apply to every household.
--   - created_at is the only timestamp; aliases are rarely edited, and an
--     upsert refreshes nothing but re-asserts existence.

CREATE TABLE IF NOT EXISTS ingredient_alias (
    id            UUID PRIMARY KEY,
    household_id  UUID,                          -- NULL = global alias
    alias         TEXT NOT NULL,                 -- normalized nickname, e.g. 'potatis'
    ingredient_id UUID NOT NULL REFERENCES ingredient(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (household_id, alias)
);
CREATE INDEX IF NOT EXISTS idx_ingredient_alias_household ON ingredient_alias (household_id);
CREATE INDEX IF NOT EXISTS idx_ingredient_alias_ingredient ON ingredient_alias (ingredient_id);

-- +goose Down
DROP TABLE IF EXISTS ingredient_alias CASCADE;
