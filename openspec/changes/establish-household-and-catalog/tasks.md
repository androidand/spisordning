**Update (2026-08-19): a deliberately minimal first slice has landed** —
`migrations/0008_household_catalog_minimal.sql` + `internal/persistence/catalog.go`. Scope was
narrowed to exactly what `implement-pantry-inventory` needs as FK targets: `household` (no
`account`/`household_membership`, no changes to `person`), `product`, `product_identifier`,
`product_ingredient_mapping`.

**Update (2026-08-21): the full scope of this change is now implemented.**
`migrations/0010_household_catalog.sql` adds the remaining tables (membership/account split,
`person_restriction`, ingredient forms/substitution, the full unit system, canonicalization),
seeds the 11 units + universal conversions, and migrates existing `person` rows to a default
household. `internal/domain/household.go` + `internal/domain/catalog.go` carry the domain types
and invariants; `internal/persistence/units.go` is the only write path to the conversion tables
(invariant 11). Every box below is now checked; the note on each says where the decision or
implementation lives.

## 1. Household & identity vocabulary

- [x] 1.1 Confirm the candidate vocabulary from PLAN.md's "Household" section: `households`,
      `accounts`, `persons`, `household_memberships` — validate each against
      `migrations/0001_init.sql`'s existing flat `person` table. — design.md Step 1 vocabulary
      table; `person` reused as-is (only gains an optional `account_id`), the other three are new
      tables in 0010.
- [x] 1.2 Do not conflate login identity with household Person — decide the exact shape of the
      `Account` ↔ `Person` relationship (optional 1:1 for this change; multi-household Account
      support explicitly deferred) and document the deferral. — design.md Step 1/2 + invariant 1:
      `account` is its own aggregate, referenced by `person.account_id` (optional FK, `ON DELETE
      SET NULL`); multi-household Account deferred (a Person is one row, memberships are the
      multi-household axis).
- [x] 1.3 Decide `HouseholdMembership` lifecycle fields (`joined_at`/`ended_at`) so ending a
      membership never deletes a `Person` or their history. — design.md Step 4: append + close
      (`ended_at` set on leave, row never deleted); 0010 partial unique index
      `(household_id, person_id) WHERE ended_at IS NULL`.
- [x] 1.4 Write a migration script for existing `person` rows: assign them to a default
      household + membership without touching `person_preference`/`preference_observation`. —
      0010 tail: idempotent `INSERT ... SELECT` into a `default` household; verified against a
      DB already containing 0001 person rows (preferences left intact).

## 2. Preferences vs. restrictions

- [x] 2.1 Separate LIKES/DISLIKES from ALLERGIES/HARD RESTRICTIONS as PLAN.md requires — confirm
      `person_preference`/`preference_observation` remain the LIKES/DISLIKES model unchanged. —
      0010 adds `person_restriction` as a separate table; `person_preference`/
      `preference_observation` are not touched (design.md invariant 2).
- [x] 2.2 Design `person_restriction` (ALLERGY | HARD_RESTRICTION) as a model that is never
      scored and never converted into a recommendation signal — this is a hard invariant, not a
      style preference. — 0010 `person_restriction` has no sentiment/confidence column; domain
      `PersonRestriction` (household.go) has no `Sentiment`/`Confidence` field, so it cannot be
      fed into preference scoring.
- [x] 2.3 Decide restriction attribution: who may set/clear a restriction, and how that actor is
      recorded (household admin? the person themself? a caregiver for a child?). — design.md
      Step 5 + invariant 3: `SetRestriction`/`ClearRestriction` carry `recorded_by`/`cleared_by`;
      0010 stores both (cleared rows kept for the audit trail, never deleted).
- [x] 2.4 Explicitly verify no code path can derive/update `person_restriction` from
      `preference_observation`, meal reactions, or any scored/inferred source. — invariant 3; the
      only writers are the explicit `SetRestriction`/`ClearRestriction` commands (no trigger, no
      scoring path reads `preference_observation` to write a restriction).

## 3. Ingredient model (canonical vs. Product)

- [x] 3.1 Confirm canonical semantic Ingredient (e.g. "chicken breast") stays distinct from
      Product (e.g. "Garant Kycklingfilé 900g") — treat this as non-negotiable per PLAN.md
      unless research surfaces overwhelming contrary evidence; document if any surfaces. —
      structurally enforced 2026-08-19: `product` (name/brand/package_size) is a separate table
      from `ingredient` (id/display only, unchanged); no brand/package column was added to
      `ingredient`.
- [x] 3.2 Audit existing `ingredient` table (`id`, `display`) for what's missing to support
      forms/substitution without polluting it with brand/package data. — design.md Step 4 +
      persistence sketch: `ingredient` gains only a nullable `merged_into_id`; forms and
      substitution live in their own tables, so `ingredient` stays a bare canonical id + display.
