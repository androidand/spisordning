## Context

`establish-migration-and-postgres-19` puts the schema on Goose + PostgreSQL 19. This change then
re-baselines the **non-recipe** tables to one canonical identity model and one value-type model.
The recipe tables (`recipe_ref`, `recipe_family`, `recipe_variant`, `recipe_revision`,
`recipe_ingredient`, and the recipe references on `meal_event`/`meal_plan_candidate`/
`meal_plan_decision`/`favorite`) are handled in `rebaseline-recipe-domain`; the query layer is
converted to sqlc in `establish-sqlc-persistence`.

Current identity strategies in the schema (the problem this change removes):

- Slug-as-PK: `person`, `ingredient`, `household`, `product`, `inventory_location`,
  `external_recipe_source`.
- `BIGSERIAL` surrogates: `preference_observation`, `meal_event`, `meal_reaction`,
  `planning_constraint`, `meal_plan`, `meal_plan_candidate`, `shopping_requirement`,
  `recipe_import_candidate`, `shopping_list`, `shopping_list_item`, `retailer_list_binding`,
  `shopping_cart`, `shopping_cart_item`, `order`, `order_item`, `inventory_lot`,
  `inventory_event`, `meal_review`, `favorite`, `meal_participant`,
  `recipe_import_candidate_ingredient`.
- Foreign-system IDs as relationship keys: `ingredient_mapping.mealie_food_id` (this change);
  `recipe_ref.mealie_recipe_id` (recipe change).
- IEEE `float` (`DOUBLE PRECISION`) for quantities, money, and scores.

We are pre-release; dev data is disposable. This is a **fresh-bootstrap** re-baseline: the
migration chain is edited in place and must apply cleanly to an empty PostgreSQL 19.

## Goals / Non-Goals

**Goals:**
- One identity rule, applied consistently: UUIDv7 for domain entities, slug for human addresses,
  external references for foreign systems, composite keys for pure relationships, stable codes for
  small named registries.
- One value-type rule: `numeric` for quantities, minor-units + currency for transaction money,
  bounded `numeric` for scores/confidence/weight, zero-padded string for GTIN.
- Strongly-typed Go IDs so a repository call with the wrong ID type does not compile.
- A reviewed, explicit target shape for every affected non-recipe table (the schema diff below).

**Non-Goals:**
- No recipe-table changes (to `rebaseline-recipe-domain`).
- No sqlc adoption or query conversion (to `establish-sqlc-persistence`).
- No new domain features (units, restrictions, forms, substitution) — those remain in
  `establish-household-and-catalog`'s deferred scope.
- No in-place data migration of existing dev rows (disposable; fresh bootstrap only).

## Decisions

### D1 — Identity model (one rule)

| Kind | Rule | Example |
|---|---|---|
| Domain entity | `UUIDv7` PK, generated in Go | `person`, `product`, `meal_event`, `shopping_requirement` |
| Human address | `slug TEXT UNIQUE NOT NULL` alongside the UUID | `person.slug`, `ingredient.slug` |
| Foreign system | external-reference table `(provider, external_id, <entity>_id)`; no polymorphic `entity_type`/`entity_id` | `ingredient_external_ref` |
| Pure relationship | composite PK of the parent keys, no surrogate | `meal_participant(meal_event_id, person_id)` |
| Stable code | small named registry keyed by a human code | `effort_profile(weekday)`, `external_recipe_source` (validated) |

Rationale: UUIDv7 gives time-ordered, collision-free, Go-generated identity for high-volume
entities. Slugs stay on the few entities humans address by name. Foreign systems are never
identity — they are external references. Pure relationships carry no surrogate.

### D2 — Strongly-typed Go IDs

The domain layer defines named types wrapping `uuid.UUID`: `PersonID`, `IngredientID`,
`HouseholdID`, `ProductID`, `InventoryLocationID`, `MealEventID`, `MealPlanID`,
`ShoppingListID`, `ShoppingRequirementID`, `ShoppingCartID`, `OrderID`, `InventoryLotID`, etc.
Repository interfaces and domain services use these types, so `repo.GetPerson(ctx, productID)`
does not compile. The existing pgx persistence code is updated to use them in this change;
`establish-sqlc-persistence` keeps them when it converts to sqlc. Slugged entities expose both
the typed UUID and the slug.

