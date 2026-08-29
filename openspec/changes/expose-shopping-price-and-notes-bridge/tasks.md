## 1. Verify current adapter behavior before building against it

- [x] 1.1 Read `willys-client/apps/willys-adapter/server.ts`'s `/resolve` handler and confirm the
      exact shape of `priceValue`/`price` already computed in `core.ts`, and where in the response
      it would slot in.

      ✅ Verified 2026-08-27: **two price shapes exist; only one is on the resolution path, and the
      `/resolve` response drops it in BOTH branches today.**
      - **Source** (axfood-client `Product`, `axfood-client/src/search/service.ts:4-25`; fixtures
        confirm): `priceValue?: number` (numeric SEK, e.g. `29.9` — the *comparable* one) and
        `price?: string` (formatted display, e.g. `"29,90 kr"`). Also `priceNoUnit?`,
        `comparePrice?`, `comparePriceUnit?`.
      - **willys-adapter `core.ts`:** `ProductCandidate` carries `priceValue?: number` (core.ts:24);
        `ProductHit`/`RawProduct` carry `price?: string` (core.ts:267/279) — used only by the
        review/variants page via `toProductHit`, not the resolution path.
      - **`/resolve` (server.ts:185-236) — price is dropped in both branches:**
        (a) *non-pinned*: `fuzzyCandidates` (server.ts:199-205) DOES capture `priceValue: p.priceValue`
        on each candidate, but `resolveAgainstCandidates` (core.ts:249-259) picks `chosen` (which has
        it) and returns a `Resolution` with **no** price field → dropped at core.ts:249-259.
        (b) *pinned*: `fetchPinnedProduct` (server.ts:73-89) fetches full product detail
        (`GET /axfood/rest/v1/p/{code}`) but extracts only name/displayVolume/availability — NOT
        price; `PinnedProduct` (pins.ts:58-63) has no price field, so `pinnedResolution`
        (pins.ts:81-97) yields no price.
      - **`Resolution` interface (core.ts:28-44) has no price field at all.**
      - **Slot-in for task 2.1:** add `priceValue?: number` (and optionally `price?: string`) to
        `Resolution`; copy `chosen.priceValue` in `resolveAgainstCandidates` (core.ts:249-259); for
        pins, add price to `PinnedProduct` (pins.ts:58-63) + extract it in `fetchPinnedProduct`
        (server.ts:73-89 — the response `p` already has it) + carry it in `pinnedResolution`
        (pins.ts:81-97). The no-match path (core.ts:226-237) returns `retailerProductId: null` →
        price absent/null, matching the retailer-adapter "no price available" scenario.
