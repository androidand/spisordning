# ICA Adapter Design

Gates passed: tasks 1.1–1.4 in `tasks.md` (build clean, dirs resolved, live auth confirmed,
credential-hygiene issue resolved — no `.env.example` tracked on `master`).

## 2.1 Auth surface decision

`ica-client` exposes **two** auth surfaces, verified live and documented in
`src/index.ts`:

| Surface | Auth | Endpoints used by `ica-adapter` |
|---|---|---|
| **Mobile OAuth2/PKCE** (Bearer token) | DCR + PKCE + HTML-form login against `ims.icagruppen.se` | shopping lists (sync/create/delete), barcode lookup, offers, bonus, recipes, stores |
| **Anonymous ecom visitor** | No login; plain HTTP cookie jar | product search (`searchProducts`), product page detail (`getProductPage`) |
| Ecom cookie import | Browser-cookie export via `icaClient.ecomAuth.importCookies(...)` | ~~cart mutations~~, customer data, orders, favorites — **NOT used** |

**Decision: use only the mobile OAuth2/PKCE surface plus the anonymous ecom surface.
Do not use the ecom cookie-import surface.**

Rationale:
- The ecom cookie-import surface exists for cart mutations, customer data, and orders.
  `ica-adapter` implements **no checkout, no payment, no order placement** (task 2.5).
  Cart read (`getActiveCart`, `getCartStatus`) works anonymously and is not a required
  capability for the first slice.
- The mobile OAuth2/PKCE surface covers every capability `ica-adapter` needs: shopping
  list MERGE sync, barcode lookup, offers, bonus balance, recipes, stores.
- The anonymous ecom surface covers product search and product page detail — no auth
  overhead, same results as the mobile surface for these endpoints.
- The ecom cookie-import surface requires a human to export cookies from a live browser
  session (`handlaprivatkund.ica.se`). It is a friction point with no capability payoff
  for the adapter's scope.

