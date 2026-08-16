# Tasks: implement-price-intelligence

## 1. Retailer identity & retailer/store schema

- [ ] 1.1 Confirm the modeling invariant from `PLAN.md`'s "Retailer Identity" section:
      `Ingredient ← Product ← RetailerProduct → StoreOffer` stays a strict chain; a retailer SKU
      never defines the canonical food ontology. Cross-reference `establish-household-and-catalog`
      for where `Ingredient`/`Product` are actually owned — do not redefine them here.
- [ ] 1.2 Design `retailer` (id, name, e.g. ICA/Willys/Coop).
- [ ] 1.3 Design `store` (id, retailer_id, name, e.g. "ICA Maxi Lindhagen") — assortment and
      prices may be store-specific, not retailer-wide.
- [ ] 1.4 Design `retailer_product` (id, retailer_id, product_id, retailer_sku/article number,
      display name as returned by the retailer).
- [ ] 1.5 Migration: add `retailer`, `store`, `retailer_product`.

## 2. Price model

- [ ] 2.1 Resolve `PLAN.md`'s explicit open question — mutable current price vs.
      price-as-observation-series — per `design.md`'s conclusion (observation series; see
      `design.md` for full reasoning).
- [ ] 2.2 Design `store_product_offer` (id, store_id, retailer_product_id, currently_carried
      boolean/assortment fact — mutable, since assortment genuinely changes).
- [ ] 2.3 Design `price_observation` (id, store_product_offer_id, observed_at, price, price_kind
      enum `'regular'|'member'|'campaign'`, source) — append-only, no UPDATEs.
- [ ] 2.4 Design a `current_store_product_price` view (or equivalent) exposing the latest
      observation per offer/price_kind, so callers never hand-roll the latest-per-group query.
- [ ] 2.5 Define ingestion of the retailer-adapter's own search-result price fields
      (`price`/`priceValue`/`comparePrice`, per `willys-capabilities.md`) as one
      `price_observation` source (`source='willys_adapter'`) — read-only consumption, no changes
      to the adapter itself.
- [ ] 2.6 Migration: add `store_product_offer`, `price_observation`, and the current-price view.

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
