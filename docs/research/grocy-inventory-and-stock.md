# Grocy research: inventory and stock (tasks.md 3.1–3.13)

Live reference instance: `lscr.io/linuxserver/grocy:latest`, Grocy **4.6.0** (`db_version` 255,
256 migration files), at `http://192.168.1.183` — see `grocy-deployment.md`. Investigated by:
direct SQLite access (`ssh proxmox "pct exec 2320 -- sqlite3 /config/data/grocy.db ..."`), the
REST API (an API key was inserted directly into the `api_keys` table via SQLite, sidestepping a
broken `/login` POST route on this LinuxServer build — `username`/`password` form fields, not
`user`/`password`; see `grocy-deployment.md`-adjacent notes below), and Grocy's PHP source read
directly from the running container (`/app/www/services/StockService.php`,
`/app/www/controllers/StockApiController.php`) and from GitHub
(`https://github.com/grocy/grocy`, `migrations/*.sql`).

## Representative test data actually created and exercised

Not a paper exercise — every workflow below was run against the live instance and the resulting
rows were read back from SQLite.

**Locations**: `Fridge` (id 2, Grocy default), `Pantry` (id 3), `Freezer` (id 4, `is_freezer=1`).

**Units**: `Piece`/`Pack` (Grocy defaults, ids 2/3), `Gram` (4), `Kilogram` (5), `Liter` (6).
Global conversion `Kilogram→Gram` factor 1000 (`product_id` NULL).

**Products**:

| Product | Purchase unit | Stock unit | Location | `default_best_before_days` | Barcode |
|---|---|---|---|---|---|
| Milk (id 1) | Liter | Liter (1:1) | Fridge | 7 | `7300156119012` |
| Rice (id 2) | Kilogram | Gram (global 1000× conversion) | Pantry | 0 | none |
| Eggs (id 3) | Pack | Piece (product-specific 6× conversion) | Fridge | 14 | `7300156119029` |
| Frozen Peas (id 4) | Piece | Piece (1:1) | Freezer | 180 | `7300156119036` |

**Workflows exercised end-to-end** (API call → `stock`/`stock_log` rows inspected via SQLite
after each): PURCHASE all four products; ADJUST (inventory correction) Eggs and Rice; CONSUME
Milk (partial); DISCARD Rice (`consume` + `spoiled:true`); TRANSFER Frozen Peas Freezer→Fridge;
OPEN Milk (partial, split); MARK_EMPTY-equivalent (consume-to-zero) Frozen Peas at Fridge;
barcode-based lookup and consume; UNDO a booking. Exact request/response bodies are quoted
per-section below.

One methodological finding earned its own callout because it shaped how every subsequent test
was interpreted: **the stock write API takes `amount` in the product's stock unit, never the
purchase unit** — see §3.8/§3.15. The first Rice and Eggs purchases below were deliberately
issued "wrong" (in purchase-unit terms) to surface this, then corrected via ADJUST, which
doubles as live evidence for §3.12.

---

## 3.1 Products

**User behavior.** A product is the single row every stock entry, barcode, recipe ingredient,
shopping list item, and chore/meal-plan reference hangs off. Creating one in the UI asks for a
name, a default location, a purchase unit, a stock unit, and (optionally) `min_stock_amount`,
expiry defaults, tare-weight handling, a "consume this as a substitute for its parent" wiring,
and around a dozen more behavioral flags.

**API behavior.** `POST /api/objects/products` (generic CRUD, `establish-household-and-catalog`
pattern-equivalent). Live example used to create Eggs:

```json
POST /api/objects/products
{"name":"Eggs","location_id":2,"qu_id_purchase":3,"qu_id_stock":2,"default_best_before_days":14}
→ {"created_object_id":"3"}
```

**DB mutation.** `products` (see full column list in `grocy-api-and-database.md`'s archaeology
section — it is **46 columns**, not a lean "name + unit + location" row). Four INSERT triggers
fire on every product creation: `enforce_parent_product_id_null_when_empty_INS`,
`enforce_min_stock_amount_for_cumulated_childs_INS`, `default_qu_id_consume` (defaults
`qu_id_consume` to `qu_id_stock` if unset), `default_qu_id_price` (defaults `qu_id_price` to
`qu_id_purchase`), and — critically — `products_default_qu_conversions_INS`, which
**silently inserts a 1:1 `quantity_unit_conversions` row** whenever `qu_id_stock !=
qu_id_purchase` (or `!= qu_id_consume`/`qu_id_price`) and no conversion exists yet (see §3.16).

**Source implementation.** `services/ProductsService.php` for validation; the real behavioral
weight lives in the triggers on the `products` table itself (`.schema products` on the live DB)
and in `migrations/0001.sql` → `migrations/0103.sql`/`0082.sql` for how the table grew.

**Tests.** None. The Grocy repository has **no `tests/` directory, no PHPUnit dependency in
`composer.json`, and no test-running CI workflow** (`.github/workflows/` does not exist — only
`CONTRIBUTING.md`, `FUNDING.yml`, issue/PR templates). Confirmed by listing the repo root and
`.github/` via the GitHub API. See §3.24 for the full implication.

**Strengths.** One product row is genuinely the hub of the whole system — every other table
(stock, barcodes, recipe positions, shopping list, meal plan, chores) points at `products.id`,
so there is exactly one place "what is this food, generically" lives. The `parent_product_id`
"sub-product" mechanism (one level deep only — enforced by
`enfore_product_nesting_level` trigger) gives a crude but real substitution/aggregation
primitive: a generic "Tomatoes" parent can roll up stock from "Cherry Tomatoes" and "Beef
Tomatoes" children via `stock_current`'s aggregation UNION.

**Weaknesses.** 46 columns on one table is a strong "this grew by accretion, never
refactored into aggregates" smell — tare-weight handling, freezer/thaw defaults, label-printer
behavior, calorie tracking, and shopping-list/consume-location defaults are all first-class
columns on `products` rather than separate concerns. There is no `Ingredient` distinct from
`Product` at all — Grocy's `products.name` is simultaneously "Milk" (generic) and "Arla
Mellanmjölk 1L" (branded SKU); a household using Grocy strictly must choose one level of
abstraction and lose the other.

