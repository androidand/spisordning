## Context

`PLAN.md`'s "Pantry" and "Inventory Uncertainty" sections leave two questions genuinely open
rather than pre-deciding them:

1. Should `inventory_lot` be mutable current-state, or should everything be derived from
   `inventory_event` history (pure event sourcing)?
2. Where does confidence (`EXACT`/`LIKELY`/`ESTIMATED`/`UNKNOWN`) live — on the lot, on
   observations, derived from event history, or some combination?

`PLAN.md`'s "Database Design Process" (Steps 1-9) requires vocabulary → aggregates →
relationships → lifecycle → commands → invariants before persistence is proposed. This is
performed below. Unlike `establish-household-and-catalog`, this is genuinely greenfield —
`docs/research/current-state.md` confirms zero pantry/inventory code or schema exists anywhere
in this repo or its siblings — but it is not designed in a vacuum: `migrations/0001_init.sql`
already contains one directly analogous pattern (`person_preference`, a derived current belief,
paired with `preference_observation`, an append-only evidence ledger), and this design
deliberately reuses that shape rather than inventing a new one.

**Update (2026-08-16): `establish-reference-lab`'s Grocy investigation is now complete** — see
`docs/research/grocy-inventory-and-stock.md`, `grocy-units-and-planning.md`, and
`grocy-api-and-database.md`. The cross-check flagged throughout this document as a future
revisit has been performed; see the new "Findings from establish-reference-lab" section below,
and D7. The original text below is left intact as the reasoning trail; only D7 and the Risks
section reflect the update.

**Update (2026-08-19): two household-requested refinements** — graduated item specificity
(D8) and location taxonomy/hierarchy (D9), both amending Step 1/3/5/7 below. Marked inline
where they touch prior text; the reasoning in D1-D7 is unaffected.

## Goals / Non-Goals

**Goals:**
- Model a lot as the unit of physical household inventory, never a `products.current_quantity`
  field.
