-- Curated seed data for a small set of common Swedish recipe ingredients:
-- canonical ingredient rows, and example ingredient_mapping rows showing the
-- Swedish-unit → grams → package-size shape task 2.3 asks for.
--
-- NOT auto-applied: this lives outside migrations/ (docker-compose only mounts
-- migrations/ into /docker-entrypoint-initdb.d) so it never silently runs
-- against a fresh database. Apply explicitly once persistence lands:
--
--   psql "$DATABASE_URL" -f migrations/seed/ingredient_mappings.sql
--
-- Honest limitation: ingredient_mapping.mealie_food_id is a specific Mealie
-- installation's per-food UUID (see migrations/0001_init.sql's comment on that
-- table) — it cannot be known ahead of syncing against a real Mealie instance.
-- The PLACEHOLDER-* mealie_food_id rows below capture the curated Swedish-unit
-- conversion knowledge (grams_per_unit, default_form) so it doesn't need
-- re-deriving, but each needs_review row must be re-pointed at the real
-- mealie_food_id a live sync assigns once one of these ingredients actually
-- appears in a synced recipe — via the review surface (task 2.3), not by
-- editing this file. Delete the corresponding PLACEHOLDER- row once a real one
-- takes its place.
--
-- The same curated set is mirrored in Go at internal/ingredients/seed.go so the
-- in-memory CLI review surface (food-brain ingredients) can show it before
-- Postgres persistence exists.

BEGIN;

INSERT INTO ingredient (id, display) VALUES
    ('vetemjol',  'Vetemjöl'),
    ('mjolk',     'Mjölk'),
    ('smor',      'Smör'),
    ('salt',      'Salt'),
    ('vispgrädde','Vispgrädde'),
    ('gul-lök',   'Gul lök'),
    ('falukorv',  'Falukorv'),
    ('ketchup',   'Ketchup')
ON CONFLICT (id) DO NOTHING;

-- grams_per_unit is "grams per one recipe unit" (dl/msk/tsk/förp/st), i.e.
-- what a recipe_ingredient.quantity in that unit converts to in mass — the
-- Swedish dl/msk/tsk/förp → grams → package-size chain migrations/0001_init.sql's
-- ingredient_mapping comment describes.
INSERT INTO ingredient_mapping (mealie_food_id, ingredient_id, grams_per_unit, default_form, needs_review) VALUES
    ('PLACEHOLDER-vetemjol-dl',  'vetemjol',   60,  'dry',   true), -- 1 dl vetemjöl ≈ 60 g; pkg 1 kg
    ('PLACEHOLDER-smor-msk',     'smor',       14,  'solid', true), -- 1 msk smör ≈ 14 g;  pkg 250 g
    ('PLACEHOLDER-salt-tsk',     'salt',        6,  'dry',   true), -- 1 tsk salt ≈ 6 g;    pkg 500 g
    ('PLACEHOLDER-falukorv-forp','falukorv',  400, 'fresh', true)  -- 1 förp falukorv ≈ 400 g; pkg 400 g
ON CONFLICT (mealie_food_id) DO NOTHING;

COMMIT;