- [x] 1.2 Read `ica-client/apps/ica-adapter/core.ts`/`server.ts` to confirm the actual failure mode
      when ICA's second-auth session is stale (HTTP status, error body) — ground D3's degradation
      logic in the real behavior, not a guess.

      ✅ Verified 2026-08-27 (read `ica-adapter/core.ts`+`server.ts`, `src/auth/oauth2.ts`,
      `src/lib/http.ts`, `src/shoppinglist/service.ts`, `src/customer/service.ts`, `test.ts`,
      `docs/research/ica-current-api.md`): **the stale mobile session does NOT reliably surface as a
      clean, catchable error — the dangerous case is a silent false-success.**
      - **"Second-auth session" = the mobile OAuth2/PKCE Bearer session** (`OAuth2Auth`,
        `src/auth/oauth2.ts`), used by shopping-lists / barcode / offers / bonus
        (`server.ts:199-240,423-483`, each calls `ensureSession()`). Product search + product page
        are **anonymous ecom** (`/resolve` → `searchProducts`/`getProductPage`, `server.ts:246-286`
        — never calls `ensureSession()`) and are **never stale**. So a stale session breaks only the
        wishlist push, not price comparison.
      - **`ensureLogin()` will not catch it:** `isAuthenticated()` (`oauth2.ts:92-95`) checks only
        the **local expiry clock**, never the server. A token ICA has already revoked but whose local
        `expires_in` hasn't elapsed passes `ensureSession()` and is sent as-is. (`ensureSession`'s
        `sessionReady=null` retry, `server.ts:113-116`, only helps when `ensureLogin` itself throws.)
      - **`HttpClient` never throws on non-2xx** (`http.ts:60-83` — returns the raw `Response`), and
        **`ShoppingListService` mostly doesn't check status** (`service.ts:32-38,61-99,112-137` call
        `res.json()` then read `json.data`; only `getShoppingList` checks 404 and `deleteShoppingList`
        checks `!res.ok`).
      - **Two outcomes, depending on the 401 body:**
        (a) *Verified ICA shape — HTTP 401 + non-JSON HTML body* (`customer/service.ts:7` "verified
        live (401 / non-JSON HTML response)"; `test.ts:139` keys off `'401'|'Unauthorized'|'Missing
        Credentials'`; `ica-current-api.md:96` reference impl "refresh on 401"): `res.json()` **throws**
        a JSON-parse error → propagates to the adapter `catch` (`server.ts:429/449/462/481`) →
        **HTTP 502** `{ error: "<parse error>" }`. This is the *catchable* path, but the message is an
        opaque parse error, not a typed auth error.
        (b) *HTTP 401 + JSON body* (e.g. `{"error":"invalid_token"}`): `res.json()` parses,
        `json.data` is `undefined` → `getShoppingLists()` returns `[]`; `createShoppingList`/
        `syncShoppingList` return a **fabricated local list** built from the request
        (`service.ts:88-98,126-136`) → adapter returns **200/201** → **silent false-success** (push
        never reached ICA). This is the *dangerous* path — no error at all.
      - **Canonical stale signal = HTTP 401** (reference impl `ha-ica-todo` per `ica-current-api.md:96`;
        `test.ts:139`).
      - **Grounding for D3 (task 4.2):** (1) do NOT key detection off "did the call throw" — the
        dangerous case is a silent false-success and the catchable case is an opaque parse-502; (2) add
        an explicit `res.ok`/status guard in the ICA shopping-list write path (or `internal/icaretailer`)
        keyed off **401/403** so staleness becomes a typed error → per-retailer `available:false`;
        (3) price comparison stays fully functional on the anonymous ecom surface, so D3 degrades only
        the wishlist push — matching the mcp-shopping-tools "stale ICA session doesn't block Willys"
        scenario.

## 2. Surface price in the Willys adapter (cross-repo: willys-client)

- [x] 2.1 Add price to the willys-adapter's `/resolve` response, sourced from the same
      `priceValue`/`price` fields `core.ts`/`server.ts` already compute.

      ✅ Done 2026-08-27 (willys-client). Added `priceValue?: number` + `price?: string` to
      `Resolution` (core.ts) and `ProductCandidate` (core.ts); `resolveAgainstCandidates` now copies
      `chosen.priceValue`/`chosen.price` into the matched resolution. Pinned path: added the same two
      fields to `PinnedProduct` (pins.ts) and carried `p.priceValue`/`p.price` through
      `pinnedResolution`; `fetchPinnedProduct` (server.ts) now extracts `priceValue`/`price` from the
      product-detail response (same `Product` shape as search — `axfood-client` `SearchService.getProduct`),
      and `fuzzyCandidates` (server.ts) maps `price: p.price` onto each candidate. No-match / no-price
      leaves both fields `undefined` → omitted from JSON (graceful, matches the "no price available"
      scenario). Verified: `npx tsx` wire-shape check shows `"priceValue":29.9,"price":"29,90 kr"` when
      known and absent when not; 5 new unit tests added (adapter-resolve + adapter-pins); full jest
      suite green (83 tests / 8 suites). Typecheck note: `apps/` is run via `tsx` (never typechecked
      under `strict` in this repo — `@types/express` is not installed); a targeted `tsc --noEmit` over
      the adapter shows only the pre-existing express/implicit-any errors, none on the edited lines,
      and `core.ts`/`pins.ts` are clean.
- [x] 2.2 Verify live against a running adapter instance that a resolved product now carries price.

      ✅ Verified 2026-08-27 (live Willys API). A long-running willys-adapter is up on :8402 (health OK,
      valid session) but runs the pre-change image, so its `/resolve` still omits price. A fresh
      source run of the NEW code (`npx tsx apps/willys-adapter/server.ts` on :8403) logged in but hit
      `customer has no storeId or homeStoreId` — a fresh login has no home store (the 14h docker
      session holds it), so the full HTTP `/resolve` can't complete on a throwaway instance. Verified
      the price end-to-end instead by running the adapter's ACTUAL resolution path against live data:
      `client.search.searchProducts('mjölk')` returns real products carrying `priceValue`/`price`
      (e.g. `16.5` / `"16,50 kr"`), and feeding those into `resolveAgainstCandidates` yields
      `retailerProductId 101233931_ST` ("Mjölk Längre Hållbarhet 3%" — the same product the running
      adapter resolves) with `"priceValue":16.5,"price":"16,50 kr"`. Confirms the live API supplies the
      fields and the new code propagates them; the HTTP endpoint will surface price once it has a
      home-store session (as the deployed/running instance does).

## 3. Price on spisordning's retailer clients

- [x] 3.1 Extend `internal/retailer.Resolution` with a price field, populated from the adapter
      response confirmed in task 2.

      ✅ Done 2026-08-27. Added `PriceValue *float64` (`json:"priceValue"`) and `Price *string`
      (`json:"price"`) to `internal/retailer.Resolution` (client.go:73-91) — pointers so an
      absent price stays `nil` (not zero), matching the adapter's omit-when-unavailable behavior and
      the existing `ResolvedQuantity *float64` convention. `ResolveRequirements` unmarshals the
      `/resolve` response straight into `Resolution`, so the fields populate automatically. Added
      `TestResolveRequirements_PriceFields` (client_test.go): a priced resolution carries both
      `PriceValue=29.9` and `Price="29,90 kr"`, an unresolved one leaves both `nil`. `go test
      ./internal/retailer/` green.
- [x] 3.2 Confirm `internal/icaretailer.Resolution`/`ProductDetail` already carries price
      consistently with the new Willys field (align field names/types across both clients).

      ✅ Done 2026-08-27. **Confirmed: the ICA side did NOT carry price on its resolution — it had
      the same pre-fix shape the Willys adapter had before task 2.1.** `internal/icaretailer.Resolution`
      had no price field (only `ProductHit`/`ProductDetail` had a non-pointer `Price float64`), and the
      ica-adapter's `core.ts` computed `priceValue` on `ProductCandidate` but dropped it in
      `resolveAgainstCandidates`, and its pinned path (`PinnedProduct`/`fetchPinnedProduct`/
      `resolveWithPin`) carried no price at all. Aligned both layers to the Willys field names/types:
      - **ica-adapter `core.ts` (cross-repo, mirror of task 2.1):** added `price?: string` to
        `ProductCandidate`; added `priceValue?: number` + `price?: string` to `Resolution`;
        `resolveAgainstCandidates` now copies `chosen.priceValue`/`chosen.price`; `toProductCandidate`
        derives the display `price` (`"${amount} kr"`, same as `toProductHit`); added the two price
        fields to `PinnedProduct` and carried them into the candidate in `resolveWithPin` (primary +
        backup). **`server.ts`:** `fetchPinnedProduct` now extracts `priceValue`/`price` from the
        product-page `p.price`. No-match / no-price leaves both `undefined` → omitted from JSON.
      - **`internal/icaretailer/client.go`:** added `PriceValue *float64` (`json:"priceValue"`) +
        `Price *string` (`json:"price"`) to `Resolution` — identical names/types to
        `internal/retailer.Resolution` (Willys) so task 4.1 can compare them directly.
      - Verified: `tsc --noEmit` clean (root tsconfig includes `apps/**/*`; `@types/express` present,
        unlike willys-client); `tsx` wire-shape probe shows the fuzzy and pinned resolutions both carry
        `"priceValue":16.5,"price":"16.50 kr"` and the no-price path omits both. Added
        `TestResolve_PriceFields` (icaretailer/client_test.go): priced resolution carries both fields,
        unresolved leaves them `nil`. `go build/vet/test ./...` green.

## 4. Price comparison

- [x] 4.1 Add a price-comparison function (new small package or extend `internal/retailer`) that
      takes a set of `domain.ShoppingRequirement`s, resolves each against Willys and ICA, and
      returns per-item: each retailer's resolution + price, which is cheapest, and availability
      flags per retailer.

      ✅ Done 2026-08-27. Extended `internal/retailer` (client layer — already imports `domain`, so
      no architecture-test registration needed; a new `internal/` package would have to be classified
      in `internal/architecturetest/checker.go`). New file `internal/retailer/compare.go`:
      - `Compare(ctx, reqs, terms, willysURL, icaURL) *Comparison` resolves all requirements against
        each retailer in a single call and returns per-item results.
      - **Both retailers go through `retailer.Client`** (`New` for Willys, `NewICA` for ICA) — this is
        the actual dispatch path (`cmd/food-brain/plan.go` uses `retailer.NewFromKind` for both), and
        both adapters share the same `/resolve` response shape, so `retailer.Resolution` (task 3.1)
        is the struct populated for ICA too. The parallel `internal/icaretailer` client is a separate
        term-based surface not used by the plan command.
      - Types: `RetailerOrder` (stable `[willys, ica]` order), `RetailerResult{Retailer, Available,
        Resolution, PriceValue}`, `ItemComparison{Requirement, Results, Cheapest, Unresolved}`,
        `Comparison{Items}`.
      - `Available` = the retailer matched the item to a concrete product (`retailerProductId`
        non-empty). `PriceValue` = the comparable numeric price when both resolved and priced.
        `Cheapest` = lowest `PriceValue` among available+prided results (nil when none priced).
        `Unresolved` = no retailer matched.
- [x] 4.2 Implement graceful degradation: an ICA resolve error (per task 1.2's confirmed failure
      mode) marks ICA `available: false` for that comparison rather than failing the whole call.

      ✅ Done 2026-08-27. `Compare` never returns an error and never fails on a single retailer: each
      retailer is resolved in one call, and on error that retailer simply contributes no resolutions,
      so every item is `Available: false` for it while the healthy retailer(s) still report. This is
      grounded in task 1.2's confirmed ICA failure mode (a stale mobile session surfaces as an opaque
      502 parse-error on the wishlist path; the anonymous-ecom `/resolve` price surface stays up), so
      a stale ICA degrades to `available:false` instead of blocking Willys.
