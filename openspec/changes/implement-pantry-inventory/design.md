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
| **InventoryLocation** | A named place physical inventory sits (pantry shelf, fridge, freezer, garage freezer), scoped to a `Household`. |
| **InventoryLot** | A distinguishable physical quantity of a `Product` sitting in one `InventoryLocation` at one time — the unit of physical inventory. |
| **InventoryEvent** | An immutable, append-only record of something that happened to inventory: a lot was purchased, consumed from, discarded, adjusted, transferred, marked empty, or opened. |
| **Confidence** | How sure the system is that an `InventoryLot`'s current recorded quantity/existence reflects physical reality: `EXACT` / `LIKELY` / `ESTIMATED` / `UNKNOWN`. |
| **Barcode (GTIN/EAN)** | A normalized numeric identifier printed on packaging, used only as a lookup key onto a `Product` via `ProductIdentifier` (from `establish-household-and-catalog`) — never as identity itself. |

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
InventoryLocation
    │
    │ (1) hosts (N)
    ▼
InventoryLot ──────► Product (establish-household-and-catalog)
    ▲                    │
    │                    ▼
    │              ProductIdentifier (barcode, optional, many-to-one onto Product)
    │
    └──── projected from ────  InventoryEvent (append-only)
                                    │ kind: PURCHASE | CONSUME | DISCARD | ADJUST
                                    │       | TRANSFER | MARK_EMPTY | OPEN
                                    ▼
                          concrete typed references per kind:
                            lot_id (nullable — PURCHASE creates one)
                            from_location_id / to_location_id (TRANSFER only)
                            product_id (PURCHASE only, to create the lot)
```

Key relationship decisions:
- `InventoryEvent` references `InventoryLot` (nullable — only `PURCHASE` may create a new lot
  without one existing yet), `InventoryLocation` (source/destination, `TRANSFER` uses both),
  and `Product` (via the lot, or directly on `PURCHASE`). All real FKs — no polymorphic
  `entity_type`/`entity_id` column (see D5).
- A lot's `product_id` is set once at creation and immutable; if what's physically in the lot
  changes identity (rare — e.g. a corrected barcode misread), that is a `DISCARD` + new
  `PURCHASE`-kind `ADJUST`, not a mutation of `product_id` on the existing lot, so the ledger
  stays truthful about what was on hand when.

## Step 4 — Lifecycle (mutable vs. immutable)

| Entity | Lifecycle |
|---|---|
| `InventoryLocation` | Mutable (rename); soft-archived, never hard-deleted while lots reference it. |
| `InventoryLot` | Mutable **projection** — every field-level change happens only as the side effect of inserting an `InventoryEvent` in the same transaction (see D2); never edited directly by application code outside that path. |
| `InventoryEvent` | Append-only, immutable once written — a correction is a new event (e.g. a mis-recorded `CONSUME` is fixed by an `ADJUST`, not an `UPDATE`/`DELETE` on the original row). |
| Confidence | Mutable, but only ever changed as part of writing an `InventoryEvent` — never edited standalone (see D3). |

## Step 5 — Commands

- `CreateInventoryLocation(householdId, name)`
- `RecordPurchase(productId, locationId, quantity, unit, bestBefore?, source)` → creates a new
  `InventoryLot` + a `PURCHASE` event
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
(`purchase_receipt` | `barcode_scan` | `manual_count` | `shopping_order` | `inferred_decay`,
extensible) — because `source` is the raw material Step 6's confidence rule uses.

## Step 6 — Invariants

1. **A lot is the unit of physical inventory.** `products.current_quantity`-style single-field
   tracking SHALL NOT exist; `InventoryLot` (scoped to product + location) is the only model of
   "how much of this do we have."
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

## Step 7 — Persistence (sketch)

- `inventory_location(id, household_id FK, name, archived_at NULL)`
- `inventory_lot(id, product_id FK, location_id FK, quantity, unit, confidence CHECK IN (...),
  best_before NULL, opened_at NULL, created_at, updated_at)`
- `inventory_event(id, kind CHECK IN ('PURCHASE','CONSUME','DISCARD','ADJUST','TRANSFER',
  'OPEN'), lot_id FK NULL, product_id FK NULL, from_location_id FK NULL,
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
  `manual_count` | `shopping_order` | `inferred_decay`, extensible) that is the evidence
  justifying whatever confidence the lot is set to as part of that same transaction — same
  treatment as `preference_observation`.
- Confidence transitions are a deterministic function of `(event kind, source)`, applied at
  write time, not silently recomputed by a background process:
  - `PURCHASE` with a known quantity (receipt, barcode-confirmed, or shopping-order-derived) →
    `EXACT`.
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
