## 1. Household & identity vocabulary

- [ ] 1.1 Confirm the candidate vocabulary from PLAN.md's "Household" section: `households`,
      `accounts`, `persons`, `household_memberships` — validate each against
      `migrations/0001_init.sql`'s existing flat `person` table.
- [ ] 1.2 Do not conflate login identity with household Person — decide the exact shape of the
      `Account` ↔ `Person` relationship (optional 1:1 for this change; multi-household Account
      support explicitly deferred) and document the deferral.
- [ ] 1.3 Decide `HouseholdMembership` lifecycle fields (`joined_at`/`ended_at`) so ending a
      membership never deletes a `Person` or their history.
- [ ] 1.4 Write a migration script for existing `person` rows: assign them to a default
      household + membership without touching `person_preference`/`preference_observation`.

## 2. Preferences vs. restrictions

- [ ] 2.1 Separate LIKES/DISLIKES from ALLERGIES/HARD RESTRICTIONS as PLAN.md requires — confirm
      `person_preference`/`preference_observation` remain the LIKES/DISLIKES model unchanged.
- [ ] 2.2 Design `person_restriction` (ALLERGY | HARD_RESTRICTION) as a model that is never
      scored and never converted into a recommendation signal — this is a hard invariant, not a
      style preference.
- [ ] 2.3 Decide restriction attribution: who may set/clear a restriction, and how that actor is
      recorded (household admin? the person themself? a caregiver for a child?).
- [ ] 2.4 Explicitly verify no code path can derive/update `person_restriction` from
      `preference_observation`, meal reactions, or any scored/inferred source.

## 3. Ingredient model (canonical vs. Product)

- [ ] 3.1 Confirm canonical semantic Ingredient (e.g. "chicken breast") stays distinct from
      Product (e.g. "Garant Kycklingfilé 900g") — treat this as non-negotiable per PLAN.md
      unless research surfaces overwhelming contrary evidence; document if any surfaces.
- [ ] 3.2 Audit existing `ingredient` table (`id`, `display`) for what's missing to support
      forms/substitution without polluting it with brand/package data.
- [ ] 3.3 Design ingredient canonicalization (merge duplicates) without breaking FKs from
      `recipe_ingredient`/`ingredient_mapping`/new tables — evaluate a `merged_into_id`
      self-reference vs. a separate alias table.

## 4. Ingredient forms

- [ ] 4.1 Investigate representing fresh/dried/canned/frozen states: candidate
      `ingredients` + `ingredient_forms` table vs. a related-ingredient graph (as PLAN.md poses
      both options) — pick one and justify against the fresh/dried basil, fresh/canned/crushed
      tomato, fresh/dried pasta, fresh/frozen vegetable examples.
- [ ] 4.2 Research external taxonomies and mature implementations (Mealie "foods", Grocy
      product form/quantity-unit handling, Open Food Facts categories) for prior art before
      finalizing the shape.
- [ ] 4.3 Decide whether a `default_form` belongs on `ingredient`, on `ingredient_mapping`
      (as it does today, `default_form TEXT`), or on both, and reconcile.

## 5. Ingredient substitution

- [ ] 5.1 Model substitution as explicit and directional, per PLAN.md's candidate categories:
      `EQUIVALENT`, `GOOD`, `ACCEPTABLE`, `FORM`, `DIETARY`, `EMERGENCY`.
- [ ] 5.2 Research quantity conversion semantics for substitution (fresh basil → dried basil,
      conversion != 1:1) — decide whether ratio is a flat multiplier, a range, or a note-only
      field for cases too irregular to encode.
- [ ] 5.3 Decide whether substitution can target a specific `IngredientForm` (e.g. "fresh
      tomato → canned tomato" as a FORM substitution vs. "chicken → tofu" as a DIETARY
      substitution with no form involved).

## 6. Unit system

- [ ] 6.1 Study both Mealie's and Grocy's unit models (already investigated in
      `docs/research/` reference-lab material where available; re-verify against
      `docs/research/current-state.md`) for their unit/conversion representations.
- [ ] 6.2 Define the universal unit set PLAN.md lists: g, kg, ml, dl, l, piece, tbsp, tsp,
      pinch, package, can — with explicit dimension (mass/volume/count) per unit.
- [ ] 6.3 Keep universal same-dimension conversions (kg↔g, l↔dl↔ml) distinct from
      ingredient-specific conversions (dl flour → g) — do not invent a universal density value
      for any ingredient.
- [ ] 6.4 Decide migration path for the free-text `unit TEXT` columns already in
      `recipe_ingredient` and `shopping_requirement` — this change defines the `unit` table;
      wiring those columns to it is scoped to whichever change next touches recipes/planning.

## 7. Product (household-facing only)

- [ ] 7.1 Model `products`, `product_identifiers`, `product_ingredient_mappings` per PLAN.md's
      candidate, covering commercial packaged, commercial unpackaged, and manual/generic
      products.
- [ ] 7.2 Confirm barcode is optional on `product_identifiers`, not a required identity field.
- [ ] 7.3 Explicitly stop at the `Product` boundary: do not model `RetailerProduct` or
      `StoreOffer` here — confirm the expected relationship
      `Ingredient ← Product ← RetailerProduct → StoreOffer` from PLAN.md's "Retailer Identity"
      section stays intact for Epic F to build on.
- [ ] 7.4 Reconcile against today's Mealie-food-id-keyed `ingredient_mapping` table — decide
      whether it is superseded, kept parallel, or folded into `product_ingredient_mapping`
      later (do not rename/drop it in this change).

## 8. Persistence (Step 7+)

- [ ] 8.1 For every new table, answer PLAN.md's Database Review Questions: domain concept,
      owner, mutator, mutability, history requirement, lifecycle, deletion behavior,
      uniqueness constraints, external ids, indexing, FK-ability, and whether any JSON column
      is used because it's correct or because modeling was too hard.
- [ ] 8.2 Write the additive migration (`migrations/0002_household_and_catalog.sql` or similar)
      extending `0001_init.sql` — no destructive changes to existing tables/data.
- [ ] 8.3 Add Go domain types in `internal/domain` for Household, Person, Account,
      PersonRestriction, IngredientForm, IngredientSubstitution, Unit, UnitConversion, Product.
- [ ] 8.4 Confirm no `entity_type`/`entity_id`/`value` polymorphic table was introduced without
      a conscious, documented tradeoff (PLAN.md's "Do Not Use Generic Polymorphism Carelessly").

## 9. Verification

- [ ] 9.1 `openspec validate establish-household-and-catalog` passes.
- [ ] 9.2 Migration applies cleanly against a fresh Postgres and against a database already
      containing `0001_init.sql` data.
- [ ] 9.3 Unit tests for new Go domain types (invariants: restriction never scored, substitution
      directional, unit dimension checks).