- Give every inventory mutation an auditable, replayable history (PLAN.md: "needs careful
  history/audit design").
- Place inventory confidence somewhere that is both queryable (fast "show me all UNKNOWN lots")
  and honest about its provenance (why is this lot's confidence what it is).
- Avoid generic polymorphism (`entity_type`/`entity_id`/`value`) in the event ledger.
- Treat barcode strictly as a lookup key onto `Product`, never as identity.

**Non-Goals:**
- Not designing Ingredient/Product/Unit — consumed from `establish-household-and-catalog`.
- Not designing recipe-feasibility logic against inventory — that is
  `implement-recipe-availability`.
- Not implementing a full event-sourcing framework with replay/versioning/snapshots — the
  household-scale write volume here does not justify that machinery (see D4).
- Not deciding shopping-driven `PURCHASE` event creation from `implement-shopping-and-commerce`'s
  future `order` completion — only reserving the shape it will target.

## Step 1 — Vocabulary

| Term | Definition |
|---|---|
| **InventoryLocation** | A named place physical inventory sits (pantry shelf, fridge, freezer, garage freezer), scoped to a `Household`, typed by an optional `LocationType` and optionally nested under a parent `InventoryLocation` (D9). |
| **LocationType** | An optional taxonomy hint on an `InventoryLocation` — `CUPBOARD` / `DRAWER` / `FRIDGE` / `FREEZER` / `BASEMENT` / `BALCONY` / `BREADBOX` / `OTHER`, extensible (D9). Not identity — two locations of the same type are still distinct rows. |
| **InventoryLot** | A distinguishable physical quantity sitting in one `InventoryLocation` at one time — the unit of physical inventory. Always anchored to an `Ingredient`; anchored to a specific `Product` only once known (D8). |
| **InventoryEvent** | An immutable, append-only record of something that happened to inventory: a lot was purchased, consumed from, discarded, adjusted, transferred, marked empty, or opened. |
| **Confidence** | How sure the system is that an `InventoryLot`'s current recorded quantity/existence reflects physical reality: `EXACT` / `LIKELY` / `ESTIMATED` / `UNKNOWN`. |
| **Barcode (GTIN/EAN)** | A normalized numeric identifier printed on packaging, used only as a lookup key onto a `Product` via `ProductIdentifier` (from `establish-household-and-catalog`) — never as identity itself. Distinct from a printed inventory-label barcode, which references an `InventoryLot`, not a `Product` (see `research-inventory-label-printing`). |

## Step 2 — Aggregates

- **InventoryLot aggregate** (root: `InventoryLot`). Represents current physical state: which
  product, how much, where, since when, in what confidence. This is a *read-optimized
  projection*, not an independently-editable record — see D2.
- **InventoryEvent** is not owned by the lot aggregate as a child collection in the usual sense;
  it is the append-only *cause* of lot state changes, closer in spirit to
  `preference_observation`'s relationship to `person_preference` than to a parent-owns-children
  aggregate. Treating the event ledger as its own append-only stream (rather than "inside" the
  lot aggregate) keeps a `PURCHASE` event free to *create* a new lot rather than requiring a lot
  to already exist before its own history can be recorded.
- **InventoryLocation** is household-owned reference data, structurally similar to
  `effort_profile` (small, curated, household-scoped) — not its own complex aggregate.

## Step 3 — Conceptual relationships

```text
Household
    │
    ▼
InventoryLocation ──── parent_location_id (optional, self-referential, D9)
    │
    │ (1) hosts (N)
    ▼
InventoryLot ──────► Ingredient (establish-household-and-catalog, always set)
    │        ╲
    │          ╲ (optional, D8)
    │            ▼
    │          Product (establish-household-and-catalog) ──► ProductIdentifier (barcode)
    │
    └──── projected from ────  InventoryEvent (append-only)
                                    │ kind: PURCHASE | CONSUME | DISCARD | ADJUST
                                    │       | TRANSFER | MARK_EMPTY | OPEN
                                    ▼
                          concrete typed references per kind:
                            lot_id (nullable — PURCHASE creates one)
                            from_location_id / to_location_id (TRANSFER only)
                            ingredient_id, product_id (nullable) (PURCHASE only, to create the lot)
```

Key relationship decisions:
- `InventoryEvent` references `InventoryLot` (nullable — only `PURCHASE` may create a new lot
  without one existing yet), `InventoryLocation` (source/destination, `TRANSFER` uses both),
  and `Ingredient`/`Product` (via the lot, or directly on `PURCHASE`). All real FKs — no
  polymorphic `entity_type`/`entity_id` column (see D5).
- A lot's `product_id` is set once at creation (or added later via `RefineLotProduct`, D8) and
  never silently overwritten; if what's physically in the lot changes identity (rare — e.g. a
  corrected barcode misread), that is a `DISCARD` + new `PURCHASE`-kind `ADJUST`, not a
  reassignment of `product_id` on the existing lot, so the ledger stays truthful about what was
  on hand when. `ingredient_id`, once set at lot creation, is immutable for the same reason.

## Step 4 — Lifecycle (mutable vs. immutable)

| Entity | Lifecycle |
|---|---|
| `InventoryLocation` | Mutable (rename); soft-archived, never hard-deleted while lots reference it. |
| `InventoryLot` | Mutable **projection** — every field-level change happens only as the side effect of inserting an `InventoryEvent` in the same transaction (see D2); never edited directly by application code outside that path. |
| `InventoryEvent` | Append-only, immutable once written — a correction is a new event (e.g. a mis-recorded `CONSUME` is fixed by an `ADJUST`, not an `UPDATE`/`DELETE` on the original row). |
| Confidence | Mutable, but only ever changed as part of writing an `InventoryEvent` — never edited standalone (see D3). |

## Step 5 — Commands

- `CreateInventoryLocation(householdId, name, locationType?, parentLocationId?)` (D9)
- `RecordPurchase(ingredientId, productId?, locationId, quantity, unit, bestBefore?, source)` →
  creates a new `InventoryLot` + a `PURCHASE` event; `productId` is required when `source` is
  `shopping_order` (the retailer resolution already knows the exact product), optional otherwise
  (D8)
- `RefineLotProduct(lotId, productId, source)` → attaches a specific `Product` to a
  previously ingredient-only lot; does not itself change quantity/location/confidence (D8)
- `ListCandidateProductsForIngredient(ingredientId, query?)` → the picker `RefineLotProduct`
  is driven from: `Product` rows already linked to `ingredientId` via
  `ProductIngredientMapping`, plus a name-match fallback against the ingredient's canonical
  name when no mapping yet exists, never an unscoped catalog-wide product search (D8)
- `RecordConsume(lotId, quantity, source)` → decrements the lot, appends a `CONSUME` event
- `RecordDiscard(lotId, quantity, reason, source)` → decrements the lot, appends a `DISCARD`
  event (reason: expired / spoiled / other)
- `RecordAdjust(lotId, newQuantity, reason, source)` → corrects the lot to an observed quantity,
  appends an `ADJUST` event
- `RecordTransfer(lotId, fromLocationId, toLocationId, quantity, source)` → moves (all or part
  of) a lot between locations, appends a `TRANSFER` event
- `RecordMarkEmpty(lotId, source)` → sets quantity to zero / closes the lot, appends a
  `MARK_EMPTY` event
- `RecordOpen(lotId, source)` → marks a sealed lot opened (does not by itself change quantity or
  confidence — see D3), appends an `OPEN` event
- `LookupBarcode(gtin)` → normalizes the GTIN, resolves via `ProductIdentifier`, falling back to
  Open Food Facts, then retailer lookup, then manual entry (see D6)

Every command above requires a `source` — where the observation came from
(`purchase_receipt` | `barcode_scan` | `manual_count` | `shopping_order` | `inferred_decay` |
`home_prepared`, extensible) — because `source` is the raw material Step 6's confidence rule
uses. `home_prepared` is `RecordPurchase`'s source for inventory that didn't come from a
retailer at all — a home-cooked meal portioned and frozen — resolving
`research-inventory-label-printing`'s "how does a frozen home-cooked meal become a lot at all"
open question without a new event kind: it's still "new inventory came into existence," just
with a `source` that isn't a retailer transaction, so `PURCHASE`'s existing semantics (creates a
lot, `ingredientId` required, `productId` optional per D8) already fit without a change to the
kind vocabulary in D7.

## Step 6 — Invariants

1. **A lot is the unit of physical inventory.** `products.current_quantity`-style single-field
   tracking SHALL NOT exist; `InventoryLot` (scoped to ingredient/product + location) is the
   only model of "how much of this do we have."
2. **`InventoryEvent` rows are immutable once written.** Corrections are new events, never
   updates or deletes of existing ones.
3. **`InventoryLot` current state is never edited outside an `InventoryEvent` write.** Every
   quantity/confidence change on a lot happens in the same transaction as the event that caused
   it (see D2) — there is no code path that mutates a lot row without a corresponding event.
4. **Confidence is one of exactly four tiers.** `EXACT` / `LIKELY` / `ESTIMATED` / `UNKNOWN`,
   never a free-form value.
5. **A barcode never defines identity.** Resolving a GTIN yields a `Product` reference via
   `ProductIdentifier`; no downstream table (`InventoryLot`, `InventoryEvent`) stores or keys on
   a barcode directly — they reference `product_id`.
6. **No generic polymorphism.** `InventoryEvent` SHALL use concrete, nullable, typed FK columns
   per event kind's actual references, never an `entity_type`/`entity_id`/`value` shape.
7. **A lot always has an ingredient; a specific product is optional and only ever added, never
   silently invented.** (D8)
8. **A location's type and parent are hints, not identity — the same free-form-plus-lookup
   discipline as barcode (invariant 5).** No behavior SHALL depend on `location_type` alone
   defining what a location *is*; two `FRIDGE`-typed locations remain distinct rows. (D9)

## Step 7 — Persistence (sketch)

- `inventory_location(id, household_id FK, name, location_type CHECK IN ('CUPBOARD','DRAWER',
  'FRIDGE','FREEZER','BASEMENT','BALCONY','BREADBOX','OTHER') NULL, parent_location_id FK NULL
  REFERENCES inventory_location(id), archived_at NULL)` — self-referential parent, D9
- `inventory_lot(id, ingredient_id FK NOT NULL, product_id FK NULL, location_id FK, quantity,
  unit, confidence CHECK IN (...), best_before NULL, opened_at NULL, created_at, updated_at)`
  — `product_id` nullable, D8
- `inventory_event(id, kind CHECK IN ('PURCHASE','CONSUME','DISCARD','ADJUST','TRANSFER',
  'OPEN'), lot_id FK NULL, ingredient_id FK NULL, product_id FK NULL, from_location_id FK NULL,
  to_location_id FK NULL, quantity_delta, reason NULL, source NOT NULL, recorded_at)`
  — **six kinds, not seven: see D7.** `MARK_EMPTY` is a `RecordMarkEmpty` *command* (Step 5)
  that writes a `CONSUME` event with `source: 'mark_empty'`, not its own `kind` value.
- `product_identifier` — reused from `establish-household-and-catalog`, not redefined here; this
  change adds the GTIN normalization/lookup application logic on top (Open Food Facts client,
  retailer fallback, manual entry) as `internal/pantry/barcode` (or equivalent), not a new table.

(Full column-level constraints, indexes, and the migration file itself are implementation work,
tracked in `tasks.md`.)

## Decisions

### D1: Vocabulary and scope match `PLAN.md` literally

The event kind enum is the literal `PURCHASE`/`CONSUME`/`DISCARD`/`ADJUST`/`TRANSFER`/
`MARK_EMPTY`/`OPEN` list `PLAN.md` gives, not a paraphrase or a collapsed subset — this keeps
the design traceable back to the Grocy-referenced behavior `establish-reference-lab` will
document per kind.

### D2: Ledger-plus-projection, not pure current-state and not pure event-sourcing

Three shapes were considered:

- **(a) Pure mutable current-state** (a single `inventory_lot` row updated in place per
  operation, no event table). Rejected: `PLAN.md` explicitly wants audit/history ("needs careful
  history/audit design"), and a pure current-state model can't answer "why is the milk gone" —
  this is exactly the `products.current_quantity` anti-pattern `PLAN.md` names and rejects.
- **(b) Pure event-sourcing** (no stored `inventory_lot` row at all; current state is always
  computed by replaying `inventory_event` from the beginning, or from the last snapshot). Gives
  perfect auditability but requires replay/snapshot machinery, versioned event schemas, and
  either slow reads or a caching layer that re-introduces most of (c)'s complexity anyway. At
  household scale (a handful of locations, tens to low hundreds of lots, a few events per lot
  per week) this machinery is not justified by the write volume or query patterns; revisit only
  if reporting/analytics needs later prove otherwise.
- **(c) Ledger plus transactionally-maintained projection** (chosen): `inventory_event` is the
  append-only source of truth for *what happened*; `inventory_lot` is a mutable row that is
  *always* written in the same transaction as the event that changes it, so it is never allowed
  to drift from the ledger. This is the same shape `migrations/0001_init.sql` already uses for
  `person_preference` (derived current belief) + `preference_observation` (append-only
  evidence) — proven in this codebase, not a new pattern.

Every command in Step 5 is implemented as: insert one `inventory_event` row, then
insert-or-update the corresponding `inventory_lot` row, in one database transaction. The lot can
always be rebuilt from the event history if it ever needs to be (a recovery/consistency-check
path), but that is a fallback, not the primary read path.

**Validated by Grocy's real design (2026-08-16):** `docs/research/grocy-api-and-database.md`
confirms Grocy's `stock` table — its current-state projection — genuinely `DELETE`s a lot's row
once its quantity reaches zero; only the separate `stock_log` table preserves any history of
that lot ever existing. This is a live instance of exactly the option-(a) failure this design
rejected ("can't answer why is the milk gone") — not a hypothetical. It directly supports
choosing (c) over (a): a projection alone, even a well-intentioned one, degrades to lossy
current-state the moment nothing forces it to keep a durable, append-only counterpart.

### D3: Confidence placement — stored on the lot, justified per-event (the combination option)

`PLAN.md` asks explicitly whether confidence belongs on current lot state, on observations,
derived from event history, or some combination. Each option in isolation has a real weakness:

- **Solely on the lot** (a plain denormalized column, no linkage to why): cheap to query
  ("`WHERE confidence = 'UNKNOWN'`" is trivial), but nothing explains *why* a lot is `LIKELY`
  vs. `ESTIMATED`, and it's easy for application code to drift from the truth over time.
- **Solely on events/observations** (no column on the lot; confidence only exists per-event):
  preserves full provenance, but every "show me uncertain inventory" query needs a join to each
  lot's most recent event, and there is no cheap current-state answer.
- **Purely derived/computed at read time** (no storage at all — confidence is a function
  evaluated against event history and elapsed time on every read): always consistent by
  construction and needs no write-path discipline, but cannot be indexed, is expensive for
  cross-household queries ("all `UNKNOWN` lots this week"), and — critically — decay-by-time
  (a lot not touched in 3 weeks should degrade in confidence) has no natural home if nothing is
  ever written to reflect it.

**Decision: a combination**, mirroring D2's ledger-plus-projection shape exactly:
- `inventory_lot.confidence` is the current, queryable, indexed belief — same treatment as
  `person_preference.confidence`.
- Every `inventory_event` carries a `source` (`purchase_receipt` | `barcode_scan` |
  `manual_count` | `shopping_order` | `inferred_decay` | `home_prepared`, extensible) that is
  the evidence
  justifying whatever confidence the lot is set to as part of that same transaction — same
  treatment as `preference_observation`.
- Confidence transitions are a deterministic function of `(event kind, source)`, applied at
  write time, not silently recomputed by a background process:
  - `PURCHASE` with a known quantity (receipt, barcode-confirmed, shopping-order-derived, or a
    counted home-prepared portion) → `EXACT`.
  - `CONSUME`/`ADJUST`/`DISCARD` with an explicit counted quantity → `EXACT`; with an estimated
    quantity ("about half") → `ESTIMATED`.
  - `OPEN` does not by itself downgrade confidence (quantity is still known) but marks the lot
    eligible for time-based decay, since an opened item's actual remaining quantity drifts
    fastest.
  - `TRANSFER` preserves the source lot's confidence (moving inventory doesn't change how sure
    we are of the amount).
  - `MARK_EMPTY` always sets `EXACT` (zero is a certain observation) and closes the lot.
- **Time-based decay** (an untouched, opened, estimated lot should eventually read as `UNKNOWN`)
  is deliberately *not* implemented as a background job silently rewriting `confidence` columns
  — that would violate invariant 3 (lot state changes only via an event write) unless the decay
  job itself writes an `ADJUST`-with-`source: 'inferred_decay'` event, which is the option this
  design prefers if/when decay is implemented: decay becomes just another event, auditable like
  any other, rather than a special silent mutation path. Whether decay ships in this change's
  first slice or is deferred is a `tasks.md` item, not a `design.md` commitment.

This resolves PLAN.md's question with "a combination" made concrete: storage location (lot,
for queryability) plus provenance (event, for auditability) plus a stated, deterministic
transition function (not ad hoc), and an explicit decision that even automatic decay stays
inside the append-only-event discipline rather than becoming a silent mutation exception.

