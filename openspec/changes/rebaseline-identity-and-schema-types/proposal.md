# Re-baseline identity and schema value types (non-recipe)

## Why

The schema uses four different identity strategies at once — slug-as-PK (`person`, `ingredient`,
`household`, `product`, `inventory_location`), `BIGSERIAL` surrogates (most event/plan/order tables),
foreign-system IDs as relationship keys (`ingredient_mapping.mealie_food_id`), and `TEXT`/`BIGINT`
foreign keys that do not encode what they reference. It also stores quantities, money, and scores as
IEEE `float` (`DOUBLE PRECISION`), which is wrong for money and imprecise for quantities. Pre-release,
with disposable dev data, is the only low-cost window to establish one canonical identity model and one
value-type model before the schema and its Go types are depended on.

## What Changes

- **Identity** (non-recipe tables only):
  - UUIDv7 primary keys, generated in Go, for domain entities (no `BIGSERIAL` surrogate, no slug-as-PK).
  - `slug TEXT NOT NULL UNIQUE` retained on human-addressable entities (`person`, `ingredient`,
    `household`, `product`, `inventory_location`); the UUID is the referential identity.
  - Foreign systems stored in domain-specific external-reference tables keyed by `(provider,
    external_id)`; `ingredient_mapping.mealie_food_id` becomes `ingredient_external_ref`. No generic
    polymorphic `entity_type`/`entity_id`.
  - Pure relationships use composite primary keys (no surrogate): `meal_participant`,
    `recipe_import_candidate_ingredient`, and the confirmed borderline tables.
  - `product_identifier` simplified to `PK (scheme, value)` + `product_id`; GTIN is a zero-padded
    14-digit string under `scheme='GTIN'`.
  - Foreign-key columns re-typed to `UUID` where they reference a converted primary key.
- **Value types**:
  - Quantities to `numeric(12,3)`.
  - Transaction money to `amount_minor BIGINT` + `currency CHAR(3) NOT NULL DEFAULT 'SEK'`
    (`order.total_price`, `order_item.total_price`, `shopping_cart_item.resolved_price`).
  - `order_item.unit_price` to `numeric(12,3)`; `person.weight`, `person_preference.confidence`, and
    `recipe_import_candidate.rating` to bounded `numeric` with `CHECK`.
- **Go**: strongly-typed ID types so a repository call with the wrong ID type does not compile.
- Recipe-referencing columns stay transitional `mealie_recipe_id TEXT` until `rebaseline-recipe-domain`.

## Impact

- Affected specs: `canonical-identity` (new), `canonical-value-types` (new).
- Affected code: non-recipe migrations (edited in place on the `establish-migration-and-postgres-19`
  baseline), Go ID types and the repositories that use them.
- Depends on `establish-migration-and-postgres-19` (the Goose/PG19 substrate). Feeds
  `rebaseline-recipe-domain` (recipe tables) and `establish-sqlc-persistence` (query layer).
