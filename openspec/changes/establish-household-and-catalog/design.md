## Context

`PLAN.md`'s "Database Design Process" is explicit: do not begin by drafting tables. Work
vocabulary → aggregates → relationships → lifecycle → commands → invariants, and only then
propose persistence. This design.md performs that sequence for the four domains this change
owns: Household/Person/identity, Preferences/Restrictions, Ingredient (canonical
vocabulary, forms, substitution), and the Unit system, plus the household-facing half of
Product (canonical Product, distinct from `RetailerProduct`/`StoreOffer`, which stay in
Epic F).

This is **not** a greenfield design. `migrations/0001_init.sql` already ships `person`,
`person_preference`, `preference_observation`, `ingredient`, and `ingredient_mapping`, built
for `food-brain-first-slice`'s narrower need (a flat family, Mealie-sourced ingredient
strings). Those tables encode real decisions worth keeping — notably the
preference/observation split (a derived, confidence-weighted `sentiment` belief separated
from an append-only observation ledger) and the refusal to let `ingredient` hold anything
except a canonical id + display string. This change extends that schema to add the concepts
it doesn't yet have (`household`, `household_membership`, an explicit
allergy/restriction model separate from `person_preference`, ingredient forms,
directional substitution, a real unit table, and household-facing `product`), and reconciles
naming rather than silently replacing what's there.

## Step 1 — Vocabulary

| Term | Definition |
|---|---|
| **Account** | A login identity (credential, email, auth). Exists independently of any household. |
| **Household** | The unit that plans, cooks, and shops together. Owns membership, preferences, cookbook. |
| **Person** | A household member as a food-domain subject (has preferences, eats meals, gets rated reactions). May or may not have an Account. |
| **HouseholdMembership** | The join between a Household and a Person, with a lifecycle (joined/left) — not a bare join table. |
| **Preference** | An explicit or observed LIKE/DISLIKE sentiment a Person holds toward an ingredient or tag. Advisory, scored, never safety-critical. |
| **Restriction** | An ALLERGY or HARD RESTRICTION a Person holds against an ingredient or tag. Categorical, not scored, safety-critical. |
| **Ingredient** | The canonical semantic foodstuff ("chicken breast"), independent of brand or packaging. |
| **IngredientForm** | A preparation/preservation state of an Ingredient (fresh, dried, canned, frozen) that changes how it's used and measured. |
| **IngredientSubstitution** | A directed, categorized relationship from one Ingredient(+Form) to another, with a non-implied 1:1 quantity ratio. |
| **Unit** | A universal, dimensioned measure (mass/volume/count) — g, kg, ml, dl, l, piece, tbsp, tsp, pinch, package, can. |
| **UnitConversion** | A conversion between two Units. Universal within a dimension (1 dl = 100 ml); cross-dimension (volume→mass) only ever ingredient-specific, never a global density. |
| **Product** | A concrete, purchasable good ("Garant Kycklingfilé 900g"), household-facing and retailer-agnostic. Distinct from `RetailerProduct`/`StoreOffer` (Epic F), which attach a specific retailer SKU and price to a Product. |
| **ProductIngredientMapping** | The link from a Product to the canonical Ingredient(s) it represents, e.g. for shopping-list resolution. |

## Step 2 — Aggregates

- **Household aggregate** (root: `Household`). Contains `HouseholdMembership` rows. A
  `Household` is the consistency boundary for "who is in this house" — memberships are
  created/ended only through the household.
- **Account** is its own aggregate (root: `Account`), entirely outside the food domain. It is
  *referenced* by a `Person` (optional FK) but never owned by a Household — an Account is not
  invited to or removed from a household; a `HouseholdMembership` is.
- **Person aggregate** (root: `Person`). Owns that person's `PersonPreference`,
  `PreferenceObservation`, and `PersonRestriction` rows. A Person exists once per household
  membership context — see Step 6 for the multi-household question this raises.
- **Ingredient aggregate** (root: `Ingredient`). Owns its `IngredientForm` rows. Curated,
  household-independent, shared vocabulary (like a dictionary, not household data).
  `IngredientSubstitution` is a relationship *between* two Ingredient aggregates, so it is
  modeled as its own small aggregate (root: the substitution edge itself) rather than owned by
  either endpoint — it has its own lifecycle (proposed/curated/retired) independent of either
  ingredient's.
- **Unit** is reference/seed data — effectively a fixed enumeration with a conversion table,
  not a household- or ingredient-owned aggregate. `UnitConversion` rows that are
  ingredient-specific (dl flour → g) are owned by the `Ingredient` aggregate they qualify.