**Cross-checked against Grocy (2026-08-16):** Grocy has **no uncertainty/confidence modeling of
any kind** — `docs/research/grocy-inventory-and-stock.md` confirms every stock quantity Grocy
records is treated as exactly known, with no tier, no "estimated" flag, nothing. This decision
therefore has no reference-system precedent to validate or contradict it either way; it stands
entirely on `PLAN.md`'s own reasoning above. Recorded explicitly so implementation doesn't
mistake silence in the research docs for agreement — it's absence of a comparison, not
confirmation.

### D4: Why not full event sourcing (elaboration on D2's rejected option (b))

Beyond the machinery cost, full replay-based event sourcing would also complicate the exact
question D3 needs answered simply: "what is this lot's confidence right now." Under pure
event-sourcing, confidence itself would need to be derived at replay time, re-introducing
option (c)'s query-cost problem from D3 as well. Rejecting (b) in D2 and choosing the
combination in D3 are the same underlying decision applied twice.

### D5: No generic polymorphism in the event table

`inventory_event` uses concrete nullable columns (`lot_id`, `product_id`, `from_location_id`,
`to_location_id`) scoped to what each event kind actually needs, rather than a generic
`entity_type TEXT, entity_id TEXT, value JSONB` shape. This costs a few always-null columns per
row depending on kind (e.g. `from_location_id`/`to_location_id` are null except for `TRANSFER`)
but keeps every reference a real, checkable foreign key — directly following `PLAN.md`'s "Do Not
Use Generic Polymorphism Carelessly," which names exactly this `entity_type`/`entity_id`/`value`
pattern as something to avoid absent a consciously accepted loss of FK integrity.

