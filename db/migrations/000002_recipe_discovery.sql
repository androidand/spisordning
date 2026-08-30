-- +goose Up
-- Recipe discovery: provenance + import-candidate staging.
--
-- This migration adds the tables that back the external-recipe import
-- pipeline (see openspec/changes/implement-recipe-discovery/design.md).
-- Ownership: this database owns the SOURCE registry and the CANDIDATE staging
-- area. Candidates are NOT cookbook content: promotion to a
-- RecipeFamily/RecipeVariant/RecipeRevision is a separate, explicit review
-- action (design.md, Section 4) owned by the
-- implement-recipe-family-and-revisions change.
--
-- Invariants enforced here:
--   * A candidate is uniquely keyed per source (by external id when the
--     source has one, else by source URL) so re-importing the same page does
--     not create a duplicate candidate.
--   * Candidate ingredient lines keep the raw source text; resolution to a
--     canonical ingredient is a review step (needs_review), reusing the
--     ingredient_mapping.needs_review pattern.
--   * promoted_variant_id is set only when status = 'promoted'; the FK to
--     recipe_variant is deferred until that table exists.


-- ── External recipe sources ─────────────────────────────────────────────────

-- A registered external recipe source. `decision` records the
-- INTEGRATE/DEFER/OMIT verdict from the source evaluation
-- (docs/research/recipe-data-sources.md) so the registry is self-documenting.
CREATE TABLE external_recipe_source (
    id           TEXT PRIMARY KEY,        -- slug, e.g. 'ica', 'koket', 'arls'
    name         TEXT NOT NULL UNIQUE,
    kind         TEXT NOT NULL,           -- 'jsonld_web' | 'api' | 'manual'
    base_url     TEXT,
    license_note TEXT,
    decision     TEXT NOT NULL DEFAULT 'defer'
                 CHECK (decision IN ('integrate_now', 'defer', 'omit')),
    enabled      BOOLEAN NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Import candidates (staged, not cookbook content) ────────────────────────

-- One externally fetched recipe, staged for review. Parsed content is stored
-- denormalized for the review surface; raw_jsonld is retained for re-sync and
-- audit. status gates the lifecycle: candidate -> promoted | rejected.
CREATE TABLE recipe_import_candidate (
    id                 UUID PRIMARY KEY,
    source_id          TEXT NOT NULL REFERENCES external_recipe_source(id) ON DELETE RESTRICT,
    source_url         TEXT NOT NULL,
    external_id        TEXT,              -- source's own recipe id, when it has one
    title              TEXT NOT NULL,
    description        TEXT,
    image_url          TEXT,
    servings           INT,
    prep_time_sec      INT,
    cook_time_sec      INT,
    total_time_sec     INT,
    category           TEXT,
    cuisine            TEXT,
    attribution        TEXT,              -- author, carried into the promoted variant
    rating             numeric(3,1) CHECK (rating >= 0 AND rating <= 5),
    rating_count       INT,
    nutrition          JSONB,
    raw_jsonld         JSONB,
    license_note       TEXT,
    imported_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    first_served_at    TIMESTAMPTZ,       -- set when first planned/cooked (nudge signal)
    status             TEXT NOT NULL DEFAULT 'candidate'
                       CHECK (status IN ('candidate', 'promoted', 'rejected')),
    promoted_variant_id UUID              -- set only when status = 'promoted'; FK deferred
);

-- A source's own id wins when present; otherwise the URL is the identity.
CREATE UNIQUE INDEX ON recipe_import_candidate (source_id, external_id) WHERE external_id IS NOT NULL;
CREATE UNIQUE INDEX ON recipe_import_candidate (source_url) WHERE external_id IS NULL;
CREATE INDEX ON recipe_import_candidate (status);
CREATE INDEX ON recipe_import_candidate (source_id);

-- ── Candidate ingredient lines ──────────────────────────────────────────────

-- One parsed ingredient line of a candidate. raw_text is always kept (the
-- source line); ingredient_id is NULL until a review step resolves it to a
-- canonical ingredient, at which point needs_review clears. This reuses the
-- ingredient_mapping.needs_review pattern rather than inventing a parallel one.
CREATE TABLE recipe_import_candidate_ingredient (
    candidate_id  UUID NOT NULL REFERENCES recipe_import_candidate(id) ON DELETE CASCADE,
    line_no       INT NOT NULL,
    raw_text      TEXT NOT NULL,
    ingredient_id UUID REFERENCES ingredient(id) ON DELETE SET NULL,
    quantity      numeric(12,3),
    unit          TEXT,
    needs_review  BOOLEAN NOT NULL DEFAULT true,
    PRIMARY KEY (candidate_id, line_no)
);
CREATE INDEX ON recipe_import_candidate_ingredient (candidate_id, needs_review);

-- +goose Down
DROP TABLE IF EXISTS recipe_import_candidate_ingredient CASCADE;
DROP TABLE IF EXISTS recipe_import_candidate CASCADE;
DROP TABLE IF EXISTS external_recipe_source CASCADE;
