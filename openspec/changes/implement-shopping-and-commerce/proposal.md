# Implement shopping and commerce

## Why

`migrations/0001_init.sql` already defines `shopping_requirement` — the planner's canonical,
retailer-independent output (`{ ingredientId, quantity, unit, acceptableForms[], preferredForm }`,
scoped to one `meal_plan`). The `retailer-adapter` capability (merged, live-verified, code in the
sibling `~/dev/willys/willys-client` repo) resolves those requirements to concrete Willys
products and creates a durable per-week **wishlist** in Willys's own system
(`POST /shopping-lists`), with a separate opt-in `POST /shopping-lists/:id/to-cart` step. Nothing
in between is durable in spisordning's own schema: there is no `shopping_lists` table a person can
edit outside of a single plan run, no record of retailer-list sync state, no `shopping_carts`, and
no `orders`/`order_items` — `docs/research/current-state.md` confirms "nothing writes to
[the schema] yet" beyond what `food-brain plan` produces per run, and `shopping_requirement` is the
only shopping-adjacent table that exists.

`PLAN.md`'s "Local Shopping Intent", "Retailer Lists", "Carts", and "Orders" sections ask for
exactly this missing layer: a canonical shopping list a household can manage independent of any
single plan or retailer, a record of how that list projects onto a retailer's own list/wishlist
(and how conflicts in that projection are handled), an explicit, non-conflated cart stage, and a
persisted order record preserving what was actually bought. `docs/research/willys-capabilities.md`
additionally confirms the Willys client has **no real order/purchase-history API** — only an
untyped stub (`{ order: any; digitalReceipt: any }`) — which means `PLAN.md`'s "Receipts" section
(explicitly not prioritized in general) matters *more* for Willys specifically than `PLAN.md`
assumes, since it may be the only practical way to ever populate an `order` for that retailer.

This change does **not** re-propose retailer product resolution, pinning, review-and-pick, or
size-aware matching — all of that is done and lives in `openspec/specs/retailer-adapter/spec.md`.
It builds strictly on top of the adapter's existing primitives (search, resolve, wishlist,
to-cart) to give spisordning its own persisted domain objects for the list/cart/order lifecycle.

## What Changes

- **`shopping_lists` / `shopping_list_items`**: a durable, retailer-independent, human-editable
  shopping list owned by spisordning. Seeded from one or more `meal_plan`s'
  `shopping_requirement`s, but also accepts manual/ad-hoc items (e.g. "paper towels") that have no
  `ingredient_id`. Distinguishes "Need 500g chicken breast" from any retailer's specific product.
- **`retailer_list_bindings`**: records that a `shopping_list` is projected onto a specific
  external list (e.g. a Willys wishlist id returned by `POST /shopping-lists`), plus sync
  direction and last-synced state. Two-way sync and conflict handling (a person edits the Willys
  app directly; the next spisordning sync must reconcile) is researched here; see `design.md` for
  the proposed v1 direction (outbound-only) and what two-way sync would require.
- **`shopping_carts` / `shopping_cart_items`**: a distinct stage from both the shopping list and
  the order. Not the retailer's own cart state (Willys owns that) — a spisordning-side record of
  *when and what* was moved into a retailer cart via the adapter's existing
  `POST /shopping-lists/:id/to-cart`, so later order reconciliation has a checkpoint. Never
  conflated with `shopping_lists` or `orders`.
- **`orders` / `order_items`**: a persisted record of a completed purchase, preserving actual
  quantity, actual price, the resolved retailer product, and any substitutions made at checkout
  (a human action — see Non-Goals). A completed order MAY later create inventory `PURCHASE`
  events; that write path belongs to Epic D's `implement-pantry-inventory` change (not yet
  implemented — schema TBD there), so this change only defines the `order`/`order_item` shape and
  an explicit extension point, and does not implement inventory writes itself.
- **Receipt import — research only, not prioritized**: candidate sources (retailer API, PDF, Kivra
  export, email, manual entry), evaluated per source for feasibility. Given Willys's confirmed
  lack of a real order-history API, receipt import is flagged as a plausible *primary* path to
  populate `orders` for Willys specifically, not merely a fallback as `PLAN.md`'s general framing
  implies.

## Non-Goals

- No automated checkout, payment, or slot booking — this remains a human action via the retailer's
  own app/site, consistent with the retailer-adapter's existing, deliberate omission of checkout.
- No re-design of requirement→product resolution, pinning, or review — that capability is done.
- No inventory/pantry write path — this change defines the extension point only; implementation is
  Epic D's `implement-pantry-inventory`.
- No ICA-specific cart/order handling — this change's schema is retailer-agnostic; ICA specifics
  are `research-and-integrate-ica`'s job.

## Capabilities

### New Capabilities

- `shopping-and-commerce`: the retailer-independent shopping list, its projection onto retailer
  lists, the cart checkpoint stage, and the persisted order/receipt record — the layer between
  the planner's canonical requirements and the already-existing retailer adapter.

### Modified Capabilities

<!-- none — retailer-adapter's requirements (resolution, pinning, review, wishlist creation,
     to-cart) are unchanged; this change is a consumer, not a modifier, of that capability -->

## Impact

- New migration(s) adding `shopping_list`, `shopping_list_item`, `retailer_list_binding`,
  `shopping_cart`, `shopping_cart_item`, `order`, `order_item` to spisordning's own Postgres
  schema, building on the existing `shopping_requirement` table (not replacing it — see
  `design.md`).
- Depends on `retailer-adapter` (merged) for all retailer-facing operations (`/resolve`,
  `POST /shopping-lists`, `POST /shopping-lists/:id/to-cart`); this change never talks to Willys
  directly.
- Forward dependency: `order` completion creating inventory `PURCHASE` events depends on Epic D's
  `implement-pantry-inventory` existing first; until then, order completion is a terminal,
  inventory-inert event.
- Depends on `establish-enforced-go-architecture` for real Postgres persistence and an HTTP
  surface to expose these new domains through.
- Part of Epic F: Retailer, Pricing & Commerce (tracking issue #6).