- **Product aggregate** (root: `Product`). Owns `ProductIngredientMapping` rows (a Product may
  map to more than one Ingredient — e.g. a spice mix). Products are global/shared catalog
  data like Ingredient, not household-scoped (a household's *pin* toward a product is a
  separate, already-shipped concept in the retailer-adapter's pin store, out of scope here).

## Step 3 — Conceptual relationships

```text
Account (0/1) ── linked to ── (0/1) Person
                                   │
                                   │ (1) via HouseholdMembership (N)
                                   ▼
                               Household
                                   │
                     ┌─────────────┴─────────────┐
                     ▼                            ▼
              PersonPreference              PersonRestriction
              (LIKE/DISLIKE, tag)           (ALLERGY/HARD, tag)
                     │                            │
                     └──────────┬─────────────────┘
                                ▼
                            Ingredient ◄── IngredientForm (fresh/dried/canned/frozen)
                                │
                     ┌──────────┴──────────┐
                     ▼                      ▼
          IngredientSubstitution        ProductIngredientMapping
          (directional, categorized,          │
           ratio, form-aware)                 ▼
                                            Product
                                                │
                                     (Epic F, out of scope)
                                                ▼
                                          RetailerProduct → StoreOffer

Unit ── UnitConversion (universal, same dimension)
Ingredient ── UnitConversion (ingredient-specific, cross-dimension, e.g. "1 dl flour = 60 g")
```

Key relationship decisions:
- `PersonPreference`/`PersonRestriction` target a *tag* (free-text, as `person_preference`
  already does), not strictly an `Ingredient` FK — preferences also apply to cuisines,
  textures, and traits ("spicy", "fish") that aren't ingredients. Where a tag happens to name
  a canonical ingredient, no FK is enforced; this trades referential integrity for the
  flexibility PLAN.md's preference model needs. This is a conscious exception, not a lapse
  into `entity_type/entity_id/value` polymorphism — there is exactly one target shape (tag
  string), not a polymorphic union of tables.
- `IngredientSubstitution` is **not** symmetric: it has a `from_ingredient_id`,
  `to_ingredient_id`, `category`, and `ratio` (to's quantity per from's quantity). A→B being
  EQUIVALENT does not create B→A automatically.
- `Product → Ingredient` is many-to-many via `ProductIngredientMapping` (a "taco kit" product
  maps to several ingredients), each mapping optionally carrying a quantity (how much of the
  ingredient one unit of the product yields).

## Step 4 — Lifecycle (mutable vs. immutable)

| Entity | Lifecycle |
|---|---|
| `Household` | Mutable (rename); soft states (`active`/`archived`), never hard-deleted while it has history. |
| `Person` | Mutable (name, weight); membership start/end tracked on `HouseholdMembership`, not by deleting the Person. |
| `Account` | Mutable, owned by its own future auth change — this change only reserves the FK slot on `Person`. |
| `HouseholdMembership` | Append + close: a row is created on join and gets an `ended_at` on leaving; never deleted (history matters for "who was in the household when this meal was rated"). |
| `PersonPreference` | Mutable, **derived** — recomputed from `PreferenceObservation` history (unchanged from the existing schema). |
| `PreferenceObservation` | Append-only ledger (unchanged). |
| `PersonRestriction` (allergy/hard restriction) | Mutable but **never derived** — set and cleared only by explicit command, each change attributed (who recorded it, when) because it's safety-critical. Not scored, not decayed, not inferred from reactions. |
| `Ingredient` | Effectively immutable identity once referenced by recipes/products; display text may be corrected; canonicalization merges (duplicate → canonical) are modeled as an explicit `merged_into_id`, not a delete, so history/FKs survive. |
| `IngredientForm` | Mutable reference data, curated. |
| `IngredientSubstitution` | Mutable, curated (can be retired without deleting, so past recommendations remain explainable). |
| `Unit` | Effectively immutable/seeded — the 11 units PLAN.md lists ship in a migration, not created via the app. |
| `UnitConversion` | Mutable, curated (a new ingredient-specific conversion can be added or corrected). |
| `Product` | Mutable (display name, default form); a genuine repackaging (900g → 800g) is a *new* Product, not a mutation, since quantity is part of identity for shopping resolution. |
| `ProductIngredientMapping` | Mutable, curated (same shape as today's `ingredient_mapping`, generalized off Mealie-food-id onto Product). |

## Step 5 — Commands

- `CreateHousehold(name)`
- `AddPersonToHousehold(householdId, personName, weight?)` → creates `Person` +
  `HouseholdMembership`
- `EndHouseholdMembership(householdId, personId)`
- `LinkAccountToPerson(accountId, personId)`
- `SetPreference(personId, tag, sentiment, source)` (existing shape, unchanged)
- `RecordPreferenceObservation(personId, tag, sentiment, source)` (existing, unchanged)
- `SetRestriction(personId, tag, kind: ALLERGY|HARD_RESTRICTION, note?, recordedBy)`
- `ClearRestriction(personId, tag, recordedBy)`
- `DefineIngredient(canonicalId, display)`
- `MergeIngredients(duplicateId, canonicalId)`
- `AddIngredientForm(ingredientId, form, notes?)`
- `DefineSubstitution(fromIngredientId, fromForm?, toIngredientId, toForm?, category, ratio)`
- `RetireSubstitution(substitutionId)`
- `DefineUnitConversion(fromUnit, toUnit, factor)` (universal) or
  `DefineIngredientUnitConversion(ingredientId, fromUnit, toUnit, factor)` (ingredient-specific)
- `RegisterProduct(name, defaultUnit, quantity, barcode?, kind: PACKAGED|UNPACKAGED|MANUAL)`
- `MapProductToIngredient(productId, ingredientId, quantity?)`

## Step 6 — Invariants

1. **Login identity is never conflated with household Person.** `Account` and `Person` are
   separate tables with an optional FK; a Person may exist with no Account (a child); deleting
   an Account SHALL NOT delete a Person or their history.
2. **A restriction is never scored as a preference.** `PersonRestriction` (ALLERGY/HARD
   RESTRICTION) SHALL be a distinct table/model from `PersonPreference`
   (LIKE/DISLIKE). The system SHALL NOT use a restriction as positive or negative input to a
   preference sentiment/confidence computation.
3. **Restriction changes are explicit and attributed.** A `PersonRestriction` row SHALL NOT be
   created, changed, or cleared by inference from `PreferenceObservation` or any automated
   scoring path — only by an explicit command with an attributed actor.
4. **Ingredient is canonical and product-independent.** `Ingredient` SHALL NOT carry a brand,
   package size, barcode, or retailer identifier. Those live on `Product` or downstream on
   `RetailerProduct` (Epic F).
5. **A Product maps to Ingredient(s), never the reverse ontology.** `Ingredient` SHALL NOT
   reference `Product`; `ProductIngredientMapping` is the only link, and it is optional (an
   unmapped Product may exist, flagged for review, mirroring today's
   `ingredient_mapping.needs_review`).
6. **An IngredientForm belongs to exactly one Ingredient.**
7. **Substitution is directional.** An `IngredientSubstitution` from A to B SHALL NOT imply a
   substitution from B to A; the reverse, if valid, is a separate row (possibly a different
   category/ratio).
8. **Substitution quantity is never assumed 1:1.** Every `IngredientSubstitution` SHALL carry
   an explicit ratio (default 1.0 only when genuinely equivalent), not an implicit one.
9. **Units carry an explicit dimension; conversions never cross dimensions globally.** A
   `UnitConversion` between units of the same dimension (mass↔mass, volume↔volume) is
   universal. Any mass↔volume conversion (e.g. dl flour → g) SHALL be scoped to a specific
   Ingredient (and optionally Form) — the system SHALL NOT invent or apply a universal density.
10. **Household membership has history.** Ending a membership SHALL NOT delete the `Person` or
    their preference/restriction/meal history — it closes the `HouseholdMembership` row.

## Persistence sketch (bridging to Step 7, detailed in tasks.md/spec deltas)

Extends `migrations/0001_init.sql` rather than replacing it:
- **New tables**: `household`, `household_membership` (FK to existing `person`),
  `account` (minimal — real auth is a separate future change), `person_restriction`, `unit`,
  `unit_conversion`, `ingredient_unit_conversion`, `ingredient_form`,
  `ingredient_substitution`, `product`, `product_identifier`, `product_ingredient_mapping`.
- **Reused as-is**: `person`, `person_preference`, `preference_observation`, `ingredient`
  (only relaxed to add `merged_into_id` nullable self-FK for canonicalization).
- **Reconciled naming**: today's `ingredient_mapping` (Mealie-food-id-keyed) stays scoped to
  the Mealie-sync use case; the new, general `product_ingredient_mapping` is not a rename of
  it — both can coexist, or a follow-up change may fold Mealie food ids into `Product` as a
  `source: 'mealie'` kind once this schema exists. This change does not delete or rename
  `ingredient_mapping`.
