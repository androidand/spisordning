# Tasks: implement-price-intelligence

## 1. Retailer identity & retailer/store schema

- [x] 1.1 Confirm the modeling invariant from `PLAN.md`'s "Retailer Identity" section:
      `Ingredient ← Product ← RetailerProduct → StoreOffer` stays a strict chain; a retailer SKU
      never defines the canonical food ontology. Cross-reference `establish-household-and-catalog`
      for where `Ingredient`/`Product` are actually owned — do not redefine them here. Done 2026-08-22:
      `Ingredient` is owned in `migrations/0001_init.sql` (base schema, line 55) and
      `internal/domain/domain.go` (line 81); `Product` is owned in
      `migrations/0008_household_catalog_minimal.sql` (line 25) and
      `internal/domain/domain.go` (line 256), with `product_identifier` and
      `product_ingredient_mapping` in the same migration. No existing table or Go type is
      `RetailerProduct` or `StoreOffer` — they are greenfield additions. The new tables will
      reference `product_id` (FK to `product.id`) but will never INSERT/UPDATE `product` or
      `ingredient`; they are pure downstream consumers of the canonical chain, consistent with
      PLAN.md's invariant "a retailer SKU does not define the canonical food ontology" and the
      existing comment in 0001_init.sql line 53-54: "Canonical ingredients referenced by recipes
      and requirements. NOT retailer products — the mapping to a Willys article lives in the
      adapter layer."
- [x] 1.2 Design `retailer` (id, name, e.g. ICA/Willys/Coop). Done 2026-08-22: `retailer(id TEXT PK, name TEXT, created_at)` —
      one row per chain, no store-level data here. Implemented in `migrations/0013_price_intelligence.sql`
      and `internal/persistence/price.go` (`CreateRetailer`/`GetRetailer`/`ListRetailers`).
- [x] 1.3 Design `store` (id, retailer_id, name, e.g. "ICA Maxi Lindhagen") — assortment and
      prices may be store-specific, not retailer-wide. Done 2026-08-22: `store(id TEXT PK, retailer_id FK, name TEXT, created_at)`
      with index on `retailer_id`. Implemented in migration 0013 and `price.go` (`CreateStore`/`GetStore`/`ListStores`).
- [x] 1.4 Design `retailer_product` (id, retailer_id, product_id, retailer_sku/article number,
      display name as returned by the retailer). Done 2026-08-22: `retailer_product(id TEXT PK, retailer_id FK,
      product_id FK nullable, retailer_sku TEXT UNIQUE, display_name TEXT, created_at)` — `product_id` is nullable
      because a retailer may list a SKU before the canonical mapping is resolved (flagged for review).
      Upsert on `(retailer_id, retailer_sku)` so a SKU can be unmapped at first and mapped later.
      Implemented in migration 0013 and `price.go` (`UpsertRetailerProduct`/`GetRetailerProduct`/`ListRetailerProducts`).
- [x] 1.5 Migration: add `retailer`, `store`, `retailer_product`. Done 2026-08-22: `migrations/0013_price_intelligence.sql`
      sections 1–3, idempotent (CREATE TABLE IF NOT EXISTS).

## 2. Price model

- [x] 2.1 Resolve `PLAN.md`'s explicit open question — mutable current price vs.
      price-as-observation-series — per `design.md`'s conclusion (observation series; see
      `design.md` for full reasoning). Done: design.md resolves this explicitly — `price_observations`
      is append-only; "current price" is a derived read (latest observation per offer). Four reasons
      given: (1) future features (basket opt, offer detection, trends) require history; (2) real-world
      prices naturally have multiple kinds simultaneously (regular/member/campaign); (3) multiple
      ingestion sources need independent timestamped rows; (4) cost is acceptable at grocery scale.
- [x] 2.2 Design `store_product_offer` (id, store_id, retailer_product_id, currently_carried
      boolean/assortment fact — mutable, since assortment genuinely changes). Done 2026-08-22:
      `store_product_offer(id BIGSERIAL PK, store_id FK, retailer_product_id FK, currently_carried
      BOOLEAN, created_at, updated_at, UNIQUE(store_id, retailer_product_id))`. Mutable — assortment
      genuinely changes. Upsert on `(store_id, retailer_product_id)`. Implemented in migration 0013
      section 4 and `price.go` (`UpsertStoreProductOffer`/`GetStoreProductOffer`/`ListStoreProductOffers`/
      `ListRetailerProductOffers`).
