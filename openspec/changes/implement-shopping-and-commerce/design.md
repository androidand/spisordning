## Context

`PLAN.md` lists four adjacent-sounding concepts — "Local Shopping Intent", "Retailer Lists",
"Carts", "Orders" — and explicitly warns: "Do not conflate shopping list / cart / order." Two of
the four layers this change would otherwise touch already exist and are out of scope:
`shopping_requirement` (the planner's output, `migrations/0001_init.sql`) and the retailer
resolution/wishlist adapter (`openspec/specs/retailer-adapter/spec.md`, code in the sibling
`willys-client` repo). This design makes the resulting five-stage pipeline explicit so the
layering — and this change's actual scope, stages 3 and 5 below — is unambiguous.

## Goals / Non-Goals

**Goals:**
- Give spisordning a durable, retailer-independent shopping list a household can manage directly,
  not only as a per-plan-run artifact.
- Define how that list projects onto a retailer's own list (Willys wishlist today; ICA or others
  later) without pretending two-way sync is a solved problem.
- Cleanly separate cart (a checkpoint, not the retailer's real cart) from order (a completed,
  fidelity-preserving purchase record).
- Leave an explicit, typed extension point for inventory `PURCHASE` events without implementing
  them.

**Non-Goals:**
- Not designing or re-designing product resolution — done, in `retailer-adapter`.
- Not implementing two-way retailer-list sync — v1 is outbound-only; see Decision D2.
- Not implementing receipt parsing — research and source evaluation only.
- Not implementing inventory writes — Epic D's job.

## The pipeline

```
shopping_requirement          shopping_list              [retailer resolution]      retailer_list_binding   shopping_cart          order
(EXISTS — planner output,     (NEW — this change,        (EXISTS — the adapter,     (NEW — this change,     (NEW — this change,   (NEW — this change,
 retailer-independent,        retailer-independent,       unchanged: /resolve,       records the external    retailer-specific,     retailer-specific,
 ephemeral, one per            durable, human-editable,    POST /shopping-lists,      wishlist id + sync       a checkpoint, not     a completed, fidelity-
 meal_plan × ingredient)       accepts manual items)       POST .../to-cart)          state)                   the real cart)         preserving record)
        │                            │                            │                          │                        │                      │
        └──── seeds ────────────────▶│                            │                          │                        │                      │
                                      └──── resolved via ─────────▶│                          │                        │                      │
                                                                    └──── recorded as ────────▶│                        │                      │
                                                                                                 └──── to-cart call ────▶│                      │
                                                                                                                          └──── confirmed as ───▶│
                                                                                                                                                  └──▶ (future) inventory PURCHASE event — Epic D
```

Stages 1 and 3 (`shopping_requirement`, retailer resolution) are pre-existing and unchanged.
Stages 2, 4, 5, and 6 (`shopping_list`, `retailer_list_binding`, `shopping_cart`, `order`) are this
change's scope.

## Decisions

### D1: `shopping_list` seeds from `shopping_requirement`, does not replace it

`shopping_requirement` stays exactly as it is — an ephemeral, per-plan, retailer-independent
snapshot the scorer/planner emits (`UNIQUE (plan_id, ingredient_id)`). `shopping_list_item` is a
separate, longer-lived row that:
- MAY reference the `shopping_requirement` it was seeded from (`shopping_requirement_id`,
  nullable),
- MAY reference an `ingredient_id` directly (a person adds "cucumber" outside any plan),
- MAY carry neither and instead be a free-text `label` (e.g. "paper towels" — not every shopping
  need is a food ingredient, and Mealie/Grocy both support non-recipe list items).

A `shopping_list` can aggregate requirements from more than one `meal_plan` (e.g. "this week" +
"stock up on pantry staples") and survives after the plan it was seeded from is archived. This
keeps `shopping_requirement`'s existing, already-tested contract (`food-brain-first-slice`'s
`meal-planning` spec) untouched while giving the household a durable, editable list.