### D6: Barcode is a lookup key, resolved through a fallback chain, never identity

`LookupBarcode(gtin)` resolution order: normalize the GTIN (validate check digit, canonicalize
GTIN-8/12/13/14 to a comparable form) → look up `ProductIdentifier` (already-known product) →
Open Food Facts (read-only enrichment: name, brand, ingredients, allergens, nutrition, images,
categories) → retailer lookup (via the existing `internal/retailer` client / willys-adapter) →
manual fallback (a person types in what it is). Whichever step resolves, the result is a
`Product` reference (existing or newly registered), and a `ProductIdentifier` row is written so
the same barcode resolves instantly next time. No table in this change's scope stores a raw
GTIN as a join key for inventory — `InventoryLot`/`InventoryEvent` always reference `product_id`.

### D7: `MARK_EMPTY` collapses into `CONSUME`; `DISCARD` stays distinct; undo is a compensating event (revised after Grocy findings, 2026-08-16)

D1 committed to the literal `PURCHASE`/`CONSUME`/`DISCARD`/`ADJUST`/`TRANSFER`/`MARK_EMPTY`/
`OPEN` vocabulary from `PLAN.md`, pending the Grocy cross-check. That cross-check
(`docs/research/grocy-inventory-and-stock.md`) is now done and changes one part of the
persistence-layer design, confirms another, and adds a hard divergence:

