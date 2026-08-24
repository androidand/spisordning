# ica-adapter design

## 1. Auth surface decision (task 2.1)

ICA exposes **three** auth surfaces, verified live in `ica-client` `src/index.ts` and each
service's own docstring. The adapter MUST treat them as separate, non-interchangeable
machines — not assume one covers everything.

### Surface map

| Surface | Mechanism | What it unlocks | Adapter use |
|---|---|---|---|
| **Anonymous** | No login; plain visitor session on `handlaprivatkund.ica.se` | `product.searchProducts`, `product.getProductPage`, `cart.getActiveCart`, `cart.getCartStatus` | `/search` (free-text product search) — **no auth needed** |
| **Mobile OAuth2/PKCE** | DCR + auth-code + PKCE + HTML-form login (personal-ID + PIN) against `ims.icagruppen.se`; Bearer token on `apimgw-pub.ica.se` | `shoppingList.*`, `product.lookupByBarcode`, `recipe.*`, `store.*`, `offer.*`, `bonus.*`, `getArticles`, `getArticleGroups` | `/offers`, `/shopping-lists`, barcode lookup, bonus balance — **required** |
| **Ecom cookie-session** | Browser-cookie import (Playwright-assisted live login); Akamai+WAF blocks automated login | `customer.*`, `graphql.*`, `favorites.*`, cart writes (`addToCart`/`updateQuantity`/`removeFromCart`/`setDeliveryAddress`) | **Not used by the adapter.** The adapter never writes cart items or reads customer profile. The cookie session is a hard limitation (bot-detection bypass not attempted) and none of the adapter's capabilities depend on it. |

### Decision

**Use anonymous + mobile OAuth2 only. Skip ecom cookie-session entirely.**

Rationale:
- Every capability the adapter needs (`/search`, `/offers`, `/shopping-lists`, `/resolve`, `/pins`, `/review/queue`, barcode lookup, bonus balance) is covered by the anonymous or mobile surfaces. `/products/:code` and barcode lookup both use the mobile surface (`product.lookupByBarcode`).
- The ecom surface gates cart writes and customer data — neither is in scope. The adapter never places orders (inherited invariant, task 2.5).
- The ecom surface requires a human to log in via a visible browser window (Playwright), which is incompatible with a headless adapter service. Even with cached cookies, `global_sid` expires server-side after ~9 hours, so the browser would reopen frequently.
- `ica-client`'s own `test.ts` confirms this split: 29/33 tests pass (all mobile-surface endpoints green); the 4 failures are exactly the ecom-surface endpoints (`getCustomer`, `getDeliveryDestinations`, `getOrders`, `graphql`).

### Session model

The adapter owns a single `OAuth2Auth` instance (mobile surface) for the lifetime of the
process. Session state (client registration, access token, refresh token, user info) is
kept in-memory; on restart the full DCR + PKCE + HTML-form flow runs again from
`ICA_USERNAME` (personal ID) + `ICA_PASSWORD` (PIN) stored encrypted at rest.

Token refresh: on 401, retry once with `refresh_token`; on refresh failure, full re-login
(re-register app + re-auth). This mirrors `ica-client`'s own `OAuth2Auth.ensureLogin()`
logic.

Token lifetime: `ica-client` declares `expires_in: 2592000` (30 days) but a 2025 issue
comment reports tokens expiring "on the order of minutes." The adapter SHALL treat the
token as short-lived and refresh aggressively on 401, not assume the 30-day figure is
correct.

### Credential storage

ICA credentials are **personal ID + PIN** — high-sensitivity PII. The adapter SHALL:
- Store them encrypted at rest (same scheme as existing retailer adapter credentials).
- Never log them or expose them in error messages.
- Require them only at adapter startup; never transmit them outside the adapter process.

This is consistent with `retailer-adapter`'s existing credential handling for Willys.

---

## 2. HTTP surface (task 2.2)

Mirrors `willys-adapter`'s exact shape, adding ICA-specific routes only where ICA's
capability genuinely differs.

### Endpoints

