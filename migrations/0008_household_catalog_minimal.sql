-- Minimal household/catalog prerequisite slice.
--
-- establish-household-and-catalog (openspec/changes/establish-household-and-catalog) designs a
-- much larger schema (account, household_membership, person_restriction, ingredient_form,
-- ingredient_substitution, the full unit/unit_conversion system). This migration deliberately
-- ships only the four tables implement-pantry-inventory needs as FK targets — household scoping
-- for inventory_location, and Product/ProductIdentifier/ProductIngredientMapping for D8's
-- graduated item specificity and D6's barcode resolution. The rest of that change's scope is
-- explicitly deferred; see openspec/changes/establish-household-and-catalog/tasks.md's
-- 2026-08-19 update note.

BEGIN;

-- The unit that owns inventory locations. No account/membership modeling yet — that is
-- establish-household-and-catalog's own scope, deferred.
CREATE TABLE household (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A concrete, purchasable good ("Garant Kycklingfilé 900g"), household-facing and
-- retailer-agnostic (design.md Step 1). Distinct from ingredient: no brand/package data ever
-- lives on `ingredient`.
CREATE TABLE product (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    brand         TEXT,
    package_size  TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- GTIN/EAN lookup key onto a Product — never identity itself (implement-pantry-inventory
-- design.md D6/invariant 5).
CREATE TABLE product_identifier (
    id          BIGSERIAL PRIMARY KEY,
    product_id  TEXT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    gtin        TEXT NOT NULL UNIQUE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Product → Ingredient is many-to-many (design.md Step 3: "a taco kit product maps to several
-- ingredients"). Absence of any row for a product_id means "unmapped, flagged for review" per
-- ingredient-catalog spec's "A Product without a resolved mapping is still valid" scenario — no
-- placeholder row is inserted to represent "no mapping yet."
CREATE TABLE product_ingredient_mapping (
    product_id     TEXT NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    ingredient_id  TEXT NOT NULL REFERENCES ingredient(id),
    quantity       DOUBLE PRECISION,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (product_id, ingredient_id)
);
CREATE INDEX ON product_ingredient_mapping (ingredient_id);

COMMIT;
