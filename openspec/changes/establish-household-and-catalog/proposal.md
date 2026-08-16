## Why

`food-brain-first-slice` built a flat `person` table with no notion of a household, and a
preference model (`person_preference` + `preference_observation`) with no counterpart for
allergies or hard restrictions — a real safety gap once anyone other than the original
family uses this. It also has an `ingredient` table that is correctly canonical (no brand,
no package size) but nothing above it: no `product`, no ingredient forms (fresh vs. dried vs.
canned), no substitution model, and no real `unit` table (units are free-text strings on
`recipe_ingredient`/`shopping_requirement` today). Epic B's tracking issue calls for exactly
this: Household, Person, Preferences/Restrictions, Ingredient, Unit, and the household-facing
half of Product — the foundation every later Epic B/C/D/E/F change (meals, recipe family,
pantry, planning, retailer pricing) will sit on. Per `PLAN.md`'s Database Design Process, this
proposal works through vocabulary/aggregates/relationships/lifecycle/commands/invariants in
`design.md` before proposing any table.

## What Changes

- Introduce `Household` and `HouseholdMembership`, and decouple login identity
  (`Account`, minimally reserved here) from the food-domain `Person` that already exists in
  `migrations/0001_init.sql`. Every existing `person` row gets a household and a membership;
  no existing preference/observation data is touched.
- Introduce `PersonRestriction` (ALLERGY / HARD_RESTRICTION) as a model **separate** from
  `person_preference` — never scored, never derived from observations, only settable/clearable
  by an explicit, attributed command. This is the hard invariant PLAN.md calls out: an allergy
  must never become a recommendation signal.
- Extend the `ingredient` table with `IngredientForm` (fresh/dried/canned/frozen) and a
  directional `IngredientSubstitution` model (EQUIVALENT/GOOD/ACCEPTABLE/FORM/DIETARY/EMERGENCY
  categories, explicit non-1:1 quantity ratio).
- Introduce a real `Unit` reference table (g, kg, ml, dl, l, piece, tbsp, tsp, pinch, package,
  can) with universal same-dimension conversions, and a separate `ingredient_unit_conversion`
  table for the cross-dimension (volume→mass) conversions that must be ingredient-specific
  (no universal density).
- Introduce household-facing `Product` (+ `ProductIdentifier` for optional barcode,
  `ProductIngredientMapping`), distinct from `Ingredient`, and explicitly stopping short of
  `RetailerProduct`/`StoreOffer`/pricing, which stay in Epic F.
- Reconcile with, not replace, the existing schema: `person`, `person_preference`,
  `preference_observation`, and `ingredient` are extended/referenced, not renamed or dropped.

## Capabilities

### New Capabilities

- `household`: household/account/person/membership modeling and the
  preference-vs-restriction split.
- `ingredient-catalog`: canonical ingredient vocabulary, forms, directional substitution, the
  unit system, and household-facing Product distinct from Ingredient and from
  RetailerProduct/StoreOffer.

### Modified Capabilities

<!-- none — extends existing tables via new migrations, no prior spec capability existed for
     these tables -->

## Impact

- Affected code: `migrations/` (new migration extending `0001_init.sql`'s domain, additive
  only), `internal/domain` (new Household/Person/Restriction/Ingredient/Unit/Product types).
- No breaking change to `food-brain-first-slice`'s scorer or `internal/retailer` — this change
  only adds tables/types; wiring persistence for meal planning to consume Household/Person
  instead of a flat person list is follow-up work (tracked separately, not in this change).
- Explicitly out of scope: `RetailerProduct`, `StoreOffer`, pricing (Epic F); real
  authentication/session handling for `Account` (future change — this change only reserves the
  FK).