### D3 — Value types

- **Quantities** (`DOUBLE PRECISION`): to `numeric(12,3)` — `shopping_requirement.quantity`,
  `shopping_list_item.quantity`, `shopping_cart_item.quantity`, `order_item.quantity`,
  `inventory_lot.quantity`, `inventory_event.quantity_delta`, `product_ingredient_mapping.quantity`,
  `recipe_import_candidate_ingredient.quantity`.
- **Transaction money** (`DOUBLE PRECISION`): to `amount_minor BIGINT NOT NULL` +
  `currency CHAR(3) NOT NULL DEFAULT 'SEK'` — `order.total_price`, `order_item.total_price`,
  `shopping_cart_item.resolved_price`.
- **Derived unit price**: `order_item.unit_price` to `numeric(12,3)` (a per-unit price that may
  need sub-minor precision; not a transaction amount).
- **Scores / confidence / weight** (reviewed individually):
  - `person.weight` `DOUBLE CHECK > 0` to `numeric(6,3) CHECK (weight > 0)`.
  - `person_preference.confidence` `DOUBLE CHECK 0..1` to `numeric(5,4) CHECK (confidence >= 0 AND confidence <= 1)`.
  - `recipe_import_candidate.rating` `DOUBLE` to `numeric(3,1) CHECK (rating >= 0 AND rating <= 5)`.
  - `meal_plan_candidate.score` `DOUBLE`: computed recommendation score with a JSONB breakdown —
    stays `DOUBLE PRECISION` (transient/computed, not a stored canonical measure).

### D4 — Product identifiers

`product_identifier` is simplified to a lookup key, not an entity:

```
product_identifier (
    scheme        TEXT NOT NULL,
    value         TEXT NOT NULL,
    product_id    UUID NOT NULL REFERENCES product(id) ON DELETE CASCADE,
    provenance    TEXT,
    confidence    numeric(5,4),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (scheme, value)
)
```

- `gtin` becomes `scheme='GTIN'`, `value` = the zero-padded 14-digit string.
- `value` is always `TEXT`; scheme-specific validation (GTIN length/check-digit, Willys/ICA
  article format) happens at the Go boundary, not in one shared DB constraint.
- One `(scheme, value)` maps to exactly one `Product` (enforced by the PK).
- `provenance`/`confidence` record how the mapping was established (scan, import, manual).

### D5 — Ingredient external references

`ingredient_mapping(mealie_food_id, ...)` is restructured into a domain-specific external
reference table (no polymorphic `entity_type`/`entity_id`):

```
ingredient_external_ref (
    provider        TEXT NOT NULL,
    external_id     TEXT NOT NULL,
    ingredient_id   UUID NOT NULL REFERENCES ingredient(id) ON DELETE CASCADE,
    grams_per_unit  numeric(12,3),
    default_form    TEXT,
    needs_review    BOOLEAN NOT NULL DEFAULT false,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, external_id)
)
```

`provider='mealie'` carries the former `mealie_food_id` values. Real FKs are preserved.

## Schema diff (database-review checkpoint)

Current source: `migrations/0001`–`0010` (read in full). Target shapes below. "Id" = canonical
identity; "Ext" = external identity; "M" = mutable columns; "I" = immutable columns.

### A. Slug-PK to UUIDv7 PK + slug

| Table | Current PK | Target PK | Target shape (key columns) |
|---|---|---|---|
| `person` | `id TEXT` | `id UUIDv7` | `id UUID PK`, `slug TEXT NOT NULL UNIQUE`, `name TEXT NOT NULL`, `weight numeric(6,3) CHECK > 0`, `created_at` (I) |
| `ingredient` | `id TEXT` | `id UUIDv7` | `id UUID PK`, `slug TEXT NOT NULL UNIQUE`, `display TEXT NOT NULL` |
| `household` | `id TEXT` | `id UUIDv7` | `id UUID PK`, `slug TEXT NOT NULL UNIQUE`, `name TEXT NOT NULL`, `created_at` (I) |
| `product` | `id TEXT` | `id UUIDv7` | `id UUID PK`, `slug TEXT NOT NULL UNIQUE`, `name`, `brand`, `package_size`, `created_at` (I) |
| `inventory_location` | `id TEXT` | `id UUIDv7` | `id UUID PK`, `slug TEXT NOT NULL UNIQUE`, `household_id UUID NOT NULL FK`, `name`, `location_type CHECK`, `parent_location_id UUID FK`, `archived_at` |

