-- +goose Up
-- Recipe source reference: maps a native recipe_family to an external source
-- recipe (Mealie slug, structured-text import, discovery promotion, etc.).
--
-- Bidirectional uniqueness:
--   UNIQUE (source, source_recipe_id) — an external recipe maps to at most one
--     family of a given source.
--   UNIQUE (recipe_family_id) — a family maps to at most one external source
--     recipe (across all sources).
--
-- This is the single source of truth for cross-system recipe identity. Every
-- resolution path (planner, recommender, shopping, reactions, favorites)
-- resolves through it (design.md D1/D2).

CREATE TABLE recipe_source_ref (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipe_family_id UUID NOT NULL REFERENCES recipe_family(id) ON DELETE CASCADE,
    source           TEXT NOT NULL,
    source_recipe_id TEXT NOT NULL,
    imported_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    imported_by      TEXT,
    UNIQUE (source, source_recipe_id),
    UNIQUE (recipe_family_id)
);

CREATE INDEX idx_recipe_source_ref_family ON recipe_source_ref (recipe_family_id);
CREATE INDEX idx_recipe_source_ref_source ON recipe_source_ref (source, source_recipe_id);

-- +goose Down
DROP TABLE IF EXISTS recipe_source_ref;
