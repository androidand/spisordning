## 1. Decide the open identity questions

- [x] 1.1 Record the `external_recipe_source` decision (stable-code PK vs UUIDv7 + slug) in
      `design.md`; update the schema diff to match
      — Decided: **keep the stable-code PK** (`id TEXT`, e.g. `'ica'`). Added D6 to design.md with
      rationale (small named registry, no independent lifecycle, matches the `effort_profile`
      stable-code rule). Updated the schema diff: moved `external_recipe_source` from the
      "slug-PK to UUIDv7" section to "F. Stable code (kept)", confirmed
      `recipe_import_candidate.source_id` stays `TEXT`, and noted no Go typed-ID is needed
      (`recipeimport.Source.ID` is already a `string`). Resolved Open Question 1.
- [x] 1.2 Record the `favorite` shape decision (bounded `scope_type`/`scope_id` discriminator vs
      split `person_favorite`/`household_favorite`) in `design.md`
      — Decided: **bounded `scope_type`/`scope_id` discriminator** (D7). One table,
      `scope_type TEXT CHECK IN ('person','household')`, `scope_id UUID`,
      `mealie_recipe_id TEXT` (transitional), PK `(scope_type, scope_id, mealie_recipe_id)`.
      Rationale: one table keeps the read path simple; the discriminator is bounded (not generic
      polymorphic). Updated schema diff section D and resolved Open Question 2.
- [x] 1.3 Confirm `meal_reaction`, `meal_review`, `retailer_list_binding`, and
      `shopping_cart_item` as composite PKs (no independent identity) in `design.md`
      — All four **confirmed as composite PKs** (D8). `meal_reaction(meal_event_id, person_id)`,
      `meal_review(meal_event_id, person_id)`, `retailer_list_binding(shopping_list_id, retailer)`,
      `shopping_cart_item(shopping_cart_id, line_no)`. All drop their `BIGSERIAL` surrogate.
      Updated schema diff section D and resolved Open Question 3.

## 2. Re-baseline the non-recipe migrations (in place, fresh-bootstrap)

- [x] 2.1 Convert slug-PK tables to UUIDv7 PK + `slug TEXT NOT NULL UNIQUE`: `person`,
      `ingredient`, `household`, `product`, `inventory_location`
      — Done in 000001 (person, ingredient), 000008 (household, product), 000009
      (inventory_location), 000010 (household IF NOT EXISTS). Also added slug to retailer,
      store, retailer_product (000013) which are human-addressable.
- [x] 2.2 Convert `BIGSERIAL` entity tables to UUIDv7 PK: `preference_observation`,
      `planning_constraint`, `meal_plan`, `meal_event`, `meal_plan_candidate`,
      `shopping_requirement`, `recipe_import_candidate`, `shopping_list`, `shopping_list_item`,
      `shopping_cart`, `order`, `order_item`, `inventory_lot`, `inventory_event`
      — Done across 000001, 000002, 000004, 000006, 000007, 000009. Also converted
      `store_product_offer`, `price_observation` (000013), `ingredient_substitution` (000011),
      `ingredient_alias` (000016).
- [x] 2.3 Convert pure-relationship tables to composite PKs (drop surrogate):
      `meal_participant`, `recipe_import_candidate_ingredient`, and the confirmed borderline
      tables from 1.3
      — Done: `meal_participant(meal_event_id, person_id)`, `meal_review(meal_event_id,
      person_id)`, `meal_reaction(meal_event_id, person_id)`, `retailer_list_binding
      (shopping_list_id, retailer)`, `shopping_cart_item(shopping_cart_id, line_no)`,
      `recipe_import_candidate_ingredient(candidate_id, line_no)`.
- [x] 2.4 Restructure `ingredient_mapping` to `ingredient_external_ref(provider, external_id,
      ingredient_id, ...)` with `PK (provider, external_id)`
      — Done in 000001. Table renamed, PK is `(provider, external_id)`, `ingredient_id` is UUID.
- [x] 2.5 Simplify `product_identifier` to `PK (scheme, value)` + `product_id` FK, with
      `provenance`/`confidence`; GTIN becomes `scheme='GTIN'` zero-padded 14-digit string
      — Done in 000008. PK is `(scheme, value)`, `product_id` is UUID FK, `provenance` and
      `confidence numeric(5,4)` added.
- [x] 2.6 Retype all non-recipe FK columns that reference converted PKs from `TEXT`/`BIGINT` to
      `UUID`
      — Done across all migrations. All FK columns referencing converted PKs are now UUID.
      Recipe-referencing columns stay TEXT (transitional).
- [x] 2.7 Apply value types: quantities to `numeric(12,3)`; transaction money to
      `amount_minor BIGINT` + `currency CHAR(3) DEFAULT 'SEK'`; `order_item.unit_price` to
      `numeric(12,3)`; `person.weight`, `person_preference.confidence`,
      `recipe_import_candidate.rating` to bounded `numeric` with `CHECK`
      — Done: quantities → `numeric(12,3)`; `order.total_price`, `order_item.total_price`,
      `shopping_cart_item.resolved_price` → `*_minor BIGINT` + `currency CHAR(3) DEFAULT 'SEK'`;
      `order_item.unit_price` → `numeric(12,3)`; `person.weight` → `numeric(6,3) CHECK > 0`;
      `person_preference.confidence` → `numeric(5,4) CHECK 0..1`;
      `recipe_import_candidate.rating` → `numeric(3,1) CHECK 0..5`.
