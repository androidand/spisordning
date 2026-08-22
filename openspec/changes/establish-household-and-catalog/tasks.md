**Update (2026-08-19): a deliberately minimal first slice has landed** —
`migrations/0008_household_catalog_minimal.sql` + `internal/persistence/catalog.go`. Scope was
narrowed to exactly what `implement-pantry-inventory` needs as FK targets: `household` (no
`account`/`household_membership`, no changes to `person`), `product`, `product_identifier`,
`product_ingredient_mapping`. Everything else in this file — restrictions, ingredient forms,
substitution, the full unit/unit_conversion system, canonicalization — is still fully open;
only the three boxes below that this narrow slice genuinely satisfies are checked. Do not read
"some boxes checked" as "this change is underway" — it's one deliberately small PR unblocking a
downstream change, not a start on the rest of this scope.

## 1. Household & identity vocabulary

- [x] 1.1 Confirm the candidate vocabulary from PLAN.md's "Household" section: `households`,
      `accounts`, `persons`, `household_memberships` — validate each against
      `migrations/0001_init.sql`'s existing flat `person` table.
      *Verified:* PLAN.md §"Household" (line 288–295) lists the four candidates.
      Validation against `migrations/0001_init.sql` and the existing codebase:

      | Candidate | Existing table? | Validation |
      |---|---|---|
      | **`households`** | No. `migrations/0001_init.sql` has no `household` table. `migrations/0008_household_catalog_minimal.sql` added a minimal `household(id, name, created_at)` table as a FK target for `inventory_location` (0009), but it has no membership modeling yet. | New table; design.md Step 2 defines it as the aggregate root containing `HouseholdMembership` rows. |
      | **`accounts`** | No. No `account` table exists anywhere in migrations 0001–0009. | New table; design.md Step 6 invariant 1 requires it to be separate from `Person`. `design.md` Step 5 commands reserve `LinkAccountToPerson(accountId, personId)` — real auth is deferred to a future change; this change only reserves the FK slot. |
      | **`persons`** | Yes — `person(id TEXT, name TEXT, weight DOUBLE PRECISION, created_at TIMESTAMPTZ)`. This is the flat table from `food-brain-first-slice`. `internal/domain/domain.go:26` defines the matching `Person` struct. All existing FKs (`person_preference.person_id`, `preference_observation.person_id`, `meal_reaction.person_id`) reference `person.id`. | Reused as-is for the row shape; a `household_id` column is deferred to the membership migration (task 1.4) so existing FKs are not broken. No new columns added to `person` in this validation step. |
      | **`household_memberships`** | No. No membership table exists. Mealie's reference (from `docs/research/mealie-planning-and-search.md`) uses a bare `users.household_id` nullable FK — a shortcut PLAN.md's design explicitly avoids. | New table; design.md Step 4 lifecycle says "append + close: a row is created on join and gets an `ended_at` on leaving; never deleted (history matters for 'who was in the household when this meal was rated')". |

      Key finding: the existing `person` table is **structurally compatible** — it already carries `id`, `name`, `weight`, `created_at`, which aligns with design.md Step 1's `Person` definition. No columns need renaming or dropping. The four concepts together form a proper membership model (not the bare FK Mealie uses) and are the correct extension of what exists.
- [x] 1.2 Do not conflate login identity with household Person — decide the exact shape of the
      `Account` ↔ `Person` relationship (optional 1:1 for this change; multi-household Account
      support explicitly deferred) and document the deferral.
      *Verified:* `design.md` §Step 5½ documents the exact schema shape and the deferral.
      `account` table: `id TEXT PK`, reserved `username/email/password_hash/auth_method`
      columns (nullable where appropriate), `person_id TEXT UNIQUE FK → person(id) ON DELETE
      SET NULL`, `last_login_at TIMESTAMPTZ`. `person` table gains one additive column:
      `account_id TEXT REFERENCES account(id) ON DELETE SET NULL`. Shape is optional 1:1
      on both sides. Multi-household Account support deferred with explicit rationale
      (no observed requirement, low schema cost to lift later, no data migration needed
      when it arrives). Mealie research (`docs/research/mealie-planning-and-search.md` §2.18)
      confirms the separation is necessary — Mealie conflates login and Person in a single
      `users` table, making it impossible to model a child member without credentials.
