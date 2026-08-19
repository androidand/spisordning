# Implement pantry inventory

## Why

`docs/research/current-state.md` confirms there is currently **zero** pantry/inventory/
Grocy-adjacent code or schema anywhere in this repo or its siblings — this is genuinely
greenfield, unlike most of Epic D/E which extends `migrations/0001_init.sql`. `PLAN.md`'s
"Pantry" section is explicit that a lot represents physical household inventory and that
`products.current_quantity` (a single mutable field) must not be the complete inventory model;
its "Inventory Uncertainty" and "Inventory Events" sections flag exactly the two hardest design
questions here — where confidence lives, and what the event vocabulary is — as open, not
pre-decided. `PLAN.md` explicitly calls this out as needing careful history/audit design.

This change's Inventory Events scope (`PURCHASE`/`CONSUME`/`DISCARD`/`ADJUST`/`TRANSFER`/
`MARK_EMPTY`/`OPEN`) is directed by `PLAN.md` to use **Grocy's behavior as the primary
reference** ("edge cases accumulated through years of inventory use"). `establish-reference-lab`
is the change that performs that Grocy investigation (stock, stock journal, lots, expiry,
purchase/consume/discard/transfer/adjust/mark-empty, unit conversion) and is not yet complete —
this change **depends on `establish-reference-lab`'s Grocy findings** landing first, or at
minimum being far enough along to ground the event semantics in observed Grocy behavior rather
than assumption. Where that research is not yet available, this change's `design.md` reasons
from `PLAN.md`'s stated vocabulary and the "Database Design Process" directly, and tasks below
flag the specific points that should be revisited once the Grocy findings land.

This change also **depends on `establish-household-and-catalog`** for `Household`, `Person`,
`Ingredient`, `IngredientForm`, `Unit`, and `Product`/`ProductIdentifier` — it consumes that
model rather than re-litigating Ingredient-vs-Product modeling. In particular,
`establish-household-and-catalog`'s persistence sketch already reserves a `product_identifier`
table for "optional barcode"; this change's Barcode scope builds the normalization/lookup
workflow on top of that table rather than defining a second, competing barcode table.

**Update (2026-08-19): two household-requested refinements, incorporated before implementation
starts.** Neither changes this change's core ledger-plus-projection shape (D2) or event
vocabulary (D7); both amend Step 3/7's shape for `InventoryLot` and `InventoryLocation`
specifically:

1. **Graduated item specificity.** The household wants to record "we have mjölk" without being
   forced to pick which specific product it is, and to optionally refine that later — or
   automatically, when the milk came from a completed online order that already knows the exact
   `Product`. `establish-household-and-catalog`'s own spec already establishes that a `Product`
   can exist without a resolved `Ingredient` mapping ("a `Product` without a resolved mapping is
   still valid... no `Ingredient` row is invented or guessed automatically"); this change takes
   the symmetric position for `InventoryLot` — see D8.
2. **Location taxonomy and optional hierarchy.** The household's actual storage is not one flat
   list of same-shaped places — cupboard, drawer, fridge, freezer, basement, balcony, breadbox,
   and arbitrary others, some nested inside others (a chest freezer in the basement, a shelf
   inside the fridge). Task 3.2 already flagged this exact question ("is nesting needed, or is a
   flat named-location list sufficient for v1") as open; it is now answered — see D9.

## What Changes

- **`inventory_location`**: named places physical inventory sits (pantry, fridge, freezer, ...),
  scoped to a `Household` (from `establish-household-and-catalog`), typed by an optional
  `location_type` (cupboard/drawer/fridge/freezer/basement/balcony/breadbox/other — extensible,
  a hint not an identity) and optionally nested under a parent `inventory_location` to arbitrary
  depth (D9).
- **`inventory_lot`**: a distinguishable physical quantity in an `inventory_location`, anchored
  to an `Ingredient` always and a specific `Product` only when known, carrying current quantity,
  confidence, best-before/expiry, and open/sealed state (D8). A lot is the unit of physical
  inventory — never a `products.current_quantity` field.
- **`inventory_event`**: an append-only ledger of `PURCHASE`/`CONSUME`/`DISCARD`/`ADJUST`/
  `TRANSFER`/`MARK_EMPTY`/`OPEN` events, each with concrete typed references (not a generic
  `entity_type`/`entity_id`/`value` table — see `design.md` D5 and PLAN.md's "Do Not Use Generic
  Polymorphism Carelessly"). `inventory_lot`'s current state is a transactionally-maintained
  projection of this ledger, not an independently-mutated row (see `design.md` D2/D4).
- **Inventory confidence**: `EXACT`/`LIKELY`/`ESTIMATED`/`UNKNOWN` tiers, placed per the
  reasoning in `design.md` D3 — stored as a derived field on `inventory_lot` (queryable), with
  each `inventory_event` recording the observation source that justified the confidence set at
  that time (auditable), mirroring the existing `person_preference`/`preference_observation`
  shape already proven in `migrations/0001_init.sql`.
- **Barcode**: GTIN/EAN normalization (check-digit validation, GTIN-8/12/13/14 canonicalization)
  onto `establish-household-and-catalog`'s `product_identifier`; an Open Food Facts lookup
  client (read-only enrichment); a retailer barcode lookup fallback (via the existing
  `internal/retailer` client / willys-adapter); and a manual fallback entry flow. Hard
  invariant: a barcode SHALL NOT define product identity — it is a lookup key onto a `Product`,
  never the identity itself.

## Capabilities

### New Capabilities

- `pantry-inventory`: inventory locations, lots, the inventory event ledger, confidence tiers,
  and barcode-assisted product identification for physical household inventory.

### Modified Capabilities

<!-- none — greenfield; consumes but does not modify household/ingredient-catalog -->

## Impact

- Affected code: new `migrations/` file (additive, extends `0001_init.sql`'s style; no existing
  table renamed or dropped), new `internal/pantry` (or equivalent) domain package.
- Depends on `establish-reference-lab` (Grocy investigation grounding the event vocabulary) and
  `establish-household-and-catalog` (`Household`, `Person`, `Ingredient`, `IngredientForm`,
  `Unit`, `Product`, `ProductIdentifier`) — this change should land after both, or with explicit
  placeholders revisited once they land.
- Feeds `implement-recipe-availability` (this change's `inventory_lot` is that change's primary
  read input) and, later, `implement-recommendations`' pantry-availability/expiry scoring
  inputs.
- `implement-shopping-and-commerce`'s `order`/`order_item` design already reserves an explicit
  extension point for a completed order to create inventory `PURCHASE` events; this change
  defines that write path's target shape (`inventory_event` of kind `PURCHASE`) but does not
  itself modify `implement-shopping-and-commerce`. Per D8, an order-derived `PURCHASE` already
  knows the exact `Product` (the retailer resolution pipeline resolved it), so these events are
  created at full product-level specificity automatically — manual quick entry (ingredient-only)
  is for the physical-shopping/no-order path, not the online one.
- Feeds two sibling changes gated on this one landing first:
  `research-barcode-scanning-devices` (scan-driven `RecordPurchase`/`RecordConsume`/
  `RecordDiscard`/`RecordMarkEmpty` calls) and `research-inventory-label-printing` (label
  content is read from `InventoryLot`, and a scanned label barcode resolves to a specific
  `InventoryLot`, not a `Product` GTIN — a distinct barcode namespace from this change's D6).
- No changes to `internal/scoring`, `internal/planning`, or the existing `meal_plan`/
  `shopping_requirement` tables.