- **`MARK_EMPTY` is not a distinct transaction kind in Grocy at all.** Grocy has no
  `mark-as-empty` API endpoint or stock-log transaction type — "mark empty" in its UI is sugar
  that pre-fills a `CONSUME` call with the lot's full remaining quantity. There is no Grocy
  behavior to reference for a separate kind because none exists.
  **Revision:** `RecordMarkEmpty(lotId, source)` (Step 5) stays as a command — it's genuine,
  useful UX (one tap, no need to know or estimate the exact remaining quantity) — but it no
  longer emits its own `inventory_event.kind`. It emits a `CONSUME` event with
  `quantity_delta` set to the lot's full current quantity. The event-kind enum in Step 7
  shrinks to `PURCHASE`/`CONSUME`/`DISCARD`/`ADJUST`/`TRANSFER`/`OPEN` (six, not seven).
  Distinguishing "consumed via mark-empty" from an ordinary partial `CONSUME` (useful for
  future analytics — e.g. "the household keeps under-buying milk") is carried on `source`
  (add `'mark_empty'` to the extensible `source` vocabulary in Step 5), not on `kind`. This
  keeps the event table's core discriminator (`kind`) matched to genuinely distinct *effects*,
  not UI entry points — the same reasoning D5 already applies to keep FKs concrete rather than
  generic; here it's the mirror move, collapsing a kind that turned out not to be one.