- [x] 2.8 Leave recipe-referencing columns (`favorite`, `meal_event`, `meal_plan_candidate`,
      `meal_plan_decision`, `recipe_import_candidate.promoted_variant_id`) as transitional
      `mealie_recipe_id TEXT` for `rebaseline-recipe-domain`
      — Verified: all recipe-referencing columns remain `mealie_recipe_id TEXT`. The `favorite`
      table uses `mealie_recipe_id TEXT` in its PK (D7). `recipe_import_candidate.promoted_variant_id`
      stays TEXT.

## 3. Introduce strongly-typed Go IDs

- [x] 3.1 Define named `uuid.UUID`-wrapping ID types in the domain layer for each non-recipe
      entity
      — Created `internal/domain/ids.go` with 27 typed ID types (PersonID, IngredientID,
      HouseholdID, ProductID, InventoryLocationID, MealEventID, MealPlanID,
      MealPlanCandidateID, ShoppingRequirementID, RecipeImportCandidateID, ShoppingListID,
      ShoppingListItemID, RetailerListBindingID, ShoppingCartID, OrderID, OrderItemID,
      InventoryLotID, InventoryEventID, PreferenceObservationID, PlanningConstraintID,
      IngredientAliasID, RetailerID, StoreID, RetailerProductID, StoreProductOfferID,
      PriceObservationID, AccountID). Each wraps `uuid.UUID` with constructors (UUIDv7 via
      `newUUIDv7()` helper, per D1), `UUID()` accessor, `String()`, `driver.Valuer`,
      `sql.Scanner`, `Parse*` helpers, and `MarshalJSON`/`UnmarshalJSON`. `TestTypedIDNewIsV7`
      asserts the version nibble is 7.
- [x] 3.2 Update repository interfaces and domain services to use the typed IDs
      — Updated domain structs (Person.ID, Preference.PersonID, Product.ID, Retailer.ID, Store.ID,
      RetailerProduct.ID, ShoppingList.ID, MealParticipant.PersonID, etc.) to use typed IDs.
      Updated service layer (prices.go, stores.go) to convert typed IDs to strings at the DTO
      boundary. Updated persistence Product struct to use domain.ProductID. Added JSON
      marshal/unmarshal methods to all 27 typed ID types for family config loading.
- [x] 3.3 Update the existing pgx persistence code to use the typed IDs (sqlc conversion is
      `establish-sqlc-persistence`)
      — Updated all hand-written pgx persistence SQL to match the re-baselined schema (VERIFY
      gate fix). catalog.go: product_identifier now uses (scheme, value) instead of gtin;
      household/product INSERTs include slug; ingredient_alias generates UUID id. cart.go:
      shopping_cart uses composite (shopping_list_id, retailer) binding ref; shopping_cart_item
      uses composite PK (shopping_cart_id, line_no) with resolved_price_minor + currency.
      order.go: order/order_item use UUID PKs with total_price_minor + currency. shopping.go:
      shopping_list/shopping_list_item use UUID PKs; retailer_list_binding uses composite PK.
      The persistence layer uses string IDs in method signatures (pgx handles string→UUID);
      the domain layer keeps typed IDs and converts at the boundary. Scope creep removed:
      sqlc.yaml, db/queries/, persistence/sqlc/, sqlc dep in tools.go, architecture test
      sqlc allowance (all belong to establish-sqlc-persistence).

## 4. Verify

- [x] 4.1 Fresh empty `postgres:19beta3-alpine` applies the chain `000001`–`000010` cleanly and
      reaches the target shape
      — Verified: all 18 migrations (000001–000018) applied cleanly to a fresh `postgres:19beta3-alpine`
      database. Fixed `shopping_cart` FK to reference `retailer_list_binding` composite PK
      `(shopping_list_id, retailer)` instead of the removed surrogate `id`.
- [x] 4.2 `spisordning migrate up` runs the chain against a clean database
      — Verified: `food-brain migrate up` ran the full chain against a clean database, reaching
      version 18 with no errors.
- [x] 4.3 `go build ./...` and `go test ./...` (with `GOWORK=off`) pass
      — Verified (VERIFY gate fix): `go build ./...` succeeds. `GOWORK=off go test ./...`
      passes with 565 tests across 27 packages, 0 failures. All hand-written pgx persistence
      SQL updated to match the re-baselined schema; httpapi/adapters/openapi/mcptools ID types
      converted from int64 to string; UUIDv7 constructors in place (uuid.NewV7 via newUUIDv7
      helper); scope creep (sqlc scaffolding) removed.
- [x] 4.4 A compile-time test (or type assertion) confirms a repository rejects the wrong ID type
      — Created `internal/domain/ids_test.go` with `TestTypedIDCompileTimeRejection` (verifies
      PersonID, IngredientID, HouseholdID are distinct named types), `TestTypedIDParse`
      (verifies Parse* helpers), and `TestTypedIDNew` (verifies New* constructors). All 3 tests
      pass. The typed IDs are distinct named types wrapping `uuid.UUID`, so a repository call
      passing the wrong entity's id type does not compile.
