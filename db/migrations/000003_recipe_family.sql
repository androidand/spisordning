-- +goose Up
-- Recipe family / variant / revision hierarchy.
--
-- This migration adds the four tables that back the git-like recipe hierarchy
-- (see openspec/changes/implement-recipe-family-and-revisions/design.md):
--   recipe_family           a conceptual dish ("Korvstroganoff")
--   recipe_variant          one recognizable fork/style/source of a family
--   recipe_revision         an immutable snapshot of a variant's content
--   recipe_revision_parent  a lineage edge (child revision -> parent revision)
--
-- Ownership: this database owns the household's recipe hierarchy. It is
-- ADDITIVE alongside recipe_ref: recipes not yet migrated into the hierarchy
-- remain referenced via recipe_ref (Mealie is the source of truth for those).
-- This migration does not touch recipe_ref or force any migration of it.
--
-- Invariants enforced here (schema-level):
--   * A variant belongs to exactly one family        (recipe_variant.family_id FK).
--   * A revision belongs to exactly one variant      (recipe_revision.variant_id FK).
--   * A family's default_variant_id references a real variant (FK).
--   * A revision's parent edge references real revisions (FKs).
--   * A revision that is someone's parent cannot be deleted (parent-side RESTRICT).
-- Invariants enforced in the application layer (internal/recipefamily), NOT here:
--   * Revision content is immutable (no UPDATE path; a correction is a new row).
--   * Revision parentage never cycles (checked at edge-insert time).
--   * default_variant_id resolves within its own family (checked at set time).


-- ── Recipe families (conceptual dishes) ─────────────────────────────────────

-- A conceptual dish. `default_variant_id` is the manually-pinned expanded
-- variant (design.md, "Decisions — default_variant_id"); its FK is added after
-- recipe_variant exists to break the circular reference.
CREATE TABLE recipe_family (
    id                 UUID PRIMARY KEY,
    slug               TEXT NOT NULL UNIQUE,
    name               TEXT NOT NULL UNIQUE,
    description        TEXT,
    default_variant_id UUID,               -- FK added below (circular reference)
    archived           BOOLEAN NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── Recipe variants (recognizable forks) ────────────────────────────────────

-- One recognizable fork/style/source of a family — the unit a household cooks
-- and rates. `source_attribution` is a label ('household', 'ICA Kök', ...);
-- imported variants also carry the recipe_import_candidate.promoted_variant_id
-- back-reference once promoted.
CREATE TABLE recipe_variant (
    id                 UUID PRIMARY KEY,
    slug               TEXT NOT NULL UNIQUE,
    family_id          UUID NOT NULL REFERENCES recipe_family(id) ON DELETE RESTRICT,
    title              TEXT NOT NULL,
    source_attribution TEXT,
    archived           BOOLEAN NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON recipe_variant (family_id);

-- ── Recipe revisions (immutable content snapshots) ──────────────────────────

-- One immutable snapshot of a variant's content. Content is structured JSONB
-- (design.md, "Revision content model"): `ingredients` is an array of
-- {ingredient_id, quantity, unit, raw_text}; `steps` is an ordered array of
-- strings. A revision is never updated; a correction is a new row.
CREATE TABLE recipe_revision (
    id             UUID PRIMARY KEY,
    variant_id     UUID NOT NULL REFERENCES recipe_variant(id) ON DELETE CASCADE,
    servings       INT,
    prep_time_sec  INT,
    cook_time_sec  INT,
    total_time_sec INT,
    description    TEXT,
    ingredients    JSONB NOT NULL DEFAULT '[]',
    steps          JSONB NOT NULL DEFAULT '[]',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX ON recipe_revision (variant_id);

-- ── Revision lineage (DAG edges) ────────────────────────────────────────────

-- One edge: revision_id (child) was derived from parent_revision_id. A child
-- may have many parents (merge); a revision may have zero parents (a variant's
-- first revision). Acyclicity is enforced in the application layer, not here.
-- Child-side CASCADE (deleting a revision drops its own parent-edges);
-- parent-side RESTRICT (a revision that is someone's parent cannot be deleted,
-- which also protects cross-variant fork edges such as A3 -> C1).
CREATE TABLE recipe_revision_parent (
    revision_id        UUID NOT NULL REFERENCES recipe_revision(id) ON DELETE CASCADE,
    parent_revision_id UUID NOT NULL REFERENCES recipe_revision(id) ON DELETE RESTRICT,
    PRIMARY KEY (revision_id, parent_revision_id)
);
CREATE INDEX ON recipe_revision_parent (parent_revision_id);

-- ── Circular FK: family.default_variant_id -> variant ───────────────────────
-- Added after recipe_variant exists. RESTRICT so a variant that is a family's
-- pinned default cannot be deleted (re-pin first). The "resolves within its own
-- family" invariant is enforced in the application layer (SetDefaultVariant).
ALTER TABLE recipe_family
    ADD CONSTRAINT recipe_family_default_variant_fk
    FOREIGN KEY (default_variant_id) REFERENCES recipe_variant(id) ON DELETE RESTRICT;

-- ── Deferred FK: recipe_import_candidate.promoted_variant_id -> variant ─────
-- Bound now that recipe_variant exists. RESTRICT so a variant that has been
-- promoted from an import candidate cannot be deleted.
ALTER TABLE recipe_import_candidate
    ADD CONSTRAINT recipe_import_candidate_promoted_variant_fk
    FOREIGN KEY (promoted_variant_id) REFERENCES recipe_variant(id) ON DELETE RESTRICT;

-- +goose Down
DROP TABLE IF EXISTS recipe_revision_parent CASCADE;
DROP TABLE IF EXISTS recipe_revision CASCADE;
ALTER TABLE recipe_import_candidate DROP CONSTRAINT IF EXISTS recipe_import_candidate_promoted_variant_fk;
DROP TABLE IF EXISTS recipe_variant CASCADE;
DROP TABLE IF EXISTS recipe_family CASCADE;
