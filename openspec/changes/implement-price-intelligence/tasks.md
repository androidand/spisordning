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

- [ ] 3.1 **Primat** — deserves particularly deep evaluation: `PLAN.md`'s own initial research
      indicates it exposes retailer/store price data through a REST API, but `PLAN.md` explicitly
      flags this note as needing reverification. Treat every existing claim about Primat as
      unverified until re-checked against current terms: API availability, license, rate limits,
      number of retailers, store-level granularity, EAN identity, campaigns, member prices,
      history, update interval, commercial use.
- [ ] 3.2 **Matpriskollen** — API availability, license, rate limits, number of retailers,
      store-level granularity, EAN identity, campaigns, member prices, history, update interval,
      commercial use.
- [ ] 3.3 **Matmoms** — API availability, license, rate limits, number of retailers, store-level
      granularity, EAN identity, campaigns, member prices, history, update interval, commercial
      use.
- [ ] 3.4 **Matpriser.nu** — API availability, license, rate limits, number of retailers,
      store-level granularity, EAN identity, campaigns, member prices, history, update interval,
      commercial use.
- [ ] 3.5 **Comparator** — API availability, license, rate limits, number of retailers,
      store-level granularity, EAN identity, campaigns, member prices, history, update interval,
      commercial use.
- [ ] 3.6 **Open Prices** (price-focused evaluation; distinct from its use for external product
      data in §4) — API availability, license, rate limits, number of retailers, store-level
      granularity, EAN identity, campaigns, member prices, history, update interval, commercial
      use. Determine Swedish coverage specifically before relying on it for anything, per
      `PLAN.md`'s explicit caution.
- [ ] 3.7 Rank the sources by which are actually usable today (viable API, acceptable license/rate
      limits, real Swedish/retailer coverage) vs. which are aspirational or currently unusable;
      recommend which (if any) to build a real ingestion pipeline against first.

## 4. External product data (price-relevant slice)

- [ ] 4.1 **Livsmedelsverket** — research the official public food database API for canonical
      ingredient vocabulary, nutrition, and classification. Explicitly evaluate but do not adopt
      its ontology wholesale — cross-check against whatever `establish-household-and-catalog`
      decides for `Ingredient` before mining any of its structure.
- [ ] 4.2 **Open Prices** (product-data angle) — confirm scope overlap/duplication with §3.6; if
      the same source, do not re-run identical research twice — cross-reference.
- [ ] 4.3 Record findings in `docs/research/external-product-data.md`, explicitly noting that Open
      Food Facts barcode lookup is out of scope for this change (owned by
      `implement-pantry-inventory`).

## 5. Persistence & API

- [ ] 5.1 Postgres repositories for `retailer`, `store`, `retailer_product`,
      `store_product_offer`, `price_observation`, following
      `establish-enforced-go-architecture`'s persistence-layer convention.
- [ ] 5.2 REST endpoints (OpenAPI-first) for retailer/store lookup, retailer-product lookup, and
      price-history queries (current price, price-over-time for a given offer).
- [ ] 5.3 Integration tests against a real/containerized Postgres, including a test asserting
      `price_observation` rows are never UPDATEd, only INSERTed.

## 6. Verification & docs

- [ ] 6.1 `go build ./... && go test ./... && go vet ./...` green.
- [ ] 6.2 `docs/research/swedish-price-data.md` and `docs/research/external-product-data.md`
      exist, cite sources, and clearly mark reverified-vs-unverified claims (especially for
      Primat).
- [ ] 6.3 Update `docs/research/current-state.md`'s schema summary once these tables land.