**Spisordning lesson.** `establish-household-and-catalog`'s `Ingredient`-vs-`Product` split is
directly validated by this weakness — Grocy conflates exactly the two concepts PLAN.md's
"Ingredient Model" section calls "NON-NEGOTIABLE" to keep separate, and the result is a single
table doing double duty with no clean way to ask "what does a household generically buy" versus
"which specific SKU is on the shelf." The one-level `parent_product_id` substitution mechanism
is worth studying as prior art for `IngredientSubstitution`, but it is a poor model to copy
directly: it is unnamed/untiered (no `EQUIVALENT`/`GOOD`/`ACCEPTABLE` distinction), silently
activated by an `allow_subproduct_substitution` boolean flag on the *consuming* call rather than
being an explicit, inspectable relationship, and capped at one nesting level by a hard trigger
rather than a real design decision.

---

## 3.2 Barcodes

**User behavior.** Scan or type a GTIN/EAN in the purchase/consume/inventory-correction forms;
Grocy resolves it to a product (or offers to create one, optionally via Open Food Facts — see
§3.2's API section and `grocy-units-and-planning.md`'s external-lookup notes). A product can
have **multiple** barcodes (multipacks, regional variants, reprints).

**API behavior.**

```
GET  /api/stock/products/by-barcode/{barcode}          → full ProductDetails payload
POST /api/stock/products/by-barcode/{barcode}/add       → same body as .../products/{id}/add
POST /api/stock/products/by-barcode/{barcode}/consume
POST /api/stock/products/by-barcode/{barcode}/transfer
POST /api/stock/products/by-barcode/{barcode}/inventory
POST /api/stock/products/by-barcode/{barcode}/open
GET  /api/stock/barcodes/external-lookup/{barcode}      → invokes the configured plugin (Open Food Facts by default on this image)
```

Live-exercised:

```json
GET /api/stock/products/by-barcode/7300156119012
→ {"product":{"id":1,"name":"Milk",...},"product_barcodes":[{"barcode":"7300156119012",...}],"stock_amount":1.5,...}

POST /api/stock/products/by-barcode/0000000000000        (unregistered barcode)
→ 400 {"error_message":"No product with barcode 0000000000000 found"}
```

Every `by-barcode/*` endpoint is a thin wrapper: `GetProductIdFromBarcode($barcode)` then a
call into the same underlying method the numeric-id endpoint uses (`StockApiController.php`
lines ~163–175, ~296–309, ~528–539, ~582–605, ~856–879) — **there is no separate barcode-aware
business logic**, only ID resolution glued in front.

**DB mutation.** `product_barcodes(id, product_id, barcode, qu_id, amount, shopping_location_id,
last_price, note)`. `barcode` has a `UNIQUE` index (a barcode can map to exactly one product).
Two triggers: `default_qu_INS`/`_UPD` default `qu_id` to the product's stock unit if unset;
`prevent_adding_barcodes_for_not_existing_products` rejects orphan barcodes. The optional
`qu_id`/`amount` columns let a *specific* barcode declare "this barcode is a 6-pack" — a second,
barcode-scoped unit-conversion mechanism layered on top of `quantity_unit_conversions` (§3.16),
not unified with it.

**Source implementation.** `services/StockService::GetProductIdFromBarcode` (throws if not
found); `plugins/OpenFoodFactsBarcodeLookupPlugin.php` for external lookup (see below).

**Tests.** None (repo-wide, see §3.1).

**Strengths.** A real one-to-many `product_barcodes` table (not a single column on `products`)
is the *current* state — but see Database Archaeology in `grocy-api-and-database.md` for what it
replaced: migration `0001.sql` originally had a single `products.barcode TEXT` column that (per
migration `0103.sql`'s data-migration CTE) was **already being abused as a comma-separated list
of multiple barcodes** before the dedicated table existed. Barcode-to-product resolution is
strict and unambiguous (`UNIQUE` on `barcode`) — no fuzzy matching, no per-household barcode
scoping.

**Weaknesses.** Grocy's own default Open Food Facts plugin
(`plugins/OpenFoodFactsBarcodeLookupPlugin.php`, confirmed enabled in this image's
`/config/data/config.php`) requests **only `product_name` and `image_url`** from OFF's API
(`fields=product_name,image_url,product_name_<locale>`) — despite OFF exposing brand,
ingredients, allergens, nutrition, and categories, none of that is fetched. The plugin also only
*pre-fills a new-product form*; it never writes anything to stock or `product_barcodes`
automatically — a human still confirms. Barcode uniqueness is global, not per-household (a
single-tenant assumption throughout Grocy — see `grocy-api-and-database.md`).

**Spisordning lesson.** This is strong, concrete evidence for `implement-pantry-inventory`
design.md D6's resolution chain (`ProductIdentifier` lookup → Open Food Facts → retailer → manual
fallback): Grocy proves the pattern works operationally, but also proves that "integrate Open
Food Facts" is not a solved problem just because Grocy ships a plugin for it — Spisordning's own
OFF client should deliberately pull the richer field set (ingredients/allergens/nutrition/
categories) PLAN.md's "Open Food Facts" section asks for, since Grocy's reference implementation
conspicuously does not. The comma-separated-barcodes-in-one-column history is a textbook example
of exactly the anti-pattern `establish-household-and-catalog`'s `product_identifier` table (a
real one-to-many table from day one) already avoids — good confirmation, not a new lesson.

---

## 3.3 Locations