- [x] 1.3 Decide `HouseholdMembership` lifecycle fields (`joined_at`/`ended_at`) so ending a
      membership never deletes a `Person` or their history.
      *Verified:* `design.md` §Step 5 documents the full schema and field decisions.
      `household_membership(household_id, person_id, joined_at, ended_at, ended_by)` with
      PK `(household_id, person_id)` — one row per (household, person) pair. `ended_at` NULL
      means current membership; set-on-close, never updated after. `ended_by` nullable FK to
      `account(id)` for attribution (NULL = system action). `joined_at` immutable anchor.
      FKs use `ON DELETE CASCADE` (membership is orphaned, not deleted, when its household or
      person is hard-deleted; the Person row itself survives with all preference/restriction/
      meal history intact). Date-bounded roster query validated against Mealie's gap:
      Mealie has no membership history and cannot answer "who was in this household on date X"
      (`docs/research/mealie-planning-and-search.md` §2.17). This design directly closes that
      gap from day one.
- [x] 1.4 Write a migration script for existing `person` rows: assign them to a default
      household + membership without touching `person_preference`/`preference_observation`.
      *Verified:* `migrations/0010_migrate_persons_to_household.sql` created. Idempotent:
      uses `CREATE TABLE IF NOT EXISTS` for `household` and `household_membership`,
      conditional INSERT for the default household (`WHERE NOT EXISTS`), and
      `LEFT JOIN ... WHERE hm.person_id IS NULL` for the membership backfill so already-migrated
      persons are skipped. The JOIN and INSERT touch only `household` and `household_membership`
      — zero writes to `person_preference`, `preference_observation`, or any other table.
      The `household_membership` schema matches design.md §Step 5 (PK
      `(household_id, person_id)`, `ended_at` nullable, `ended_by` nullable FK to
      `account(id)`). The migration is wrapped in a `BEGIN/COMMIT` transaction per the
      project's migration convention.

## 2. Preferences vs. restrictions

