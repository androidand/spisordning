# Tasks: implement-shopping-and-commerce

## 1. Shopping list domain

- [ ] 1.1 Design `shopping_list` (id, household/owner reference, name, status, created_at) and
      `shopping_list_item` (id, shopping_list_id, optional `shopping_requirement_id`, optional
      `ingredient_id`, optional free-text `label`, quantity, unit, checked/state, added_at).
- [ ] 1.2 Confirm at least one of `shopping_requirement_id`, `ingredient_id`, or `label` is
      required per item (no fully-empty item) — encode as a constraint or application-layer
      invariant, per `PLAN.md`'s "Do not use generic polymorphism carelessly."
- [ ] 1.3 Define how a `shopping_list` is seeded from one or more `meal_plan`s'
      `shopping_requirement`s without mutating or duplicating `shopping_requirement` itself.
- [ ] 1.4 Define manual item add/remove/check-off operations independent of any plan.
- [ ] 1.5 Migration: add `shopping_list`, `shopping_list_item` to the schema, building on
      `migrations/0001_init.sql`'s existing `shopping_requirement` (see `design.md` D1).

## 2. Retailer list bindings

- [ ] 2.1 Design `retailer_list_binding` (id, shopping_list_id, retailer, external_list_id,
      sync_direction, last_pushed_at, last_push_status).
- [ ] 2.2 Wire outbound push through the existing adapter's `POST /shopping-lists` (additive,
      per its existing contract) — no new retailer-facing code, only the binding record and the
      Go-side call.
- [ ] 2.3 Confirm push idempotency/additivity against the adapter's actual behavior (does
      re-pushing the same list duplicate items in the Willys wishlist, or merge?).
- [ ] 2.4 Research task (not implemented in this change): what would reading back Willys-side
      wishlist edits and reconciling them against `shopping_list` require — polling cadence,
      diffing strategy, and a conflict policy (last-write-wins vs. per-item timestamp vs. surfacing
      conflicts through a review queue similar to the adapter's existing needs-review queue).
      Document findings; do not implement two-way sync.
- [ ] 2.5 Migration: add `retailer_list_binding`.

## 3. Carts

- [ ] 3.1 Design `shopping_cart` (id, retailer_list_binding_id, created_at, status) and
      `shopping_cart_item` (id, shopping_cart_id, retailer_product_id, quantity, resolved_price)
      as a checkpoint record of a to-cart call, not a mirror of live retailer cart state (see
      `design.md` D3).
- [ ] 3.2 Wire cart creation to the adapter's existing `POST /shopping-lists/:id/to-cart`
      endpoint; record the response as the checkpoint.
- [ ] 3.3 Confirm no code path in this change or its consumers can trigger checkout, payment, or
      slot booking — those remain human actions in the retailer's own app/site.
- [ ] 3.4 Migration: add `shopping_cart`, `shopping_cart_item`.

## 4. Orders

- [ ] 4.1 Design `order` (id, shopping_cart_id nullable, retailer, source enum
      `'manual'|'retailer_api'|'receipt_import'`, ordered_at, total_price) and `order_item` (id,
      order_id, retailer_product_id, quantity, unit_price, total_price,
      `substituted_for_item_id` self-reference nullable) preserving actual quantity, price,
      retailer product, and substitutions per `PLAN.md`'s "Orders" section.
- [ ] 4.2 Design the manual order-confirmation flow (pre-filled from the most recent
      `shopping_cart` checkpoint) as the default path, since no in-scope retailer has a real
      order-history API today.
- [ ] 4.3 Define — but do not implement — the extension point for a future
      `inventory_event(kind='PURCHASE')` referencing `order_item.id`, so Epic D's
      `implement-pantry-inventory` can add it without migrating this change's tables. Note this
      forward dependency explicitly in the migration comments.
- [ ] 4.4 Migration: add `order`, `order_item`.

## 5. Receipts — research only, not prioritized

- [ ] 5.1 Evaluate retailer API as a receipt source per retailer in scope (Willys: confirmed no
      real order-history API today, per `docs/research/willys-capabilities.md` — flag as
      currently infeasible; ICA: pending `research-and-integrate-ica` findings).
- [ ] 5.2 Evaluate PDF receipt import (format variability across retailers, OCR/structured-text
      extraction feasibility).
- [ ] 5.3 Evaluate Kivra digital-mailbox export as a Swedish-specific receipt source (what
      retailers deliver receipts to Kivra, what export/API access Kivra offers).
- [ ] 5.4 Evaluate email receipt parsing (structured vs. unstructured retailer emails).
- [ ] 5.5 Evaluate manual receipt entry as the fallback baseline (already implicitly covered by
      `order.source = 'manual'` in section 4).
- [ ] 5.6 Record findings in `docs/research/` (new doc, e.g. `receipt-import-sources.md`); do not
      implement a receipt parser in this change.
- [ ] 5.7 Call out explicitly: because Willys has no real order/purchase-history API, receipt
      import is a plausible *primary* path to populate `orders` for Willys, not merely a
      fallback — re-prioritize in a future change if this research confirms it's tractable.

## 6. API & persistence

- [ ] 6.1 Postgres repositories for all new tables, following whatever persistence-layer
      convention `establish-enforced-go-architecture` establishes.
- [ ] 6.2 REST endpoints (OpenAPI-first, per that same change's convention) for shopping-list
      CRUD, retailer-list push, cart-checkpoint creation, and order confirmation/history.
- [ ] 6.3 Integration tests against a real/containerized Postgres for each new repository.
- [ ] 6.4 API integration tests for the new endpoints.

## 7. Verification & docs

- [ ] 7.1 `go build ./... && go test ./... && go vet ./...` green.
- [ ] 7.2 Manual end-to-end check: plan → shopping_list → push to Willys wishlist → to-cart →
      manual order confirmation, with no automated checkout at any step.
- [ ] 7.3 Update `docs/research/current-state.md`'s schema summary once these tables land.