| Route | Method | Purpose | ICA-specific note |
|---|---|---|---|
| `/search` | POST | Free-text product search | Uses **anonymous** surface (`product.searchProducts`). No auth needed. Returns same shape as Willys search. |
| `/products/:code` | GET | Product detail by barcode/EAN | Uses **mobile** surface (`product.lookupByBarcode`). Returns `IcaProduct` mapped to adapter's product shape. |
| `/offers` | GET | Store offers / promotions | Uses **mobile** surface (`offer.getStoreOffers`). Filters by household's pinned store. |
| `/shopping-lists` | GET | List user's shopping lists | Uses **mobile** surface (`shoppingList.getShoppingLists`). |
| `/shopping-lists/:id` | GET | Get specific list | Uses **mobile** surface (`shoppingList.getShoppingList`). |
| `/shopping-lists/:id/sync` | POST | Sync (MERGE) a list | Uses **mobile** surface (`shoppingList.syncShoppingList`). See §3 for sync strategy. |
| `/shopping-lists` | POST | Create a new list | Uses **mobile** surface (`shoppingList.createShoppingList`). |
| `/shopping-lists/:id` | DELETE | Delete a list | Uses **mobile** surface (`shoppingList.deleteShoppingList`). |
| `/resolve` | POST | Resolve a term to a product | Uses **anonymous** search first, falls back to **mobile** barcode if term is a barcode. Pins checked first (local store). |
| `/pins` | GET | List pins | Local store, no retailer auth. |
| `/pins` | POST | Add/update a pin | Local store, no retailer auth. |
| `/pins/:term` | DELETE | Remove a pin | Local store, no retailer auth. |
| `/review/queue` | GET | List terms needing review | Local store, no retailer auth. |
| `/review/queue/:term` | POST | Pick a product for a queued term | Local store, no retailer auth. Creates pin, clears queue. |
| `/bonus` | GET | Current bonus balance | Uses **mobile** surface (`bonus.getCurrentBonus`). ICA-specific — Willys has no analogue. |
| `/barcode/:code` | GET | Barcode lookup (explicit) | Uses **mobile** surface (`product.lookupByBarcode`). ICA-specific — Willys uses `/products/:code` for the same thing. Kept separate because ICA's barcode endpoint is the primary product-identity path. |

### Shape decisions

- `/search` and `/products/:code` return the **same shape** as `willys-adapter` (product code, name, barcode, price, availability). ICA-specific fields (e.g. `articleGroup`) are included but not required by consumers.
- `/bonus` returns `{ balance, vouchers, discountSummary }` — ICA-specific, not mirrored from Willys.
- `/barcode/:code` is a dedicated route because ICA's barcode lookup is a first-class capability (unlike Willys where barcode search is folded into `/search`). Spisordning's barcode intake flow can call this directly.

---

## 3. Shopping-list sync strategy (task 2.3)

### ICA's MERGE semantics

ICA's `syncShoppingList` accepts an `IcaShoppingListSync` body with three delta arrays:
- `createdRows`: new rows (id must be `0`, not `null` — server rejects null with 400)
- `changedRows`: modified existing rows
- `deletedRows`: rows to remove

The server merges these into the existing list. This is a **delta-based, optimistic-concurrency** protocol — no lock, no version number, just the delta.

### Conflict-resolution mode: `MERGE` with row-ID tracking

The adapter SHALL use **MERGE mode** with in-memory row-ID tracking. Semantics:
- Rows in `createdRows` are added; the server reconciles duplicates rather than blindly appending.
- Rows in `changedRows` replace the matching row by ID.
- Rows in `deletedRows` are removed by ID.

**Why MERGE with row-ID tracking, not APPEND:**
- APPEND mode blindly adds all `createdRows`, creating duplicates on every full re-push.
- MERGE mode reconciles the full state: the server removes rows not in the delta and adds/updates rows that are. This is the correct behavior when Spisordning is the source of truth and pushes the full current state.
- MERGE does not "silently drop" ICA-side edits in the sense the review feared — it replaces the entire list with the Spisordning state, which is the correct behavior for a push-only adapter where Spisordning owns the list.

### Row-ID tracking