**User behavior.** Named physical places ("Pantry", "Fridge", "Freezer", "Garage freezer").
Each product has one *default* location; individual stock entries can live at a different
location (`stock.location_id`, nullable — falls back to the product's default via a trigger).
A location can be flagged `is_freezer` (drives automatic freeze/thaw expiry recalculation on
TRANSFER — see §3.7/§3.11).

**API behavior.** `GET/POST/PUT/DELETE /api/objects/locations`. Live-created:

```json
POST /api/objects/locations {"name":"Pantry"}          → {"created_object_id":"3"}
POST /api/objects/locations {"name":"Freezer","is_freezer":1} → {"created_object_id":"4"}
```

**DB mutation.** `locations(id, name UNIQUE, description, is_freezer, active)`. No household/
tenant scoping column at all — Grocy is single-household by design (see
`grocy-api-and-database.md`).

**Source implementation.** `services/LocationsService.php`; `is_freezer` consumed by
`StockService::TransferProduct` and `AddProduct` (see §3.11).

**Tests.** None.

**Strengths.** Deliberately minimal — a location is just a name plus a freezer flag, and every
other table (`stock`, `stock_log`, `products.location_id`,
`products.default_consume_location_id`) references it by real FK. `is_freezer` earning
first-class behavioral treatment (auto-adjusting best-before dates on transfer) is a genuinely
useful, non-obvious feature born from "years of accumulated edge cases."

**Weaknesses.** No hierarchy (a location can't be "Freezer > Drawer 2"); no household scoping —
this is fine for Grocy's single-household assumption but is a real gap if Spisordning ever needs
multi-household location namespacing beyond simple `household_id` scoping.

**Spisordning lesson.** `implement-pantry-inventory` design.md already scopes
`inventory_location` to `Household` (correct — Grocy has no equivalent because it doesn't need
one). The `is_freezer` flag and its cascading best-before-date effects are the single most
concrete, non-obvious "years of accumulated edge cases" finding from the whole Locations/
Transfer/Expiry cluster and are worth deliberately reproducing: freezing something should be
able to extend its confidence/expiry window, and thawing it should be able to shrink it, driven
by *where* a TRANSFER event moves a lot to, not by a separate manual edit.

---

## 3.4 Stock

**User behavior.** "Stock" is what the household believes it currently has: a stock overview
screen lists one row per product (aggregated across all locations and lots), with drill-down to
individual lots/locations.

**API behavior.** `GET /api/stock` → `StockService::GetCurrentStock()`, backed by the `stock`
table directly (not a per-product aggregate query at read time beyond what SQL views compute).
`GET /api/stock/products/{id}` → `GetProductDetails`, which layers in average/current price,
next-due-date, opened-amount, and unit-conversion metadata around the raw stock rows.

**DB mutation.** The `stock` table itself: **one row per physical lot** (see §3.6), holding
`product_id, amount, best_before_date, purchased_date, stock_id (a UUID-ish text key), price,
open, opened_date, location_id, shopping_location_id, note`. This is the mutable **current-state
projection** — rows are inserted on purchase, updated in place on partial consume/transfer/open,
and **deleted outright** when a lot's amount reaches zero (confirmed live: after consuming the
Frozen Peas lot at Fridge to zero, `SELECT * FROM stock WHERE stock_id='6a81d8928f2ee'` returns
only the still-nonzero Freezer-location row — the Fridge-location row is gone entirely from
`stock`, though it remains fully reconstructable from `stock_log`, see §3.5).

**Source implementation.** `.schema stock` on the live DB; every mutating method in
`StockService.php` (`AddProduct`, `ConsumeProduct`, `TransferProduct`, `InventoryProduct`,
`OpenProduct`, `EditStockEntry`) writes to both `stock` and `stock_log` in the same PHP request
(SQLite has no explicit multi-statement transaction wrapping visible in this code path beyond
SQLite's own autocommit-per-statement semantics — a real robustness gap, see Weaknesses).

**Tests.** None.

**Strengths.** The "current state is a real table, not just a computed view" design is fast to
query and simple to reason about for the 99% case (stock overview, "is this in stock").
FEFO/FIFO consumption ordering (`stock_next_use` view, §3.9) operates directly and efficiently
over this table with a single indexed `ORDER BY`.

**Weaknesses. This is the load-bearing finding for `implement-pantry-inventory`'s D2 decision.**
`stock` is **not** an append-only or even a stable-identity table: a lot's row is deleted the
moment its amount hits zero, and a "split" (partial consume, partial open, partial transfer)
creates an entirely new `stock_id`/row rather than preserving the original lot's identity across
the split. Current-state truth genuinely lives only in this mutable table — `stock_log` is a
parallel record of *deltas*, not a source you replay to reconstruct `stock` in the general case
without also knowing which `stock-edit-old`/`stock-edit-new` pairs represent manual corrections
(see §3.5). There is no visible transaction wrapper around the `stock` + `stock_log` dual write
in `StockService.php` — a crash between the two writes would desync them, and nothing in the
schema (no FK from `stock.stock_id` back to a `stock_log` "opening" row) would catch it.

**Spisordning lesson.** This is strong, direct validation of `implement-pantry-inventory`
design.md's D2 ("ledger-plus-projection, not pure current-state and not pure event-sourcing")
over option (a) (pure current-state, no ledger) — Grocy's `stock` table alone is provably
insufficient for "why is the milk gone" (PLAN.md's own test case), because completed lots
disappear from it entirely. But it is a caution against D2's specific implementation detail: the
design states every command is "insert one `inventory_event` row, then insert-or-update the
corresponding `inventory_lot` row, in one database transaction" — Grocy shows what happens when
that transactional discipline is *not* enforced at the storage layer (PHP-level sequential
writes, no visible `BEGIN`/`COMMIT` wrapping in `StockService.php`), and the fact that Grocy's
`stock` table can silently drift from `stock_log` (no reconciliation job, no FK enforcing
lot-existed-before-log-entry) is exactly the failure mode D2's "the lot can always be rebuilt
from event history... as a recovery/consistency-check path" fallback is designed to catch. Build
that consistency-check path for real, since Grocy's own history suggests it will eventually be
needed.

---

## 3.5 Stock journal

**User behavior.** "Stock journal" (`/stockjournal` in the UI) is the append-style transaction
log — every purchase, consume, transfer-leg, open, and correction, filterable/sortable, each
entry showing a human-readable transaction type and (for consume) whether it was spoiled.

**API behavior.** No single `/api/stock/journal` endpoint exists in the OpenAPI spec (73 paths
fetched from the live instance's `/api/openapi/specification`); the UI's journal view queries
`uihelper_stock_journal`/`uihelper_stock_journal_summary` views directly through the generic
`/api/objects/{entity}` mechanism, and individual transactions are fetchable via
`GET /api/stock/transactions/{transactionId}` and `GET /api/stock/bookings/{bookingId}`.
`POST /api/stock/bookings/{bookingId}/undo` and `POST /api/stock/transactions/{transactionId}/undo`
undo a single booking or an entire multi-row transaction (e.g. a whole TRANSFER, both legs).

**DB mutation.** `stock_log(id, product_id, amount, best_before_date, purchased_date, used_date,
spoiled, stock_id, transaction_type, price, undone, undone_timestamp, opened_date, location_id,
recipe_id, correlation_id, transaction_id, stock_row_id, shopping_location_id, user_id, note)`.
`amount` is signed (positive for purchase/inventory-up/transfer-in, negative for
consume/transfer-out). `transaction_id` groups all rows written by one API call (e.g. a single
purchase, even the multi-row `stockLabelType==2` per-unit case); `correlation_id` additionally
links the two legs of a TRANSFER. **Live-exercised UNDO**:

```
POST /api/stock/bookings/13/undo         (undoes the Eggs "-2" consume booking)
```

resulted in `stock_log.id=13` getting `undone=1, undone_timestamp='2026-08-16 15:36:44'` — **the
original row is mutated in place**, not superseded by a new compensating row — while `stock`
correctly regained a row for the Eggs lot (amount restored). No new `stock_log` row was written
to represent the undo itself as its own event.

**Source implementation.** `StockService::UndoBooking`/`UndoTransaction` (lines ~1484, ~1667).
`products_average_price` (a view feeding `cache__products_average_price`) explicitly filters
`WHERE sl.undone = 0`, confirming undone rows are meant to be excluded from computed aggregates
but are never deleted.

**Tests.** None.

**Strengths.** `transaction_id`/`correlation_id` grouping is a clean, cheap way to answer "what
else happened as part of this same action" without a separate transactions table — useful for
undo-the-whole-thing and for UI grouping. Keeping undone rows (rather than deleting them) does
preserve *some* audit trail — an undo is visible, not silently erased.

**Weaknesses. Second load-bearing finding for `implement-pantry-inventory`'s invariants.** The
undo mechanism directly **mutates a historical `stock_log` row** (`undone`, `undone_timestamp`).
This contradicts a pure "events are immutable once written, corrections are new events" ledger
discipline — Grocy's own real, years-battle-tested design chose the cheaper "flag and exclude"
approach over "write a compensating event" for its undo feature specifically, presumably because
undo is meant to represent "that never should have counted," not "a new fact was learned" (which
*is* what `ADJUST` is for, and `ADJUST` correctly stays additive — see §3.12).

**Spisordning lesson.** This directly touches `implement-pantry-inventory` design.md's invariant
2 ("`InventoryEvent` rows are immutable once written... corrections are new events, never updates
or deletes of existing ones"). Grocy's own behavior is evidence *for* keeping that invariant
strict rather than following Grocy's shortcut: an "undo" feature that mutates history is exactly
the kind of accumulated convenience-over-correctness compromise PLAN.md's Testing section warns
against preserving ("Do not preserve reference-system bugs merely because they exist"). If/when
Spisordning wants an undo capability, it should be modeled as design.md's own D3 pattern already
prescribes for decay — a new event (e.g. `ADJUST` with `source: 'correction'`, or a dedicated
`kind` if the semantics genuinely differ) that references the event it corrects, never an
`UPDATE` on the original `inventory_event` row. This is one clear, well-evidenced place where
Grocy's real design should be **improved on, not copied**.

---

## 3.6 Lots

**User behavior.** Grocy calls a lot a "stock entry" — a specific purchase batch: quantity, one
best-before date, one purchase price, one location, opened/sealed state. Buying milk twice in a
week with different expiry dates produces two lots, consumed in a defined priority order.

**API behavior.** `GET /api/stock/products/{id}/entries` → `ProductStockEntries`, one row per
`stock` row for that product. `PUT /api/stock/entry/{entryId}` → `EditStockEntry`, a **direct
edit** of a lot's amount/best-before/location/price/open-state outside the
purchase/consume/transfer/inventory verb set — the one true "just fix this row" escape hatch.

**DB mutation.** Editing a lot via `EditStockEntry` writes **two** `stock_log` rows —
`stock-edit-old` (the pre-edit values) and `stock-edit-new` (the post-edit values) — *in
addition to* directly `UPDATE`-ing the `stock` row. `stock_edited_entries` (a helper table/view)
tracks which `stock_id` has been edited and which `stock-edit-new` row is the newest, so
`products_average_price` can correctly use the corrected values instead of the stale originals.

**Source implementation.** `StockService::EditStockEntry` (line 526); `stock_next_use` view
(quoted in full below) is the FEFO/FIFO lot-selection algorithm, straight from its own inline
comment:

```sql
-- The default consume rule is:
-- Opened first, then first due first, then first in first out
-- Apart from that products at their default consume location should be consumed first
-- ORDER BY priority DESC, open DESC, best_before_date ASC, purchased_date ASC
```

i.e. **default-consume-location match, then already-opened, then soonest-expiring
(FEFO), then oldest-purchased (FIFO) as the final tiebreak** — a four-level, explicitly ordered
priority, not a single FIFO/FEFO rule.

**Tests.** None.

**Strengths.** The lot-selection priority order is genuinely sophisticated and well-motivated —
"consume opened things before sealed things" and "prefer the default consume location" are both
real, non-obvious rules that only show up after actual household use, exactly the kind of edge
case PLAN.md asks to mine Grocy for. The dual-write `stock-edit-old`/`stock-edit-new` pattern for
direct edits is a clean way to keep manual corrections auditable without a separate "corrections"
table.

**Weaknesses.** A lot has no stable identity across a partial split — consuming half a lot
updates the original row in place (same `stock_id`), but *opening* half a lot creates a **new**
`stock_id` for the still-sealed remainder while the original `stock_id` becomes the opened
portion (confirmed live: opening 1 of 1.5 L of Milk left `stock_id 6a81d892...` at amount=1,
open=1, and created a fresh `stock_id 6a81d8c5...` at amount=0.5, open=0 — the *opposite* of the
naive assumption that the original identity would stay with the untouched remainder). This
asymmetry (consume-split keeps identity, open-split does not) is undocumented behavior a caller
has to discover by reading source or testing live, exactly as this research did.

**Spisordning lesson.** `InventoryLot` should have a genuinely stable identity through partial
consumption *and* partial opening — Grocy's inconsistency here (identity survives one kind of
split but not the other) is a real, discoverable footgun and a concrete "do differently" item:
Spisordning's `RecordOpen`/`RecordConsume`/`RecordTransfer` commands (design.md Step 5) should
define, up front and symmetrically, which side of a split keeps the original lot id — plausibly
neither, if `InventoryLot.id` is meant to be stable and splits instead spawn genuinely new lot
rows referencing a shared "origin" lineage. FEFO/FIFO-with-priority-tiers (default location →
opened → expiry → purchase-date) is worth adopting close to verbatim as
`implement-recipe-availability`/future consume-suggestion logic's lot-selection order.

---

## 3.7 Expiry

**User behavior.** Every lot can carry a `best_before_date`. Products have a
`due_type` (1 = "best before," still meaningfully edible after; 2 = "expiration date," meant to
be discarded after) and several *default* expiry-window fields:
`default_best_before_days` (purchase → first due date), `default_best_before_days_after_open`,
`default_best_before_days_after_freezing`, `default_best_before_days_after_thawing`.

**API behavior.** `best_before_date` is accepted (optionally) on `/add`, `/inventory`, and the
`by-barcode` equivalents; if omitted, the server computes it from the defaults (see DB mutation).
`GET /api/stock/volatile?due_soon_days=N` returns `due_products`/`overdue_products`/
`expired_products`/`missing_products` in one call — powering the dashboard's "use this soon"
list.

**DB mutation / edge cases actually observed live.** `AddProduct`'s default-date logic
(`StockService.php` lines 145–180) is a real, if surprising, priority chain:

1. If the destination location `is_freezer` and `default_best_before_days_after_freezing >= -1`,
   use the freezing default (`-1` is a sentinel for "never expires," rendered as the literal date
   `2999-12-31`).
2. Else if `default_best_before_days == -1`, also `2999-12-31` (never expires).
3. Else if `default_best_before_days > 0`, `today + N days`.
4. **Else (the `0` case), `best_before_date = today`.**

Live-confirmed: Rice was created with `default_best_before_days=0` (intended, in this test, to
mean "no expiry tracking"). Purchasing it with no explicit `best_before_date` produced
`best_before_date = '2026-08-16'` (the purchase date itself) — and it *immediately* appeared in
`GET /api/stock/volatile`'s `due_products` list, because "due today" reads as already at/past its
best-before window. **`0` is not "untracked," it is "expires today."** The only way to actually
disable expiry tracking for a product is `default_best_before_days = -1`.

**Source implementation.** `products_volatile_status` view computes `current_due_status` from
`JULIANDAY(best_before_date) - JULIANDAY('now')` thresholded against a per-user
`stock_due_soon_days` setting, mapped to `ok`/`due_soon`/`overdue`/`expired` (the last two
distinguished by `due_type`, so a "best before" product past its date reads `overdue` while an
"expiration date" product reads the harsher `expired`).

**Tests.** None.

**Strengths.** Distinguishing `due_type` (best-before vs. hard-expiration) is a real semantic
Spisordning should keep — "expired yogurt is often still fine, expired raw chicken is not," and
conflating the two into one boolean would lose that. Freeze/thaw-aware default recalculation
(§3.11) is genuinely clever, born from real freezer use.

**Weaknesses.** The `0`-means-"expires today" default is an unmarked footgun, not a documented
design choice — nothing in the product-edit UI or API response warns that `0` differs
semantically from "untracked." A household product deliberately meant to have no expiry concept
(salt, dried pasta with no printed date the household cares about) must know to set `-1`
specifically, an easy mistake to make (this research made it, in exactly this way, before
reading the source).

**Spisordning lesson.** `implement-pantry-inventory` should make "this product/lot has no
tracked expiry" a genuinely distinct, explicit state — never a `0`/sentinel day-count that
silently degrades into "expires immediately." Given the confidence model already in design.md
(`EXACT`/`LIKELY`/`ESTIMATED`/`UNKNOWN`), a lot with no known best-before date is better
represented as `best_before_date IS NULL` with no special-casing required downstream, rather than
inventing a numeric sentinel the way Grocy's `default_best_before_days` overloads `0` and `-1`.
The `due_type` (best-before vs. expiration) distinction and the freeze/thaw recalculation
trigger-point (on `TRANSFER` into/out of an `is_freezer` location) are both worth adopting.

---

## 3.8 Purchase

**User behavior.** Record a quantity of a product entering stock: amount, unit (purchase unit in
the UI form), best-before date, price, location. The single most common inventory action.

**API behavior.**

```
POST /api/stock/products/{productId}/add
{"amount": <number, in the product's STOCK unit — see below>,
 "best_before_date": "YYYY-MM-DD" (optional),
 "purchased_date": "YYYY-MM-DD" (optional, defaults today),
 "price": <number> (optional, total or per-stock-unit depending on price_type),
 "location_id": <int> (optional, defaults to product's default location),
 "shopping_location_id": <int> (optional),
 "transaction_type": "purchase" | "inventory-correction" | "self-production" (optional, defaults purchase),
 "note": <string> (optional)}
→ 200, array of created stock_log rows (usually length 1; length N for stockLabelType=2, one row per physical unit)
```

**This is the finding that shaped the rest of the test-data exercise**: `amount` in this
endpoint is *always* in the product's **stock unit** (`qu_id_stock`), never the purchase unit
(`qu_id_purchase`), and the server performs **no conversion whatsoever** —
confirmed by reading `AddProduct` end to end (`StockService.php` lines 112–305): `$amount` is
written to `stock`/`stock_log` completely unmodified from the request body. Live-reproduced by
accident and then deliberately: `POST .../products/3/add {"amount":1}` for Eggs (purchase unit
Pack, stock unit Piece, 6:1 conversion) recorded **1 Piece**, not 6 — the caller (normally
Grocy's own purchase-form JS, which multiplies the entered purchase-unit quantity by the
resolved conversion factor client-side before calling this endpoint) is entirely responsible for
converting. A REST client that reads "amount" and assumes it means "amount in whatever unit I'm
submitting" will silently record the wrong stock level. This was corrected in testing via an
ADJUST (see §3.12), which is the realistic recovery path a real user would also take.

**DB mutation.** New `stock` row (or, if `stockLabelType==2`, N rows of amount 1 each — "one
label per physical unit"); one (or N) `stock_log` row(s), `transaction_type='purchase'`,
positive `amount`. `CompactStockEntries($productId)` runs afterward, merging any now-identical
lots (same best-before date, location, price, open-state) into one row — Grocy actively avoids
lot-row proliferation from repeated same-day purchases of the same product/date/price.

**Source implementation.** `StockService::AddProduct` (line 112); default-best-before-date logic
detailed in §3.7; `CompactStockEntries` (line 1729).

**Tests.** None.

**Strengths.** `CompactStockEntries` merging same-shaped lots is a thoughtful anti-clutter
measure most naive event-sourced designs would skip and later regret (an event ledger with one
row per purchase call is fine; a *lot table* with dozens of functionally-identical rows for the
same milk carton bought Tuesday and re-scanned Wednesday is not). `stockLabelType==2`
(label-per-unit, e.g. for physically labeling six individually-trackable cheese wheels) is a
real, non-obvious granularity option.

**Weaknesses.** The purchase-unit-vs-stock-unit API contract is genuinely under-documented — the
OpenAPI spec's field description for `amount` does not call this out, and it is only discoverable
by reading `StockService.php` or (as here) getting it wrong against live data.

**Spisordning lesson.** `RecordPurchase(productId, locationId, quantity, unit, bestBefore?,
source)` (design.md Step 5) already takes an explicit `unit` parameter rather than assuming
stock-unit — this is the right call and Grocy's ambiguity is a direct cautionary example for why:
the API layer, not just the domain layer, must make unit-of-the-quantity-being-passed
unambiguous, ideally by requiring the caller to always state the unit explicitly (never
positionally implied) and having the application layer perform the conversion server-side rather
than trusting the client to have already done it. `CompactStockEntries`'s lot-merging behavior is
worth deliberately deciding for/against — it trades a smaller `inventory_lot` table for losing
the ability to distinguish "bought twice, same day" from "bought once," which may or may not
matter for Spisordning's audit goals.

---

## 3.9 Consume

**User behavior.** Record a quantity leaving stock because it was used/eaten. The everyday
"I used 200g of rice cooking dinner" action.

**API behavior.**

```
POST /api/stock/products/{productId}/consume
{"amount": <number, stock unit>, "spoiled": <bool, default false>,
 "location_id": <int, optional — restrict to lots at this location>,
 "stock_entry_id": <string, optional — target one specific lot instead of FEFO/FIFO order>,
 "recipe_id": <int, optional — attributes this consume to a recipe>,
 "exact_amount": <bool>, "allow_subproduct_substitution": <bool>}
```

Live-exercised: `{"amount":0.5,"spoiled":false}` against Milk (stock 1.5 L available) →
consumed from the single existing lot, updated in place to 1.0 L remaining, one `stock_log` row
`amount=-0.5, transaction_type='consume', spoiled=0`.

**DB mutation.** Walks `stock_next_use` (the FEFO/FIFO-with-priority view, §3.6) lot by lot,
either updating a lot's `amount` down (partial) or `DELETE`-ing it outright (fully consumed),
writing one negative-`amount` `stock_log` row per touched lot. **`spoiled` is a boolean column
on the `stock_log` row, not a distinct transaction type** — see §3.10 immediately below.

**Source implementation.** `StockService::ConsumeProduct` (line 365) — see the FIFO/FEFO walk
loop, lines 430–511.

**Tests.** None.

**Strengths.** Multi-lot consumption (an amount spanning more than one lot) is handled
transparently and correctly follows the priority order — a caller never has to know or care how
many physical lots satisfied the request. `stock_entry_id` lets a caller (e.g. a barcode scan of
a *specific* labeled unit, via Grocycode — see `grocy-api-and-database.md`) bypass FEFO/FIFO and
target one exact lot, useful when the physical item scanned is known precisely.

**Weaknesses.** `spoiled` and "empty" are not distinct verbs at the API level at all (§3.10/
§3.13) — everything routes through `ConsumeProduct`.

**Spisordning lesson.** design.md's `RecordConsume(lotId, quantity, source)` targets a specific
lot directly rather than doing FEFO resolution inside the command itself — this is a deliberate
and reasonable divergence from Grocy (Grocy's FEFO/FIFO walk happens *inside* `ConsumeProduct`,
conflating "which lot" with "how much"); keeping lot selection as a separate read-then-command
step (or a distinct `SuggestConsumeLot` query) keeps `RecordConsume` simpler and testable in
isolation, which is worth preserving. The lot-priority order itself (§3.6) is exactly what should
drive that suggestion.

---

## 3.10 Discard

**User behavior.** In the Grocy UI, "consume" and "consume as spoiled/discard" are two buttons
on the same form — but they are the **same underlying action** with a checkbox.

**API behavior.** There is **no dedicated discard endpoint**. `POST
/api/stock/products/{id}/consume` with `{"spoiled": true}` is the entirety of Grocy's DISCARD.
Live-exercised: `POST /api/stock/products/2/consume {"amount":200,"spoiled":true}` against Rice
(stock 1000g after an ADJUST, see §3.12) → `stock_log` row `amount=-200,
transaction_type='consume', spoiled=1`.

**DB mutation.** Identical code path and identical `transaction_type='consume'` to a normal
consume (§3.9) — the *only* database difference is `stock_log.spoiled = 1` on the resulting
row(s). No separate transaction-type constant exists for it (confirmed:
`StockService.php`'s `const TRANSACTION_TYPE_*` list has no `DISCARD`/`SPOILED` entry — only
`CONSUME`, `INVENTORY_CORRECTION`, `PRODUCT_OPENED`, `PURCHASE`, `SELF_PRODUCTION`,
`STOCK_EDIT_NEW`, `STOCK_EDIT_OLD`, `TRANSFER_FROM`, `TRANSFER_TO`).

**Source implementation.** Same as §3.9 — `ConsumeProduct`'s `$spoiled` parameter flows straight
into the `stock_log` row's `spoiled` column, with zero other behavioral branching in
`StockService.php`. (The `stock_missing_products`/shopping-list "add expired products" reports
do filter/report on `spoiled=1` separately, so the flag is not inert — it's just not a distinct
*transaction*.)

**Tests.** None.

**Strengths.** Minimal surface area — one endpoint handles both "I ate it" and "I threw it out,"
which is arguably the right level of abstraction if the only thing downstream code needs to know
is "it left stock" plus "was it waste." Waste reporting (a real Grocy dashboard feature) reads
directly off the `spoiled` flag with no extra joins needed.

**Weaknesses.** Collapsing DISCARD into CONSUME-with-a-flag means anything that wants to treat
"waste" as a first-class reportable *reason* (not just a boolean) — e.g. `reason: 'expired' |
'spoiled' | 'other'`, which `implement-pantry-inventory` design.md's `RecordDiscard` command
explicitly wants — has no natural home in Grocy's schema; `spoiled` is binary, with no room for
"discarded because I bought too much" versus "discarded because it went bad."

**Spisordning lesson.** This is a genuine, evidence-backed disagreement point for the "should
MARK_EMPTY/DISCARD be a distinct top-level event kind" question — see the combined discussion at
the end of §3.13, since both findings point the same direction. In short: Grocy's own real
design treats DISCARD as CONSUME-plus-a-boolean-flag, not as a separate first-class transaction
type, which is evidence *against* `PLAN.md`'s and design.md's assumption that DISCARD needs to be
its own `inventory_event.kind` value distinct from CONSUME — but Spisordning's richer `reason`
requirement (expired/spoiled/other, not just a boolean) is a legitimate reason to still keep it
distinct, since a boolean genuinely can't carry that. Recommendation: keep `DISCARD` as its own
`kind` (richer `reason` than Grocy supports), but treat this as a deliberate improvement over
Grocy's minimalism, not an assumption validated by it.

---

## 3.11 Transfer

**User behavior.** Move (all or part of) a lot from one location to another — freezer to fridge
to thaw, pantry to counter, etc.

**API behavior.**

```
POST /api/stock/products/{productId}/transfer
{"amount": <number>, "location_id_from": <int>, "location_id_to": <int>,
 "stock_entry_id": <string, optional>}
```

Live-exercised: `{"amount":1,"location_id_from":4,"location_id_to":2}` (Frozen Peas,
Freezer→Fridge) → **two** `stock_log` rows sharing one `correlation_id`:
`transaction_type='transfer_from'` (amount `-1`, `location_id=4`) and
`transaction_type='transfer_to'` (amount `+1`, `location_id=2`), same `stock_id` preserved
across both (unlike the OPEN split in §3.6, TRANSFER keeps lot identity intact across the move).

**DB mutation.** Walks matched lots at the source location (again via the lot-priority view,
scoped to `location_id_from`); for each, writes the `transfer_from`/`transfer_to` row pair and
either updates the lot's `location_id` in place (full-lot transfer) or splits it (partial
transfer, new `stock_id` for the moved portion — the *same* split-creates-new-identity behavior
as OPEN, but this time the *moved* portion gets the new id, not the *remaining* portion, another
asymmetry worth noting). **Freeze/thaw side effect**: if `GROCY_FEATURE_FLAG_STOCK_PRODUCT_FREEZING`
is enabled and the transfer crosses an `is_freezer` boundary, `best_before_date` is recalculated
using `default_best_before_days_after_freezing`/`_after_thawing` — the *same* logic path as the
purchase-time freezer default (§3.7), but triggered by a location change on an *existing* lot,
not just at creation time.

**Source implementation.** `StockService::TransferProduct` (line 1265); freeze/thaw block lines
~1325–1348.

**Tests.** None.

**Strengths.** Freeze/thaw-aware date recalculation triggered specifically by crossing an
`is_freezer` location boundary (not by any manual date edit) is the single most "years of
accumulated real household use" feature in the whole Grocy stock module — genuinely worth
studying closely.

**Weaknesses.** Tare-weight-handling products are **hard-blocked** from transfer entirely
("Transferring tare weight enabled products is not yet possible" — a real, acknowledged gap in
Grocy's own source, not a documentation omission). Partial-transfer identity handling is yet
another distinct split-identity rule (see §3.6), making three different split behaviors across
CONSUME/OPEN/TRANSFER with no unifying principle documented anywhere.

**Spisordning lesson.** `RecordTransfer(lotId, fromLocationId, toLocationId, quantity, source)`
in design.md is lot-scoped (a single `lotId`) rather than product+location-scoped the way
Grocy's endpoint is — this sidesteps Grocy's FEFO-selection-inside-transfer ambiguity entirely,
which is good. The freezer/thaw-triggered best-before recalculation is genuinely worth adopting:
model it as `TRANSFER`'s handler being permitted to also emit a best-before update as part of the
same event/transaction when the destination location's freezer flag differs from the source's,
which fits cleanly inside design.md's existing "every command writes one event + updates the lot
projection in the same transaction" shape (D2) without needing a new event kind.

---

## 3.12 Adjust

**User behavior.** "Inventory correction" — the household physically counts what's on the shelf
and tells Grocy the true current amount, overriding whatever the ledger currently believes.

**API behavior.**

```
POST /api/stock/products/{productId}/inventory
{"new_amount": <number, stock unit, ABSOLUTE not delta>,
 "best_before_date", "location_id", "price", "shopping_location_id", "purchased_date", "note" (all optional)}
```

Live-exercised twice, both to fix earlier unit-conversion test mistakes (§3.8) — a realistic
scenario, since "I miscounted/mis-scanned and need to correct the number" is exactly what ADJUST
is for in real use:

```json
POST /api/stock/products/3/inventory {"new_amount":6,"best_before_date":"2026-08-30"}
→ transaction_type='inventory-correction', amount=+5 (6 - the existing 1)

POST /api/stock/products/2/inventory {"new_amount":1000}
→ transaction_type='inventory-correction', amount=+999 (1000 - the existing 1)
```

**DB mutation / crucial source finding**: `InventoryProduct` (`StockService.php` line 915) is
**not its own write path** — it computes the delta between `new_amount` and current stock, then
calls straight into `AddProduct(..., self::TRANSACTION_TYPE_INVENTORY_CORRECTION, ...)` if the
delta is positive, or `ConsumeProduct(..., self::TRANSACTION_TYPE_INVENTORY_CORRECTION)` if
negative. **If `new_amount` exactly equals the current stock amount, Grocy throws** ("The new
amount cannot equal the current stock amount") — a genuine no-op correction is rejected outright,
not silently accepted.

**Source implementation.** `StockService::InventoryProduct`, lines 915–977 (the whole method is
short — it is entirely a thin dispatcher over `AddProduct`/`ConsumeProduct`).

**Tests.** None.

**Strengths.** Absolute-target semantics (`new_amount`, not `delta`) match how a human actually
performs a physical count — "I counted 6" is what happened, not "I'm adding 5." Reusing
`AddProduct`/`ConsumeProduct` internally means ADJUST automatically inherits FEFO/FIFO-aware
lot selection on the decrease side and default-best-before-date computation on the increase
side, for free — a real economy-of-mechanism strength, not just laziness.

**Weaknesses.** Because ADJUST literally *is* a PURCHASE or a CONSUME under a different
`transaction_type` label, it inherits every purchase/consume edge case (including the amount-is-
stock-unit ambiguity of §3.8) with no additional safeguards specific to "this is a correction, be
extra careful." Rejecting exact-equal `new_amount` as an error (rather than a harmless no-op) is
a minor but real UX rough edge — a user re-submitting the same count (e.g. a double-tap) gets an
error instead of silence.

**Spisordning lesson.** `RecordAdjust(lotId, newQuantity, reason, source)` in design.md is
already lot-scoped with absolute target semantics, matching Grocy's `new_amount` approach — good
convergent validation. Grocy's implementation-as-thin-dispatcher-over-Add/Consume is worth
noting as a *possible* simplification for Spisordning's own `RecordAdjust` handler (compute the
delta, delegate to the same write path RecordPurchase/RecordConsume use, single source of truth
for "how does the lot projection actually change"), provided design.md's D3 confidence-transition
table is still applied at the `ADJUST` call site specifically (Grocy has no confidence concept to
get this wrong, so there's nothing to directly validate or contradict there — see §3.6's parent
document's confidence discussion in `grocy-api-and-database.md`'s summary).

---

## 3.13 Mark empty

**User behavior.** There is no "mark empty" button distinct from a full consume in the Grocy UI.
The stock-overview product card's "consume all" quick-action (confirmed in
`public/viewjs/stockoverview.js` line 338: `.attr('data-consume-amount', result.stock_amount)`)
simply pre-fills the consume form's amount with the product's *entire current stock amount* and
submits the same `/consume` call as any partial consume.

**API behavior.** None distinct from §3.9 — `POST /api/stock/products/{id}/consume` with
`amount` set to the full current stock amount. Live-exercised: after transferring 1 Frozen Peas
unit to the Fridge (§3.11), `POST /api/stock/products/4/consume {"amount":1,"location_id":2}`
(the entire amount present at that location) → the matched `stock` row deleted outright (§3.4).

**DB mutation.** Identical to §3.9 — a normal `consume` transaction that happens to zero out a
lot. No distinct flag, column, or transaction type marks "this consume happened to be the last
of it" — that fact is only inferable after the fact (the lot row is simply absent from `stock`
afterward).

**Source implementation.** `public/viewjs/stockoverview.js` (client-side amount pre-fill only);
no server-side counterpart exists.

**Tests.** None.

**Strengths.** Zero added complexity — "empty" is a natural, derived *consequence* of consuming
the last of something, not a state that needs its own machinery to reach or represent.

**Weaknesses.** There is no way to query "which lots were marked empty vs. partially consumed and
still technically has a trace amount" after the fact from `stock_log` alone without recomputing
running balances — "this reached zero" is not a recorded fact, only an inferable one.

**Spisordning lesson — combined with §3.10's finding, this is the single most important
"reconsider" signal from the whole Grocy investigation.** `PLAN.md`'s "Inventory Events" section
and `implement-pantry-inventory` design.md both list `MARK_EMPTY` as a distinct event kind
alongside `PURCHASE`/`CONSUME`/`DISCARD`/`ADJUST`/`TRANSFER`/`OPEN`. Grocy's real, years-
battle-tested design has **no such distinct kind for either DISCARD or MARK_EMPTY** — both are
`CONSUME` with, respectively, a boolean flag and a client-computed "consume the full amount"
convenience. This does not automatically mean Spisordning's richer vocabulary is wrong — Grocy's
minimalism costs it exactly the two things just identified (no discard *reason* beyond a
boolean; no recorded "this hit zero" fact) — but it is real, first-party evidence that at least
one of the two (`MARK_EMPTY`) is very plausibly **UI/application-layer sugar over `RecordConsume`
with the full lot quantity**, not a materially different domain event, since Grocy's own
`RecordConsume`-equivalent already fully determines "did this reach zero" from the resulting
`quantity_delta` with no extra information needed. `DISCARD` has a better case for staying
distinct (Spisordning's structured `reason` enum is a real capability gap in Grocy's boolean that
composes cleanly as its own event kind). Recommendation for the eventual OpenSpec update: keep
`DISCARD` as a first-class `inventory_event.kind` (Grocy's boolean-flag minimalism is a limitation
to improve on, not copy), but revisit whether `MARK_EMPTY` needs to be its own `kind` versus being
implemented as `RecordConsume` called with the lot's full remaining quantity (still logged as a
`CONSUME` event, with "did this zero out the lot" derivable from the resulting projection state
rather than the event's `kind` itself) — Grocy's real design supports the latter.