**Auth storage:** the adapter owns the ICA session. Credentials (`ICA_USERNAME` = personal
ID, `ICA_PASSWORD` = PIN) are read from environment, the DCR+PKCE flow runs lazily on
first request, and the access token + refresh token are cached in memory. Full re-login
runs on refresh failure (≤2 retries, per `ica-client`'s own `oauth2.ts` logic).

**Store selection:** the adapter picks a home store once at session init (same pattern as
`willys-adapter`'s `ensureHomeStore()`) and reuses it for all per-store calls (product
search, offers, shopping list creation). The store is configurable via `ICA_STORE_ID` env
var; falls back to the user's favorite store if unset.

## 2.2 HTTP surface

Mirrors `willys-adapter`'s exact shape. Same routes, same semantics, same `Resolution`
shape. ICA-specific routes only where ICA's capability genuinely differs.

| Route | Method | Purpose | Auth surface |
|---|---|---|---|
| `/health` | GET | Liveness probe | none |
| `/search` | GET | Free-text product search | anonymous ecom |
| `/products/:code` | GET | Product detail by barcode/EAN | anonymous ecom |
| `/barcode/:ean` | GET | Barcode → product lookup (ICA-specific) | mobile OAuth2 |
| `/offers` | GET | Current offers for home store | mobile OAuth2 |
| `/bonus` | GET | Bonus balance + voucher summary (ICA-specific) | mobile OAuth2 |
| `/shopping-lists` | GET | List shopping lists | mobile OAuth2 |
| `/shopping-lists` | POST | Create shopping list | mobile OAuth2 |
| `/shopping-lists/:id` | DELETE | Delete shopping list | mobile OAuth2 |
| `/shopping-lists/:id/sync` | POST | MERGE-sync a shopping list | mobile OAuth2 |
| `/resolve` | POST | Resolve requirement → product | anonymous ecom (search) + mobile (barcode fallback) |
| `/pins` | GET | List pins | none |
| `/pins` | POST | Add/update pin | none |
| `/review/queue` | GET | List needs-review terms | none |
| `/review/:term` | DELETE | Dismiss a queued term | none |
| `/review` | GET | Review & pick HTML page | mobile OAuth2 |

**Notes:**
- `/barcode/:ean` is ICA-specific because Willys does not have a dedicated barcode
  endpoint — Willys product codes *are* the lookup key, passed via `/products/:code`.
  ICA separates barcode (EAN) lookup from product-page lookup; both feed the same
  resolution pipeline.
- `/bonus` is ICA-specific — Willys has no equivalent bonus/voucher endpoint.
- `/search` and `/products/:code` use the anonymous ecom surface (no auth overhead,
  same results). If the anonymous surface ever returns insufficient data, the adapter
  falls back to the mobile OAuth2 surface transparently.
- `/resolve` uses the same `Resolution` shape as `willys-adapter` (`matchType`,
  `confidence`, `needsReview`, `quantityUncertain`) — see task 2.4.
- `/pins` and `/review/queue` are adapter-local (in-memory pin store + review queue,
  same as `willys-adapter`). Pins survive restarts via atomic file write.
- No `/shopping-lists/:id/to-cart` equivalent — no cart mutations.

## 2.3 Shopping-list sync strategy

**Conflict-resolution mode: `MERGE`.**

ICA's `syncShoppingList` accepts `IcaShoppingListSync` with three delta arrays:

```ts
interface IcaShoppingListSync {
  offlineId: string;
  createdRows?: IcaShoppingListEntry[];
  changedRows?: IcaShoppingListEntry[];
  deletedRows?: string[];  // offlineIds of rows to remove
}
```

**Decision: always send `createdRows` + `deletedRows`. Never send `changedRows`.**

Rationale:
- Spisordning's list is the source of truth. We never modify an existing ICA row in-place;
  we delete the old row and create a new one with the updated content. This avoids
  ambiguity about which side "owns" a changed row.
- `MERGE` conflict mode (as opposed to `APPEND` or `IGNORE`) tells the ICA server to
  apply our deltas on top of whatever the server currently has, resolving conflicts by
  preferring the client's deltas. This is the correct semantics for a Spisordning-driven
  list: if the user edited the ICA list directly, our next sync overwrites those edits
  with Spisordning's canonical state (the user can always re-sync from Spisordning).
- `APPEND` would duplicate rows on every sync. `IGNORE` would silently discard our
  changes. `MERGE` is the only mode that correctly expresses "this is the canonical
  state."

**Sync protocol:**
1. `GET /shopping-lists` → find or create the Spisordning-owned list (matched by
   `offlineId`, which is the Spisordning household+week identifier).
2. Compare Spisordning's current requirement set against the ICA list's rows.
3. Build `createdRows` (new rows), `deletedRows` (rows whose product is no longer
   required), and `changedRows` (empty — we delete+recreate instead).
4. `POST /shopping-lists/{offlineId}/sync` with the delta.
5. On sync response: if the server returns `data`, use it. If not, fall back to
   `getShoppingList(offlineId)`. If that also fails, trust the sent delta (known
   flaky immediate-consistency issue, documented in `ica-client`).

**Row mapping:** each Spisordning requirement (after resolution) maps to one
`IcaShoppingListEntry`:
- `productName` ← resolved product name
- `productEan` ← resolved retailer product code (EAN/barcode)
- `quantity` ← resolved quantity (packages × package size, converted to ICA's unit)
- `unit` ← ICA unit string (from product detail or requirement unit)
- `id` ← `null` for new rows (server assigns)
- `offlineId` ← `null` for new rows

**New-row id rule:** per `ica-client`'s live-verified behaviour, `id` must be `0` (not
`null`) when constructing a new row for a write. The type allows `number | null`
because GET responses can omit it, but callers constructing a new row for a write
must use `0`.

## 2.4 Resolution shape

**Unchanged from `willys-adapter`.** The `Resolution` interface carries over exactly:

```ts
interface Resolution {
  ingredientId: string;
  retailerProductId: string | null;  // ICA EAN/barcode
  productName?: string;
  packages: number;
  resolvedQuantity: number | null;
  matchType: 'pinned' | 'pinned-backup' | 'exact' | 'fuzzy' | 'none';
  confidence: number;                // [0,1], name-match quality only
  needsReview: boolean;
  quantityUncertain: boolean;
}
```

No ICA-specific weakening:
- `matchType` values are the same. ICA barcode lookup produces `exact` matches (the
  barcode is the product identifier); ICA product search produces `fuzzy` matches
  scored the same way as Willys.
- `confidence` reflects name-match quality only. Package-size uncertainty sets
  `quantityUncertain: true` but does not lower confidence.
- `needsReview` threshold is the same (`REVIEW_THRESHOLD = 0.7`).
- `retailerProductId` is the ICA EAN/barcode (not an internal ICA article ID), keeping
  retailer product identity distinct from the canonical `ingredient`.

The resolution pipeline is the same as `willys-adapter`'s:
1. Check pin store (term → ICA EAN).
2. If pinned and primary available → `matchType: 'pinned'`, confidence 1.0.
3. If pinned primary unavailable and backup available → `matchType: 'pinned-backup'`.
4. Fuzzy search via anonymous ecom product search.
5. If best candidate confidence < 0.7 → `needsReview: true`.
6. Barcode lookup as fallback for resolved requirements that have an EAN but no search
   hit (e.g. scanned items).

## 2.5 No checkout / payment / order endpoint

**Re-affirmed invariant.** `ica-client`'s cart service exposes cart item CRUD
(`addToCart`, `updateQuantity`, `removeFromCart`, `setDeliveryAddress`) behind the
ecom cookie-import surface. `ica-adapter` **does not implement any of these**.

The adapter's terminal output is a durable per-week shopping list synced to ICA via
the MERGE endpoint. The user manually converts the ICA shopping list to a cart in the
ICA app — payment and slot booking remain human actions, exactly as with Willys.

This invariant is enforced by design (no cart-mutation routes in the HTTP surface,
see task 2.2) and by the `ica-adapter` capability spec in
`openspec/specs/ica-adapter/spec.md`.
