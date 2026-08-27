## Context

Three things converge here: (1) the shopping pipeline exists but stops at spisordning's HTTP API,
unreachable from MCP/chat; (2) no code anywhere compares price across retailers, and the one field
needed to start (price on a resolution) is missing from the Willys adapter's response even though
the adapter tracks it internally; (3) the household's real ingredient list is an Apple Note on a
Mac, read today by a standalone script that only knows about Willys and never talks to spisordning
at all. spisordning itself has no deployment target yet (see `deploy-food-brain-to-proxmox`), which
bounds how far the notes-bridge work can go in this change.

## Goals / Non-Goals

**Goals:**
- An MCP tool call can take a set of shopping requirements, resolve them against Willys and ICA
  (Hemköp once its adapter exists), and report the cheapest available match per item with enough
  info (retailer, product name, price, confidence) for a person to sanity-check before pushing.
- An MCP tool call can push a chosen set of resolutions to a wishlist on the chosen retailer.
- The Apple Notes → shopping-list path is designed precisely enough to implement once
  `deploy-food-brain-to-proxmox` lands, even if the Mac-side script itself isn't fully rewritten in
  this change.
- ICA's session staleness is surfaced, not silently swallowed — a stale ICA session degrades the
  comparison (report Willys/Hemköp only, flag ICA as unavailable) rather than failing the whole
  request.

**Non-Goals:**
- No `to-cart`, checkout, payment, or delivery-slot automation — the MCP tool surface stops at
  wishlist creation, matching the user's explicit scope decision.
- No new external price-comparison-site integration (Matpriskollen/Matmoms) — those require business
  outreach not yet done; out of scope until that outreach resolves (`docs/research/swedish-price-
  data.md`).
- No Hemköp adapter service in this change — `hemkop-client` (an `axfood-client` subclass) exists,
  but there's no `hemkop-adapter` HTTP service yet; this change should not block on building one.
  Price comparison ships Willys+ICA now, with Hemköp added as a follow-up once its adapter exists
  (same shape as `internal/retailer.NewFromKind`, which already anticipates a third retailer kind).

## Decisions

**D1 — Add price to Willys's `/resolve` response rather than a separate price-lookup call.**
`willys-adapter/core.ts` already computes `priceValue`/`price` per candidate for its review-queue
UI (`server.ts:203`); the resolve handler just needs to include it in the JSON it already returns.
Alternative considered: a separate `/price/:productId` endpoint spisordning calls after resolving —
rejected as an extra round-trip for data the adapter already has in hand at resolve time.

**D2 — Price comparison lives in spisordning (Go), not in either adapter.** Each adapter stays a
single-retailer client; spisordning's `internal/retailer` (or a new small `internal/pricecompare`
package) calls both adapters' resolve endpoints for the same requirement set and picks the cheapest
per item. Alternative considered: teaching one adapter to call the other — rejected, adapters own
one retailer's session/auth model each and should not know about each other.

**D3 — ICA staleness degrades gracefully via an explicit "unavailable" result, not a hard error.**
`internal/icaretailer.Client.Resolve` errors (auth failure, timeout) are caught by the comparison
step and turned into a per-retailer `available: false` marker in the comparison output, rather than
failing the whole comparison. This matches the user's stated operational reality (ICA needs
occasional manual re-login) rather than pretending it's as reliable as Willys/Hemköp.

**D4 — MCP tools stop at wishlist creation; `to-cart` stays HTTP-API-only.** `internal/mcptools`
gets tools for: creating a shopping list from requirements, comparing price, and pushing the chosen
resolutions to a wishlist (`POST /shopping-lists`, already implemented in `internal/retailer`/
`internal/icaretailer`). The existing `ToCart` client method is deliberately not wrapped in a new
MCP tool in this change — revisit in the deferred cart/checkout change.