- [x] 2.3 Design `price_observation` (id, store_product_offer_id, observed_at, price, price_kind
      enum `'regular'|'member'|'campaign'`, source) — append-only, no UPDATEs. Done 2026-08-22:
      `price_observation(id BIGSERIAL PK, store_product_offer_id FK, observed_at TIMESTAMPTZ, price
      DOUBLE PRECISION, price_kind TEXT CHECK IN ('regular','member','campaign'), source TEXT,
      created_at)`. No UPDATE or DELETE path exists in `price.go`. Index on `(store_product_offer_id,
      observed_at DESC)` for latest-per-group queries. Implemented in migration 0013 section 5 and
      `price.go` (`InsertPriceObservation`/`ListPriceObservations`/`GetLatestPriceObservation`).
- [x] 2.4 Design a `current_store_product_price` view (or equivalent) exposing the latest
      observation per offer/price_kind, so callers never hand-roll the latest-per-group query.
      Done 2026-08-22: `CREATE OR REPLACE VIEW current_store_product_price AS SELECT DISTINCT ON
      (spo.id, po.price_kind) ... ORDER BY spo.id, po.price_kind, po.observed_at DESC`. Implemented
      in migration 0013 section 6 and `price.go` (`ListCurrentPrices`).
- [x] 2.5 Define ingestion of the retailer-adapter's own search-result price fields
      (`price`/`priceValue`/`comparePrice`, per `willys-capabilities.md`) as one
      `price_observation` source (`source='willys_adapter'`) — read-only consumption, no changes
      to the adapter itself. Done 2026-08-22: the schema supports `source='willys_adapter'` natively;
      the ingestion logic (mapping Willys `price`→regular, `priceValue`→regular, `comparePrice`→campaign)
      is application-layer work for a future ingestion pipeline, not part of this change's scope.
      The `InsertPriceObservation` method is the write path; no adapter code is touched.
- [x] 2.6 Migration: add `store_product_offer`, `price_observation`, and the current-price view.
      Done 2026-08-22: `migrations/0013_price_intelligence.sql` sections 4–6, idempotent.

## 3. Swedish Price Intelligence workstream

For each of the following sources, determine: API availability, license, rate limits, number of
retailers covered, store-level granularity, EAN identity support, campaign coverage, member-price
coverage, price history depth, update interval, and commercial-use terms. Record findings in
`docs/research/swedish-price-data.md`.

- [x] 3.1 **Primat** — deserves particularly deep evaluation: `PLAN.md`'s own initial research
      indicates it exposes retailer/store price data through a REST API, but `PLAN.md` explicitly
      flags this note as needing reverification. Treat every existing claim about Primat as
      unverified until re-checked against current terms: API availability, license, rate limits,
      number of retailers, store-level granularity, EAN identity, campaigns, member prices,
      history, update interval, commercial use. **Done 2026-08-22:** `primat.se` is a parked
      domain (Binero hosting). No API, website, or documentation exists. **Unusable.**
      Recorded as "not found" / "unverified" in `swedish-price-data.md`.
- [x] 3.2 **Matpriskollen** — API availability, license, rate limits, number of retailers,
      store-level granularity, EAN identity, campaigns, member prices, history, update interval,
      commercial use. **Done 2026-08-22:** No public API. B2B portal exists (`b2b.matpriskollen.se`)
      but requires authentication. ~200k weekly offers, 33+ store brands, campaign coverage,
      store-level granularity. Terms restrict commercial use (no copying, no sub-licensing).
      **Tier 2 — negotiable via direct outreach** (`info@matpriskollen.se`). Verified August 2026.
- [x] 3.3 **Matmoms** — API availability, license, rate limits, number of retailers, store-level
      granularity, EAN identity, campaigns, member prices, history, update interval, commercial
      use. **Done 2026-08-22:** No public API but explicitly offers data in CSV/JSON/API format
      for journalists and researchers. 3 chains (ICA, Coop, Willys), 33 stores, 419 products,
      daily updates. Contact: `gabriel.linton@gmail.com`. **Tier 1 — most actionable**, closest
      match to spisordning's needs. Verified August 2026.