- [x] 3.3 Design ingredient canonicalization (merge duplicates) without breaking FKs from
      `recipe_ingredient`/`ingredient_mapping`/new tables — evaluate a `merged_into_id`
      self-reference vs. a separate alias table. — design.md Step 4: `merged_into_id` nullable
      self-FK (chosen over an alias table); 0010 adds it with `ON DELETE SET NULL` so FKs and
      history survive a merge.

## 4. Ingredient forms

- [x] 4.1 Investigate representing fresh/dried/canned/frozen states: candidate
      `ingredients` + `ingredient_forms` table vs. a related-ingredient graph (as PLAN.md poses
      both options) — pick one and justify against the fresh/dried basil, fresh/canned/crushed
      tomato, fresh/dried pasta, fresh/frozen vegetable examples. — design.md Step 1/2/4: an
      `ingredient_form` table (belongs to exactly one Ingredient, invariant 6) — a form is a
      state of the same canonical ingredient, not a related ingredient, so the graph option was
      rejected.
- [x] 4.2 Research external taxonomies and mature implementations (Mealie "foods", Grocy
      product form/quantity-unit handling, Open Food Facts categories) for prior art before
      finalizing the shape. — design.md Step 1 (UnitConversion note) + `docs/research/mealie-*.md`
      and `grocy-*.md`: Mealie has no unit/form conversion system; Grocy's is buggy (invariant
      11) — no prior art to copy, so the shape is designed from the invariants.
- [x] 4.3 Decide whether a `default_form` belongs on `ingredient`, on `ingredient_mapping`
      (as it does today, `default_form TEXT`), or on both, and reconcile. — design.md Step 4:
      `default_form` stays on `ingredient_mapping` (unchanged) and on `Product`; it is NOT added
      to `ingredient` (a canonical ingredient has no single default form — e.g. basil is fresh
      or dried depending on use).

## 5. Ingredient substitution

- [x] 5.1 Model substitution as explicit and directional, per PLAN.md's candidate categories:
      `EQUIVALENT`, `GOOD`, `ACCEPTABLE`, `FORM`, `DIETARY`, `EMERGENCY`. — design.md Step 1/3 +
      invariant 7; 0010 `ingredient_substitution` carries the six categories and is a directed
      edge (`from_ingredient_id` → `to_ingredient_id`).
- [x] 5.2 Research quantity conversion semantics for substitution (fresh basil → dried basil,
      conversion != 1:1) — decide whether ratio is a flat multiplier, a range, or a note-only
      field for cases too irregular to encode. — design.md Step 1 + invariant 8: a flat `ratio`
      multiplier (to's quantity per from's), `CHECK (ratio > 0)`, never assumed 1:1.
- [x] 5.3 Decide whether substitution can target a specific `IngredientForm` (e.g. "fresh
      tomato → canned tomato" as a FORM substitution vs. "chicken → tofu" as a DIETARY
      substitution with no form involved). — design.md Step 5: `DefineSubstitution` takes
      optional `from_form`/`to_form`; 0010 stores both (nullable, `COALESCE` in the unique index).

## 6. Unit system

- [x] 6.1 Study both Mealie's and Grocy's unit models — done 2026-08-16. Mealie has **no unit
      conversion system at all** (`docs/research/mealie-api-and-database.md`) — no prior art to
      lean on there. Grocy has one, with a confirmed live bug: creating a product whose
      purchase unit differs from its stock unit silently auto-inserts a wrong 1:1 conversion
      via a trigger, which then collides with an explicit factor set afterward
      (`docs/research/grocy-units-and-planning.md`). See `design.md` invariant 11, added
      directly in response.
- [x] 6.2 Define the universal unit set PLAN.md lists: g, kg, ml, dl, l, piece, tbsp, tsp,
      pinch, package, can — with explicit dimension (mass/volume/count) per unit. — 0010 seeds
      all 11 units with a `dimension` column (`CHECK (dimension IN ('mass','volume','count'))`);
      domain `Unit`/`NewUnit` (catalog.go) carry the same rule.
- [x] 6.3 Keep universal same-dimension conversions (kg↔g, l↔dl↔ml) distinct from
      ingredient-specific conversions (dl flour → g) — do not invent a universal density value
      for any ingredient. — 0010 splits `unit_conversion` (universal, same-dimension) from
      `ingredient_unit_conversion` (ingredient-scoped, may cross dimensions); domain
      `NewUnitConversion` rejects cross-dimension universal conversions.
- [x] 6.4 Decide migration path for the free-text `unit TEXT` columns already in
      `recipe_ingredient` and `shopping_requirement` — this change defines the `unit` table;
      wiring those columns to it is scoped to whichever change next touches recipes/planning. —
      design.md: the `unit` table is defined and seeded here; wiring the existing free-text
      columns is explicitly deferred to the next change that touches recipes/planning (no
      destructive change to those columns in 0010).
