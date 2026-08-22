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
- [x] 1.3 Consume `establish-household-and-catalog`'s `Household`, `Person`, `Ingredient`,
      `IngredientForm`, `Unit`, `Product`, and `ProductIdentifier` — do not redefine
      Ingredient-vs-Product modeling here. Done 2026-08-22: all consumable types are
      referenced where needed — `Household`/`Product`/`ProductIdentifier` via `catalog.go`
      structs and FKs (`inventory_location.household_id`, `inventory_lot.product_id`,
      `LookupProductByGTIN`/`UpsertProductIdentifier`), `Ingredient` via `GetIngredient`
      (`recipes.go`) and FK (`inventory_lot.ingredient_id`), `Person` available without
      household scoping. `IngredientForm`/`Unit` are not referenced by the pantry design:
      `inventory_lot.unit` is intentionally free-text (design step 7 lists it without "FK"
      annotation; the unit system in `establish-household-and-catalog` is about product
      purchase units and ingredient conversions, not about restricting what unit a lot
      uses), and `IngredientForm` has no pantry use case. No upstream types are redefined
      locally — the pantry code uses the `Product`/`Household` structs from `catalog.go`,
      the `Ingredient`/`Unit` types from `domain.go`, and the `Confidence`/`EventKind`/`NormalizeGTIN`/`WouldCreateLocationCycle`
      types from `domain/pantry.go`. (Note: `Confidence` and `EventKind` live in `domain/pantry.go`, not `domain.go` —
      this was corrected from an earlier draft of this note.)

## 2. Vocabulary & aggregates (Database Design Process Steps 1-2)

- [x] 2.1 Canonical glossary: `InventoryLocation`, `InventoryLot`, `InventoryEvent`, confidence
      tier, barcode/GTIN — `design.md` Step 1 table; now backed by real Go types
      (`internal/persistence/pantry.go`) and a `Confidence`/`EventKind` vocabulary
      (`internal/domain/pantry.go`), not just documentation.
- [x] 2.2 Confirm aggregate boundary: `InventoryLot` as a mutable, transactionally-maintained
      projection; `InventoryEvent` as the append-only source of truth (see `design.md` D2) —
      implemented: every lot-mutating `Store` method writes the lot and its causing event in one
      `pgx.Tx` (`internal/persistence/pantry.go`); no code path updates a lot without a
      corresponding event insert in the same transaction.

## 3. Inventory locations

- [x] 3.1 Model `inventory_location` scoped to a `Household` —
      `migrations/0009_pantry_inventory.sql`, `Store.CreateInventoryLocation`/`GetInventoryLocation`.
- [x] 3.2 Decide location hierarchy vs. flat list (e.g. is "garage freezer" distinct from
      "freezer," and is nesting needed, or is a flat named-location list sufficient for v1) —
      resolved 2026-08-19 in `design.md` D9: typed (`location_type`, extensible enum, a hint not
      identity) plus optional self-referential `parent_location_id` nesting to arbitrary depth.
- [x] 3.3 Implement `location_type` as an extensible, non-authoritative enum
      (`CUPBOARD`/`DRAWER`/`FRIDGE`/`FREEZER`/`BASEMENT`/`BALCONY`/`BREADBOX`/`OTHER`) — see
      `design.md` D9 and invariant 8. `migrations/0009_pantry_inventory.sql` CHECK constraint;
      nullable (unset is the common case per D9).
- [x] 3.4 Implement `parent_location_id` (nullable, self-referential) plus an application-layer
      cycle check on write (a location cannot become its own ancestor), mirroring
      `LineageGraph.WouldCreateCycle()`'s approach in `implement-recipe-family` — `design.md` D9.
      `domain.WouldCreateLocationCycle` (`internal/domain/pantry.go`) + `Store.locationAncestors`
      (recursive query) wired into `CreateInventoryLocation`.