`external_recipe_source`: **validate** — leaning stable-code PK (small named registry: `ica`,
`koket`, `arls`). If kept, `id TEXT` stays the PK (no UUID); `recipe_import_candidate.source_id`
stays `TEXT`. Decision recorded as a task in this change.

### B. BIGSERIAL to UUIDv7 (independent entities)

| Table | Target PK | Key FKs (target types) | Notes |
|---|---|---|---|
| `preference_observation` | `id UUIDv7` | `person_id UUID FK` | append-only evidence; `observed_at` (I) |
| `planning_constraint` | `id UUIDv7` | — | `kind`, `value`, `active` (M) |
| `meal_plan` | `id UUIDv7` | — | `week_start DATE NOT NULL UNIQUE`, `status` (M) |
| `meal_event` | `id UUIDv7` | `meal_plan_id UUID FK`, `meal_plan_slot_date DATE` | recipe ref column moved to canonical in recipe change; composite FK to `meal_plan_decision` retyped |
| `meal_plan_candidate` | `id UUIDv7` | `plan_id UUID FK` | `score DOUBLE` (computed, stays float); recipe ref to canonical in recipe change |
| `shopping_requirement` | `id UUIDv7` | `plan_id UUID FK`, `ingredient_id UUID FK` | confirmed UUID; `UNIQUE (plan_id, ingredient_id)` kept as index; `quantity numeric(12,3)` |
| `recipe_import_candidate` | `id UUIDv7` | `source_id TEXT FK` | `rating numeric(3,1) CHECK 0..5`; `promoted_variant_id` retyped in recipe change |
| `shopping_list` | `id UUIDv7` | `owner_person_id UUID FK` | `status` (M) |
| `shopping_list_item` | `id UUIDv7` | `shopping_list_id UUID FK`, `shopping_requirement_id UUID FK`, `ingredient_id UUID FK` | `quantity numeric(12,3)`; CHECK (req OR ing OR label) kept |
| `shopping_cart` | `id UUIDv7` | `retailer_list_binding_id UUID FK` | `status` (M) |
| `order` | `id UUIDv7` | `shopping_cart_id UUID FK` | `total_price` to `amount_minor BIGINT` + `currency CHAR(3)`; `source` CHECK kept |
| `order_item` | `id UUIDv7` | `order_id UUID FK`, `substituted_for_item_id UUID FK` (self) | `quantity numeric(12,3)`, `unit_price numeric(12,3)`, `total_price` to minor+currency |
| `inventory_lot` | `id UUIDv7` | `ingredient_id UUID FK`, `product_id UUID FK`, `location_id UUID FK` | `quantity numeric(12,3)`; `confidence` text enum kept; `updated_at` (M) |
| `inventory_event` | `id UUIDv7` | `lot_id UUID FK`, `ingredient_id UUID FK`, `product_id UUID FK`, `from/to_location_id UUID FK` | `quantity_delta numeric(12,3)`; append-only ledger; `recorded_at` (I) |

### C. BIGSERIAL + UNIQUE to composite PK (pure relationships)

| Table | Current | Target PK | Rationale |
|---|---|---|---|
| `meal_participant` | `id BIGSERIAL` + `UNIQUE(meal_event_id, person_id)` | `(meal_event_id UUID, person_id UUID)` | attendance; one row per (meal, person); no independent lifecycle |
| `recipe_import_candidate_ingredient` | `id BIGSERIAL` + `UNIQUE(candidate_id, line_no)` | `(candidate_id UUID, line_no INT)` | ordered lines of a candidate; no independent lifecycle |

### D. Borderline tables (re-review; decision recorded per table)