**D5 — The notes-bridge target is a new spisordning HTTP endpoint, not a rewrite of the adapter's
resolve/wishlist endpoints.** The Mac-local script keeps doing exactly what it does today (osascript
read → parse checklist), but instead of (or in addition to) POSTing straight to `willys-adapter`, it
POSTs the parsed items to a new spisordning endpoint (e.g. `POST /shopping-lists/from-checklist`)
that creates a `shopping_list` + items, then the household drives price-comparison/wishlist-push
from there (via MCP chat or the same endpoint chaining). This keeps the existing single-retailer
Willys-only note flow (the merged `retailer-adapter` spec's "Apple Notes checklists drive resolution
through the adapter" requirement) working unmodified for anyone still using it directly, while adding
the cross-retailer path as a new, separate capability. Full implementation of the Mac-side script
change is gated on `deploy-food-brain-to-proxmox` (task 4 below is design/stub only until that
lands).

**D5a — Concrete `POST /shopping-lists/from-checklist` contract + localhost stub.** The endpoint
takes the parsed checklist in one call and returns the created list plus its items, so the caller
needs no second fetch:

```jsonc
// request
{ "name": "Köp Mat Andreas",
  "items": [ { "label": "Mjölk", "quantity": 1, "unit": "liter" },
             { "label": "Ägg",   "quantity": 6, "unit": "st" } ] }
// response (201 Created)
{ "id": 12, "name": "Köp Mat Andreas", "status": "active", "created_at": "…",
  "items": [ { "id": 100, "shopping_list_id": 12, "label": "Mjölk", "quantity": 1, "unit": "liter", "checked": false, "added_at": "…" } ] }
```

It is a thin convenience over `persistence.CreateShoppingList` + N×`CreateShoppingListItem` (the
same calls `POST /shopping-lists` and `POST /shopping-lists/{id}/items` already make), so no new
schema is introduced. The Mac-local stub lives in the sibling `willys-client` repo at
`apps/notes-sync/spisordning-bridge.ts` (`npm run notes:spisordning[:apply]`): it reuses the
existing `notes.ts` osascript reader and `core.ts` checklist parser, maps unchecked items to the
shape above (unit defaults to `st`), and POSTs to `SPISORDNING_URL` (default
`http://localhost:8080`, the compose-exposed food-brain port). It is dry-run by default and
deployment-gated — once `deploy-food-brain-to-proxmox` lands, point `--url`/`SPISORDNING_URL` at
the real host and pass `--apply` to activate the live cross-retailer notes path. The existing
Willys-only `bridge.ts` flow is untouched.

## Risks / Trade-offs

- [Willys-adapter price changes are a cross-repo dependency spisordning doesn't control the release
  cadence of] → Mitigation: land the adapter-side change first as its own small PR in
  `willys-client`, verify live, then build spisordning's comparison logic against the confirmed
  response shape — don't guess the JSON shape ahead of time.
- [ICA's second-auth/cookie-expiry issue is user-reported, not yet reproduced/verified in
  `ica-adapter` code] → Mitigation: task 1 includes verifying the actual failure mode
  (`ica-adapter/core.ts`/`server.ts`) before building the graceful-degradation path, so D3 handles
  the real error shape, not a guessed one.
- [Hemköp being unavailable this change means "cheapest across stores" is really "cheapest of two
  stores" until a `hemkop-adapter` exists] → Mitigation: state this limitation plainly in the tool
  description so an AI chat doesn't overclaim completeness to the user; track the Hemköp adapter as
  explicit follow-up work, not silently assumed.
- [A resolution's price is a point-in-time value (like `shopping_cart_item.resolved_price` already
  is per `implement-shopping-and-commerce`'s design) — it can go stale between comparison and
  push] → Mitigation: no mitigation needed in this change; this is the same "checkpoint, not live
  price" model `implement-shopping-and-commerce` already accepted for cart items — reuse that
  precedent rather than inventing a live-price guarantee.

## Migration Plan

1. Land the Willys-adapter price field (sibling repo, small/isolated).
2. Land spisordning's `Resolution` type extension + price-comparison logic against the confirmed
   adapter response.
3. Land the new MCP tools (additive — no existing tool behavior changes).
4. Design/stub the notes-bridge endpoint; full Mac-script rewrite follows once
   `deploy-food-brain-to-proxmox` gives it a real target to call.
5. No rollback complexity: all additions are additive (new fields default-absent, new endpoints/
   tools are new surface, nothing existing is removed or renamed).

## Open Questions

- Confirm the willys-adapter's actual `/resolve` JSON shape once price is added (field name,
  currency handling, price-per-unit vs. price-per-package) before finalizing spisordning's
  `Resolution` struct change.
- Confirm ICA's actual second-auth failure mode (HTTP status, error body) from `ica-adapter` source
  before finalizing D3's degradation logic.