### D2: Retailer list sync is outbound-only in v1; two-way is a researched extension point

`PLAN.md` asks to "study two-way synchronization and conflicts." A full two-way sync (spisordning
→ Willys wishlist, and Willys wishlist edits made in the phone app → back into spisordning) needs
either polling the adapter's wishlist-read surface for diffs or a webhook Willys does not offer;
either way it needs a merge policy (last-write-wins? per-item timestamps? human review of
conflicts, similar to the existing needs-review queue?) that does not yet have evidence behind it.

v1 ships **outbound-only**: `retailer_list_binding` records `{ shopping_list_id, retailer,
external_list_id, sync_direction, last_pushed_at, last_push_status }`; pushing calls the
adapter's `POST /shopping-lists` with the shopping list's current items. Note the adapter's push
is **additive, not idempotent** (re-pushing increments quantities) — see "Push semantics:
additive, not idempotent" below. Reading back Willys-side edits and reconciling them is documented
as a follow-on research task (task 2.4), not implemented, so it isn't quietly declared solved.

### D3: Cart is a checkpoint record, not the retailer's cart

The adapter already exposes `POST /shopping-lists/:id/to-cart` as "an explicit, separate cart-fill
step" (`willys-capabilities.md`). Willys owns the actual cart state server-side; spisordning has
no visibility into it beyond having triggered the to-cart call. `shopping_cart` /
`shopping_cart_item` therefore record *that a to-cart call was made, for which binding, with what
resolved items and quantities at that moment* — a checkpoint for order reconciliation, not a
mirror of live retailer cart state. This keeps "cart" meaningfully distinct from both "list"
(retailer-independent intent) and "order" (confirmed, priced purchase).

### D4: Order preserves purchase fidelity; source is explicit

`order_item` stores actual `quantity`, actual `unit_price`/`total_price`, the resolved
`retailer_product_id`, and an optional `substituted_for_item_id` (self-referencing) when a
retailer substituted a product at fulfillment — all per `PLAN.md`'s explicit "Preserve actual:
quantity, price, retailer product, substitutions." Because no retailer in scope has a real
order-history API today (confirmed absent for Willys; unknown/TBD for ICA pending
`research-and-integrate-ica`), `order.source` is an explicit enum (`'manual' | 'retailer_api' |
'receipt_import'`) rather than assumed to be `'retailer_api'`. Most orders will start as
`'manual'` (a person confirms what they actually bought) until a retailer API or receipt pipeline
exists.

### D5: Inventory PURCHASE events are a typed extension point, not implemented here

`order.id` is designed to be a stable, referenceable id from day one so a future
`implement-pantry-inventory` change can add an `inventory_event(kind='PURCHASE', order_item_id
REFERENCES order_item(id))` without a migration that touches this change's tables. This change
does not create any `inventory_*` table itself.

## Concrete schema (v1)

The column-level shape of the two shopping-list tables (section 1). Types, nullability,
and FK conventions follow `migrations/0001_init.sql` (BIGSERIAL ids, `TIMESTAMPTZ ... DEFAULT
now()`, `quantity DOUBLE PRECISION` / `unit TEXT` mirroring `shopping_requirement`). The actual
`CREATE TABLE` statements land in the section-1 migration (task 1.5); this is the design they
implement.

### `shopping_list`