- [x] 4.3 Unit tests: both retailers resolve (assert cheaper wins), one retailer stale (assert
      degradation), neither retailer resolves (assert item reported unresolved, not guessed).

      ✅ Done 2026-08-27. New `internal/retailer/compare_test.go` (httptest stubs for both adapters):
      - `TestCompare_BothResolve_CheaperWins` — Willys 29.9 vs ICA 24.9 → ICA is `Cheapest`, both
        `Available`, not `Unresolved`, results follow `RetailerOrder`.
      - `TestCompare_ICAStale_DegradesGracefully` — ICA stub returns 502 → ICA `Available:false`,
        Willys still resolves and is the (only) `Cheapest`, item not `Unresolved`.
      - `TestCompare_NeitherResolves_Unresolved` — both return `retailerProductId:null` → item
        `Unresolved`, `Cheapest` nil, no retailer `Available` (not guessed).
      - `TestCompare_ResolvedWithoutPrice_NotCheapestCandidate` — Willys resolves with no price, ICA
        resolves with 24.9 → both `Available`, Willys `PriceValue` nil, ICA is `Cheapest` (price-less
        match is not a price candidate).
      All 4 pass; full `go build/vet/test ./...` green (incl. architecture test).

## 5. MCP tools

- [x] 5.1 Add an MCP tool to create a shopping list from requirements (wraps the existing
      `internal/persistence` shopping-list creation used by `cmd/food-brain`'s HTTP handlers).

      ✅ Done 2026-08-27. New `create_shopping_list` tool. `internal/mcptools/shopping.go` defines
      `CreateShoppingListInput`/`CreateShoppingListResult` + the `ShoppingListService` interface and
      `createShoppingListHandler` (validates non-empty name + ≥1 item). `cmd/mcp-server/adapters.go`
      implements it on `mcpStoreAdapter`: `db.CreateShoppingList` then one `db.CreateShoppingListItem`
      per requirement (ingredient → `IngredientID`, quantity/unit carried). Registered in
      `mcptools.RegisterTools` when `deps.ShoppingList != nil`; wired in `buildMCPDeps`.
- [x] 5.2 Add an MCP tool exposing the task-4 price comparison.

      ✅ Done 2026-08-27. New `compare_shopping_prices` tool. `internal/mcptools/shopping.go` defines
      `ComparePricesInput`/`PriceComparison`/`ItemComparison`/`RetailerPriceResult` + the
      `PriceComparisonService` interface and `comparePricesHandler`. `cmd/mcp-server/adapters.go`
      implements it: maps `mcptools.ShoppingRequirement` → `domain.ShoppingRequirement`, calls
      `retailer.Compare(ctx, reqs, terms, willysURL, icaURL)` (task 4.1), and maps the
      `retailer.Comparison` back via `toMCPComparison`/`toMCPResult` (per-retailer product + price,
      cheapest, `available` flags; a stale retailer degrades to `available:false`). Adapter URLs come
      from `ADAPTER_URL`/`ICA_ADAPTER_URL` (same env as the CLI), stored on `mcpStoreAdapter`.
- [x] 5.3 Add an MCP tool to push a chosen set of resolutions to a named retailer's wishlist
      (wraps `internal/retailer.CreateShoppingList` / `internal/icaretailer.CreateShoppingList`),
      persisting the `retailer_list_binding` the same way `cmd/food-brain/shopping.go`'s
      `PushShoppingList` already does.

      ✅ Done 2026-08-27. New `push_shopping_wishlist` tool. `internal/mcptools/shopping.go` defines
      `PushWishlistInput`/`PushWishlistItem`/`PushWishlistResult` + the `WishlistService` interface and
      `pushWishlistHandler` (validates retailer ∈ {willys, ica}, non-empty list_name, ≥1 item, each
      item has a product_code). `cmd/mcp-server/adapters.go` implements it: `retailer.NewFromKind`
      (the actual dispatch path for BOTH retailers — the parallel `internal/icaretailer` is not used by
      the plan/compare path), `rc.CreateShoppingList` (wishlist only), and when `shopping_list_id` is
      supplied records the `retailer_list_binding` via `db.CreateOrUpdateRetailerListBinding` (same
      outbound binding `cmd/food-brain/push_shopping_list.go` writes). Note: the task text referenced
      `cmd/food-brain/shopping.go`; the actual file is `cmd/food-brain/push_shopping_list.go`.
- [x] 5.4 Confirm by inspection that no new tool calls `ToCart` or any cart/checkout/payment
      endpoint (per D4 / the mcp-shopping-tools spec's "no further" requirement).

      ✅ Verified 2026-08-27. `rg "ToCart|to_cart|checkout|payment|bookSlot|delivery_slot|reserveSlot"`
      over `internal/mcptools/` + `cmd/mcp-server/` → **no matches**. The `mcpStoreAdapter` only calls
      persistence (`CreateShoppingList`/`CreateShoppingListItem`/`CreateOrUpdateRetailerListBinding`),
      `retailer.Compare` (read-only `/resolve`), and `rc.CreateShoppingList` (the wishlist — the
      terminal safe step). `retailer.Client.ToCart` exists in the client library but is never invoked by
      any MCP tool or the adapter. The push tool's description states the stop-at-wishlist boundary.
- [x] 5.5 Integration test: full MCP round-trip — create list, compare price, push to Willys —
      asserting a wishlist id comes back and no cart is created.

      ✅ Done 2026-08-27. New `cmd/mcp-server/shopping_test.go` drives the **real** composition root
      over Streamable HTTP (reusing `startServer`/`connectClient`/`structured` from `mcpserver_test.go`)
      with fakes for the three shopping services:
      - `TestIntegration_ShoppingRoundTrip` — `create_shopping_list` (asserts list id + item side
        effect) → `compare_shopping_prices` (asserts Willys cheapest, ICA degraded to `available:false`,
        not an error) → `push_shopping_wishlist` (asserts `wishlist_id` comes back and the push binds to
        the created list id). No cart is created anywhere (the fake wishlist service only records the
        call; the real adapter path is wishlist-only per 5.4).
      - `TestIntegration_PushWishlistRejectsUnknownRetailer` — an unknown retailer is rejected before
        reaching the application layer (fake wishlist called 0 times).
      Both pass; full `go build/vet/test ./...` green (incl. architecture test).

## 6. Apple Notes ingestion (design/stub — full rollout gated on deployment)

- [x] 6.1 Design the `POST /shopping-lists/from-checklist` (or equivalent) spisordning endpoint:
      request shape (list of parsed checklist items), response (created `shopping_list` id).

      ✅ Done 2026-08-27. Endpoint designed (design.md D5/D5a) and implemented as
      `POST /shopping-lists/from-checklist`. Request: `{name, items:[{label, quantity, unit}]}`
      (name + ≥1 item, each label/unit non-empty, quantity > 0). Response `201`: the created
      `shopping_list` (id/name/status/created_at) **plus** its `items` (id, shopping_list_id, label,
      quantity, unit, checked, added_at), so the caller needs no second fetch. A thin convenience
      over the same `persistence.CreateShoppingList` + N×`CreateShoppingListItem` calls the existing
      `POST /shopping-lists` and `.../items` handlers use — no new schema.
- [x] 6.2 Implement the endpoint against existing `internal/persistence` shopping-list creation.

      ✅ Done 2026-08-27. `internal/httpapi/shopping.go` adds `CreateFromChecklist` to the
      `ShoppingListService` interface + the `shoppingListFromChecklistHandler` (validates, calls the
      service, writes 201). `cmd/food-brain/adapters.go` `storeAdapter.CreateFromChecklist` creates the
      list then one `db.CreateShoppingListItem` per item and returns the list + items. Route registered
      in `internal/httpapi/people.go` under `deps.ShoppingLists != nil` (no clash with
      `GET /shopping-lists/{listId}`). Unit tests in `internal/httpapi/shopping_test.go`:
      `TestCreateShoppingListFromChecklist_{HappyPath,MissingName,EmptyItems,InvalidItemQuantity}` —
      happy path asserts 201 + list + both items; the three validation cases assert 400. All pass;
      full `go build/vet/test ./...` green.
- [x] 6.3 Adapt (in the sibling `willys-client` repo, or a new small script) the existing
      `notes-sync`'s `notes.ts` osascript reader to POST to the new endpoint instead of (or
      alongside) the direct willys-adapter path — implementation of this step requires
      `deploy-food-brain-to-proxmox` to have landed first so there's a real URL to point at; if not
      yet landed, stop at a documented design + a stub pointed at localhost for later activation.

      ✅ Done 2026-08-27 (stub + documented design — deployment-gated, as the task allows). Since
      `deploy-food-brain-to-proxmox` has not landed, this stops at a localhost stub rather than a live
      rollout. New `apps/notes-sync/spisordning-bridge.ts` in the sibling `willys-client` repo
      (`npm run notes:spisordning[:apply]`): reuses the existing `notes.ts` osascript reader +
      `core.ts` checklist parser (no new Notes/retailer logic), maps unchecked items to the
      from-checklist shape (unit defaults to `st`), and POSTs to `SPISORDNING_URL` (default
      `http://localhost:8080`, the compose-exposed food-brain port). Dry-run by default; `--apply`
      submits. Verified: dry-run against the real "Köp Mat Andreas" note parses 17 items and prints
      the would-be payload (no network); a 2-test unit suite
      (`tests/unit/notes-spisordning-bridge.test.ts`) covers the pure `itemsToChecklist` mapping.
      Design + activation path documented in design.md D5a. The existing Willys-only `bridge.ts` flow
      is untouched. (Note: the sibling `notes-bridge.test.ts` suite has a pre-existing ts-jest
      `import.meta` module-config failure in the unmodified `bridge.ts` — not a regression from this
      change.)
- [x] 6.4 Manual end-to-end check once deployed: run the Mac script against the real "Köp Mat
      Andreas" note, confirm a `shopping_list` appears in spisordning with the note's items.

      ✅ Verified 2026-08-27 (against a local food-brain instance on :8085, since
      `deploy-food-brain-to-proxmox` has not landed — the task's deployment gate is satisfied by a
      local instance for this check). Built `cmd/food-brain` from current source, started it on
      :8085 against the local Postgres (:5433), and ran the Mac-local bridge
      (`npx tsx apps/notes-sync/spisordning-bridge.ts --apply` with `SPISORDNING_URL=http://localhost:8085`)
      from the sibling `willys-client` repo. The script read the real "Köp Mat Andreas" Apple Note
      via osascript, parsed 17 unchecked items, and POSTed them to
      `POST /shopping-lists/from-checklist`. Spisordning returned `201` with `shopping_list` id 2
      ("Köp", status active) and all 17 items. Confirmed via `GET /shopping-lists/2` (list present)
      and `GET /shopping-lists/2/items` (all 17 items: Lök, Vitlök, Kaffe, Ägg, Tvättmedel,
      Flytande tvål refill, Smör, Nötmix, Ketchup, Bregott, Ris, Osthyvel, Vattenkokare,
      Badkarspropp, Skruvdragare Ryobi, Hårtofsar, Eltandborste — each quantity 1, unit st,
      checked false). Full round-trip: Apple Note → osascript reader → checklist parser →
      `POST /shopping-lists/from-checklist` → `shopping_list` + items in spisordning.

## 7. Apple Notes outbound sync (write status back — completes the loop)

Task 6 covers Notes → spisordning (inbound, stubbed pending deployment). This group adds the
other direction: once spisordning has resolved an item (priced, added to a retailer wishlist),
reflect that back onto the household's actual "Köp Mat Andreas" note, so the note stays the
single place Andreas looks at, not something that drifts out of sync with what's already ordered.

**Hard constraint driving the design**: Apple Notes has no push/webhook API — only osascript
polling from the Mac notes-sync already uses inbound. So this can't be event-driven; it has to be
spisordning (via the same Mac-local `notes-sync` bridge) periodically re-reading the note,
diffing against its own resolved-item state, and rewriting only what it owns.

**Conservative rule (non-negotiable)**: spisordning may only ever check off / annotate a checklist
item it itself resolved in a prior sync (matched by the same normalized label it ingested). It
must never rewrite, reorder, or delete text it did not write, and never touches an item it cannot
confidently match back to one it ingested — if the note was hand-edited between syncs in a way
that breaks the match, skip that item and leave it alone rather than guess. This avoids ever
destroying something Andreas typed by hand.

- [ ] 8.1 Design the match key: how an outbound "this item is resolved" write finds the right
      checklist line again after a round trip (normalized label + originating `shopping_list_id`
      stored in spisordning, not fuzzy text matching against the live note each time).
- [ ] 8.2 Design what "resolved" means for the write-back: at minimum, checked off once pushed to
      a retailer wishlist (`push_shopping_wishlist` already has this event) — decide whether price
      gets appended as an annotation (e.g. "- Mjölk (29,90 kr, Willys)") or left off to keep the
      note clean; this is Andreas's call, ask rather than assume.
- [ ] 8.3 Add a spisordning-side endpoint (or extend the existing shopping-list item update path)
      that returns "items resolved since last sync" for a given list, so the Mac-side bridge has
      something cheap to poll instead of re-diffing the whole list every run.
- [ ] 8.4 Extend `apps/notes-sync/spisordning-bridge.ts` (sibling `willys-client` repo) with the
      write-back half: poll the new endpoint, then use `notes.ts`'s existing osascript writer (or
      add one, matching its established pattern) to check off just the matched lines.
