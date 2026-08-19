# Tasks: implement-pantry-inventory

## 1. Upstream dependencies

- [x] 1.1 Consume `establish-reference-lab`'s Grocy investigation findings (products, barcodes,
      locations, stock, stock journal, lots, expiry, purchase, consume, discard, transfer,
      adjust, mark empty, unit conversion) as the primary reference for event semantics before
      finalizing this change's migration — done 2026-08-16, see `docs/research/
      grocy-inventory-and-stock.md` / `grocy-units-and-planning.md` / `grocy-api-and-database.md`
      and `design.md` D2/D3/D7
- [x] 1.2 Document explicitly where this change's event semantics diverge from Grocy's, and why
      — do not mechanically port Grocy's model (`PLAN.md`'s First Principle). Done in `design.md`
      D7: `MARK_EMPTY` collapses into `CONSUME` (Grocy has no distinct kind — this design
      diverges from the literal `PLAN.md` list, not from Grocy); `DISCARD` stays distinct
      (Grocy's `spoiled`-boolean-on-`CONSUME` is a documented weakness, deliberately not
      copied); undo is a compensating event, never a mutation of history (Grocy's own "undo"
      mutates `stock_log` in place — explicitly rejected)
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
- [x] 3.2 Decide location hierarchy vs. flat list (e.g. is "garage freezer" distinct from
      "freezer," and is nesting needed, or is a flat named-location list sufficient for v1) —
      resolved 2026-08-19 in `design.md` D9: typed (`location_type`, extensible enum, a hint not
      identity) plus optional self-referential `parent_location_id` nesting to arbitrary depth.
- [ ] 3.3 Implement `location_type` as an extensible, non-authoritative enum
      (`CUPBOARD`/`DRAWER`/`FRIDGE`/`FREEZER`/`BASEMENT`/`BALCONY`/`BREADBOX`/`OTHER`) — see
      `design.md` D9 and invariant 8
- [ ] 3.4 Implement `parent_location_id` (nullable, self-referential) plus an application-layer
      cycle check on write (a location cannot become its own ancestor), mirroring
      `LineageGraph.WouldCreateCycle()`'s approach in `implement-recipe-family` — `design.md` D9
- [ ] 3.5 Recursive "everything under this location" query (e.g. all lots under "basement"
      including nested "chest freezer") — `design.md` D9

## 4. Inventory lots

- [ ] 4.1 A lot represents physical household inventory — not a `products.current_quantity`
      field
- [ ] 4.2 Define lot lifecycle fields: ingredient reference (required), product reference
      (optional, `design.md` D8), location, quantity, unit, best-before/expiry, opened/sealed
      state, confidence
- [ ] 4.3 Implement lot state as fully derived from / maintained alongside `inventory_event`
      writes (never mutated independently) per `design.md` D2/invariant 3
- [ ] 4.4 Implement `RecordPurchase` with `productId` optional except when `source` is
      `shopping_order`, where it is required (`design.md` D8, invariant 7) — enforce at the
      command boundary, not by convention
- [ ] 4.5 Implement `RefineLotProduct(lotId, productId, source)`: attaches a specific `Product`
      to an existing ingredient-only lot without changing quantity, location, or confidence
      (`design.md` D8)
- [ ] 4.6 Implement `ListCandidateProductsForIngredient(ingredientId, query?)`:
      `ProductIngredientMapping` matches first, name-match fallback against
      `Ingredient.display` — powers the `RefineLotProduct` picker, never an unscoped catalog
      search (`design.md` D8)
- [ ] 4.7 Add `home_prepared` to the `source` vocabulary; confirm `RecordPurchase` with this
      source creates an ingredient-only lot with no `productId` (`design.md` D8, Step 5)

## 5. Inventory events

- [ ] 5.1 Model the revised six-kind event vocabulary (`design.md` D7): `PURCHASE`, `CONSUME`,
      `DISCARD`, `ADJUST`, `TRANSFER`, `OPEN` — `MARK_EMPTY` is a command that writes a
      `CONSUME` event with `source: 'mark_empty'`, not its own `kind`
- [x] 5.2 Use Grocy's stock journal behavior (via `establish-reference-lab`) as the primary
      reference for each event's semantics and side effects — done, see `docs/research/
      grocy-inventory-and-stock.md`
- [ ] 5.3 Define per-event-kind required fields and concrete typed FK references (lot, product,
      from/to location) — no generic `entity_type`/`entity_id`/`value` table (`design.md` D5,
      `PLAN.md`'s "Do Not Use Generic Polymorphism Carelessly")
- [ ] 5.4 Define `PURCHASE`'s lot-creation semantics (an event with no pre-existing lot)
- [ ] 5.5 Define `TRANSFER`'s partial-quantity and cross-location semantics
- [x] 5.6 Define `MARK_EMPTY`'s closing semantics — resolved in `design.md` D7: it's a `CONSUME`
      event for the lot's full remaining quantity (retains the lot's history like any other
      `CONSUME`; does not delete the lot row)
- [ ] 5.7 Reserve the target shape for a future `implement-shopping-and-commerce` order
      completion to create a `PURCHASE` event, without implementing that write path here
- [ ] 5.8 Implement undo as a compensating event referencing the event it reverses (`reason` or
      a future `corrects_event_id` column, `source` carrying an `'undo'`-flavored value) —
      never an `UPDATE`/`DELETE` on the original `inventory_event` row (`design.md` D7,
      explicit divergence from Grocy's mutate-history undo behavior)

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
      — `PLAN.md`: "Do not preserve reference-system bugs merely because they exist"):
      FIFO lot selection on partial `CONSUME` when multiple lots of the same product exist
      (Grocy's actual default consumption order, worth matching); a `TRANSFER` that moves only
      part of a lot's quantity leaves the source lot's remaining quantity correct and the
      destination as a distinct lot, not a merge. Explicitly test AGAINST (assert Spisordning
      does NOT reproduce) Grocy's zero-quantity row deletion and its mutate-history undo —
      these are the bugs, not behavior to preserve
- [ ] 9.6 Unit conversion collision regression test, coordinated with
      `establish-household-and-catalog`: creating a product whose purchase unit differs from
      its stock unit, then explicitly setting a conversion factor, must never silently retain
      or collide with an auto-inserted default — reproduces the exact failure mode found live
      in Grocy (`docs/research/grocy-units-and-planning.md`)
- [ ] 9.4 Barcode normalization unit tests (valid/invalid check digits, GTIN-8/12/13/14
      canonicalization)
- [ ] 9.5 `openspec validate implement-pantry-inventory`
- [ ] 9.7 Graduated-specificity tests: ingredient-only lot creation, `RefineLotProduct` leaves
      quantity/location/confidence untouched, `shopping_order`-sourced `PURCHASE` rejected
      without a `productId` (`design.md` D8)
- [ ] 9.8 Location hierarchy tests: nested location creation, cycle rejection (self- and
      descendant-as-parent), recursive "everything under this location" query correctness
      (`design.md` D9)
- [ ] 9.9 `ListCandidateProductsForIngredient` tests: mapped products returned first, name-match
      fallback when no mapping exists, unrelated products excluded (`design.md` D8)
- [ ] 9.10 `home_prepared` source tests: `RecordPurchase` creates a lot with no `productId`,
      confidence resolves to `EXACT` on a counted quantity (`design.md` D8/D3)
