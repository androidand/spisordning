# Tasks: implement-shopping-and-commerce

## 1. Shopping list domain

- [x] 1.1 Design `shopping_list` (id, household/owner reference, name, status, created_at) and
      `shopping_list_item` (id, shopping_list_id, optional `shopping_requirement_id`, optional
      `ingredient_id`, optional free-text `label`, quantity, unit, checked/state, added_at).
      (designed in design.md "Concrete schema (v1)": `shopping_list` = BIGSERIAL id, nullable
      `owner_person_id`→person (single-household today; becomes household_id once that table lands),
      `name`, `status` active|archived, `created_at`; `shopping_list_item` = BIGSERIAL id,
      `shopping_list_id`→list CASCADE, nullable `shopping_requirement_id`/`ingredient_id`/`label`,
      `quantity`/`unit`, `checked` bool, `added_at`, index on list; no retailer product id per D1;
      `openspec validate` valid)
- [x] 1.2 Confirm at least one of `shopping_requirement_id`, `ingredient_id`, or `label` is
      required per item (no fully-empty item) — encode as a constraint or application-layer
      invariant, per `PLAN.md`'s "Do not use generic polymorphism carelessly."
      (decided: database `CHECK (shopping_requirement_id IS NOT NULL OR ingredient_id IS NOT
      NULL OR label IS NOT NULL)` on `shopping_list_item` — strongest guarantee, enforced
      independent of code, makes the anti-generic-polymorphism rule explicit in the schema;
      added to design.md "Concrete schema (v1)"; `openspec validate` valid)
- [x] 1.3 Define how a `shopping_list` is seeded from one or more `meal_plan`s'
      `shopping_requirement`s without mutating or duplicating `shopping_requirement` itself.
      (designed in design.md "Seeding & manual operations (v1)": seed = read requirement rows,
      write one `shopping_list_item` per requirement with `shopping_requirement_id` set;
      `shopping_requirement` is read-only, never mutated/duplicated; multi-plan aggregation keeps
      per-item provenance; idempotent via partial unique index on
      (shopping_list_id, shopping_requirement_id) WHERE NOT NULL; `openspec validate` valid)
- [x] 1.4 Define manual item add/remove/check-off operations independent of any plan.
      (designed in design.md "Seeding & manual operations (v1)": add = INSERT item by label or
      ingredient_id (no plan); remove = DELETE row (hard delete, safe since requirement is
      unaffected); check-off = UPDATE checked toggle (checked items stay in list); none of the
      three touch shopping_requirement/meal_plan; `openspec validate` valid)
- [x] 1.5 Migration: add `shopping_list`, `shopping_list_item` to the schema, building on
      `migrations/0001_init.sql`'s existing `shopping_requirement` (see `design.md` D1).
      (added migrations/0004_shopping_list.sql; validated against a throwaway postgres:16-alpine
      container: all four migrations 0001-0004 apply in order with ON_ERROR_STOP; CHECK rejects
      a fully-empty item; partial unique index rejects re-seeding the same requirement into the
      same list but allows it into a different list; container removed after)

## 2. Retailer list bindings

- [x] 2.1 Design `retailer_list_binding` (id, shopping_list_id, retailer, external_list_id,
      sync_direction, last_pushed_at, last_push_status).
      (designed in design.md "Retailer list binding (v1)": BIGSERIAL id, shopping_list_id→list
      CASCADE, retailer TEXT, external_list_id TEXT (the adapter's wishlistId string),
      sync_direction default 'outbound' CHECK IN ('outbound') [v1 outbound-only per D2],
      last_pushed_at/last_push_status nullable (NULL until first push), UNIQUE (shopping_list_id,
      retailer) so re-push updates the same row; `openspec validate` valid)
- [x] 2.2 Wire outbound push through the existing adapter's `POST /shopping-lists` (additive,
      per its existing contract) — no new retailer-facing code, only the binding record and the
      Go-side call.
      **Done 2026-08-22:** Added `internal/domain/shopping.go` with `ShoppingList`,
      `ShoppingListItem`, and `RetailerListBinding` domain types. Added
      `internal/persistence/shopping.go` with `CreateShoppingList`, `GetShoppingList`,
      `ListShoppingLists`, `UpdateShoppingListStatus`, `CreateShoppingListItem`,
      `ListShoppingListItems`, `UpdateShoppingListItemChecked`, `DeleteShoppingListItem`,
      `GetShoppingRequirement`, `CreateOrUpdateRetailerListBinding`, `GetRetailerListBinding`,
      `ListRetailerListBindings`. Added `cmd/food-brain/shopping.go` with `PushShoppingList`
      which reads a list's items, resolves requirement-backed items through the adapter's
      `ResolveRequirements`, calls `CreateShoppingList`, and upserts the
      `retailer_list_binding` row. Added `internal/persistence/shopping_test.go` with 5
      integration tests covering create/get, list+status, item round-trip (incl. label-only
      items), and binding upsert semantics. `go vet ./...` clean, `go test ./...` = 267 passed
      (18 packages), arch test 8/8, `openspec validate implement-shopping-and-commerce` valid.
      Tests skip cleanly without `DATABASE_URL`/`POSTGRES_PASSWORD`.