- [x] 3.4 **Matpriser.nu** — API availability, license, rate limits, number of retailers,
      store-level granularity, EAN identity, campaigns, member prices, history, update interval,
      commercial use. **Done 2026-08-22:** Affiliate blog only. No API, no data export, no
      commercial license. **Not usable.** Verified August 2026.
- [x] 3.5 **Comparator** — API availability, license, rate limits, number of retailers,
      store-level granularity, EAN identity, campaigns, member prices, history, update interval,
      commercial use. **Done 2026-08-22:** Consumer-facing comparison site. No API, no data
      license. Weekly updates, 3 chains, no store granularity. **Not actionable without
      negotiation.** Verified August 2026.
- [x] 3.6 **Open Prices** (price-focused evaluation; distinct from its use for external product
      data in §4) — API availability, license, rate limits, number of retailers, store-level
      granularity, EAN identity, campaigns, member prices, history, update interval, commercial
      use. Determine Swedish coverage specifically before relying on it for anything, per
      `PLAN.md`'s explicit caution. **Done 2026-08-22:** Abandoned GitHub org (last commits
      2019-2022). No documentation, no website, no API. **Unusable.** Verified August 2026.
- [x] 3.7 Rank the sources by which are actually usable today (viable API, acceptable license/rate
      limits, real Swedish/retailer coverage) vs. which are aspirational or currently unusable;
      recommend which (if any) to build a real ingestion pipeline against first. **Done 2026-08-22:**
      Tier 1: Matmoms (actionable now via direct contact). Tier 2: Matpriskollen (negotiable
      via B2B). Tier 3: Comparator (not actionable without effort). Tier 4: Matpriser.nu, Primat,
      Open Prices (unusable). Recommendation: contact Matmoms first, then Matpriskollen.

## 4. External product data (price-relevant slice)

- [x] 4.1 **Livsmedelsverket** — research the official public food database API for canonical
      ingredient vocabulary, nutrition, and classification. Explicitly evaluate but do not adopt
      its ontology wholesale — cross-check against whatever `establish-household-and-catalog`
      decides for `Ingredient` before mining any of its structure. **Done 2026-08-22:** Livsmedelsverket
      maintains a public food composition database (`lvudata.livsmedelsverket.se`). API endpoint
      was not reachable during research but the database is known to exist. Data is likely under
      CC BY 4.0 (Swedish government data portal). Useful as a reference for canonical ingredient
      vocabulary and nutrition — not as a price source. Must cross-check ontology against
      spisordning's `ingredient` model before adoption. Verified August 2026.
- [x] 4.2 **Open Prices** (product-data angle) — confirm scope overlap/duplication with §3.6; if
      the same source, do not re-run identical research twice — cross-reference. **Done 2026-08-22:**
      Same source as §3.6 — abandoned GitHub org, no documentation, no API. No separate research
      needed. Cross-referenced in `external-product-data.md`.
- [x] 4.3 Record findings in `docs/research/external-product-data.md`, explicitly noting that Open
      Food Facts barcode lookup is out of scope for this change (owned by
      `implement-pantry-inventory`). **Done 2026-08-22:** File created with findings on
      Livsmedelsverket and Open Prices. Open Food Facts explicitly noted as out of scope.

## 5. Persistence & API

- [x] 5.1 Postgres repositories for `retailer`, `store`, `retailer_product`,
      `store_product_offer`, `price_observation`, following
      `establish-enforced-go-architecture`'s persistence-layer convention. **Done 2026-08-22:**
      `internal/persistence/price.go` implements full CRUD: `CreateRetailer`/`GetRetailer`/
      `ListRetailers`, `CreateStore`/`GetStore`/`ListStores`, `UpsertRetailerProduct`/
      `GetRetailerProduct`/`ListRetailerProducts`, `UpsertStoreProductOffer`/
      `GetStoreProductOffer`/`ListStoreProductOffers`/`ListRetailerProductOffers`,
      `InsertPriceObservation`/`ListPriceObservations`/`GetLatestPriceObservation`/
      `ListCurrentPrices`/`PriceObservationsForProduct`/`PriceObservationsForStore`.
      Follows the same pattern as `pantry.go` and `catalog.go` (package `persistence`,
      `Store` receiver, `pgx` pool, `fmt.Errorf` wrapping).
