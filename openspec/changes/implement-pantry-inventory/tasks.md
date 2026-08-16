# Tasks: implement-pantry-inventory

## 1. Upstream dependencies

- [ ] 1.1 Consume `establish-reference-lab`'s Grocy investigation findings (products, barcodes,
      locations, stock, stock journal, lots, expiry, purchase, consume, discard, transfer,
      adjust, mark empty, unit conversion) as the primary reference for event semantics before
      finalizing this change's migration
- [ ] 1.2 Document explicitly where this change's event semantics diverge from Grocy's, and why
      — do not mechanically port Grocy's model (`PLAN.md`'s First Principle)
- [ ] 1.3 Consume `establish-household-and-catalog`'s `Household`, `Person`, `Ingredient`,
      `IngredientForm`, `Unit`, `Product`, and `ProductIdentifier` — do not redefine
      Ingredient-vs-Product modeling here

## 2. Vocabulary & aggregates (Database Design Process Steps 1-2)

- [ ] 2.1 Canonical glossary: `InventoryLocation`, `InventoryLot`, `InventoryEvent`, confidence
      tier, barcode/GTIN
- [ ] 2.2 Confirm aggregate boundary: `InventoryLot` as a mutable, transactionally-maintained
      projection; `InventoryEvent` as the append-only source of truth (see `design.md` D2)

## 3. Inventory locations

- [ ] 3.1 Model `inventory_location` scoped to a `Household`
- [ ] 3.2 Decide location hierarchy vs. flat list (e.g. is "garage freezer" distinct from
      "freezer," and is nesting needed, or is a flat named-location list sufficient for v1)

## 4. Inventory lots

- [ ] 4.1 A lot represents physical household inventory — not a `products.current_quantity`
      field
- [ ] 4.2 Define lot lifecycle fields: product reference, location, quantity, unit, best-before/
      expiry, opened/sealed state, confidence
- [ ] 4.3 Implement lot state as fully derived from / maintained alongside `inventory_event`
      writes (never mutated independently) per `design.md` D2/invariant 3

## 5. Inventory events

- [ ] 5.1 Model the literal event kind vocabulary: `PURCHASE`, `CONSUME`, `DISCARD`, `ADJUST`,
      `TRANSFER`, `MARK_EMPTY`, `OPEN`
- [ ] 5.2 Use Grocy's stock journal behavior (via `establish-reference-lab`) as the primary
      reference for each event's semantics and side effects
- [ ] 5.3 Define per-event-kind required fields and concrete typed FK references (lot, product,
      from/to location) — no generic `entity_type`/`entity_id`/`value` table (`design.md` D5,
      `PLAN.md`'s "Do Not Use Generic Polymorphism Carelessly")
- [ ] 5.4 Define `PURCHASE`'s lot-creation semantics (an event with no pre-existing lot)
- [ ] 5.5 Define `TRANSFER`'s partial-quantity and cross-location semantics
- [ ] 5.6 Define `MARK_EMPTY`'s closing semantics (does it delete/archive the lot, or zero its
      quantity and retain it for history?)
- [ ] 5.7 Reserve the target shape for a future `implement-shopping-and-commerce` order
      completion to create a `PURCHASE` event, without implementing that write path here

## 6. Inventory confidence / uncertainty

- [ ] 6.1 Support `EXACT`/`LIKELY`/`ESTIMATED`/`UNKNOWN` confidence tiers
- [ ] 6.2 Implement the placement decided in `design.md` D3: stored on `inventory_lot`
      (queryable), justified per `inventory_event.source` (auditable) — not solely on one side
- [ ] 6.3 Implement the deterministic `(event kind, source) → confidence` transition table from
      `design.md` D3
- [ ] 6.4 Decide whether time-based confidence decay ships in this change's first slice or is
      deferred; if shipped, implement it as an `ADJUST` event with `source: 'inferred_decay'`,
      never a silent row mutation (`design.md` D3)

## 7. Barcode

- [ ] 7.1 GTIN/EAN normalization: check-digit validation, GTIN-8/12/13/14 canonicalization
- [ ] 7.2 Resolve against `establish-household-and-catalog`'s `ProductIdentifier` table before
      any external lookup
- [ ] 7.3 Open Food Facts lookup integration (read-only enrichment: name, brand, ingredients,
      allergens, nutrition, images, categories); evaluate current API version and licensing
      terms before integrating
- [ ] 7.4 Retailer barcode lookup fallback via the existing `internal/retailer` client /
      willys-adapter
- [ ] 7.5 Manual fallback entry flow when no source resolves the barcode
- [ ] 7.6 Hard invariant: a barcode SHALL NOT define product identity — every downstream
      reference (`InventoryLot`, `InventoryEvent`) uses `product_id`, never a raw GTIN

## 8. Persistence (Database Design Process Step 7)

- [ ] 8.1 New migration extending `migrations/0001_init.sql`'s style (no `products.current_quantity`
      column, real FKs throughout)
- [ ] 8.2 Indexes for the "all `UNKNOWN`-confidence lots" / "all lots in this location" /
      "event history for this lot" query shapes
- [ ] 8.3 Deletion behavior: confirm `InventoryEvent` rows are never deleted; confirm
      `InventoryLot` rows are archived/closed (`MARK_EMPTY`), never hard-deleted while events
      reference them

## 9. Verification & docs

- [ ] 9.1 Domain unit tests for event → lot-state derivation (each event kind, including
      `PURCHASE` lot creation and `TRANSFER` cross-location effects)
- [ ] 9.2 Domain unit tests for the confidence transition table
- [ ] 9.3 Reference-behavior tests for Grocy edge cases worth deliberately preserving (not bugs
      — `PLAN.md`: "Do not preserve reference-system bugs merely because they exist")
- [ ] 9.4 Barcode normalization unit tests (valid/invalid check digits, GTIN-8/12/13/14
      canonicalization)
- [ ] 9.5 `openspec validate implement-pantry-inventory`
