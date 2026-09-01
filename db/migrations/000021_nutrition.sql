-- +goose Up
-- Nutrition data model (Livsmedelsverket / Dabas cross-reference layer).
--
-- research-nutrition-data-sources (task 5): the SLV client already fetches
-- ~2,600 foods with per-100g nutrition, and Dabas/Matpriskollen clients fetch
-- products by GTIN. These three namespaces need a local, queryable home so the
-- planner can enrich candidates with nutrition profiles without a live SLV call
-- on every plan run.
--
--   foods             one row per SLV nummer (the canonical nutrition key)
--   nutrients         one row per (food, nutrient) — per 100g edible portion
--   product_mappings  the cross-reference glue: GTIN / Dabas Arident / canonical
--                     ingredient all resolve to the SLV food that represents it
--
-- The nutrition tables are intentionally denormalised-friendly: nutrients carry
-- no aggregate columns because the planner filters on individual nutrients
-- (e.g. "low sodium" → score down high-natrium candidates), so a per-row
-- nutrients table is the cheapest shape for those predicates.
--
-- FORWARD DEPENDENCY (task 7.2): product_mappings.canonical_ingredient_id
-- references ingredient.id (000001_init.sql) so a recipe ingredient line can be
-- resolved to a nutrition profile through (ingredient -> canonical_ingredient
-- mapping -> foods[slv_nummer] -> nutrients). The column is nullable: an
-- ingredient that has no canonical food mapping yet simply yields no profile,
-- never a NULL crash.

CREATE TABLE foods (
    slv_nummer        INTEGER PRIMARY KEY,            -- SLV's own numeric id
    namn              TEXT NOT NULL,                   -- Swedish name
    venskapligtNamn   TEXT,                            -- Latin binomial
    livsmedels_typ    TEXT,                            -- food type (FoodEx2-ish)
    projekt           TEXT,                            -- SLV project/flag
    version           TEXT,                            -- SLV record version
    synced_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A nutrient is a single (food, nutrient) pair, per 100g edible portion.
CREATE TABLE nutrients (
    food_nummer       INTEGER NOT NULL REFERENCES foods(slv_nummer) ON DELETE CASCADE,
    eurofir_kod       TEXT,                            -- EuroFIR classification code
    namn              TEXT NOT NULL,                   -- nutrient name ("Natrium")
    varde             DOUBLE PRECISION NOT NULL,       -- value
    enhet             TEXT NOT NULL,                   -- unit ("mg")
    metodtyp          TEXT,                            -- measurement method
    synced_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (food_nummer, namn, enhet)
);
CREATE INDEX ON nutrients (food_nummer);

-- Cross-reference glue. A row may hold any ONE of (gtin, dabas_arident) and an
-- optional canonical ingredient. slv_nummer is populated once the mapping has
-- been resolved; the whole row is the "this product = this SLV food" statement.
CREATE TABLE product_mappings (
    id                    SERIAL PRIMARY KEY,
    gtin                  TEXT UNIQUE,                 -- GTIN/EAN (nullable)
    dabas_arident         TEXT UNIQUE,                 -- Dabas Arident (nullable)
    slv_nummer            INTEGER REFERENCES foods(slv_nummer) ON DELETE SET NULL,
    canonical_ingredient_id UUID REFERENCES ingredient(id) ON DELETE SET NULL,
    mapped_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Last successful full-sync bookkeeping per source (SLV, Dabas). One row per
-- source, keyed on (source). Powers incremental syncs and observability (how
-- recently was the nutrition DB refreshed, how many records did it hold).
CREATE TABLE nutrition_sync_status (
    source      TEXT PRIMARY KEY,
    last_synced TIMESTAMPTZ NOT NULL,
    record_count INTEGER NOT NULL DEFAULT 0
);

-- +goose Down
DROP TABLE IF EXISTS nutrition_sync_status;
DROP TABLE IF EXISTS product_mappings;
DROP TABLE IF EXISTS nutrients;
DROP TABLE IF EXISTS foods;