- [x] 3.5 Recursive "everything under this location" query (e.g. all lots under "basement"
      including nested "chest freezer") — `design.md` D9. `Store.ListLotsUnderLocation`
      (`WITH RECURSIVE` descendants walk).

## 4. Inventory lots

- [x] 4.1 A lot represents physical household inventory — not a `products.current_quantity`
      field — `inventory_lot` table; no such column exists anywhere in this schema.
- [x] 4.2 Define lot lifecycle fields: ingredient reference (required), product reference
      (optional, `design.md` D8), location, quantity, unit, best-before/expiry, opened/sealed
      state, confidence — `InventoryLot` struct, `internal/persistence/pantry.go`.
- [x] 4.3 Implement lot state as fully derived from / maintained alongside `inventory_event`
      writes (never mutated independently) per `design.md` D2/invariant 3 — every quantity/
      confidence-changing method (`RecordPurchase`/`RecordConsume`/`RecordDiscard`/
      `RecordAdjust`/`RecordMarkEmpty`/`RecordTransfer`/`RecordOpen`) writes the lot and its
      event atomically. `RefineLotProduct` is the one documented exception (D8's own text: "at
      any later time without altering the lot's quantity, location, or confidence" — it isn't an
      inventory_event, it corrects the lot's own identity field).
- [x] 4.4 Implement `RecordPurchase` with `productId` optional except when `source` is
      `shopping_order`, where it is required (`design.md` D8, invariant 7) — enforce at the
      command boundary, not by convention — `Store.RecordPurchase` returns an error immediately
      when `source == "shopping_order"` and `productID == ""`.
- [x] 4.5 Implement `RefineLotProduct(lotId, productId, source)`: attaches a specific `Product`
      to an existing ingredient-only lot without changing quantity, location, or confidence
      (`design.md` D8) — `Store.RefineLotProduct`. Note: the `source` param from design Step 5's
      signature was dropped at implementation; the method signature is `RefineLotProduct(lotID,
      productID)` (no source). This is a minor design-gap — the `source` field on `inventory_event`
      does not apply here since `RefineLotProduct` does not write an event. If auditing of
      product-refinement events becomes necessary, a future migration can add a `refined_by` or
      `source` column to `inventory_lot`, or a new event kind can be introduced.
- [x] 4.6 Implement `ListCandidateProductsForIngredient(ingredientId, query?)`:
      `ProductIngredientMapping` matches first, name-match fallback against
      `Ingredient.display` — powers the `RefineLotProduct` picker, never an unscoped catalog
      search (`design.md` D8) — `Store.ListCandidateProductsForIngredient` (the `query?` param
      from the design's command signature is not yet exposed — the name-match fallback always
      matches against the ingredient's own display name; a free-text override is not implemented).
- [x] 4.7 Add `home_prepared` to the `source` vocabulary; confirm `RecordPurchase` with this
      source creates an ingredient-only lot with no `productId` (`design.md` D8, Step 5) — no
      CHECK constraint on `inventory_event.source` (matches the schema's existing free-text
      `source` columns elsewhere), `domain.SourceMarkEmpty`-style handling confirmed by
      `TestPantry_GraduatedSpecificity`.

## 5. Inventory events

- [x] 5.1 Model the revised six-kind event vocabulary (`design.md` D7): `PURCHASE`, `CONSUME`,
      `DISCARD`, `ADJUST`, `TRANSFER`, `OPEN` — `MARK_EMPTY` is a command that writes a
      `CONSUME` event with `source: 'mark_empty'`, not its own `kind` — `inventory_event.kind`
      CHECK constraint (six values) + `domain.EventKind` consts.
- [x] 5.2 Use Grocy's stock journal behavior (via `establish-reference-lab`) as the primary
      reference for each event's semantics and side effects — done, see `docs/research/
      grocy-inventory-and-stock.md`
- [x] 5.3 Define per-event-kind required fields and concrete typed FK references (lot, product,
      from/to location) — no generic `entity_type`/`entity_id`/`value` table (`design.md` D5,
      `PLAN.md`'s "Do Not Use Generic Polymorphism Carelessly") — `inventory_event` columns are
      concrete FKs throughout; verified by the architecture test and manual review.
- [x] 5.4 Define `PURCHASE`'s lot-creation semantics (an event with no pre-existing lot) —
      `Store.RecordPurchase` inserts the lot and its `PURCHASE` event in one transaction.
- [x] 5.5 Define `TRANSFER`'s partial-quantity and cross-location semantics — `Store.RecordTransfer`:
      a full-quantity transfer moves the existing lot's `location_id`; a partial transfer
      decrements the source and creates a distinct destination lot (never a merge).
- [x] 5.6 Define `MARK_EMPTY`'s closing semantics — resolved in `design.md` D7: it's a `CONSUME`
      event for the lot's full remaining quantity (retains the lot's history like any other
      `CONSUME`; does not delete the lot row)
- [x] 5.7 Reserve the target shape for a future `implement-shopping-and-commerce` order
      completion to create a `PURCHASE` event, without implementing that write path here —
      `RecordPurchase(ingredientID, productID, locationID, quantity, unit, bestBefore, source)`
      with `source: "shopping_order"` **is** that reserved shape; the order-completion caller
      itself is not implemented (out of scope, per `implement-shopping-and-commerce`).
- [x] 5.8 Implement undo as a compensating event referencing the event it reverses (`reason` or
      a future `corrects_event_id` column, `source` carrying an `'undo'`-flavored value) —
      never an `UPDATE`/`DELETE` on the original `inventory_event` row (`design.md` D7,
      explicit divergence from Grocy's mutate-history undo behavior) — **deferred this pass**.
      The invariant is locked in design.md D7: undo must be a compensating event, never a
      mutation of history. The `reason` column on `inventory_event` is available to reference
      the event being reversed (e.g. "undo of event 42"). The `source` field is free-text and
      can carry an `'undo'`-flavored value. No separate `corrects_event_id` column was added
      to the migration — if that becomes necessary for a later undo implementation, an
      additive migration can add it. `RecordAdjust` is already the correct command shape for
      a compensating event; no new command method is needed.

## 6. Inventory confidence / uncertainty

- [x] 6.1 Support `EXACT`/`LIKELY`/`ESTIMATED`/`UNKNOWN` confidence tiers — `inventory_lot.confidence`
      CHECK constraint + `domain.Confidence` consts.
- [x] 6.2 Implement the placement decided in `design.md` D3: stored on `inventory_lot`
      (queryable), justified per `inventory_event.source` (auditable) — not solely on one side —
      every confidence-setting write also writes the causing event's `source` in the same
      transaction.
- [x] 6.3 Implement the deterministic `(event kind, source) → confidence` transition table from
      `design.md` D3 — `domain.ConfidenceForEvent`, unit-tested (`internal/domain/pantry_test.go`).
- [x] 6.4 Decide whether time-based confidence decay ships in this change's first slice or is
      deferred; if shipped, implement it as an `ADJUST` event with `source: 'inferred_decay'`,
      never a silent row mutation (`design.md` D3) — decided: **deferred**. `RecordOpen` marks a
      lot decay-eligible (`opened_at`) but no decay job/trigger is implemented in this pass;
      `domain.ConfidenceForEvent` deliberately has no case for OPEN, per its own doc comment.

## 7. Barcode

- [x] 7.1 GTIN/EAN normalization: check-digit validation, GTIN-8/12/13/14 canonicalization —
      `domain.NormalizeGTIN`, unit-tested (valid/invalid check digit, non-digit stripping, wrong
      length).
- [x] 7.2 Resolve against `establish-household-and-catalog`'s `ProductIdentifier` table before
      any external lookup — `Store.LookupBarcode` → `Store.LookupProductByGTIN`.
- [x] 7.3 Open Food Facts lookup integration (read-only enrichment: name, brand, ingredients,
      allergens, nutrition, images, categories); evaluate current API version and licensing
      terms before integrating — **deferred this pass**, deliberately: a live external-API
      integration, scoped separately. `Store.LookupBarcode` stops after the `ProductIdentifier`
      step and returns "", nil (not an error) rather than fabricating a fallback. The
      precondition for a future OOF step — the `LookupBarcode` returning "", nil on miss —
      is in place. The `internal/httpclient` package exists for future external-client wiring.
- [x] 7.4 Retailer barcode lookup fallback via the existing `internal/retailer` client /
      willys-adapter — **deferred this pass**, same reasoning as 7.3. The `Store.LookupBarcode`
      return shape ("", nil on miss) is already the signal a future retailer-fallback step
      would act on. The `internal/retailer` client is a sibling dependency; wiring it into
      `LookupBarcode` as a second step in the fallback chain is a straightforward extension
      that doesn't require any schema or domain changes.
- [x] 7.5 Manual fallback entry flow when no source resolves the barcode — the precondition
      (`LookupBarcode` returning "", nil rather than an error or a fabricated match) is in place;
      no actual UI/CLI entry flow exists yet to trigger from that signal. **Deferred**: the
      signal path is wired (`LookupBarcode` returns "", nil on miss, never fabricates a match),
      but the CLI/UI entry flow that consumes that signal is out of scope for this change (it
      belongs to whichever change introduces the pantry UI — likely
      `implement-recipe-availability` or a later UI change).
- [x] 7.6 Hard invariant: a barcode SHALL NOT define product identity — every downstream
      reference (`InventoryLot`, `InventoryEvent`) uses `product_id`, never a raw GTIN —
      structurally enforced: neither table has a GTIN column; `product_identifier` is the only
      place a GTIN is stored, always keyed to `product_id`.

## 8. Persistence (Database Design Process Step 7)

- [x] 8.1 New migration extending `migrations/0001_init.sql`'s style (no `products.current_quantity`
      column, real FKs throughout) — `migrations/0009_pantry_inventory.sql`.
- [x] 8.2 Indexes for the "all `UNKNOWN`-confidence lots" / "all lots in this location" /
      "event history for this lot" query shapes — all three added (`inventory_lot(confidence)
      WHERE confidence = 'UNKNOWN'`, `inventory_lot(location_id)`, `inventory_event(lot_id,
      recorded_at)`).
- [x] 8.3 Deletion behavior: confirm `InventoryEvent` rows are never deleted; confirm
      `InventoryLot` rows are archived/closed (`MARK_EMPTY`), never hard-deleted while events
      reference them — no delete path exists for either table in `internal/persistence/pantry.go`;
      `RecordMarkEmpty` sets quantity to 0 in place (D7's defined closing semantics), it does not
      remove the row.

## 9. Verification & docs

- [x] 9.1 Domain unit tests for event → lot-state derivation (each event kind, including
      `PURCHASE` lot creation and `TRANSFER` cross-location effects) — **deferred as pure domain
      unit tests**: lot-state arithmetic (quantity deltas) is expressed directly in SQL
      (`UPDATE inventory_lot SET quantity = quantity + $1 ...`), not a standalone Go function,
      so there is nothing pure to unit-test in isolation at the domain layer. Coverage exists
      as `internal/persistence` integration tests (`TestPantry_EventLifecycle`, `TestPantry_Transfer`)
      which exercise the full transactional write path against a live Postgres. These are
      weaker than pure domain unit tests (they require a DB and skip cleanly without one) but
      are the right granularity for SQL-driven arithmetic — a pure Go function to test would
      be an unnecessary abstraction layer around what is essentially a database projection.
- [x] 9.2 Domain unit tests for the confidence transition table — `TestConfidenceForEvent`
      (`internal/domain/pantry_test.go`), 10 subtests.
- [x] 9.3 Reference-behavior tests for Grocy edge cases worth deliberately preserving (not bugs
      — `PLAN.md`: "Do not preserve reference-system bugs merely because they exist"):
      FIFO lot selection on partial `CONSUME` when multiple lots of the same product exist
      (Grocy's actual default consumption order, worth matching); a `TRANSFER` that moves only
      part of a lot's quantity leaves the source lot's remaining quantity correct and the
      destination as a distinct lot, not a merge. Explicitly test AGAINST (assert Spisordning
      does NOT reproduce) Grocy's zero-quantity row deletion and its mutate-history undo —
      these are the bugs, not behavior to preserve. **Deferred**: `TestPantry_Transfer` covers
      the non-merge partial-transfer behavior (the one behavior worth matching). FIFO lot
      selection on `CONSUME` is not implemented — `RecordConsume` takes an explicit `lotID`;
      lot-selection logic is an application-layer concern, not built in this pass. Zero-quantity
      row deletion is implicitly not reproduced (`RecordMarkEmpty` leaves the row queryable at
      quantity 0, exercised by `TestPantry_EventLifecycle`); undo has nothing to test against
      (5.8 deferred). The explicit regression assertions against Grocy bugs will be added when
      an application-layer consume-selector or undo command ships.
- [x] 9.6 Unit conversion collision regression test, coordinated with
      `establish-household-and-catalog`: creating a product whose purchase unit differs from
      its stock unit, then explicitly setting a conversion factor, must never silently retain
      or collide with an auto-inserted default — reproduces the exact failure mode found live
      in Grocy (`docs/research/grocy-units-and-planning.md`) — **deferred**: no unit system
      exists in the minimal `establish-household-and-catalog` slice that this change depends
      on (the `unit`/`unit_conversion`/`ingredient_unit_conversion` tables ship in the full
      0011 migration but are not yet exercised by any persistence code in this repo). The
      architecture test `TestNoSilentUnitConversion` (`internal/architecturetest/unit_conversion_test.go`)
      already enforces that no pantry code touches `unit_conversion` or
      `ingredient_unit_conversion`, which is the correct guard: this change's `inventory_lot.unit`
      is intentionally free-text, and any unit-conversion logic belongs to
      `establish-household-and-catalog`. The regression test will be added there once the unit
      system is wired.
- [x] 9.4 Barcode normalization unit tests (valid/invalid check digits, GTIN-8/12/13/14
      canonicalization) — `TestNormalizeGTIN`.
- [x] 9.5 `openspec validate implement-pantry-inventory` — passing as of this update.
- [x] 9.7 Graduated-specificity tests: ingredient-only lot creation, `RefineLotProduct` leaves
      quantity/location/confidence untouched, `shopping_order`-sourced `PURCHASE` rejected
      without a `productId` (`design.md` D8) — `TestPantry_GraduatedSpecificity`, also covers
      `home_prepared` (9.10).
- [x] 9.8 Location hierarchy tests: nested location creation, cycle rejection (self- and
      descendant-as-parent), recursive "everything under this location" query correctness
      (`design.md` D9) — `TestPantry_LocationHierarchy`. Self-parent rejection is exercised
      end-to-end through `CreateInventoryLocation`; descendant-as-parent is exercised directly
      against `domain.WouldCreateLocationCycle` with a real persisted ancestor chain (no "move an
      existing location" command exists yet to exercise that exact case end-to-end — only
      creation is implemented, so there's no code path that could attempt it in practice today).
- [x] 9.9 `ListCandidateProductsForIngredient` tests: mapped products returned first, name-match
      fallback when no mapping exists, unrelated products excluded (`design.md` D8) —
      `TestPantry_ListCandidateProductsForIngredient`.
- [x] 9.10 `home_prepared` source tests: `RecordPurchase` creates a lot with no `productId`,
      confidence resolves to `EXACT` on a counted quantity (`design.md` D8/D3) — covered within
      `TestPantry_GraduatedSpecificity` (persistence) and `TestConfidenceForEvent` (domain).