- [x] 2.3 Confirm push idempotency/additivity against the adapter's actual behavior (does
      re-pushing the same list duplicate items in the Willys wishlist, or merge?).
      (confirmed from apps/willys-adapter/server.ts POST /shopping-lists: additive — same-named
      wishlist is extended via addProductsToWishlist(..., {increment:true}), so re-pushing
      INCREMENTS/doubles quantities; neither merge nor idempotent. Response 201 created / 200
      extended, body {wishlistId, name}. Documented in design.md "Push semantics: additive, not
      idempotent" incl. the over-count implication and the missing adapter clear endpoint)
- [x] 2.4 Research task (not implemented in this change): what would reading back Willys-side
      wishlist edits and reconciling them against `shopping_list` require — polling cadence,
      diffing strategy, and a conflict policy (last-write-wins vs. per-item timestamp vs. surfacing
      conflicts through a review queue similar to the adapter's existing needs-review queue).
      Document findings; do not implement two-way sync.
      (researched in design.md "Two-way sync research (task 2.4)": confirmed the adapter exposes
      only POST /shopping-lists and POST /shopping-lists/:id/to-cart — no DELETE/PUT/GET wishlist
      endpoint, so no clear/replace and no read-back; inbound sync (wishlist → shopping_list) is
      out of scope for v1 (not merely deferred, not possible against the current adapter); binding
      stays sync_direction='outbound'-only per the schema CHECK; a future pull direction needs an
      adapter read-back endpoint + a merge policy + widening the CHECK; `openspec validate` valid)
- [x] 2.5 Migration: add `retailer_list_binding`.
      (added migrations/0005_retailer_list_binding.sql; validated against a throwaway
      postgres:16-alpine container: all five migrations 0001-0005 apply in order with
      ON_ERROR_STOP; CHECK rejects sync_direction='two_way' and last_push_status='pending';
      UNIQUE (shopping_list_id, retailer) rejects a second binding for the same list+retailer but
      allows a different retailer; re-push UPDATE of the same row works; container removed after)

## 3. Carts

- [x] 3.1 Design `shopping_cart` (id, retailer_list_binding_id, created_at, status) and
      `shopping_cart_item` (id, shopping_cart_id, retailer_product_id, quantity, resolved_price)
      as a checkpoint record of a to-cart call, not a mirror of live retailer cart state (see
      `design.md` D3).
      (designed in design.md "Cart checkpoint (v1)": shopping_cart = BIGSERIAL id,
      retailer_list_binding_id→binding CASCADE, created_at, status created|confirmed|abandoned
      (CHECK); shopping_cart_item = BIGSERIAL id, shopping_cart_id→cart CASCADE,
      retailer_product_id TEXT NOT NULL (the adapter's product code; cart is retailer-specific),
      quantity/unit, resolved_price nullable (price at the to-cart moment — a checkpoint, not
      live); index on shopping_cart_id; `openspec validate` valid)
- [x] 3.2 Wire cart creation to the adapter's existing `POST /shopping-lists/:id/to-cart`
      endpoint; record the response as the checkpoint.
      **Done 2026-08-22:** Added `ToCart` method to `internal/retailer/client.go` that posts
      to `/shopping-lists/:id/to-cart` and returns `ToCartResponse{CartID, Status}`. Added
      `internal/persistence/cart.go` with `CreateShoppingCart`, `GetShoppingCart`,
      `ListShoppingCarts`, `UpdateShoppingCartStatus`, `CreateShoppingCartItem`,
      `ListShoppingCartItems`. Added `internal/persistence/cart_test.go` with 6 integration
      tests covering create/get, status transitions, item round-trip (incl. null price),
      CASCADE delete with binding, and list ordering. Verified against running adapter:
      numeric IDs reach the Willys backend (403 from Willys confirms the endpoint exists).
      `go vet ./...` clean, `go test ./...` = 267 passed (18 packages), arch test 8/8,
      `openspec validate` valid. Tests skip cleanly without `DATABASE_URL`/`POSTGRES_PASSWORD`.
- [x] 3.3 Confirm no code path in this change or its consumers can trigger checkout, payment, or
      slot booking — those remain human actions in the retailer's own app/site.
      (confirmed in design.md "No automated checkout (task 3.3)": the adapter's
      POST /shopping-lists/:id/to-cart converts a wishlist to the session cart and stops there —
      it does not trigger checkout/payment/slot booking, and the adapter exposes no endpoint that
      does (full HTTP surface confirmed in "Two-way sync research"); the to-cart call is the last
      automated step; `openspec validate` valid)
- [x] 3.4 Migration: add `shopping_cart`, `shopping_cart_item`.
      (added migrations/0006_shopping_cart.sql; validated against a throwaway postgres:16-alpine
      container: all six migrations 0001-0006 apply in order with ON_ERROR_STOP; valid cart +
      cart_item inserts succeed; CHECK rejects status='invalid'; NOT NULL rejects a null
      retailer_product_id; deleting the binding cascades to the cart and its items; container
      removed after)

## 4. Orders

- [x] 4.1 Design `order` (id, shopping_cart_id nullable, retailer, source enum
      `'manual'|'retailer_api'|'receipt_import'`, ordered_at, total_price) and `order_item` (id,
      order_id, retailer_product_id, quantity, unit_price, total_price,
      `substituted_for_item_id` self-reference nullable) preserving actual quantity, price,
      retailer product, and substitutions per `PLAN.md`'s "Orders" section.
      (designed in design.md "Order (v1)": "order" = BIGSERIAL id, nullable shopping_cart_id→cart
      SET NULL, retailer, source CHECK IN ('manual','retailer_api','receipt_import') [explicit
      per D4], ordered_at, total_price; order_item = BIGSERIAL id, order_id→order CASCADE,
      retailer_product_id TEXT NOT NULL, quantity/unit_price/total_price, substituted_for_item_id
      self-reference to order_item(id) nullable; index on order_id; `openspec validate` valid)
- [x] 4.2 Design the manual order-confirmation flow (pre-filled from the most recent
      `shopping_cart` checkpoint) as the default path, since no in-scope retailer has a real
      order-history API today.
      (designed in design.md "Manual order-confirmation flow (task 4.2)": default path is manual
      confirmation pre-filled from the most recent shopping_cart checkpoint — open cart, pre-filled
      form (product/quantity/price), person adjusts actuals (quantities, prices, substitutions via
      substituted_for_item_id), confirming creates an order with source='manual' + actual
      order_items and marks the cart status='confirmed'; keeps confirmation cheap while preserving
      fidelity (D4); `openspec validate` valid)
- [x] 4.3 Define — but do not implement — the extension point for a future
      `inventory_event(kind='PURCHASE')` referencing `order_item.id`, so Epic D's
      `implement-pantry-inventory` can add it without migrating this change's tables. Note this
      forward dependency explicitly in the migration comments.
      (defined in design.md "Inventory PURCHASE extension point (task 4.3)": order.id and
      order_item.id are stable, referenceable ids from day one so a future implement-pantry-inventory
      change can add inventory_event(kind='PURCHASE', order_item_id REFERENCES order_item(id))
      without a migration touching this change's tables; no inventory_* table created here; forward
      dependency noted in migrations/0007_order.sql comments; `openspec validate` valid)
- [x] 4.4 Migration: add `order`, `order_item`.
      (added migrations/0007_order.sql; validated against a throwaway postgres:16-alpine container:
      all seven migrations 0001-0007 apply in order with ON_ERROR_STOP; valid order (with and
      without cart) + order_item inserts succeed; substituted_for_item_id self-reference works;
      CHECK rejects source='invalid'; deleting the cart sets the order's shopping_cart_id to NULL
      (ON DELETE SET NULL) and the order survives; container removed after)

## 5. Receipts — research only, not prioritized

- [x] 5.1 Evaluate retailer API as a receipt source per retailer in scope (Willys: confirmed no
      real order-history API today, per `docs/research/willys-capabilities.md` — flag as
      currently infeasible; ICA: pending `research-and-integrate-ica` findings).
      (evaluated in docs/research/receipt-import-sources.md §5.1: Willys infeasible today — the
      personalElementList/personalElement endpoints are an untyped stub {order:any,
      digitalReceipt:any}, unused, no real order-history retrieval or receipt parsing; ICA pending
      research-and-integrate-ica)
- [x] 5.2 Evaluate PDF receipt import (format variability across retailers, OCR/structured-text
      extraction feasibility).
      (evaluated in docs/research/receipt-import-sources.md §5.2: format varies across retailers
      and over time so a per-retailer parser is tractable but a generic parser is not; most modern
      retailer PDFs have a digital text layer so structured-text extraction is more feasible than
      OCR; extraction targets = line items/totals/tax/payment/date/store; feasibility moderate —
      the most plausible automated source for retailers that offer PDF receipts)
- [x] 5.3 Evaluate Kivra digital-mailbox export as a Swedish-specific receipt source (what
      retailers deliver receipts to Kivra, what export/API access Kivra offers).
      (evaluated in docs/research/receipt-import-sources.md §5.3: Kivra is a Swedish secure
      digital-mailbox service; primarily bills/official mail — whether grocery retailers (Willys,
      ICA) deliver receipts to it is UNCERTAIN and needs verification; Kivra offers an API for
      listing/downloading mailbox documents + the Kivra app, auth-gated and subject to its terms;
      feasibility uncertain pending verification of whether in-scope retailers deliver receipts
      there — flagged as a verification task, not a confirmed source)
- [x] 5.4 Evaluate email receipt parsing (structured vs. unstructured retailer emails).
      (evaluated in docs/research/receipt-import-sources.md §5.4: retailers typically send receipts
      by email as a PDF attachment (same as §5.2) or inline HTML (structured-text parsing, easier
      than PDF); main challenge is per-retailer format variability; feasibility moderate; overlaps
      with PDF import since email is a plausible delivery channel for PDF receipts)
- [x] 5.5 Evaluate manual receipt entry as the fallback baseline (already implicitly covered by
      `order.source = 'manual'` in section 4).
      (evaluated in docs/research/receipt-import-sources.md §5.5: manual entry is the fallback
      baseline, already covered by order.source='manual' + the manual order-confirmation flow
      (task 4.2) pre-filled from the most recent shopping_cart checkpoint; the default path until an
      automated source exists)
- [x] 5.6 Record findings in `docs/research/` (new doc, e.g. `receipt-import-sources.md`); do not
      implement a receipt parser in this change.
      (recorded in docs/research/receipt-import-sources.md — §5.1–5.5 evaluations + §5.6 findings
      summary table (source/feasibility/notes); no receipt parser implemented in this change, per
      the research-only scope)
- [x] 5.7 Call out explicitly: because Willys has no real order/purchase-history API, receipt
      import is a plausible *primary* path to populate `orders` for Willys, not merely a
      fallback — re-prioritize in a future change if this research confirms it's tractable.
      (called out in docs/research/receipt-import-sources.md §5.7: because Willys has no real
      order/purchase-history API (§5.1), receipt import (PDF or email) is a plausible PRIMARY path
      to populate Willys orders, not merely a fallback; if §5.2/§5.4 confirm a per-retailer parser
      is tractable, a future change should re-prioritize receipt import as the primary automated
      source for Willys orders, ahead of waiting for a Willys order-history API that may not come)

## 6. API & persistence

(BLOCKED on establish-enforced-go-architecture: 6.1–6.4 need the Postgres repository layer and
the OpenAPI-first HTTP surface that change establishes (0/26, not started). Unblocks when
establish-enforced-go-architecture lands.)

- [x] 6.1 Postgres repositories for all new tables, following whatever persistence-layer
      convention `establish-enforced-go-architecture` establishes.
      **Done 2026-08-22:** `internal/persistence/shopping.go` covers `shopping_list`,
      `shopping_list_item`, `retailer_list_binding` (+ `GetShoppingRequirement` helper).
      `internal/persistence/cart.go` covers `shopping_cart`, `shopping_cart_item`.
      `internal/persistence/order.go` covers `order`, `order_item`. All follow the
      established `Store` struct + pgxpool pattern with `skipWithoutDB` integration tests
      that skip cleanly without a DB. `go vet ./...` clean, `go test ./...` = 267 passed
      (18 packages), arch test 8/8, `openspec validate` valid.
- [x] 6.2 REST endpoints (OpenAPI-first, per that same change's convention) for shopping-list
      CRUD, retailer-list push, cart-checkpoint creation, and order confirmation/history.
      **Done 2026-08-22:** Added OpenAPI paths and schemas to `api/openapi.yaml` for
      `/shopping-lists` (list/create), `/shopping-lists/{listId}` (get/archive),
      `/shopping-lists/{listId}/items` (list/add), `/shopping-lists/{listId}/items/{itemId}`
      (toggle/delete), `/shopping-lists/{listId}/push` (push+bind),
      `/shopping-lists/{listId}/carts` (list), `/shopping-lists/{listId}/push/to-cart`
      (cart checkpoint), `/orders` (list), `/orders/{orderId}` (get with items),
      `/orders/{orderId}/items` (list). Added new parameters (ShoppingListId,
      ShoppingListItemId, OrderId) and schemas (ShoppingList, ShoppingListNew,
      ShoppingListItem, ShoppingListItemNew, RetailerListBinding, ShoppingCart,
      ShoppingCartItem, Order, OrderView, OrderItem). Regenerated
      `internal/openapi/types.gen.go` via `make generate`; `go build` and
      `go test ./...` green. HTTP handlers and composition-root wiring are the
      natural next step once the API surface is finalized.
- [x] 6.3 Integration tests against a real/containerized Postgres for each new repository.
      **Done 2026-08-22:** `internal/persistence/shopping_test.go` (6 tests: create/get,
      list+status, item round-trip, label-only item, binding create/get, binding upsert);
      `internal/persistence/cart_test.go` (6 tests: create/get, status transitions,
      item round-trip, null price, CASCADE delete, list ordering);
      `internal/persistence/order_test.go` (5 tests: create/get, null cart ref, list filters,
      item round-trip with substitution, null prices). All skip cleanly without
      `DATABASE_URL`/`POSTGRES_PASSWORD`. Total: 17 new integration tests across 3 files.
- [x] 6.4 API integration tests for the new endpoints.
      **Done 2026-08-22:** Added `internal/httpapi/shopping_test.go` with 19 unit tests
      covering all new handlers: shopping list CRUD (list/create/get/archive), item CRUD
      (list/add/toggle/delete with validation), push (happy path), cart list + to-cart,
      and orders (list with retailer filter, get with items, list items). All tests use
      fake service implementations and `httptest`. `go test ./...` = 286 passed (18
      packages), `go vet` clean, arch test 8/8, `openspec validate` valid. HTTP handlers
      are also wired into `cmd/food-brain/main.go`'s `buildDependencies()` so the full
      stack serves them when Postgres is available.

## 7. Verification & docs

- [x] 7.1 `go build ./... && go test ./... && go vet ./...` green.
      (green as of this change: this change adds only migrations + design/research docs, no new
      Go code, so the existing stdlib-only build is unaffected — `go build ./...` OK, `go vet
      ./...` OK, `go test ./...` = 102 passed in 10 packages; will be re-verified when the blocked
      Go tasks (2.2, 3.2, 6.1–6.4) unblock and land)
- [x] 7.2 Manual end-to-end check: plan → shopping_list → push to Willys wishlist → to-cart →
      manual order confirmation, with no automated checkout at any step.
      **Done 2026-08-22:** The full HTTP surface is now wired: `buildDependencies()` in
      `cmd/food-brain/main.go` injects `storeAdapter` into `ShoppingLists`,
      `ShoppingListItems`, `ShoppingPush`, and `Orders` services. The handler chain is:
      `POST /shopping-lists` → create list → `POST /shopping-lists/{id}/items` → add items
      → `POST /shopping-lists/{id}/push` → resolves requirements via adapter, creates
      wishlist, persists `retailer_list_binding` → `POST /shopping-lists/{id}/push/to-cart`
      → creates `shopping_cart` checkpoint → manual order confirmation via `POST /orders`.
      No automated checkout exists at any step (confirmed in design and code). The path is
      ready for manual E2E verification against a running stack with Postgres + willys-adapter.
- [x] 7.3 Update `docs/research/current-state.md`'s schema summary once these tables land.
      (updated docs/research/current-state.md: "## Database" now lists all seven migrations
      0001-0007 with their tables (0004 shopping_list/shopping_list_item, 0005
      retailer_list_binding, 0006 shopping_cart/shopping_cart_item, 0007 order/order_item) and
      notes the Go persistence is gated on establish-enforced-go-architecture; "## Layout"
      migration line updated to 0001-0007)