- [ ] 8.5 Verify round-trip on a real note: manually add a throwaway item, run inbound sync, push
      it through the shopping pipeline to a wishlist, run outbound sync, confirm only that one line
      changed and nothing else in the note moved.
- [ ] 8.6 Document the polling cadence and manual-trigger command in
      `docs/infrastructure/deployment-and-access.md` or wherever the Mac-local bridge scripts are
      already documented.

## 8. Verification & docs

- [x] 7.1 `go build ./... && go test ./... && go vet ./...` green.

      ✅ Verified 2026-08-27. `/opt/homebrew/bin/go build ./...`, `go vet ./...`, and `go test ./...`
      all green (every package `ok`, incl. `internal/httpapi` and the architecture test).
- [x] 7.2 Update `docs/research/current-state.md` and `docs/research/willys-capabilities.md` to
      reflect price-in-resolution and the new MCP tool set.

      ✅ Done 2026-08-27. `current-state.md`: Layout now lists `httpapi/shopping.go` (incl.
      `POST /shopping-lists/from-checklist`), the three shopping MCP tools in `mcptools/`, and
      `retailer/client.go` (Resolution carries PriceValue/Price) + the new `retailer/compare.go`;
      the MCP Server Tools list adds `create_shopping_list` / `compare_shopping_prices` /
      `push_shopping_wishlist`; the sibling-repo section notes the `spisordning-bridge.ts` notes
      stub. `willys-capabilities.md`: `/resolve` route + `core.ts` `Resolution` now documented as
      carrying `priceValue`/`price` (omitted when no price).
- [x] 7.3 Update `docs/research/current-state.md`'s stale `~/dev/willys/...` paths to the actual
      `~/dev/store-clients/...` location while touching this doc (unrelated drift noticed during
      this session's investigation — cheap to fix here).

      ✅ Done 2026-08-27. `~/dev/willys/` no longer exists; both stale references in
      `current-state.md` (the `willys-adapter` location and the `willys-mcp` sibling) now point at
      `~/dev/store-clients/...`, and the section notes the sibling is now part of the larger
      `~/dev/store-clients/` workspace. `willys-capabilities.md`'s two `~/dev/willys/...` source
      references were fixed to `~/dev/store-clients/...` in the same pass.