- [x] 6.5 Implement `design.md` invariant 11 as a real constraint, not just documentation:
      `RegisterProduct` has no code path that writes to `unit_conversion`/
      `ingredient_unit_conversion`; only `DefineUnitConversion`/`DefineIngredientUnitConversion`
      do. Add a regression test reproducing Grocy's exact scenario (product with differing
      purchase/stock units, then an explicit conversion factor set) and asserting no collision
      or silent default — coordinate with `implement-pantry-inventory` task 9.6, which expects
      this test to exist. — 0010 has no trigger on the conversion tables; `internal/persistence/
      units.go` `DefineUnitConversion`/`DefineIngredientUnitConversion` are the only writers;
      `TestInvariant11_RegisterProductCreatesNoConversions` (units_test.go) reproduces the Grocy
      scenario and asserts no auto-created row, no silent default, and a clean explicit insert.

## 7. Product (household-facing only)

- [x] 7.1 Model `products`, `product_identifiers`, `product_ingredient_mappings` per PLAN.md's
      candidate, covering commercial packaged, commercial unpackaged, and manual/generic
      products. — 0008 `product`/`product_identifier`/`product_ingredient_mapping` +
      `internal/persistence/catalog.go` (CreateProduct/GetProduct/identifier + mapping methods).
- [x] 7.2 Confirm barcode is optional on `product_identifiers`, not a required identity field. —
      `product_identifier` is a separate table with no row required per product (2026-08-19);
      `Store.GetProduct`/`CreateProduct` never touch it.
- [x] 7.3 Explicitly stop at the `Product` boundary: do not model `RetailerProduct` or
      `StoreOffer` here — confirm the expected relationship
      `Ingredient ← Product ← RetailerProduct → StoreOffer` from PLAN.md's "Retailer Identity"
      section stays intact for Epic F to build on. — confirmed 2026-08-19: this slice adds no
      retailer/offer table or column.
- [x] 7.4 Reconcile against today's Mealie-food-id-keyed `ingredient_mapping` table — decide
      whether it is superseded, kept parallel, or folded into `product_ingredient_mapping`
      later (do not rename/drop it in this change). — design.md persistence sketch:
      `ingredient_mapping` (Mealie-food-id-keyed) stays scoped to Mealie sync; the new
      `product_ingredient_mapping` is separate (not a rename); both coexist, a follow-up may fold
      Mealie food ids into `Product`. 0010 does not rename or drop `ingredient_mapping`.

## 8. Persistence (Step 7+)

- [x] 8.1 For every new table, answer PLAN.md's Database Review Questions: domain concept,
      owner, mutator, mutability, history requirement, lifecycle, deletion behavior,
      uniqueness constraints, external ids, indexing, FK-ability, and whether any JSON column
      is used because it's correct or because modeling was too hard. — design.md Steps 2–4
      (aggregates = owner, commands = mutator, lifecycle table = mutability/history/deletion)
      plus the per-table comments in 0010 (uniqueness, indexing, FK behavior); no JSON column is
      used in any new table.
- [x] 8.2 Write the additive migration (`migrations/0002_household_and_catalog.sql` or similar)
      extending `0001_init.sql` — no destructive changes to existing tables/data. —
      `migrations/0010_household_catalog.sql` (numbered after the already-shipped 0008/0009);
      additive only — verified to apply cleanly on a fresh DB and on a DB with existing 0001 data.
- [x] 8.3 Add Go domain types in `internal/domain` for Household, Person, Account,
      PersonRestriction, IngredientForm, IngredientSubstitution, Unit, UnitConversion, Product. —
      `internal/domain/household.go` (Household, Account, HouseholdMembership, PersonRestriction)
      and `internal/domain/catalog.go` (Unit, UnitConversion, IngredientUnitConversion,
      IngredientForm, IngredientSubstitution, Product); `Person` already exists in domain.go.
- [x] 8.4 Confirm no `entity_type`/`entity_id`/`value` polymorphic table was introduced without
      a conscious, documented tradeoff (PLAN.md's "Do Not Use Generic Polymorphism Carelessly"). —
      0010 introduces no polymorphic table; the one deliberate exception (preferences/restrictions
      target a free-text `tag`, not an Ingredient FK) is documented in design.md Step 3 as a
      single target shape, not a polymorphic union.

## 9. Verification

- [x] 9.1 `openspec validate establish-household-and-catalog` passes. — run 2026-08-21, passes.
- [x] 9.2 Migration applies cleanly against a fresh Postgres and against a database already
      containing `0001_init.sql` data. — verified 2026-08-21 against a throwaway Postgres 16:
      0001–0010 apply in order on a fresh DB, and 0010 applies on top of a DB with existing
      person/preference/ingredient rows (seeds + default-household migration confirmed).
- [x] 9.3 Unit tests for new Go domain types (invariants: restriction never scored, substitution
      directional, unit dimension checks). — `internal/domain/household_test.go` (restriction
      attribution/clearing, no scoring fields) and `internal/domain/catalog_test.go` (unit
      dimension, universal vs ingredient-specific conversion, substitution directionality).