The adapter maintains an in-memory `rowIDMapping`: `map[string]int` mapping Spisordning item
identifiers (computed as `label + "|" + quantity + "|" + unit`) to ICA row IDs from the last
successful sync. On each sync:

1. Fetch the current ICA list (via `shoppingList.getShoppingList`) to get existing row IDs.
2. Build the mapping: for each existing ICA row, map its item key to its server-assigned ID.
3. Compute deltas:
   - Items in Spisordning state but not in the mapping → `createdRows` (id: 0)
   - Items in both but with different label/quantity/unit → `changedRows` (with tracked ID)
   - Items in the mapping but not in Spisordning state → `deletedRows` (with tracked ID)
4. Send deltas to `syncShoppingList`.
5. On success, update the mapping with the response's row IDs.
6. On failure, keep the old mapping (stale but consistent with what the server last accepted).

**Restart caveat:** the mapping is in-memory only. On adapter restart, the mapping is lost.
The first sync after restart fetches the current ICA list and rebuilds the mapping, so the
first post-restart sync produces accurate deltas (no duplicates). This is a known limitation
documented in the adapter's operational runbook.

### Sync flow

1. `food-brain` calls `POST /shopping-lists/:id/sync` with the full list state.
2. Adapter fetches current ICA list to build row-ID mapping.
3. Adapter computes proper deltas (`createdRows`/`changedRows`/`deletedRows`).
4. Adapter calls `icaClient.shoppingList.syncShoppingList()` with deltas in MERGE mode.
5. On success, adapter returns the persisted list and updates the row-ID mapping.
6. On failure, adapter returns 500 with the error; mapping is unchanged.

### Limitation

The adapter does NOT implement two-way sync. ICA-side changes (user edits in the ICA app)
are not pulled back into Spisordning. The list is push-only from Spisordning to ICA.
Two-way sync is a future enhancement, not part of this change.

---

## 4. Resolution shape (task 2.4)

The `Resolution` shape carries over **unchanged** from `willys-adapter`:

```typescript
interface Resolution {
  matchType: "pinned" | "pinned-backup" | "search" | "barcode";
  confidence: number;       // 0..1, reflects name-match quality only
  needsReview: boolean;     // true when confidence is below threshold or pin is broken
  quantityUncertain: boolean; // true when package size cannot be reconciled
  packages: number;         // safe package default (1) when uncertain
  productCode: string;      // ICA product barcode/EAN
  productName: string;
  retailer: "ica";
}
```

No ICA-specific weakening:
- `matchType` gains a fourth value `"barcode"` (when the input term is a barcode and
  `product.lookupByBarcode` succeeds directly, bypassing search). This is an improvement
  over Willys, not a weakening.
- `confidence` is scored on the same name-match principle: strong lexical match = high
  confidence, weak match = low confidence. Size mismatch does NOT lower confidence
  (per `retailer-adapter`'s existing rule — size uncertainty is expressed via
  `quantityUncertain` instead).
- `needsReview` threshold is the same as Willys.
- `retailer: "ica"` distinguishes ICA resolutions from Willys resolutions in the
  consumption path.

---

## 5. No-checkout reaffirmation (task 2.5)

**Explicitly re-affirmed:** the adapter SHALL NOT implement any endpoint that places an
order, initiates payment, or books a delivery slot.

This holds regardless of what `ica-client`'s cart service technically permits:
- `ica-client`'s cart service currently exposes `addToCart`/`updateQuantity`/`removeFromCart`
  (write operations gated behind ecom cookie-session auth) and `getActiveCart`/`getCartStatus`
  (anonymous reads).
- ICA's web storefront also has order endpoints (`/stores/{storeId}/api/order/v6/orders`)
  but these are **not** exposed by `ica-client`'s cart service and are **not** in scope.
- The adapter's `/shopping-lists` endpoint creates and syncs **shopping lists**, not carts.
  A shopping list is a planning artifact; a cart is a purchase intent. The adapter never
  transitions from one to the other.

This invariant is inherited from `retailer-adapter` and `ica-integration` (see
`openspec/specs/retailer-adapter/spec.md` and
`openspec/changes/research-and-integrate-ica/specs/ica-integration/spec.md`). It is
re-affirmed here, not re-derived.