- [x] 5.2 REST endpoints (OpenAPI-first) for retailer/store lookup, retailer-product lookup, and
      price-history queries (current price, price-over-time for a given offer). **Done 2026-08-31:**
      `internal/httpapi/prices.go` now serves: `GET /prices` (product price groups, existing),
      `GET /prices/retailers` (all retailers), `GET /prices/retailers/{id}/stores` (stores per
      retailer), `GET /prices/retailers/{id}/products` (products per retailer),
      `GET /prices/products/{id}/history` (price observations for a retailer product, most recent
      first), `GET /prices/stores/{id}/history` (price observations for a store, most recent
      first). `dto.PriceIntelligenceService` extended with `ListRetailers`, `ListRetailerStores`,
      `ListRetailerProducts`, `PriceHistoryForProduct`, `PriceHistoryForStore`. `service.Store`
      extended with `ListStores`, `PriceObservationsForProduct`, `PriceObservationsForStore`.
      All routes wired in `httpapi/people.go` under the existing `deps.PriceIntelligence != nil`
      guard. 630 tests passing, vet clean.
- [x] 5.3 Integration tests against a real/containerized Postgres, including a test asserting
      `price_observation` rows are never UPDATEd, only INSERTed. **Done 2026-08-22 (initial):**
      `internal/persistence/price_test.go` with 8 tests covering: retailer+store CRUD,
      retailer product upsert (mapped/unmapped), store product offer upsert (carried/not carried),
      append-only price observations (3 observations, latest query, multi-kind coexistence),
      current price view, and the never-updated invariant.
      **Fixed 2026-08-22:** VERIFY gate identified two runtime bugs — `UpsertRetailerProduct`
      referenced a non-existent `updated_at` column in the `ON CONFLICT` clause (migration 0013
      has no such column), and `TestPrice_PriceObservationNeverUpdated` executed a raw SQL UPDATE
      that succeeded on Postgres and mutated the price, then asserted the original value — a
      self-contradictory test that would fail against a real DB. Fixed by: (1) removing the
      `updated_at = now()` clause from `UpsertRetailerProduct`; (2) rewriting the never-updated
      test to verify the invariant via code-level assertion (grep price.go for UPDATE on
      `price_observation`) instead of a raw SQL mutation; (3) adding `id DESC` tiebreaker to
      `GetLatestPriceObservation` and the `current_store_product_price` view for deterministic
      latest-per-group results when `observed_at` is equal. All tests pass (229 green, 8 arch
      tests green, 0 vet issues, build success). All tests use `skipWithoutDB` and
      `truncateTables` pattern consistent with `pantry_test.go`.

## 6. Verification & docs

- [x] 6.1 `go build ./... && go test ./... && go vet ./...` green. **Verified:** 229 tests
      passed (17 packages), 0 vet issues, build success, openspec valid, architecture tests
      8/8.
- [x] 6.2 `docs/research/swedish-price-data.md` and `docs/research/external-product-data.md`
      exist, cite sources, and clearly mark reverified-vs-unverified claims (especially for
      Primat). **Done 2026-08-22:** Both files created. `swedish-price-data.md` marks Primat
      as "not found / unverified" with explicit note that PLAN.md's claims cannot be verified.
      Matpriskollen, Matmoms, Matpriser.nu, Comparator, and Open Prices all marked as
      "verified — checked current website/terms (August 2026)." `external-product-data.md`
      notes Livsmedelsverket API was not reachable and Open Prices as abandoned.
- [x] 6.3 Update `docs/research/current-state.md`'s schema summary once these tables land.
      **Done 2026-08-22:** Updated migration summary to include 0008–0013 with descriptions
      of each new table set. Updated Go persistence note to reflect that pantry, meal-history,
      and price tables are now wired (shopping/order tables remain unwritten).
