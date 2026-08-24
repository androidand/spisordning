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

**Update (2026-08-16): `establish-reference-lab`'s Mealie and Grocy investigations are both
complete** — see `docs/research/mealie-*.md` and `docs/research/grocy-*.md`. Findings relevant
to this change are cited inline below (Steps 1 and 6) rather than collected in one place, since
they land on specific decisions already made here, not on the design as a whole.

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
| **UnitConversion** | A conversion between two Units. Universal within a dimension (1 dl = 100 ml); cross-dimension (volume→mass) only ever ingredient-specific, never a global density. **No reference-system prior art to copy here**: `docs/research/mealie-api-and-database.md` found Mealie has no unit-conversion system at all (quantities are stored but never converted between units); `docs/research/grocy-units-and-planning.md` found Grocy has one, but a buggy one — see invariant 11 below. This table is being designed with less safety net than most of this change's other concepts. |
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

## Step 5 — HouseholdMembership lifecycle

### Exact schema shape

```sql
CREATE TABLE household_membership (
    household_id  TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    person_id     TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    joined_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at      TIMESTAMPTZ,               -- NULL = current membership
    ended_by      TEXT REFERENCES account(id), -- nullable: who ended it; NULL = system/automated
    PRIMARY KEY (household_id, person_id)
);
CREATE INDEX ON household_membership (person_id, ended_at);
```

### Field decisions

