## 1. Decide the open identity questions

- [ ] 1.1 Record the `external_recipe_source` decision (stable-code PK vs UUIDv7 + slug) in
      `design.md`; update the schema diff to match
- [ ] 1.2 Record the `favorite` shape decision (bounded `scope_type`/`scope_id` discriminator vs
      split `person_favorite`/`household_favorite`) in `design.md`
- [ ] 1.3 Confirm `meal_reaction`, `meal_review`, `retailer_list_binding`, and
      `shopping_cart_item` as composite PKs (no independent identity) in `design.md`

## 2. Re-baseline the non-recipe migrations (in place, fresh-bootstrap)

- [ ] 2.1 Convert slug-PK tables to UUIDv7 PK + `slug TEXT NOT NULL UNIQUE`: `person`,
      `ingredient`, `household`, `product`, `inventory_location`
- [ ] 2.2 Convert `BIGSERIAL` entity tables to UUIDv7 PK: `preference_observation`,
      `planning_constraint`, `meal_plan`, `meal_event`, `meal_plan_candidate`,
      `shopping_requirement`, `recipe_import_candidate`, `shopping_list`, `shopping_list_item`,
      `shopping_cart`, `order`, `order_item`, `inventory_lot`, `inventory_event`
- [ ] 2.3 Convert pure-relationship tables to composite PKs (drop surrogate):
      `meal_participant`, `recipe_import_candidate_ingredient`, and the confirmed borderline
      tables from 1.3
- [ ] 2.4 Restructure `ingredient_mapping` to `ingredient_external_ref(provider, external_id,
      ingredient_id, ...)` with `PK (provider, external_id)`
- [ ] 2.5 Simplify `product_identifier` to `PK (scheme, value)` + `product_id` FK, with
      `provenance`/`confidence`; GTIN becomes `scheme='GTIN'` zero-padded 14-digit string
- [ ] 2.6 Retype all non-recipe FK columns that reference converted PKs from `TEXT`/`BIGINT` to
      `UUID`
- [ ] 2.7 Apply value types: quantities to `numeric(12,3)`; transaction money to
      `amount_minor BIGINT` + `currency CHAR(3) DEFAULT 'SEK'`; `order_item.unit_price` to
      `numeric(12,3)`; `person.weight`, `person_preference.confidence`,
      `recipe_import_candidate.rating` to bounded `numeric` with `CHECK`
- [ ] 2.8 Leave recipe-referencing columns (`favorite`, `meal_event`, `meal_plan_candidate`,
      `meal_plan_decision`, `recipe_import_candidate.promoted_variant_id`) as transitional
      `mealie_recipe_id TEXT` for `rebaseline-recipe-domain`

## 3. Introduce strongly-typed Go IDs

- [ ] 3.1 Define named `uuid.UUID`-wrapping ID types in the domain layer for each non-recipe
      entity
- [ ] 3.2 Update repository interfaces and domain services to use the typed IDs
- [ ] 3.3 Update the existing pgx persistence code to use the typed IDs (sqlc conversion is
      `establish-sqlc-persistence`)

## 4. Verify

- [ ] 4.1 Fresh empty `postgres:19beta3-alpine` applies the chain `000001`–`000010` cleanly and
      reaches the target shape
- [ ] 4.2 `spisordning migrate up` runs the chain against a clean database
- [ ] 4.3 `go build ./...` and `go test ./...` (with `GOWORK=off`) pass
- [ ] 4.4 A compile-time test (or type assertion) confirms a repository rejects the wrong ID type
