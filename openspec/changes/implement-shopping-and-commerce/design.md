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
external_list_id, last_pushed_at, last_push_status }`; pushing calls the adapter's
`POST /shopping-lists` (additive/idempotent per the adapter's existing contract) with the
shopping list's current items. Reading back Willys-side edits and reconciling them is documented
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
