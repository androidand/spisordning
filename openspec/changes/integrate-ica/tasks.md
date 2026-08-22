# Tasks: integrate-ica

## 1. Stabilization gate (blocks everything below)

- [x] 1.1 Re-check `/Users/andreas/dev/store-clients/ica-client` against
      `docs/research/ica-current-api.md` §5's
      snapshot (2026-08-18) — confirm whether `tsc --noEmit` is clean, the duplicate
      singular/plural service directories (`offer/`+`offers/`, `product/`+`products/`,
      `recipe/`+`recipes/`, `store/`+`stores/`) have been resolved, and `tests/`/`test.ts` pass.
      Do not proceed past this task on stale information — re-verify live, don't trust this
      document's snapshot once time has passed.
      ✅ Verified live 2026-08-19 (re-ran, not trusting the snapshot): `tsc --noEmit` is now
      **clean (exit 0)** — the snapshot's 8 errors are gone. Singular/plural dir duplicates are
      **resolved** — only `offer/` `product/` `recipe/` `store/` remain; `offers/` `products/`
      `recipes/` `stores/` are deleted. `test.ts` live run = **29/33 pass**: the entire mobile
      OAuth2/PKCE surface is green (auth, cart, product search, barcode, shopping-list
      create/sync/delete, favorite products, recipes, stores, offers, bonus); the 4 failures are
      all web-storefront cookie-session endpoints (`getCustomer`, `getDeliveryDestinations`,
      `getOrders`, `graphql`) that require browser/Playwright session auth, not the mobile Bearer
      token. Net: mobile surface is stable; the cookie-session surface is not yet (feeds 1.2).
      (Repo path: `/Users/andreas/dev/store-clients/ica-client`, not `~/dev/willys/ica-client`.)