```sql
CREATE TABLE shopping_list (
    id              BIGSERIAL PRIMARY KEY,
    -- Owner/attribution. The system is single-household today and the household
    -- table (establish-household-and-catalog) has not landed, so a list is
    -- attributed to a person; NULL means a shared household-level list. When the
    -- household table lands this becomes a household_id reference.
    owner_person_id TEXT REFERENCES person(id) ON DELETE SET NULL,
    name            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','archived')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### `shopping_list_item`

```sql
CREATE TABLE shopping_list_item (
    id                      BIGSERIAL PRIMARY KEY,
    shopping_list_id        BIGINT NOT NULL REFERENCES shopping_list(id) ON DELETE CASCADE,
    -- At most one of the three identifiers below is the "what is this" for the
    -- row (D1): the requirement it was seeded from, a direct ingredient, or a
    -- free-text label for non-ingredient items. No retailer product id — that is
    -- the adapter's job.
    shopping_requirement_id BIGINT REFERENCES shopping_requirement(id) ON DELETE SET NULL,
    ingredient_id           TEXT REFERENCES ingredient(id) ON DELETE SET NULL,
    label                   TEXT,
    quantity                DOUBLE PRECISION NOT NULL,
    unit                    TEXT NOT NULL,
    -- Check-off state (task 1.4's manual operations toggle this).
    checked                 BOOLEAN NOT NULL DEFAULT false,
    added_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- No fully-empty item: at least one identifier must be present (task 1.2).
    CHECK (shopping_requirement_id IS NOT NULL
           OR ingredient_id IS NOT NULL
           OR label IS NOT NULL)
);
CREATE INDEX ON shopping_list_item (shopping_list_id);
```

The "no fully-empty item" invariant (at least one of `shopping_requirement_id`,
`ingredient_id`, `label` is set) is encoded as a database `CHECK` constraint (task 1.2)
rather than an application-layer invariant: it is the strongest guarantee, enforced
independently of any code path, and it makes the rejection of generic polymorphism
(`PLAN.md`) explicit in the schema itself.

## Seeding & manual operations (v1)

### Seeding from `shopping_requirement` (task 1.3)

A `shopping_list` is seeded by **reading** one or more `meal_plan`s' `shopping_requirement`
rows and **writing** new `shopping_list_item` rows — never by mutating or duplicating a
`shopping_requirement`. Concretely:

- **Input:** a set of `meal_plan` ids (or a set of `shopping_requirement` ids directly).
- **Action:** for each source `shopping_requirement`, insert one `shopping_list_item` with
  `shopping_requirement_id` = the requirement's id, `ingredient_id` = the requirement's
  ingredient (for display convenience), `quantity`/`unit` copied from the requirement,
  `label` = NULL, `checked` = false.
- **No mutation/duplication:** `shopping_requirement` rows are read-only inputs. Seeding only
  writes `shopping_list_item` rows; it never inserts, updates, or deletes a
  `shopping_requirement`. The planner's per-plan, ephemeral contract is untouched.
- **Aggregation:** one list may aggregate requirements from several plans (e.g. "this week" +
  "pantry staples"); each seeded item keeps a `shopping_requirement_id` back to its exact
  source, so provenance survives the aggregation.
- **Idempotency:** re-seeding the same requirement into the same list is a no-op, enforced by a
  partial unique index `UNIQUE (shopping_list_id, shopping_requirement_id) WHERE
  shopping_requirement_id IS NOT NULL`. Manual items (no `shopping_requirement_id`) are exempt.

### Manual add / remove / check-off (task 1.4)

All three operations act directly on `shopping_list_item` rows and are independent of any
`meal_plan`:

- **Add:** `INSERT` a new item identified by a free-text `label` (non-ingredient, e.g. "paper
  towels") or a direct `ingredient_id` (a person adds "cucumber" outside any plan), with
  `quantity`, `unit`, `checked` = false. No plan or requirement is involved.
- **Remove:** `DELETE` the item row. Hard delete is correct for a list line (it is not a
  historical record); if the item was seeded from a requirement, that requirement is unaffected
  and can be re-seeded.
- **Check-off:** `UPDATE shopping_list_item SET checked = <bool>` — a plain toggle; checked
  items remain in the list (they are not removed), so a person can un-check or re-push.

None of these operations read or write `shopping_requirement`, `meal_plan`, or any plan table.

## Retailer list binding (v1)

### `retailer_list_binding` (task 2.1)

Records that a `shopping_list` has been projected onto a specific external retailer list (D2).
The spisordning `shopping_list` is authoritative for intent; the retailer's list is a synchronized
projection, and the binding makes projection staleness (time since last successful push)
inspectable.

```sql
CREATE TABLE retailer_list_binding (
    id               BIGSERIAL PRIMARY KEY,
    shopping_list_id BIGINT NOT NULL REFERENCES shopping_list(id) ON DELETE CASCADE,
    retailer         TEXT NOT NULL,              -- e.g. 'willys'
    external_list_id TEXT NOT NULL,              -- the adapter's wishlistId (a string)
    -- v1 is outbound-only (D2); a future two-way change widens this CHECK.
    sync_direction   TEXT NOT NULL DEFAULT 'outbound' CHECK (sync_direction IN ('outbound')),
    last_pushed_at   TIMESTAMPTZ,                -- NULL until first successful push
    last_push_status TEXT CHECK (last_push_status IN ('success','error')),
    -- One binding per (list, retailer); re-pushing updates it (spec scenario).
    UNIQUE (shopping_list_id, retailer)
);
```

- `external_list_id` is the `wishlistId` **string** returned by the adapter's
  `POST /shopping-lists` (confirmed from `apps/willys-adapter/server.ts`).
- `UNIQUE (shopping_list_id, retailer)`: a list has at most one binding per retailer; subsequent
  pushes for the same list update the same binding's `last_pushed_at`/`last_push_status` (and
  `external_list_id` if the external list was recreated), per the spec's "subsequent pushes ...
  update the same binding" scenario. The unique index also serves lookups by `shopping_list_id`.
- `sync_direction` is `'outbound'` in v1 (D2); the CHECK encodes that at the schema level and a
  future two-way change widens it.
- `last_pushed_at` is NULL until the first successful push; `last_push_status` is NULL until the
  first attempt. Staleness = the `shopping_list` was edited after `last_pushed_at` (spec scenario).

### Push semantics: additive, not idempotent (task 2.3)

Confirmed against the adapter's actual behavior (`apps/willys-adapter/server.ts`,
`POST /shopping-lists`): the endpoint is **additive** — an existing wishlist with the exact same
name is extended via `addProductsToWishlist(..., { increment: true })`, so **re-pushing the same
list increments (doubles) quantities in the Willys wishlist; it neither merges (dedupes to the
same quantity) nor is a no-op (idempotent)**. The response is `201` (created new) or `200`
(extended existing), body `{ wishlistId, name }`.

Implication for v1 (documented, not silently papered over): pushing the whole list is correct for
the *first* push, but a *re*-push after editing the list will over-count (e.g. chicken 500g →
edit to 300g → re-push → Willys shows 800g). The binding's `last_pushed_at`/`last_push_status`
make this staleness inspectable so a person can see the last push time. A "replace" semantic
(clear the external list, then re-add) is the natural fix but requires an adapter clear endpoint
that does not exist today — flagged as a follow-on enhancement, not implemented here.

### Two-way sync research (task 2.4)

Confirmed against the adapter's actual HTTP surface (`apps/willys-adapter/server.ts`), the
exposed wishlist endpoints are exactly two:

- `POST /shopping-lists` — create or extend (additive; see "Push semantics" above).
- `POST /shopping-lists/:id/to-cart` — convert a wishlist to the session cart.

There is **no** `DELETE /shopping-lists/:id` (clear), **no** `PUT /shopping-lists/:id`
(replace/update), and **no** `GET /shopping-lists[/:id]` (read-back). The adapter's internal
`client.wishlist.getWishlists(...)` call is a direct Willys-API call used only to find an
existing wishlist by name before extending it; it is not an exposed endpoint.

Consequences for v1:

- **No clear/replace:** a push can only add; it cannot clear or replace the external list (the
  over-count implication is documented under "Push semantics" above).
- **No inbound sync:** because there is no read-back endpoint, spisordning cannot pull
  Willys-side edits back into a `shopping_list`. Inbound sync (wishlist → shopping_list) is
  therefore **out of scope for v1** — it is not merely deferred, it is not possible against the
  current adapter.
- **Outbound-only binding:** the binding's `sync_direction` stays `'outbound'`-only; the schema
  `CHECK (sync_direction IN ('outbound'))` encodes this (D2).

Future extension point for a `pull` direction (documented, not implemented): a two-way change
would need (a) an adapter read-back endpoint (`GET /shopping-lists/:id` returning the wishlist's
current items), (b) a merge policy for diverged items (last-write-wins vs per-item timestamps vs
human review of conflicts, analogous to the adapter's existing needs-review queue), and (c)
widening the `sync_direction` CHECK to admit a `'pull'` (or `'two-way'`) value. None of these has
evidence behind it yet, which is why v1 stays outbound-only (D2).

## Cart checkpoint (v1)

### `shopping_cart` / `shopping_cart_item` (task 3.1)

A `shopping_cart` is a **checkpoint record of a to-cart call**, not a mirror of the retailer's
live cart state (D3). Willys owns the actual cart server-side; spisordning has no visibility into
it beyond having triggered the `POST /shopping-lists/:id/to-cart` call. The cart therefore records
*that a to-cart call was made, for which binding, with what resolved items and quantities at that
moment* — a checkpoint for later order reconciliation.

```sql
CREATE TABLE shopping_cart (
    id BIGSERIAL PRIMARY KEY,
    retailer_list_binding_id BIGINT NOT NULL REFERENCES retailer_list_binding(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- 'created' = to-cart call made; 'confirmed' = an order was confirmed from this cart;
    -- 'abandoned' = the cart was abandoned (no order).
    status TEXT NOT NULL DEFAULT 'created' CHECK (status IN ('created','confirmed','abandoned'))
);

CREATE TABLE shopping_cart_item (
    id BIGSERIAL PRIMARY KEY,
    shopping_cart_id BIGINT NOT NULL REFERENCES shopping_cart(id) ON DELETE CASCADE,
    -- The resolved retailer product (the adapter's product code); the cart is
    -- retailer-specific, unlike the retailer-independent shopping_list.
    retailer_product_id TEXT NOT NULL,
    quantity DOUBLE PRECISION NOT NULL,
    unit TEXT NOT NULL,
    -- The price at the moment of the to-cart call (a checkpoint, not a live price);
    -- NULL if the price was not known at that moment.
    resolved_price DOUBLE PRECISION
);
CREATE INDEX ON shopping_cart_item (shopping_cart_id);
```

- `retailer_list_binding_id` links the cart to the binding it was created from (the binding
  records the external wishlist; the cart records the to-cart call made on that wishlist).
- `shopping_cart_item` stores the resolved items (product codes + quantities + prices) at the
  moment of the to-cart call — a snapshot for order reconciliation, not live cart state.
- `status` tracks the cart's lifecycle: `'created'` (to-cart call made) → `'confirmed'` (an order
  was confirmed from it) or `'abandoned'` (no order).

### No automated checkout (task 3.3)

The to-cart call is the **last automated step** in this change. The adapter's
`POST /shopping-lists/:id/to-cart` converts a wishlist to the session cart and stops there — it
does not trigger checkout, payment, or slot booking, and the adapter exposes no endpoint that does.
No code path in this change (or the adapter it calls) can initiate checkout, payment, or slot
booking; those remain **human actions in the retailer's own app/site**. This is confirmed, not
merely asserted: the adapter's full HTTP surface (see "Two-way sync research" above) has no
checkout/payment/slot-booking endpoint.

## Order (v1)

### `order` / `order_item` (task 4.1)

An `order` is a **completed, fidelity-preserving purchase record** (D4). Because no retailer in
scope has a real order-history API today (confirmed absent for Willys; unknown/TBD for ICA pending
`research-and-integrate-ica`), `order.source` is an explicit enum (`'manual' | 'retailer_api' |
'receipt_import'`) rather than assumed to be `'retailer_api'`. Most orders will start as `'manual'`
(a person confirms what they actually bought) until a retailer API or receipt pipeline exists.

```sql
CREATE TABLE "order" (
    id BIGSERIAL PRIMARY KEY,
    -- Nullable: an order may be confirmed from a cart checkpoint, or entered manually
    -- with no preceding cart (e.g. a receipt import).
    shopping_cart_id BIGINT REFERENCES shopping_cart(id) ON DELETE SET NULL,
    retailer TEXT NOT NULL,
    -- Explicit source (D4): 'manual' (a person confirms the purchase), 'retailer_api'
    -- (a future retailer order-history API), 'receipt_import' (a future receipt pipeline).
    source TEXT NOT NULL CHECK (source IN ('manual','retailer_api','receipt_import')),
    ordered_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    total_price DOUBLE PRECISION
);

CREATE TABLE order_item (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES "order"(id) ON DELETE CASCADE,
    -- The actual product purchased (the adapter's product code); preserves the retailer
    -- product per PLAN.md's "Preserve actual: quantity, price, retailer product, substitutions."
    retailer_product_id TEXT NOT NULL,
    quantity DOUBLE PRECISION NOT NULL,
    unit_price DOUBLE PRECISION,
    total_price DOUBLE PRECISION,
    -- Self-reference: set when the retailer substituted this product for another at
    -- fulfillment (the referenced item is the originally-intended product).
    substituted_for_item_id BIGINT REFERENCES order_item(id)
);
CREATE INDEX ON order_item (order_id);
```

- `shopping_cart_id` is nullable: an order may be confirmed from a cart checkpoint (the common
  manual path), or entered with no preceding cart (e.g. a receipt import). `ON DELETE SET NULL`
  preserves the order if the cart is deleted.
- `order_item` stores the **actual** quantity, price, retailer product, and substitutions (D4) —
  a fidelity-preserving record, not a projection of the cart's checkpoint.
- `substituted_for_item_id` is a self-reference to `order_item(id)`: when a retailer substituted a
  product at fulfillment, the substituted item points to the originally-intended item.

### Manual order-confirmation flow (task 4.2)

The default path is **manual confirmation**, pre-filled from the most recent `shopping_cart`
checkpoint:

1. A person opens the most recent `shopping_cart` (the to-cart checkpoint) after shopping.
2. The confirmation form is **pre-filled** from the cart's items (product, quantity, price) — the
   person does not re-enter from scratch.
3. The person adjusts what they **actually** bought: quantities that differ from the cart, prices
   that changed at checkout, and any **substitutions** the retailer made (set
   `substituted_for_item_id` on the substituted item).
4. Confirming creates an `order` with `source = 'manual'` and the actual `order_item`s, and marks
   the cart `status = 'confirmed'`.

This keeps the confirmation cheap (pre-filled, not re-entry) while preserving purchase fidelity
(D4). Because no in-scope retailer has a real order-history API today, this manual path is the
default; `source = 'retailer_api'` and `source = 'receipt_import'` are future paths (the latter
see section 5's receipt research).

### Inventory PURCHASE extension point (task 4.3)

`order.id` and `order_item.id` are designed to be **stable, referenceable ids from day one** so a
future `implement-pantry-inventory` change (Epic D) can add an
`inventory_event(kind='PURCHASE', order_item_id REFERENCES order_item(id))` **without a migration
that touches this change's tables**. This change does not create any `inventory_*` table itself;
the forward dependency is noted in the migration comments (task 4.4).

## Risks / Trade-offs

- **Outbound-only sync (D2)** means a person editing the Willys app directly can silently drift
  from spisordning's list until the next push overwrites or duplicates items; mitigated by scoping
  v1 pushes as additive (matching the adapter's existing wishlist semantics) and flagging
  reconciliation as explicit follow-on research rather than a silent gap.
- **`order.source = 'manual'` as the common case** means order data quality depends on a person
  bothering to confirm a purchase; mitigated by keeping the confirmation cheap (pre-filled from the
  shopping_cart checkpoint) rather than requiring re-entry.
- **No inventory write path yet (D5)** means a completed order does not yet update pantry stock;
  explicitly called out as a forward dependency rather than quietly deferred.