- **`DISCARD` stays distinct — validated, not revised.** Grocy models discard as `CONSUME` with
  a `spoiled` boolean rather than its own kind, which the research doc flags as a real Grocy
  limitation (spoilage is analytically important — "how much do we waste and why" — and
  deserves to be a first-class, queryable kind, not a flag buried on the general consume path).
  This design already keeps `DISCARD` distinct with its own `reason` field (Step 5); Grocy's
  choice here is a documented weakness to avoid copying, not evidence to follow.
- **Undo must be a compensating event, never a mutation of history — this is a deliberate
  divergence from Grocy, not an oversight.** `docs/research/grocy-api-and-database.md` found
  that Grocy's own "undo last transaction" feature works by mutating the historical
  `stock_log` row it's undoing, rather than writing a new, opposing entry. That directly
  contradicts this design's invariant 2 (`InventoryEvent` rows are immutable once written).
  Grocy's approach is easy to build and immediately wrong for exactly the auditability goal
  `PLAN.md` states for this domain ("needs careful history/audit design") — a lot's event
  history should truthfully show that a purchase was recorded and then undone, not show only
  that nothing happened. **Decision:** if/when an "undo" command ships, it is implemented as
  `RecordAdjust`-or-kind-appropriate-compensating-event with `source` carrying an
  `'undo'`-flavored value and a reference (in `reason` or a future `corrects_event_id` column)
  back to the event it reverses — never as an `UPDATE`/`DELETE` on the original row. This is a
  `tasks.md` item (undo is not in this change's Step 5 command list today) but the invariant is
  locked in now so it isn't accidentally built the Grocy way later.

### D8: Graduated item specificity — `ingredient_id` required, `product_id` optional (2026-08-19)

The household explicitly wants two ways to know "we have milk": a quick, generic entry ("we
have mjölk," no product picked) and a detailed one (a specific product, usually populated
automatically from a completed online order). Three shapes were considered:

- **(a) `product_id` always required** (the original Step 7 sketch). Rejected outright now:
  forces a specific `Product` to be chosen even for a two-second manual "we have milk" entry,
  which the household has said is friction they want removed. Also awkward for the barcode
  scan-a-product-with-no-local-`Product`-row-yet case at the moment of scanning.
- **(b) A separate "generic lot" table alongside `inventory_lot`**, unioned at read time.
  Rejected: two tables for what is conceptually one thing (a physical quantity of stuff in a
  place) forces every downstream reader (`implement-recipe-availability`,
  `implement-recommendations`) to query and merge two sources instead of one, and re-introduces
  exactly the kind of parallel-model drift `establish-household-and-catalog` already avoided by
  making `Product`→`Ingredient` mapping optional rather than a second ingredient table.
- **(c) `ingredient_id` required, `product_id` nullable, refined in place** (chosen): mirrors
  `establish-household-and-catalog`'s own precedent almost exactly — that change already
  established "a `Product` without a resolved `Ingredient` mapping is still valid... no
  `Ingredient` row is invented or guessed automatically." This decision takes the symmetric
  position one layer up: an `InventoryLot` without a resolved `Product` is still valid, and no
  `Product` row is invented or guessed to satisfy it. One table, one read path, two honestly
  optional levels of detail.

Two write paths populate `product_id` differently, both already implied by the household's own
description:
- **Manual quick entry** (no order, e.g. after a physical-store trip without barcode scanning):
  `RecordPurchase` is called with `productId` omitted. The lot is created at `ingredient_id`-only
  specificity. `RefineLotProduct(lotId, productId, source)` (Step 5) attaches a specific
  `Product` later, at any point, without touching quantity/location/confidence — the household
  said this refinement should be optional and can happen "whenever," not only at creation time.
- **Online order completion**: `implement-shopping-and-commerce`'s order already carries a
  resolved `RetailerProduct` → `Product` for every line (the retailer resolution pipeline's
  entire job). A `PURCHASE` event with `source: 'shopping_order'` therefore always supplies
  `productId` — there is no "quick" version of this path because the specificity already exists
  for free; forcing a later manual refinement step here would throw away information the system
  already has. This is recorded as invariant 7 and enforced at the `RecordPurchase` boundary
  (Step 5's `productId` is required when `source` is `shopping_order`), not left to convention.
- **Home preparation**: a home-cooked meal portioned and frozen isn't a retailer purchase at
  all, but is still "new inventory came into existence" — `RecordPurchase` with
  `source: 'home_prepared'` covers it without a new event kind (see Step 5's source vocabulary
  update). `productId` is naturally omitted here (there's no retailer product for a home-cooked
  dish) — this path always lands at ingredient-only specificity, refinable only in the trivial
  sense of naming which dish/recipe it was, which is `Ingredient`-level information already.

**Connecting a lot to a `Product` is scoped by the lot's `Ingredient`, never an unscoped
catalog search.** The household asked for this explicitly: refining "we have mjölk" into a
specific product should be driven by the ingredient's type/name, not require already knowing
the product's own name or browsing the entire catalog. `RefineLotProduct`'s picker is powered
by the new `ListCandidateProductsForIngredient(ingredientId, query?)` (Step 5): first, `Product`
rows already linked to this `ingredient_id` via `ProductIngredientMapping` (the common case once
a household has shopped online a few times — most milk products are already mapped); falling
back to a name-match search against the ingredient's canonical name (`Ingredient.display`) when
no mapping exists yet for this exact product. This reuses `establish-household-and-catalog`'s
existing mapping table rather than adding a second index, and keeps the invariant from that
change intact — an unmapped `Product` selected this way still doesn't force a `Ingredient` row
to be invented; it's the household confirming a match, which is exactly how new
`ProductIngredientMapping` rows are expected to get created in the first place.

`ingredient_id` is immutable once a lot is created (Step 3) — if a lot turns out to have been
misidentified at the ingredient level (rare; usually only "quick" entries), the correction is a
`DISCARD` of the wrong lot plus a fresh `PURCHASE`, same pattern already used for a corrected
barcode misread (Step 3), not a mutation of the existing row.

### D9: Location taxonomy and optional hierarchy — typed, self-referential, still reference data (2026-08-19)

Task 3.2 asked directly: is nesting needed, or is a flat named-location list sufficient. The
household's own description — cupboard, drawer, fridge, freezer, basement, balcony, breadbox,
"wherever the user stores their stuff" — settles this empirically: their storage is neither flat
nor uniformly typed (a chest freezer *in* the basement; a produce drawer *inside* the fridge),
and an unbounded, household-specific set of places rules out a fixed enum of location identities.
`parent_location_id` being nullable means nesting is opt-in per location, not imposed: a
household is expected to leave most locations flat (`parent_location_id` unset) and nest only
the handful that are genuinely inside another — the household itself expects flat to be the
common case, hierarchy the exception, which this shape already gives for free without a
separate "flat mode" toggle or a forced setup step asking every location's parent.

- **Typing (`location_type`)**: an optional, extensible enum (`CUPBOARD`/`DRAWER`/`FRIDGE`/
  `FREEZER`/`BASEMENT`/`BALCONY`/`BREADBOX`/`OTHER`) on `InventoryLocation`, used as a *hint* —
  e.g. a future feature (not this change) reading `FREEZER` to suggest a longer default
  best-before window, or `research-inventory-label-printing` reading it to phrase a label
  ("frozen: 2026-08-19"). It is explicitly not identity: nothing keys behavior off "the one and
  only fridge" — a household with two fridges has two `FRIDGE`-typed rows, same as two
  `BREADBOX`-typed rows are still distinct places (invariant 8, mirroring invariant 5's barcode
  discipline: a hint you can look up, never a thing you can quietly redefine identity around).
- **Hierarchy (`parent_location_id`)**: a nullable, self-referential FK on `InventoryLocation`,
  to arbitrary depth ("freezer" → "basement" → no parent). Rejected alternatives: a fixed-depth
  `room`/`storage_unit`/`shelf` three-tier model (rejected — the household's own list already
  doesn't fit three clean tiers: is "breadbox" a room-level or shelf-level thing? forcing an
  answer is exactly the kind of premature schema the flat list already under-served) and a
  closure/materialized-path table (rejected for the same household-scale reasoning D2 used
  against full event sourcing — a handful of locations, at most a few levels deep, doesn't
  justify the extra table and maintenance cost; a plain recursive `parent_location_id` walk is
  the same technique `implement-recipe-family`'s DAG-cycle reasoning already validated as
  sufficient at this scale, applied to a strictly simpler tree rather than a DAG).
- **Still reference data, not a new aggregate** (Step 2 unchanged): nesting adds a self-reference
  column, not new lifecycle complexity — `InventoryLocation` stays mutable/renameable/
  soft-archived (Step 4), and cycle prevention (a location cannot be its own ancestor) is an
  application-layer check on write, the same shape as `establish-recipe-family`'s
  `LineageGraph.WouldCreateCycle()`, not a database constraint (self-referential FKs can't
  express "no cycles" declaratively).
- **A lot's `location_id` always references the specific location it's actually in** (e.g. the
  produce drawer, not "the fridge" in general) — querying "everything in the fridge" is a
  recursive walk down from the fridge row, not a requirement that lots be recorded at the
  coarsest ancestor. This keeps `TRANSFER` (Step 5) meaningful at whatever granularity the
  household actually uses.

## Risks / Trade-offs

- **Still depends on `establish-household-and-catalog`'s Product/Ingredient model** (not yet
  complete). The Grocy cross-check itself is now done (see "Findings from
  establish-reference-lab" inline in D2/D3/D7 above) — the confidence-transition table (D3)
  is confirmed to have no reference-system precedent either way, the ledger-plus-projection
  shape (D2) is validated by a concrete Grocy failure mode, and the event-kind vocabulary (D7)
  is revised from seven kinds to six with an explicit divergence on undo semantics.
- **Unit conversion is a live risk, not just a modeling question.** `docs/research/
  grocy-units-and-planning.md` found a real, reproduced Grocy bug: creating a product whose
  purchase unit differs from its stock unit silently auto-inserts a wrong 1:1 conversion via a
  trigger, which then collides if a correct factor is set afterward — a genuine
  inventory-accuracy hazard in a years-old, widely-used system. `implement-pantry-inventory`
  doesn't own unit conversion (`establish-household-and-catalog` does), but every quantity this
  change records depends on it being right. Action: `establish-household-and-catalog`'s task
  list should include a test asserting this exact scenario (create a product with differing
  purchase/stock units, then set an explicit conversion factor) never silently produces a wrong
  factor — flagged here so the dependency is visible from this side too.
- **Decay-by-event vs. decay-by-job**: choosing to route even automatic confidence decay through
  the event ledger (D3) is more consistent but means decay must be triggered by something (a
  scheduled reconciliation job that *writes* events, not one that silently updates rows) — the
  triggering mechanism itself is left to `tasks.md`, not decided here.
- **A few always-null columns per event kind** (D5's cost) is accepted in exchange for real FK
  integrity, per `PLAN.md`'s explicit guidance.
- **Two nullable identity columns instead of one required one** (D8's `ingredient_id`/
  `product_id` on both `inventory_lot` and `inventory_event`) is a small ongoing query-shape
  cost (every reader must handle "product unknown"), accepted because the alternative — forcing
  product selection at every entry point — is the exact friction the household asked to remove.
  `implement-recipe-availability` and `implement-recommendations` (both downstream readers) must
  be written against `ingredient_id` as the reliable join key and treat `product_id` as
  enrichment, not the reverse.
- **A self-referential `parent_location_id` needs an application-layer cycle check** (D9) —
  cheap at household scale, but it is a real code path that must exist (mirroring
  `LineageGraph.WouldCreateCycle()`), not a database constraint that enforces itself for free.
