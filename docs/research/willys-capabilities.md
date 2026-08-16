# Willys client — capability map

`PLAN.md`'s "Existing Willys Client" section asks for exactly this document before designing
any retailer interface. Source: `~/dev/willys/willys-client` (TypeScript), read directly —
not inferred.

Two related repos exist under `~/dev/willys/`:

- **`willys-client`** — the real, wired-in client + adapter (this document).
- **`willys-mcp`** — an older, separate Next.js + MCP-server exploration (puppeteer auth,
  SQLite caching, embeddings). Not referenced by spisordning or by `willys-client`. Worth
  knowing it exists so it isn't rediscovered mid-project, but out of scope here.

## `willys-client` (`src/`) — per-domain services

| Capability | Supported? | Where |
|---|---|---|
| Authentication | Yes | `src/auth/service.ts` — client-side credential encryption, CSRF refresh after login. |
| Store selection | Yes | `src/store/service.ts` — `activateStore`, `getActiveStore`, `ensureActiveStore`. Price/campaign queries are store-scoped. |
| Product search | Yes | `src/search/service.ts` — `searchProducts(query, {page, size, sort, filters})`. |
| GTIN/EAN/barcode lookup | **No** | Confirmed absent — zero matches for `ean`/`barcode`/`gtin` anywhere in `src/` or `apps/`. |
| Product detail by code | Partial | Only via the adapter, hitting the raw v1 endpoint directly (`GET /axfood/rest/v1/p/:code`) — not wrapped in the typed client. |
| Prices | Partial | Fields on search-result `Product` objects only (`price`, `priceValue`, `comparePrice`); no standalone price-history service. |
| Campaigns/promotions | Yes (adapter-level, raw endpoint) | Adapter's `GET /campaigns` hits the v1 campaigns endpoint directly. `GET /promotions/:code/products` expands "Välj & blanda" variant families. |
| Shopping list (wishlist) | Yes, thoroughly | `src/wishlist/service.ts` — create/get/rename/delete/share, add items with increment. This is the adapter's primary durable output (Willys has no standalone basket object). |
| Cart | Yes | `src/cart/service.ts` + delivery/slot/shipping services — add/set/remove/clear, delivery address, delivery mode. |
| Checkout | **No, deliberately** | No checkout/payment/slot-booking code anywhere. Architecturally enforced — BankID/Klarna payment stays a human action. |
| Orders / purchase history | **Essentially no — untyped stub only** | `src/personal/service.ts` calls `/personalElementList`/`/personalElement`; response typed as `{ order: any; digitalReceipt: any }` — known to exist, unused, untyped. No real order-history retrieval or receipt parsing. |
| Voucher | Yes (minimal) | `src/voucher/service.ts` — `getAllExternalVouchers`. |
| Menu/category tree | Yes | `src/menu/service.ts`. |
| Customer profile | Yes | `src/customer/service.ts`. |

## The `willys-adapter` HTTP service (`apps/willys-adapter/{server.ts,core.ts,pins.ts,reviewQueue.ts}`)

This is what spisordning's `internal/retailer` actually talks to. Routes:

```
GET    /health
GET    /search
GET    /products/:code
GET    /campaigns
POST   /resolve                       — core requirement→product resolution
GET    /pins
POST   /pins
GET    /review/queue
GET    /review                        — server-rendered picker page
DELETE /review/:term
GET    /promotions/:code/products     — promo variant expansion
POST   /shopping-lists                — create/extend a named wishlist (additive)
POST   /shopping-lists/:id/to-cart    — explicit, separate cart-fill step
```

`core.ts` (pure, unit-tested): Swedish display-volume parsing (`"500g"`/`"1,5 l"`/`"6-p"`),
name/size-hint splitting, pin-aware resolution producing a `Resolution` with
`matchType: 'pinned' | 'pinned-backup' | 'exact' | 'fuzzy' | 'none'`, `confidence`,
`needsReview`, `quantityUncertain`. `REVIEW_THRESHOLD = 0.7`.

## Bottom line

Auth, store selection, search, wishlist, cart (incl. delivery/slot/shipping), config, customer,
menu, and voucher are solid. The adapter adds a sophisticated, live-verified resolution/pinning/
review/size-matching layer on top. **Gaps**: no GTIN lookup, no real order/purchase-history
retrieval, no checkout (intentional). Any new shopping/commerce or receipt-import work
(`implement-shopping-and-commerce`, receipt research) must account for these gaps rather than
assume they're covered.