| Field | Decision | Rationale |
|---|---|---|
| `joined_at` | Required, defaults to `now()`. Never updated. | Immutable anchor: the exact moment this person joined this household. Needed for "who was in the household when this meal was rated" queries (Mealie's gap — `docs/research/mealie-planning-and-search.md` §2.17 confirms Mealie has no such timestamp). |
| `ended_at` | Nullable. Set on `EndHouseholdMembership`; left NULL for active memberships. Never updated after set. | A NULL means "currently a member." Setting it (not deleting the row) preserves the audit trail. Matching design.md Step 4: "a row is created on join and gets an `ended_at` on leaving; never deleted." |
| `ended_by` | Nullable FK to `account(id)`. Records who performed the end action. | Mirrors the attribution principle from §1.2 (Account/Person split) and the restriction-attribution principle from spec.md ("who recorded it, when"). A NULL means the system ended it (e.g., household archived), not a person. |
| `PRIMARY KEY (household_id, person_id)` | One membership row per (household, person) pair. | A person cannot have two active memberships in the same household. To re-join after leaving, the row is updated (`ended_at` set to NULL, `joined_at` updated to the re-join date) rather than inserting a new row — this keeps the row count minimal while preserving history via `ended_at`. |

### Active-membership query shape

```sql
-- Current members of a household
SELECT p.* FROM person p
JOIN household_membership hm ON hm.person_id = p.id
WHERE hm.household_id = $1 AND hm.ended_at IS NULL;

-- Who was in this household on a given date
SELECT p.* FROM person p
JOIN household_membership hm ON hm.person_id = p.id
WHERE hm.household_id = $1
  AND hm.joined_at <= $2
  AND (hm.ended_at IS NULL OR hm.ended_at >= $2);
```

The second query is the one Mealie cannot answer (no membership history = no date-bounded roster query). It is the single strongest validation for building this table from day one rather than retrofitting.

### Delete behavior

- **Hard-delete a Household**: cascades to `household_membership` rows (they become orphaned history with no household anchor). The Person rows and their preference/restriction/meal history remain intact because the FK is `ON DELETE CASCADE` on the membership side, not the Person side.
- **Hard-delete a Person**: same — membership rows are orphaned but preserved; the Person's history is untouched.
- **End a membership** (soft close): set `ended_at = now()`; do NOT delete the row. The Person can be queried later for "was this person ever a member?" and the household can answer "who was here on date X?"

---

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

## Step 5½ — Account ↔ Person shape and deferrals

### Exact schema shape for this change

**`account` table** (new, minimal — real auth is a separate future change):

```sql
CREATE TABLE account (
    id          TEXT PRIMARY KEY,
    -- Reserved slots for future auth columns (no real auth logic in this change).
    username    TEXT UNIQUE,            -- nullable: OIDC-only accounts may have none
    email       TEXT UNIQUE,
    password_hash TEXT,                 -- nullable: OIDC-only accounts may have none
    auth_method TEXT NOT NULL DEFAULT 'NONE'
        CHECK (auth_method IN ('NONE','LOCAL','OIDC')),
    -- Optional reference to a Person; 0..1 per Person, N per Account (multi-household).
    person_id   TEXT UNIQUE REFERENCES person(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_login_at TIMESTAMPTZ
);
```

**`person` table** — add one column (additive, no existing data touched):

```sql
ALTER TABLE person
    ADD COLUMN account_id TEXT REFERENCES account(id) ON DELETE SET NULL;
```

Shape: **optional 1:1** — `person.account_id` is nullable, `account.person_id` is unique.
Either side may be null (a Person with no Account; an Account that has not yet been linked).
`ON DELETE SET NULL` on both sides ensures deleting one side never cascades to the other —
a Person's history (preferences, reactions, restrictions) survives even if their Account is
deleted, and an Account survives even if its linked Person is removed.

### Deferred: multi-household Account support

The schema above *technically* permits an Account to be linked to at most one Person
(`account.person_id` is unique), and a Person to be linked to at most one Account
(`person.account_id` is a single nullable column). A Person could belong to multiple
households via separate `HouseholdMembership` rows (task 1.3), but an Account cannot
currently represent "this login is shared across households."

**Deferral decision:** multi-household Account support is explicitly deferred. The reasons:

1. **No observed requirement yet.** The current household is a single-family setup;
   no stakeholder has asked for one login to span multiple households.
2. **Schema cost is low but non-zero.** Supporting it would require either removing
   the `UNIQUE` on `account.person_id` (allowing a Person to be linked to multiple
   Accounts) or introducing a separate `account_person` join table. Both are
   reversible, but the first weakens the FK guarantee and the second adds a table
   that would sit empty until the feature is needed.
3. **The `ON DELETE SET NULL` invariant holds either way.** Deleting an Account that
   spans multiple Households would null out `person_id` on that Account — the Person
   and their history remain intact regardless.

When multi-household support is eventually needed, the migration is a single
`ALTER TABLE` (drop the unique constraint, or add a join table). No data migration
is required because the existing `person_id` rows already satisfy the weaker
constraint. This deferral is explicitly documented so it is not forgotten when
task 1.2's follow-up arrives.

---

## Step 5½ — PersonRestriction design

### Exact schema shape

```sql
CREATE TABLE person_restriction (
    person_id     TEXT NOT NULL REFERENCES person(id) ON DELETE CASCADE,
    tag           TEXT NOT NULL,
    kind          TEXT NOT NULL CHECK (kind IN ('ALLERGY', 'HARD_RESTRICTION')),
    note          TEXT,                          -- optional free-text context (e.g. "reacts with hives")
    recorded_by   TEXT REFERENCES account(id),   -- who recorded it; NULL = system/automated
    recorded_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    cleared_at    TIMESTAMPTZ,                   -- NULL = active; set when cleared
    cleared_by    TEXT REFERENCES account(id),
    PRIMARY KEY (person_id, tag, kind)
);
CREATE INDEX ON person_restriction (person_id, cleared_at);
```

### Field decisions

| Field | Decision | Rationale |
|---|---|---|
| `person_id` | Required FK → `person(id) ON DELETE CASCADE`. | A restriction always belongs to a person; deleting the person deletes their restrictions. |
| `tag` | Required TEXT. Free-form, same vocabulary as `person_preference.tag`. | Preferences and restrictions share the same tag namespace (e.g. "peanuts" can be both a strong dislike and an allergy). No FK to `ingredient` — a tag may name a non-ingredient concept ("gluten", "pork", "nuts"). |
| `kind` | Required CHECK constraint: `ALLERGY` or `HARD_RESTRICTION`. | Two distinct kinds so the planner can treat them differently (ALLERGY = hard block, HARD_RESTRICTION = household policy that may have exceptions). |
| `note` | Optional TEXT. | Human-readable context for the restriction (e.g. "anaphylactic", "vegetarian for ethical reasons"). Not machine-parsed. |
| `recorded_by` | Nullable FK → `account(id)`. Records who created the restriction. | Mirrors the attribution principle from §1.2. A NULL means the system created it (e.g. imported from an external source), but the import path must still be an explicit command, not inferred. |
| `recorded_at` | Required, defaults to `now()`. Immutable after insert. | Audit anchor. |
| `cleared_at` | Nullable. Set when the restriction is cleared; never updated after set. | A NULL means "active." Clearing is a soft delete — the row stays for audit, similar to `household_membership.ended_at`. |
| `cleared_by` | Nullable FK → `account(id)`. Records who cleared it. | Same attribution principle as `recorded_by`. |
| `PRIMARY KEY (person_id, tag, kind)` | One active restriction per (person, tag, kind) triple. | A person can have at most one ALLERGY to "peanuts" and one HARD_RESTRICTION to "peanuts" simultaneously. To change a restriction, update the existing row rather than insert a new one. |

### Lifecycle commands

```
SetRestriction(personId, tag, kind, note?, recordedBy)
    → INSERT OR UPDATE person_restriction SET
        kind, note, recorded_by, recorded_at = now(),
        cleared_at = NULL, cleared_by = NULL
    (upsert: clears any prior cleared state on re-insert)

ClearRestriction(personId, tag, kind, recordedBy)
    → UPDATE person_restriction SET
        cleared_at = now(), cleared_by = recordedBy
    WHERE person_id = $1 AND tag = $2 AND kind = $3
    (only affects active rows; no-op if already cleared)
```

### Hard invariant: restrictions are never scored

**Structural enforcement (already true):** a grep across the entire codebase for
`person_preference`/`preference_observation`/`PreferenceObservation` outside
`persistence/people.go` returns **zero hits**. The scoring layer (`internal/scoring`)
consumes only `domain.Preference` (from `domain.go`), never raw SQL on either table.
A new `person_restriction` table cannot leak into the scoring path unless code is
added that explicitly reads it — the current architecture provides no path.

**Schema-level enforcement:** no trigger, no computed column, no FK back-reference from
`person_preference` to `person_restriction`. The two tables are completely independent.
The planner's `PlanContext` will carry a separate `Restrictions []PersonRestriction`
field (to be added when the planner is wired to Household/Person from this change)
but will never read from `person_preference` for restrictions.

**Test enforcement (task 9.3):** a unit test will assert that a `PersonRestriction`
row, when present alongside a `PersonPreference` row for the same tag, does not
influence the `Preference.Confidence` or `Preference.Sentiment` values returned by
the persistence layer.

---

## Step 5½½ — IngredientForm design

### Schema shape

```sql
CREATE TABLE ingredient_form (
    ingredient_id  TEXT NOT NULL REFERENCES ingredient(id) ON DELETE CASCADE,
    form           TEXT NOT NULL,              -- 'fresh'|'dried'|'canned'|'frozen'|...
    notes          TEXT,                       -- free-text context, not machine-parsed
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (ingredient_id, form)
);
```

### Task 4.1 — Approach decision

Picked: **`ingredient_form` table** (not a related-ingredient graph). Justification against
the four examples PLAN.md raises:

| Example | Why graph fails | Why table works |
|---|---|---|
| fresh basil → dried basil | Same ingredient, different preparation state — not a "different thing" | `ingredient_form(ingredient_id='basil', form='fresh')` + `(ingredient_id='basil', form='dried')` |
| fresh tomato → canned tomato → crushed tomato | Three forms of one ingredient; graph would need 3 nodes and 6 edges for a fully-connected relation | Three rows on the same `ingredient_id`; the `form` column is the discriminator |
| fresh pasta → dried pasta | Different forms, same base ingredient | Same pattern — two rows |
| fresh vegetable → frozen vegetable | General case: any ingredient can have multiple forms | Table handles arbitrary N forms per ingredient |

A graph approach would model each form as a separate `ingredient` row (e.g. `basil_fresh`, `basil_dried`), which **pollutes the canonical vocabulary** — "basil" would no longer be the root concept, and `recipe_ingredient`/`shopping_requirement` would need to reference the form-specific id, breaking the canonical model. The table approach keeps `ingredient` purely semantic and attaches form as a property.

### Task 4.2 — Prior art

- **Mealie**: `foods` table has a `food_type` column but **no form concept at all** — no prior art to copy. Mealie stores "1 cup flour" as a free-text ingredient line; form is implicit in the human's reading.
- **Grocy**: `products` have a `product_group_id` and `unit_id` but no form field. `product_to_product_group` is a many-to-many that Grocy uses for "this product belongs to the 'pasta' group" — not a form model.
- **Open Food Facts**: `categories_tags` and `categories` are flat tags, not a structured form model. `product_form` exists as a single tag field (e.g. "fresh", "frozen") but has no FK to a canonical ingredient — it's product-level, not ingredient-level.

**Conclusion**: no mature prior art to copy. The `ingredient_form` table is a lightweight,
explicit model designed for Spisordning's needs.

### Task 4.3 — `default_form` placement

**Decision: `default_form` stays on `ingredient_mapping` (as it does today), NOT on `ingredient`.**

Rationale:
- `ingredient` is canonical and household-independent. "tomato" has no universal default form — the household's cooking habits determine whether fresh, canned, or crushed is preferred.
- `ingredient_mapping` is the Mealie-specific bridge (`mealie_food_id → ingredient_id`). When Mealie syncs "1 can crushed tomatoes", the mapping records `default_form = 'canned'` for that specific Mealie food item. This is per-source-of-truth knowledge, not a universal property of the ingredient.
- `shopping_requirement.preferred_form` and `acceptable_forms` (already on the table) are the household-level form preferences, applied at planning time. These are separate from the mapping-level default.
- Putting `default_form` on `ingredient` would force a single form onto every household, which contradicts the design principle that ingredient is canonical and household data lives elsewhere.

---

## Step 5½½½ — IngredientSubstitution design

### Schema shape

```sql
CREATE TABLE ingredient_substitution (
    id                   BIGSERIAL PRIMARY KEY,
    from_ingredient_id   TEXT NOT NULL REFERENCES ingredient(id),
    from_form            TEXT,                       -- NULL = any form of from_ingredient
    to_ingredient_id     TEXT NOT NULL REFERENCES ingredient(id),
    to_form              TEXT,                       -- NULL = any form of to_ingredient
    category             TEXT NOT NULL CHECK (category IN
                ('EQUIVALENT','GOOD','ACCEPTABLE','FORM','DIETARY','EMERGENCY')),
    ratio                DOUBLE PRECISION NOT NULL DEFAULT 1.0,   -- to-qty per from-qty
    retired_at           TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (from_ingredient_id, from_form, to_ingredient_id, to_form, category)
);
CREATE INDEX ON ingredient_substitution (from_ingredient_id, from_form);
CREATE INDEX ON ingredient_substitution (to_ingredient_id, to_form);
```

### Task 5.1 — Directional, categorized substitution

Already designed in design.md Step 3 and Step 5 commands. The schema above makes it concrete:
- `from_ingredient_id` + `from_form?` → `to_ingredient_id` + `to_form?`
- `category` CHECK enforces the six PLAN.md categories
- `retired_at` nullable — a retired substitution is still queryable (for explaining past
  recommendations) but not offered as a live option

### Task 5.2 — Quantity conversion semantics

**Decision: flat multiplier (`ratio` DOUBLE PRECISION).**

Rationale:
- Most real substitutions are close to a flat ratio (dried herbs: ~1/3 fresh; milk: 1:1 across forms).
- Ranges would require storing `ratio_min`/`ratio_max`, adding complexity for marginal gain — most household cooking doesn't need that precision.
- A `note` field (free-text) on the row can capture irregular cases ("use about half, to taste") without encoding them in the schema.
- The ratio is `to_qty / from_qty`: a ratio of 0.33 for dried basil → fresh basil means
  "use 0.33× the amount of dried to replace fresh" (or equivalently, "3× fresh = 1× dried").

### Task 5.3 — Form-specific substitution

**Decision: substitution CAN target a specific `IngredientForm` via nullable `from_form`/`to_form`.**

Examples:
- FORM substitution: `from_ingredient=basil, from_form='fresh', to_ingredient=basil, to_form='dried', category='FORM', ratio=0.33`
- DIETARY substitution: `from_ingredient=chicken, from_form=NULL, to_ingredient=tofu, to_form=NULL, category='DIETARY', ratio=1.0`
- A NULL form means "any form of this ingredient" — the substitution applies regardless of what form the source recipe calls for.

The `UNIQUE (from_ingredient_id, from_form, to_ingredient_id, to_form, category)` constraint
allows multiple form-specific substitutions for the same ingredient pair (e.g. fresh→dried
with ratio 0.33, and fresh→canned with ratio 1.0, as separate rows).

---

## Step 5½½½½ — Unit system design

### Schema shape

```sql
CREATE TABLE unit (
    code       TEXT PRIMARY KEY,   -- 'g','kg','ml','dl','l','piece','tbsp','tsp','pinch','package','can'
    name       TEXT NOT NULL,      -- 'gram','kilogram','milliliter',...
    dimension  TEXT NOT NULL CHECK (dimension IN ('mass','volume','count'))
);

CREATE TABLE unit_conversion (
    from_unit  TEXT NOT NULL REFERENCES unit(code),
    to_unit    TEXT NOT NULL REFERENCES unit(code),
    factor     DOUBLE PRECISION NOT NULL,   -- from * factor = to
    PRIMARY KEY (from_unit, to_unit)
    -- CHECK: from_unit and to_unit must have the same dimension (enforced in app layer;
    -- a DB CHECK would require a function, which is overkill for this stable enum)
);
CREATE INDEX ON unit_conversion (to_unit);

CREATE TABLE ingredient_unit_conversion (
    ingredient_id  TEXT NOT NULL REFERENCES ingredient(id),
    from_unit      TEXT NOT NULL REFERENCES unit(code),
    to_unit        TEXT NOT NULL REFERENCES unit(code),
    factor         DOUBLE PRECISION NOT NULL,
    PRIMARY KEY (ingredient_id, from_unit, to_unit)
);
```

### Task 6.2 — Universal unit set

| code | name | dimension |
|---|---|---|
| `g` | gram | mass |
| `kg` | kilogram | mass |
| `ml` | milliliter | volume |
| `dl` | deciliter | volume |
| `l` | liter | volume |
| `piece` | piece | count |
| `tbsp` | tablespoon | volume |
| `tsp` | teaspoon | volume |
| `pinch` | pinch | volume |
| `package` | package | count |
| `can` | can | count |

Seed in migration (immutable reference data, not app-created).

### Task 6.3 — Universal vs. ingredient-specific conversions

**Decision: two separate tables, never a single table with a nullable `ingredient_id`.**

- `unit_conversion`: universal (same-dimension only: kg↔g, l↔dl↔ml, tbsp↔tsp↔pinch). One row per (from, to) pair. Applies to any ingredient.
- `ingredient_unit_conversion`: cross-dimension (volume↔mass) only, scoped to a specific ingredient. No universal density — `ingredient_unit_conversion` must have a row for "dl flour → g" to exist; if it doesn't, the conversion is undefined and surfaced to the user.

This directly implements invariant 9 and invariant 11. A single table with a nullable
`ingredient_id` would allow a row with `ingredient_id = NULL` to act as a universal
cross-dimension conversion (a global density), which is exactly what the invariant forbids.
Two tables make the distinction structural.

### Task 6.4 — Migration path for free-text `unit` columns

**Decision: no migration in this change.** The `unit TEXT` columns on `recipe_ingredient`
and `shopping_requirement` remain as-is. Wiring them to the `unit` table is scoped to
`implement-recipe-family-and-revisions` (or whichever change next touches recipes), as
the task states. This change only adds the `unit` reference table; it does not alter
existing columns. The free-text values that don't match a `unit.code` will continue to
work (the FK is on the new table, not on the existing columns) — a follow-up change can
backfill or validate them.

---

## Step 6 — Invariants

1. **Login identity is never conflated with household Person.** `Account` and `Person` are
   separate tables with an optional FK; a Person may exist with no Account (a child); deleting
   an Account SHALL NOT delete a Person or their history. **Validated by
   `establish-reference-lab`'s Mealie findings**: `docs/research/mealie-planning-and-search.md`
   confirms Mealie's `users` table conflates login credential and food-domain person with no
   separation — Mealie literally cannot model a child household member without giving them a
   login account. This is not a hypothetical risk this invariant guards against; it's an
   observed limitation in the reference system this project is explicitly designing past.
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
11. **A `UnitConversion` (universal or ingredient-specific) is never silently auto-created.**
    Every conversion row is written only via an explicit `DefineUnitConversion` /
    `DefineIngredientUnitConversion` command; no write path (trigger, default-fill, or
    application code reacting to "purchase unit differs from stock unit") is allowed to insert
    a placeholder/default conversion on the side. **Added directly because of a reproduced
    Grocy bug**: `docs/research/grocy-units-and-planning.md` found that creating a Grocy
    product whose purchase unit differs from its stock unit silently auto-inserts a wrong 1:1
    conversion via a trigger stub, which then *collides* if a correct factor is set
    afterward — a genuine inventory-accuracy hazard, reproduced live during that research
    session, in software with years of production use. `DefineUnitConversion`/
    `DefineIngredientUnitConversion` (Step 5) SHALL be the only path that ever inserts a
    `unit_conversion`/`ingredient_unit_conversion` row; `RegisterProduct` (Step 5) SHALL NOT
    have any side effect on conversion tables even when the product's purchase and stock units
    differ — the absence of a conversion is a valid, queryable state (surfaced to the user as
    "no conversion defined yet"), not something to paper over automatically.

## Step 7 — Database Review Questions (per PLAN.md §Database Review Questions)

### account

| Question | Answer |
|---|---|
| Domain concept | Login identity; separates authentication from the food-domain Person. |
| Owner | The future auth change; this change creates the table as a reservation. |
| Mutator | Future auth change; no code in this change writes to it. |
| Mutable | Yes (username, email, password_hash, auth_method, last_login_at). |
| History required | No — auth history (login events) would be a separate audit table; not in scope. |
| Lifecycle | Created when a user signs up; survives deletion of its linked Person (ON DELETE SET NULL). |
| Deletion behavior | Soft for Person relation (SET NULL); hard for the Account row itself (no cascade). |
| Uniqueness | `id` PK; `username` UNIQUE (nullable); `email` UNIQUE (nullable); `person_id` UNIQUE (0..1 Person per Account, deferring multi-household). |
| External IDs | None in this change — auth is a future concern. |
| Indexing | PK on `id`; UNIQUE on `username`, `email`, `person_id`. |
| FK-ability | `person_id` → `person(id)` ON DELETE SET NULL. |
| JSON? | No. |

### household

Already created by migration 0008. Re-reviewed here for completeness.

| Question | Answer |
|---|---|
| Domain concept | The unit that plans, cooks, and shops together. |
| Owner | Household members (via the application). |
| Mutator | Application — `CreateHousehold`, rename operations. |
| Mutable | Yes (name); soft states (`active`/`archived`) deferred. |
| History required | No — membership history is on `household_membership`, not on `household` itself. |
| Lifecycle | Created; never hard-deleted while it has history (memberships, inventory, meal history). |
| Deletion behavior | Soft — an archived household retains its members for audit. No hard delete path in this change. |
| Uniqueness | `id` PK. |
| External IDs | None. |
| Indexing | PK on `id`. |
| FK-ability | Referenced by `household_membership.household_id`, `inventory_location.household_id`. |
| JSON? | No. |

### household_membership

| Question | Answer |
|---|---|
| Domain concept | A Person's membership in a Household, with temporal boundaries. |
| Owner | The household (administrative action by a member with permission). |
| Mutator | Application — join/leave operations. |
| Mutable | Append + close: a row is created on join and gets `ended_at` on leaving; never updated. |
| History required | Yes — "who was in the household when this meal was rated" requires active membership at that point in time. |
| Lifecycle | Row created on join; `ended_at` set when leaving; row retained for history. |
| Deletion behavior | Never deleted. A Person deleted CASCADEs their membership row (preserving the fact that they were once a member — the row stays with the Person's deleted ID, queryable for audit). |
| Uniqueness | PK on `(household_id, person_id)` — one active membership per person per household at a time. |
| External IDs | None. |
| Indexing | PK; index on `(person_id, ended_at)` for "was this person a member on date X" queries. |
| FK-ability | `household_id` → `household(id)` ON DELETE CASCADE; `person_id` → `person(id)` ON DELETE CASCADE; `ended_by` → `account(id)` nullable. |
| JSON? | No. |

### person_restriction

| Question | Answer |
|---|---|
| Domain concept | A safety-critical allergy or hard restriction a Person holds against an ingredient/tag. |
| Owner | The Person (recorded/cleared by an Account, or by the system via explicit command). |
| Mutator | Application — `SetRestriction`, `ClearRestriction`. |
| Mutable | Yes (note can be updated; `cleared_at`/`cleared_by` set on clear). |
| History required | Yes — a restriction must remain queryable after clearing (audit: "what was this person allergic to, and when was it cleared?"). |
| Lifecycle | Row created on set; `cleared_at` set on clear; row retained. |
| Deletion behavior | Cascade on Person delete (a deleted Person's restrictions go with them — they are personal medical data). Never hard-deleted while active. |
| Uniqueness | PK on `(person_id, tag, kind)` — one active restriction per (person, tag, kind). |
| External IDs | None. |
| Indexing | PK; index on `(person_id, cleared_at)` for active-restriction queries. |
| FK-ability | `person_id` → `person(id)` ON DELETE CASCADE; `recorded_by`/`cleared_by` → `account(id)` nullable. |
| JSON? | No. |

### ingredient (ALTER TABLE: add `merged_into_id`)

| Question | Answer |
|---|---|
| Domain concept | The canonical semantic foodstuff ("chicken breast"), independent of brand or packaging. |
| Owner | Catalog curators (application). |
| Mutator | Application — `DefineIngredient`, `MergeIngredients`. |
| Mutable | Display text may be corrected; canonical identity is effectively immutable once referenced (merges are explicit, not renames). |
| History required | Yes — a merged ingredient must remain queryable (audit: "what was this recipe's original ingredient id?"). The `merged_into_id` self-FK preserves this. |
| Lifecycle | Created; may be merged into another (gets `merged_into_id` set); never deleted while referenced. |
| Deletion behavior | Never hard-deleted. Merges use `merged_into_id` instead of delete, so FKs from `recipe_ingredient`, `product_ingredient_mapping`, `ingredient_form`, `ingredient_substitution` survive. |
| Uniqueness | `id` PK. |
| External IDs | None — canonical internal identity. |
| Indexing | PK on `id`; index on `merged_into_id` for "find all duplicates of X" queries. |
| FK-ability | `merged_into_id` → `ingredient(id)` nullable self-FK. |
| JSON? | No. |

### ingredient_form

| Question | Answer |
|---|---|
| Domain concept | A preparation/preservation state (fresh, dried, canned, frozen) of an Ingredient. |
| Owner | Catalog curators (reference data, curated). |
| Mutator | Application — `AddIngredientForm`. |
| Mutable | Yes (notes can be added/updated); the (ingredient_id, form) pair is stable. |
| History required | No — forms are reference data, not transactional. |
| Lifecycle | Created; retired by removal (no `retired_at` — a form is either registered or not). |
| Deletion behavior | Cascade on Ingredient delete (a deleted ingredient's forms go with it). |
| Uniqueness | PK on `(ingredient_id, form)` — one row per (ingredient, form) pair. |
| External IDs | None. |
| Indexing | PK; no additional indexes needed (lookups are by `ingredient_id` via PK scan). |
| FK-ability | `ingredient_id` → `ingredient(id)` ON DELETE CASCADE. |
| JSON? | No. |

### ingredient_substitution

| Question | Answer |
|---|---|
| Domain concept | A directed, categorized relationship from one Ingredient(+Form) to another, with a non-implied 1:1 quantity ratio. |
| Owner | Catalog curators (curated knowledge). |
| Mutator | Application — `DefineSubstitution`, `RetireSubstitution`. |
| Mutable | Yes (ratio can be corrected); retired via `retired_at` (soft retire, row retained). |
| History required | Yes — retired substitutions must remain queryable (explain past recommendations). |
| Lifecycle | Created; may be retired (`retired_at` set); row retained. |
| Deletion behavior | Never hard-deleted. Retired rows stay for auditability. |
| Uniqueness | `id` PK; UNIQUE on `(from_ingredient_id, from_form, to_ingredient_id, to_form, category)` — no duplicate substitution edges. |
| External IDs | None. |
| Indexing | PK; index on `(from_ingredient_id, from_form)` for "what can replace X?" queries; index on `(to_ingredient_id, to_form)` for "what can X be replaced by?" queries. |
| FK-ability | `from_ingredient_id` → `ingredient(id)`; `to_ingredient_id` → `ingredient(id)`; no ON DELETE — a deleted ingredient's substitutions are orphaned (acceptable: the ingredient is effectively dead if deleted, and its substitutions are no longer relevant). |
| JSON? | No. |

### unit

| Question | Answer |
|---|---|
| Domain concept | A universal, dimensioned measure (mass/volume/count). |
| Owner | Seed data — shipped in migration, not created by the application. |
| Mutator | Migration only; no application write path. |
| Mutable | No — the 11 units are a fixed enumeration. |
| History required | No. |
| Lifecycle | Created in migration; never changes. |
| Deletion behavior | Never deleted. |
| Uniqueness | `code` PK. |
| External IDs | None — internal canonical codes. |
| Indexing | PK on `code`. |
| FK-ability | Referenced by `unit_conversion.from_unit`/`to_unit`, `ingredient_unit_conversion.from_unit`/`to_unit`. |
| JSON? | No. |

### unit_conversion

| Question | Answer |
|---|---|
| Domain concept | A universal conversion factor between two Units of the same dimension (e.g. kg↔g, l↔dl↔ml). |
| Owner | Catalog curators (curated reference data). |
| Mutator | Application — `DefineUnitConversion`. |
| Mutable | Yes (a factor can be corrected). |
| History required | No — conversion factors are stable reference data. |
| Lifecycle | Created; may be updated; never deleted (a removed conversion is a data-loss event — the app should surface "no conversion defined" rather than delete). |
| Deletion behavior | Never hard-deleted in practice. |
| Uniqueness | PK on `(from_unit, to_unit)` — one conversion factor per direction pair. |
| External IDs | None. |
| Indexing | PK; index on `to_unit` for "what converts to X?" queries. |
| FK-ability | `from_unit` → `unit(code)`; `to_unit` → `unit(code)`. |
| JSON? | No. |

### ingredient_unit_conversion

| Question | Answer |
|---|---|
| Domain concept | An ingredient-specific cross-dimension conversion (e.g. dl flour → g). No universal density — this table exists precisely to avoid inventing one. |
| Owner | Catalog curators (curated, per-ingredient). |
| Mutator | Application — `DefineIngredientUnitConversion`. |
| Mutable | Yes (factor can be corrected). |
| History required | No. |
| Lifecycle | Created; may be updated. |
| Deletion behavior | Never hard-deleted. |
| Uniqueness | PK on `(ingredient_id, from_unit, to_unit)`. |
| External IDs | None. |
| Indexing | PK; no additional indexes needed (lookups are by `ingredient_id` via PK scan). |
| FK-ability | `ingredient_id` → `ingredient(id)`; `from_unit` → `unit(code)`; `to_unit` → `unit(code)`. |
| JSON? | No. |

---

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
