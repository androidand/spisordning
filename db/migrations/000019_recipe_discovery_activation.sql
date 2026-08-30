-- +goose Up
-- Activate recipe discovery: seed the default external_recipe_source and bind
-- the deferred FK from recipe_import_candidate.promoted_variant_id to
-- recipe_variant(id). The FK is already present in 000003; this migration
-- re-adds it with ON DELETE SET NULL (the original RESTRICT prevented a
-- promoted variant from being deleted, but a promoted variant is just a
-- regular variant and should be deletable; SET NULL preserves the link
-- while allowing the variant to be removed).
--
-- The web-jsonld source is seeded as the default catch-all for pages that
-- expose schema.org/Recipe JSON-LD. It is enabled and marked integrate_now
-- so the discovery service can resolve it without manual registration.

-- ── Seed the default web-jsonld source ──────────────────────────────────────

INSERT INTO external_recipe_source (id, name, kind, decision, enabled)
    VALUES ('web-jsonld', 'Web (schema.org/Recipe JSON-LD)', 'jsonld_web', 'integrate_now', true)
    ON CONFLICT (id) DO NOTHING;

-- ── Bind the deferred FK with ON DELETE SET NULL ─────────────────────────────
-- The original 000003 migration added this as RESTRICT; re-add with SET NULL
-- so promoted variants can be deleted (the candidate link is cleared rather
-- than blocking the delete).
ALTER TABLE recipe_import_candidate
    DROP CONSTRAINT IF EXISTS recipe_import_candidate_promoted_variant_fk;
ALTER TABLE recipe_import_candidate
    ADD CONSTRAINT recipe_import_candidate_promoted_variant_fk
    FOREIGN KEY (promoted_variant_id) REFERENCES recipe_variant(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE recipe_import_candidate
    DROP CONSTRAINT IF EXISTS recipe_import_candidate_promoted_variant_fk;
DELETE FROM external_recipe_source WHERE id = 'web-jsonld';