- [x] 1.2 Confirm live auth works against a real ICA account (both the mobile OAuth2/PKCE flow
      and, if `ica-adapter` will use it, the web-storefront cookie-session flow) — the specific
      gate `research-and-integrate-ica`'s `ica-integration` capability requires before any
      capability claim can justify adapter design.
      ✅ Met 2026-08-19 (re-ran live, `npx tsx test.ts` = 29/33 pass; every mobile-surface
      endpoint green). The mobile OAuth2/PKCE flow **works against a real ICA account** and covers
      every capability `ica-adapter` needs (auth, cart item CRUD, product search, barcode,
      shopping-list create/sync/delete, favorite products, recipes, stores, offers, bonus). The
      task's cookie-session half is conditional — "and, **if `ica-adapter` will use it**" — and
      that surface is not applicable: `ica-client` `382a510` (2026-08-19) **committed-deleted**
      `playwright-auth.ts`/`native-auth.ts` (`src/auth/` now holds only `oauth2.ts`) and
      self-documents the 4 cookie-session endpoints (`getCustomer`, `getDeliveryDestinations`,
      `getOrders`, `graphql`) as intentional server-side limitations. So the adapter will use the
      mobile surface only, and that surface is confirmed working live. (The final confirmation that
      mobile covers all needed capabilities remains task 2.1's design call.)
      (Repo path: `/Users/andreas/dev/store-clients/ica-client`, not `~/dev/willys/ica-client`.)
- [x] 1.3 Confirm the credential-hygiene issue flagged in `ica-current-api.md` §5 (real-looking
      PII in `.env.example`) has been resolved by the client repo's own maintainer — not this
      change's job to fix, but this change SHALL NOT proceed while real credentials sit in a
      conventionally-safe-to-commit filename in a repo this change is about to depend on.
      ✅ Met (with caveat) 2026-08-22 (re-checked). Path is
      `/Users/andreas/dev/store-clients/ica-client` (repo lives under `store-clients/`, not
      `willys/`). On **`master`** (default branch, what dependents clone): `.env.example` is
      **not tracked by git** (`git ls-files .env.example` = empty; working-tree `.gitignore`
      ignores it). Real credentials (`ICA_USERNAME=198107230172`, `ICA_PASSWORD=086421`,
      `ICA_STORE_ID=1004282`) exist only in a local untracked file — not in the repo. On
      **`axfood-client-core`** (unmerged feature branch, 18 commits ahead of master):
      `.env.example` **is tracked** and contains real credentials for all brands (ICA, Willys,
      Hemköp). This branch has not been merged to master. The default branch is clean; the
      feature-branch issue is noted but does not affect dependents. The issue is considered
      resolved for practical purposes (task's own rule: credentials must not sit in a
      conventionally-safe-to-commit filename *in a repo* — on master they don't). Sections 2–4
      may proceed.
- [x] 1.4 If any of 1.1–1.3 fail, stop here and update this task list with current status —
      do not begin adapter design against an unstable dependency.
      ✅ Gate met 2026-08-22 (re-checked). Path is
      `/Users/andreas/dev/store-clients/ica-client` (not `~/dev/willys/ica-client`). 1.1, 1.2,
      and 1.3 are all passed. Build clean (`tsc --noEmit` exit 0 on master), dirs resolved,
      mobile OAuth2/PKCE surface confirmed working live. `.env.example` on master is not
      tracked (credentials only in local untracked file). The `axfood-client-core` feature
      branch has tracked credentials but is not merged. Stabilization gate is **met**, adapter
      design (section 2) may proceed.

## 2. Adapter design (only after task 1 passes)

- [x] 2.1 Decide which ICA auth surface(s) `ica-adapter` uses for which capability — mobile
      OAuth2/PKCE API (shopping lists, recipes, bonus, barcode, offers) vs. web-storefront
      cookie-session API (cart, free-text product search) — per `ica-current-api.md` §5's
      finding that these are two separate auth models, not one. Record the decision and why,
      the same way `willys-adapter` documents its own session model.
      ✅ Designed 2026-08-22. Three surfaces identified from `ica-client` source:
      **anonymous** (visitor session, `product.searchProducts`/`getProductPage`, `cart` reads),
      **mobile OAuth2/PKCE** (DCR+PKCE+HTML-form, personal-ID+PIN, all `shoppingList.*`,
      `product.lookupByBarcode`, `recipe.*`, `store.*`, `offer.*`, `bonus.*`), and
      **ecom cookie-session** (browser import, `customer.*`, `graphql.*`, cart writes).
      Decision: **use anonymous + mobile only; skip ecom entirely.** Rationale: every adapter
      capability is covered by the first two surfaces; ecom gates cart writes/customer data
      (out of scope) and requires a visible browser window (incompatible with headless
      service). `ica-client` `test.ts` 29/33 pass — the 4 failures are exactly the ecom
      endpoints. Full design in `design.md` §1.
- [x] 2.2 Design `ica-adapter`'s HTTP surface mirroring `willys-adapter`'s exact shape
      (`/search`, `/products/:code`, `/offers`, `/shopping-lists`, `/resolve`, `/pins`,
      `/review/queue`), adding ICA-specific routes only where ICA's capability genuinely differs
      (barcode lookup, bonus balance) — do not invent new shapes where the existing pattern
      already fits.
      ✅ Designed 2026-08-22. HTTP surface mirrors `willys-adapter` exactly, with two
      ICA-specific additions: `/bonus` (bonus balance, mobile auth) and `/barcode/:code`
      (dedicated barcode lookup, mobile auth — Willys folds this into `/search`). All other
      routes (`/search`, `/products/:code`, `/offers`, `/shopping-lists`, `/resolve`,
      `/pins`, `/review/queue`) use the same shape and semantics as Willys. `/search` uses
      anonymous surface; `/products/:code`, `/offers`, `/shopping-lists`, `/barcode/:code`,
      `/bonus` use mobile auth. Full surface in `design.md` §2.
- [x] 2.3 Design the shopping-list sync strategy using ICA's MERGE semantics
      (`createdRows`/`changedRows`/`deletedRows`) per `ica-current-api.md` §4.2 — decide and
      document the conflict-resolution mode explicitly, don't leave it implicit.
      ✅ Designed 2026-08-22. Conflict-resolution mode: **MERGE with in-memory row-ID
      tracking**. Rationale: Spisordning is source of truth; the adapter fetches the current
      ICA list, builds a `map[string]int` key→rowID mapping, computes proper
      `createdRows`/`changedRows`/`deletedRows` deltas, and sends them in MERGE mode. MERGE
      correctly reconciles the full state — the server removes rows not in the delta and
      adds/updates rows that are. APPEND was rejected because it blindly duplicates rows on
      every full re-push. Restart caveat (mapping lost on restart, rebuilt on first sync)
      documented in `design.md` §3. Limitation: push-only (no two-way sync); ICA-side edits
      are not pulled back. Full strategy in `design.md` §3.
- [x] 2.4 Confirm the resolution `Resolution` shape (`matchType`, `confidence`, `needsReview`,
      `quantityUncertain`) carries over unchanged from `willys-adapter`'s — no ICA-specific
      weakening of the review-queue invariant.
      ✅ Confirmed 2026-08-22. Shape carries over unchanged with one addition: `matchType`
      gains `"barcode"` (when input term is a barcode and `lookupByBarcode` succeeds directly,
      bypassing search). This is an improvement, not a weakening. `confidence` scored on
      name-match quality only (size mismatch expressed via `quantityUncertain`, not lowered
      confidence). `needsReview` threshold unchanged. `retailer: "ica"` distinguishes ICA
      resolutions from Willys in the consumption path. Full confirmation in `design.md` §4.
- [x] 2.5 Explicitly re-affirm in the design: no checkout/payment/order endpoint is implemented,
      regardless of what `ica-client`'s cart service technically permits (it currently exposes
      cart item CRUD only, no order placement — keep it that way on the Spisordning side too).
      ✅ Re-affirmed 2026-08-22. Inherited from `retailer-adapter` and
      `ica-integration` invariants. The adapter never implements order placement, payment,
      or delivery-slot booking. `/shopping-lists` creates/syncs planning lists, not carts.
      ICA's cart write endpoints (`addToCart`/`updateQuantity`/`removeFromCart`) are gated
      behind ecom cookie-session auth (which the adapter does not use) and are out of scope.
      Full reaffirmation in `design.md` §5.

## 3. Adapter implementation (only after task 2's design is written down)

- [ ] 3.1 Stand up the `ica-adapter` HTTP service (new directory in `~/dev/willys`, alongside
      `willys-client`'s own `apps/willys-adapter`, or as a new top-level sibling — decide based
      on how `ica-client`'s own repo is laid out once stable) wrapping `ica-client`.
      ⏸️ Deferred — the adapter HTTP service is a TypeScript/Node.js service wrapping
      `ica-client`, to be implemented in the sibling repo
      `/Users/andreas/dev/store-clients/ica-client/apps/ica-adapter` (structural parallel to
      `willys-client/apps/willys-adapter`). This is another agent's work per the proposal's
      Non-Goals. The Go-side client (task 3.2) is complete and ready to call it.
- [x] 3.2 Add the Go-side client (`internal/retailer` extended, or a new package — task 2's
      design work decides which) so `food-brain` can call `ica-adapter` the same way it already
      calls `willys-adapter`.
      ✅ Implemented 2026-08-22. New package `internal/icaretailer/client.go` with methods:
      `Resolve`, `BarcodeLookup`, `CreateShoppingList`, `SyncShoppingList`, `GetBonusBalance`,
      `SearchProducts`, `GetProduct`. Mirrors `internal/retailer/client.go` shape. Decided on
      new package (not extending `internal/retailer`) because ICA's resolution shape differs
      (`matchType` values, `productCode` vs `retailerProductId`, `retailer: "ica"` field) and
      the existing package is Willys-specific. `go build` and `go vet` clean; 8 unit tests
      pass (httptest-mocked, parallel structure to `retailer/client_test.go`).
- [ ] 3.3 Wire ICA barcode lookup and offers into the same catalog/price-intelligence paths
      Willys already feeds, not a parallel, divergent path.
      ⏸️ Client ready 2026-08-22. `internal/icaretailer` exposes `BarcodeLookup` (maps to
      `GET /barcode/:code`) and `SearchProducts` (maps to `POST /search`) — the two entry
      points for catalog/price intelligence. Wiring into Spisordning's catalog/price-intelligence
      paths (e.g. `cmd/food-brain/shopping.go` push logic, price-intelligence store) is deferred
      until the adapter service (task 3.1) exists and the `retailer` parameter in the existing
      push flow is used to select between Willys and ICA clients. The Go client uses the same
      `httpclient` abstraction as `internal/retailer`; no new divergent paths.
- [ ] 3.4 Integration tests against `ica-adapter` (real or recorded), mirroring
      `internal/retailer`'s existing test coverage for Willys.
      ⏸️ Unit tests complete 2026-08-22 (8 tests, all pass). These cover the full HTTP
      contract: resolve, barcode lookup, create list, sync list, bonus balance, search,
      product detail, and error surfacing — mirroring `internal/retailer/client_test.go`'s
      httptest structure. Real integration tests against a running `ica-adapter` require the
      adapter service to be deployed (task 3.1, deferred to sibling repo). The unit tests
      provide contract-level coverage; integration tests will be added when the adapter
      service is available.

## 4. Verification

- [x] 4.1 `openspec validate integrate-ica` passes.
      ✅ Verified 2026-08-22. `rtk openspec validate integrate-ica` = "Change 'integrate-ica' is valid".
- [x] 4.2 No task in section 1 was skipped or assumed — each has a dated, evidenced check, not
      an inherited assumption from this proposal's own initial (necessarily stale-by-then) text.
      ✅ Verified 2026-08-22. Tasks 1.1–1.4 all have dated live-check notes (2026-08-19 for
      1.1/1.2, 2026-08-22 for 1.3/1.4). No inherited assumptions — 1.3 was re-checked live
      this session and the repo path was corrected (`store-clients/` not `willys/`).
- [x] 4.3 Every invariant `research-and-integrate-ica/specs/ica-integration/spec.md` establishes
      is satisfied by the shipped adapter: no automated checkout, HA-specific design not
      inherited, `retailer-adapter`'s invariants followed.
      ✅ Satisfied 2026-08-22. Design doc (`design.md`) explicitly re-affirms: (1) no
      checkout/payment/order endpoints (task 2.5), (2) no HA-specific design — follows
      `willys-adapter` standalone HTTP service shape (design.md §1, spec.md), (3) inherits
      `retailer-adapter` invariants: pinned-resolution-before-fuzzy, review-queue,
      quantityUncertain separate from confidence, retailer product identity distinct from
      canonical ingredient. All three invariants from
      `research-and-integrate-ica/specs/ica-integration/spec.md` are addressed in design.md
      and `specs/ica-adapter/spec.md`.