| Table | Current | Leaning target | Open question |
|---|---|---|---|
| `meal_reaction` | `id BIGSERIAL` + `UNIQUE(meal_event_id, person_id)` | composite `(meal_event_id UUID, person_id UUID)` | one reaction per (meal, person); `sentiment SMALLINT`, `note` (M). Confirm no independent identity needed. |
| `meal_review` | `id BIGSERIAL` + `UNIQUE(meal_event_id, person_id)` | composite `(meal_event_id UUID, person_id UUID)` | considered rating, one per (meal, person); `rating SMALLINT 1..5`, `note`, `updated_at` (M). Confirm. |
| `retailer_list_binding` | `id BIGSERIAL` + `UNIQUE(shopping_list_id, retailer)` | composite `(shopping_list_id UUID, retailer TEXT)` | has sync lifecycle (`last_pushed_at`, `last_push_status`, `sync_direction`) but one per (list, retailer). Confirm composite vs UUID. |
| `shopping_cart_item` | `id BIGSERIAL` | composite `(shopping_cart_id UUID, line_no INT)` | snapshot line for reconciliation; no external references today. Confirm vs UUID if remote line IDs appear. |
| `favorite` | `id BIGSERIAL` + `UNIQUE(person_id, recipe)` + `UNIQUE(household_id, recipe)` + CHECK | composite on (scope, recipe) | person-XOR-household scoping. Two candidate shapes: (a) bounded `scope_type`/`scope_id` discriminator + recipe ref, PK `(scope_type, scope_id, recipe_ref)`; (b) split `person_favorite` + `household_favorite`. Recipe ref moves to canonical in recipe change. Decide shape here; wire recipe ref in recipe change. |

### E. Composite PKs (already correct — kept)

| Table | PK | Change |
|---|---|---|
| `person_preference` | `(person_id, tag)` | `person_id` retyped `TEXT` to `UUID`; `confidence` to bounded `numeric` |
| `meal_plan_decision` | `(plan_id, slot_date)` | `plan_id` retyped to `UUID`; recipe ref column to canonical in recipe change |
| `product_ingredient_mapping` | `(product_id, ingredient_id)` | both retyped to `UUID`; `quantity` to `numeric(12,3)` |

### F. Stable code (kept)

| Table | PK | Change |
|---|---|---|
| `effort_profile` | `weekday SMALLINT (0-6)` | none (stable code) |

### G. Foreign-key retypes (consequence of A–E)

All `*_id` FK columns that reference a converted PK are retyped `TEXT`/`BIGINT` to `UUID`:
`person_id`, `ingredient_id`, `household_id`, `product_id`, `inventory_location_id`
(self-ref `parent_location_id`), `shopping_list_id`, `shopping_requirement_id`,
`retailer_list_binding_id`, `shopping_cart_id`, `order_id`, `lot_id`, `meal_event_id`,
`plan_id`, `candidate_id`. Recipe-referencing FKs are retyped in `rebaseline-recipe-domain`.

## Risks / Trade-offs

- **Cross-change FK coupling.** `favorite`, `meal_event`, `meal_plan_candidate`,
  `meal_plan_decision`, and `recipe_import_candidate.promoted_variant_id` reference recipe
  identity. This change retypes their non-recipe FKs and leaves the recipe-ref columns as
  transitional (still `mealie_recipe_id TEXT`) until `rebaseline-recipe-domain` rewrites them.
  The two changes must be applied in order; the intermediate state is valid SQL.
- **`favorite` scoping.** A nullable person-XOR-household composite is awkward. The bounded
  discriminator keeps one table but is a mild, documented (not generic) polymorphism. Splitting
  is cleaner but doubles the surface. Decision is a task, not a silent choice.
- **`numeric` precision.** `numeric(12,3)` is chosen for quantities; if a future unit needs more
  scale it is a localized migration, not a model change.

## Migration Plan

- Edit the in-place chain `000001`–`000010` (already relocated by
  `establish-migration-and-postgres-19`) so the fresh-bootstrap result is the target shape.
- No `ALTER`-based data migration: dev data is disposable. Existing dev volumes are dropped and
  re-bootstrapped.
- The chain must apply cleanly to an empty `postgres:19beta3-alpine` and reach the target shape.

## Open Questions

1. `external_recipe_source`: keep stable-code PK or convert to UUIDv7 + slug? (leaning stable code)
2. `favorite`: bounded discriminator vs split tables?
3. Confirm `meal_reaction`, `meal_review`, `retailer_list_binding`, `shopping_cart_item` as
   composite (no independent identity).