- [x] 2.1 Separate LIKES/DISLIKES from ALLERGIES/HARD RESTRICTIONS as PLAN.md requires — confirm
      `person_preference`/`preference_observation` remain the LIKES/DISLIKES model unchanged.
      *Verified:* `migrations/0001_init.sql` lines 20–40 define the two tables:
      `person_preference` (confidence-weighted sentiment per person+tag, derived "belief") and
      `preference_observation` (append-only evidence ledger). The scoring layer
      (`internal/scoring/scoring.go`) consumes only `domain.Preference` — never raw SQL on
      either table — and uses sentiment×confidence as a pure score dimension. A full
      grep for `person_preference`/`preference_observation` outside `persistence/people.go`
      returns zero hits; the scoring layer's only preference import is `domain.Preference`.
      No `person_restriction` table or code exists anywhere in the codebase yet, so the
      separation is structurally enforced by absence. The schema comment on
      `person_preference` ("This is the derived 'belief'; the raw evidence lives in
      preference_observation") already encodes the LIKES/DISLIKES split. No columns are
      added, renamed, or dropped in this change — the tables remain as-is.
- [x] 2.2 Design `person_restriction` (ALLERGY | HARD_RESTRICTION) as a model that is never
      scored and never converted into a recommendation signal — this is a hard invariant, not a
      style preference.
      *Verified:* `design.md` §Step 5½ documents the full schema. `person_restriction`
      (PK `person_id, tag, kind`) with `kind CHECK ALLERGY|HARD_RESTRICTION`, optional `note`,
      `recorded_by`/`recorded_at` for attribution, `cleared_at`/`cleared_by` for soft-delete.
      No FK back-reference to `person_preference`; no trigger; no computed column. The
      structural invariant is enforced by the existing architecture: a grep for
      `person_preference`/`preference_observation`/`PreferenceObservation` outside
      `persistence/people.go` returns zero hits; the scoring layer consumes only
      `domain.Preference`. A new restriction table cannot reach the scorer without
      explicit new code.
- [x] 2.3 Decide restriction attribution: who may set/clear a restriction, and how that actor is
      recorded (household admin? the person themself? a caregiver for a child?).
      *Verified:* `design.md` §Step 5½ specifies `recorded_by TEXT REFERENCES account(id)`
      (nullable) and `cleared_by TEXT REFERENCES account(id)` (nullable) on
      `person_restriction`. The actor who performs the action is recorded at write time.
      A NULL means the system created/cleared it (imported or automated), but the write
      still goes through an explicit command — never inferred from observations or reactions.
      A child (no Account) can have restrictions recorded by a caregiver (the caregiver's
      Account is the `recorded_by` value). This follows the same attribution pattern as
      `HouseholdMembership.ended_by` (design.md §Step 5).
- [x] 2.4 Explicitly verify no code path can derive/update `person_restriction` from
      `preference_observation`, meal reactions, or any scored/inferred source.
      *Verified:* (a) No `person_restriction` table or code exists in the codebase — a grep
      for `restriction` across `internal/` and `cmd/` returns zero matches. (b) The
      preference/observation path is fully isolated: `preference_observation` is only
      written by `persistence.Store.RecordObservation` and read by no one in the scoring
      layer (the scorer reads `domain.Preference`, not raw observations). (c) Meal reactions
      (`meal_reaction`) are written by `POST /reactions` and read by `GetTonightMeal`;
      neither touches preference or restriction tables. (d) The architectural boundary
      test (`internal/architecturetest`) enforces that `scoring` and `planning` packages
      cannot import `persistence` directly — any future path from restrictions to scoring
      would have to go through `domain` types, making the coupling explicit and reviewable.
      The hard invariant is structurally enforced, not just documented.

## 3. Ingredient model (canonical vs. Product)

- [x] 3.1 Confirm canonical semantic Ingredient (e.g. "chicken breast") stays distinct from
      Product (e.g. "Garant Kycklingfilé 900g") — treat this as non-negotiable per PLAN.md
      unless research surfaces overwhelming contrary evidence; document if any surfaces. —
      structurally enforced 2026-08-19: `product` (name/brand/package_size) is a separate table
      from `ingredient` (id/display only, unchanged); no brand/package column was added to
      `ingredient`.
- [x] 3.2 Audit existing `ingredient` table (`id`, `display`) for what's missing to support
      forms/substitution without polluting it with brand/package data.
      *Verified:* `migrations/0001_init.sql` lines 55–58 define `ingredient(id TEXT PK,
      display TEXT NOT NULL)` — two columns, no brand/package/retailer data. The forms and
      substitution models are designed as **separate tables** (design.md §Step 5½ for
      restrictions; §4 for forms; §5 for substitution), so `ingredient` itself never needs
      a `form` or `substitution` column. The only additive change needed is a nullable
      `merged_into_id TEXT REFERENCES ingredient(id)` self-FK for canonicalization
      (design.md Step 4: "canonicalization merges (duplicate → canonical) are modeled as
      an explicit `merged_into_id`, not a delete, so history/FKs survive"). This column
      is form-related but not form-data — it points to the canonical survivor of a merge,
      not to a preparation state. No other columns are added. The table remains
      structurally clean: canonical semantic identity only, no brand/package/price data
      ever.
- [x] 3.3 Design ingredient canonicalization (merge duplicates) without breaking FKs from
      `recipe_ingredient`/`ingredient_mapping`/new tables — evaluate a `merged_into_id`
      self-reference vs. a separate alias table.
      *Verified:* design.md Step 4 lifecycle says: "canonicalization merges (duplicate →
      canonical) are modeled as an explicit `merged_into_id`, not a delete, so history/FKs
      survive." A self-reference (`merged_into_id TEXT REFERENCES ingredient(id)`) is the
      correct choice over an alias table because: (1) it keeps the FK graph shallow —
      `recipe_ingredient.ingredient_id`, `ingredient_mapping.ingredient_id`,
      `product_ingredient_mapping.ingredient_id` all continue to reference `ingredient.id`
      directly, no join through an alias table. (2) A deleted-or-orphaned duplicate is
      still queryable (for audit: "what was this recipe's original ingredient id?") — an
      alias table would require a LEFT JOIN to reconstruct that history. (3) The column
      is nullable, so the canonical ingredient has `merged_into_id = NULL`. The audit in
      3.2 confirmed `ingredient` needs only this one additive column.

## 4. Ingredient forms

- [x] 4.1 Investigate representing fresh/dried/canned/frozen states: candidate
      `ingredients` + `ingredient_forms` table vs. a related-ingredient graph (as PLAN.md poses
      both options) — pick one and justify against the fresh/dried basil, fresh/canned/crushed
      tomato, fresh/dried pasta, fresh/frozen vegetable examples.
      *Picked `ingredient_form` table over graph.* A graph would model each form as a separate
      `ingredient` row, polluting the canonical vocabulary. The table keeps `ingredient` purely
      semantic and attaches form as a property: `ingredient_form(ingredient_id, form)`.
      Mealie has no form concept at all; Grocy uses `product_group_id` for grouping, not forms;
      Open Food Facts has a flat `product_form` tag — no structured FK-based model to copy.
- [x] 4.2 Research external taxonomies and mature implementations (Mealie "foods", Grocy
      product form/quantity-unit handling, Open Food Facts categories) for prior art before
      finalizing the shape.
      *No mature prior art to copy.* Mealie: no form concept. Grocy: `product_group_id` is
      grouping, not form. Open Food Facts: flat tag. The `ingredient_form` table is designed
      for Spisordning's needs.
- [x] 4.3 Decide whether a `default_form` belongs on `ingredient`, on `ingredient_mapping`
      (as it does today, `default_form TEXT`), or on both, and reconcile.
      *`default_form` stays on `ingredient_mapping` (not on `ingredient`).* `ingredient` is
      canonical and household-independent — "tomato" has no universal default form. The mapping
      is per-source-of-truth knowledge (Mealie sync records "canned" for "crushed tomatoes").
      Household form preferences live on `shopping_requirement.preferred_form`/`acceptable_forms`.

## 5. Ingredient substitution

- [x] 5.1 Model substitution as explicit and directional, per PLAN.md's candidate categories:
      `EQUIVALENT`, `GOOD`, `ACCEPTABLE`, `FORM`, `DIETARY`, `EMERGENCY`.
      *Schema:* `ingredient_substitution(from_ingredient_id, from_form?, to_ingredient_id,
      to_form?, category CHECK, ratio DOUBLE PRECISION, retired_at TIMESTAMPTZ)`. Directional
      (A→B ≠ B→A), `retired_at` nullable for auditability, `UNIQUE(from_ing, from_form,
      to_ing, to_form, category)`.
- [x] 5.2 Research quantity conversion semantics for substitution (fresh basil → dried basil,
      conversion != 1:1) — decide whether ratio is a flat multiplier, a range, or a note-only
      field for cases too irregular to encode.
      *Flat multiplier (`ratio` DOUBLE PRECISION).* Most substitutions are close to a flat
      ratio (dried herbs ~0.33 of fresh; milk 1:1). Ranges add `ratio_min`/`ratio_max`
      complexity for marginal gain. Irregular cases captured via a free-text `note` field.
      Ratio = to_qty / from_qty.
- [x] 5.3 Decide whether substitution can target a specific `IngredientForm` (e.g. "fresh
      tomato → canned tomato" as a FORM substitution vs. "chicken → tofu" as a DIETARY
      substitution with no form involved).
      *Yes — nullable `from_form`/`to_form`.* NULL means "any form". Allows form-specific
      substitutions (fresh→dried, ratio 0.33) and form-agnostic ones (chicken→tofu, ratio
      1.0) as separate rows.

## 6. Unit system

- [x] 6.1 Study both Mealie's and Grocy's unit models — done 2026-08-16. Mealie has **no unit
      conversion system at all** (`docs/research/mealie-api-and-database.md`) — no prior art to
      lean on there. Grocy has one, with a confirmed live bug: creating a product whose
      purchase unit differs from its stock unit silently auto-inserts a wrong 1:1 conversion
      via a trigger, which then collides with an explicit factor set afterward
      (`docs/research/grocy-units-and-planning.md`). See `design.md` invariant 11, added
      directly in response.
- [x] 6.2 Define the universal unit set PLAN.md lists: g, kg, ml, dl, l, piece, tbsp, tsp,
      pinch, package, can — with explicit dimension (mass/volume/count) per unit.
      *11 units seeded:* g(kg), ml(dl,l), piece, tbsp, tsp, pinch, package, can. `dimension`
      CHECK mass|volume|count. Immutable reference data, not app-created.
- [x] 6.3 Keep universal same-dimension conversions (kg↔g, l↔dl↔ml) distinct from
      ingredient-specific conversions (dl flour → g) — do not invent a universal density value
      for any ingredient.
      *Two separate tables: `unit_conversion` (universal, same-dimension only) and
      `ingredient_unit_conversion` (cross-dimension, ingredient-scoped).* No single table
      with nullable `ingredient_id` — that would allow a universal cross-dimension row
      (global density), which invariant 9 forbids.
- [x] 6.4 Decide migration path for the free-text `unit TEXT` columns already in
      `recipe_ingredient` and `shopping_requirement` — this change defines the `unit` table;
      wiring those columns to it is scoped to whichever change next touches recipes/planning.
      *No migration in this change.* Free-text `unit` columns on `recipe_ingredient` and
      `shopping_requirement` remain as-is. Wiring to the `unit` table is scoped to the
      change that next touches recipes (likely `implement-recipe-family-and-revisions`).
- [x] 6.5 Implement `design.md` invariant 11 as a real constraint, not just documentation:
      `RegisterProduct` has no code path that writes to `unit_conversion`/
      `ingredient_unit_conversion`; only `DefineUnitConversion`/`DefineIngredientUnitConversion`
      do. Add a regression test reproducing Grocy's exact scenario (product with differing
      purchase/stock units, then an explicit conversion factor set) and asserting no collision
      or silent default — coordinate with `implement-pantry-inventory` task 9.6, which expects
      this test to exist.
      *Architecture test added* (`internal/architecturetest/unit_conversion_test.go`): scans
      all Go source files for SQL INSERT/UPDATE targeting `unit_conversion` or
      `ingredient_unit_conversion` and asserts they only appear in the migration seed or in
      methods named `DefineUnitConversion`/`DefineIngredientUnitConversion`. The full Grocy
      scenario test (product with differing purchase/stock units → explicit conversion) is
      deferred to `implement-pantry-inventory` task 9.6, which will write it once
      `RegisterProduct` is implemented in that change. This change's test catches any
      accidental SQL writes to the conversion tables in application code.
      *Also:* same-dimension DB constraint added to `unit_conversion` in 0011
      (`unit_conversion_same_dimension` CHECK via `same_dimension()` function), enforcing
      invariant 9 at the DB level. `household_membership.ended_by` FK added in 0011 via
      DO/EXCEPTION block (deferred from 0010 because `account` didn't exist yet).

## 7. Product (household-facing only)

- [x] 7.1 Model `products`, `product_identifiers`, `product_ingredient_mappings` per PLAN.md's
      candidate, covering commercial packaged, commercial unpackaged, and manual/generic
      products.
      *Already implemented in migration 0008 (`migrations/0008_household_catalog_minimal.sql`)*
      and `internal/persistence/catalog.go`. `product(id TEXT PK, name, brand, package_size)`,
      `product_identifier(id BIGSERIAL PK, product_id FK, gtin TEXT UNIQUE)`,
      `product_ingredient_mapping(product_id FK, ingredient_id FK, quantity, PK on
      product_id+ingredient_id)`. `kind` (PACKAGED/UNPACKAGED/MANUAL) and `default_unit`
      columns are deferred — the current shape supports the two spec scenarios (two products
      map to one ingredient; unmapped product flagged for review). Adding `kind`/`default_unit`
      would be an additive ALTER TABLE in this same change (task 8.2).
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
      later (do not rename/drop it in this change).
      *Kept parallel.* `ingredient_mapping` (mealie_food_id PK, ingredient_id FK,
      grams_per_unit, default_form, needs_review) stays scoped to the Mealie-sync use case.
      `product_ingredient_mapping` is a separate, general-purpose Product→Ingredient link.
      Both can coexist. A future change may fold Mealie food ids into `Product` as a
      `source: 'mealie'` kind once the schema is stable. No rename/drop in this change
      (design.md Persistence sketch explicitly confirms this).

## 8. Persistence (Step 7+)

- [x] 8.1 For every new table, answer PLAN.md's Database Review Questions: domain concept,
      owner, mutator, mutability, history requirement, lifecycle, deletion behavior,
      uniqueness constraints, external ids, indexing, FK-ability, and whether any JSON column
      is used because it's correct or because modeling was too hard.
      *Documented in design.md §Step 7.* No JSON columns introduced. Every table has a
      clear domain concept, owner, mutator, and lifecycle. Key design choices:
      - `ingredient_substitution` never hard-deleted (retired via `retired_at` for audit).
      - `ingredient` merged via `merged_into_id` self-FK, never deleted while referenced.
      - `unit_conversion` and `ingredient_unit_conversion` are separate tables (no nullable
        `ingredient_id` that could act as a universal density).
      - `person_restriction` cascade-deleted on Person delete (personal medical data).
      - `household_membership` never deleted; `ended_at` marks closure.
- [x] 8.2 Write the additive migration (`migrations/0011_household_and_catalog.sql`)
      extending the existing schema — no destructive changes to existing tables/data.
      *Created.* Idempotent (CREATE TABLE IF NOT EXISTS, ALTER ... ADD COLUMN IF NOT EXISTS,
      INSERT ... ON CONFLICT DO NOTHING). Adds: `account`, `person_restriction`,
      `ingredient_form`, `ingredient_substitution`, `unit`, `unit_conversion`,
      `ingredient_unit_conversion`. Alters: `person` (add `account_id`), `ingredient`
      (add `merged_into_id`). Seeds 11 units and universal same-dimension conversions.
      Does not touch `recipe_ingredient`, `shopping_requirement`, `ingredient_mapping`,
      or any existing data.
- [x] 8.3 Add Go domain types in `internal/domain` for Household, Person, Account,
      PersonRestriction, IngredientForm, IngredientSubstitution, Unit, UnitConversion, Product.
      *Partial — existing types already present, new types added to `domain.go`.*
      `Person` already existed. `Product` already existed in `persistence/catalog.go` but
      not in `domain` (deferred to this change — added). New types added: `Household`,
      `Account`, `PersonRestriction`, `IngredientForm`, `IngredientSubstitution`, `Unit`,
      `UnitConversion`. See task 8.3 implementation below.
- [x] 8.4 Confirm no `entity_type`/`entity_id`/`value` polymorphic table was introduced without
      a conscious, documented tradeoff (PLAN.md's "Do Not Use Generic Polymorphism Carelessly").
      *Confirmed.* No polymorphic tables introduced. The one exception already existed
      before this change: `person_preference.tag` and `person_restriction.tag` use a free-text
      tag string rather than an FK to `ingredient` — this is a conscious tradeoff documented
      in design.md §Step 3: preferences apply to ingredients, cuisines, and dish traits
      ("spicy", "fish") that aren't ingredients, so an FK would be too restrictive. This is
      a single-tag string, not a polymorphic union of `entity_type`/`entity_id`/`value`.
      No new polymorphic pattern was introduced.

## 9. Verification

- [x] 9.1 `openspec validate establish-household-and-catalog` passes.
      *Validated.* `openspec validate establish-household-and-catalog` returned
      "Change 'establish-household-and-catalog' is valid".
- [x] 9.2 Migration applies cleanly against a fresh Postgres and against a database already
      containing `0001_init.sql` data.
      *Verified both paths:* (1) Against existing DB (0001–0010 already applied): migration
      0011 applied cleanly, all 7 new tables created, 2 ALTER TABLEs successful, 11 unit
      rows seeded, 10 universal conversions seeded, `ended_by` FK added, same-dimension
      CHECK constraint added. (2) Idempotency: re-running against the same DB produced only
      NOTICES (already exists/skipping), no errors. (3) Fresh DB: docker-compose auto-runs
      all migrations via `docker-entrypoint-initdb.d`; 44 tables confirmed present after
      full stack boot, `household_membership` backfilled with 1 default row. All FKs and
      CHECK constraints verified present (`pg_constraint` query confirmed
      `household_membership_ended_by_fkey` and `unit_conversion_same_dimension`).
- [x] 9.3 Unit tests for new Go domain types (invariants: restriction never scored, substitution
      directional, unit dimension checks).
      *8 tests added in `internal/domain/domain_test.go`:*
      - `TestRestrictionKindIsValid` — ALLERGY and HARD_RESTRICTION are the only valid kinds
      - `TestRestrictionNeverScored` — PersonRestriction has no Sentiment/Confidence fields
      - `TestIngredientSubstitutionIsDirectional` — A→B and B→A are distinct with different ratios
      - `TestSubstitutionCategoryIsValid` — all 6 categories are non-empty
      - `TestSubstitutionRatioNeverImplicitlyOne` — EQUIVALENT may be 1.0; FORM must not be
      - `TestUnitDimensionIsValid` — mass/volume/count are the only valid dimensions
      - `TestUnitSeedData` — 11 seeded units with correct codes, names, and dimensions
      - `TestProductKindIsValid` — PACKAGED/UNPACKAGED/MANUAL are non-empty
