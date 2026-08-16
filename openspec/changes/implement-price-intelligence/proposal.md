# Implement price intelligence

## Why

`PLAN.md`'s "Retailer Identity" section fixes a modeling invariant — `Ingredient → Product →
RetailerProduct → StoreOffer`, kept strictly separate, "a retailer SKU does not define the
canonical food ontology" — but nothing in this repo persists `RetailerProduct` or `StoreOffer`
today. `migrations/0001_init.sql` has no `retailers`, `stores`, `retailer_products`, or price
tables at all (only `shopping_requirement`, which is retailer-independent by design). Note:
`establish-enforced-go-architecture/tasks.md` already lists `retailer_products` and
`product_resolution_rules` as tables the first-slice schema was expected to gain — this change is
where those tables actually get designed and populated, not `establish-enforced-go-architecture`
itself, which only wires persistence for what already exists in `migrations/0001_init.sql`.

This change does **not** revisit Ingredient-vs-Product modeling — that ontology question belongs
to Epic B's `establish-household-and-catalog`. This change starts one layer down: `RetailerProduct`
(a retailer's SKU for a `Product`) and `StoreOffer` (that SKU's price/availability at a specific
store), plus the price-history question `PLAN.md` explicitly asks to research (mutable current
price vs. an observation series), plus the dedicated "Swedish Price Intelligence" workstream
(Primat, Matpriskollen, Matmoms, Matpriser.nu, Comparator, Open Prices) and the relevant slices of
"External Product Data" (Livsmedelsverket for vocabulary/nutrition, Open Prices for
location-specific pricing).

## What Changes

- **`retailers` / `stores`**: a retailer (ICA, Willys, Coop) and its stores (e.g. "ICA Maxi
  Lindhagen"), since assortment and prices may be store-specific — not retailer-wide.
- **`retailer_products`**: a retailer's SKU for a `Product`, distinct from the canonical
  `Product`/`Ingredient` chain owned elsewhere.
- **`store_product_offers` / `price_observations`**: the price model itself. `design.md` resolves
  `PLAN.md`'s explicit open question — mutable current price vs. price-as-observation-series — in
  favor of an append-only observation series with a derived "current" view, and documents why.
- **Swedish Price Intelligence workstream** (research): for each of Primat, Matpriskollen,
  Matmoms, Matpriser.nu, Comparator, and Open Prices, evaluate API availability, license, rate
  limits, retailer count, store-level granularity, EAN identity, campaign/member-price coverage,
  history depth, update interval, and commercial-use terms. Primat is flagged by `PLAN.md` as
  deserving particularly deep evaluation (initial research suggests a REST API exposing
  retailer/store price data) — `PLAN.md` also flags its own Primat notes as needing reverification,
  so this change's task list treats every Primat claim as unverified until re-checked.
- **Livsmedelsverket** (research): evaluate the official public food database for canonical
  ingredient vocabulary, nutrition, and classification — explicitly *not* to be blindly adopted as
  Spisordning's ontology, only mined for useful reference data.
- **Open Prices** (research, price-focused slice only): evaluate location-specific pricing data
  with Swedish coverage as the first gate — if Swedish coverage is inadequate, Open Prices is not
  relied on regardless of its other merits. (Open Food Facts barcode lookup itself is out of
  scope here — it belongs to Epic D's `implement-pantry-inventory`, per the task brief.)

## Non-Goals

- No re-litigation of Ingredient vs. Product modeling — `establish-household-and-catalog`'s job.
- No Open Food Facts barcode-lookup implementation — `implement-pantry-inventory`'s job.
- No basket optimization, offer detection, or trend UI — this change lays the price-history
  foundation those features would consume later; it does not build them.
- No committed integration with any Swedish price-intelligence source until its research task is
  complete and its terms (license, rate limits, commercial use) are confirmed current.

## Capabilities

### New Capabilities

- `price-intelligence`: retailer/store identity, retailer-specific product SKUs, store-scoped
  offers, and a price-observation history model, plus the research findings on external Swedish
  price/product data sources that could feed it.

### Modified Capabilities

<!-- none — retailer-adapter's existing price fields (search-result Product.price/priceValue/
     comparePrice, per willys-capabilities.md) are a data source this capability may ingest from,
     not a capability this change modifies -->

## Impact

- New migration(s) adding `retailer`, `store`, `retailer_product`, `store_product_offer`,
  `price_observation` to spisordning's own Postgres schema.
- Depends on Epic B's `establish-household-and-catalog` for the `Product`/`Ingredient` chain that
  `retailer_product` attaches to (a forward dependency, noted explicitly — this change's schema
  should not block on it being merged first, but `retailer_product.product_id` is not fully
  meaningful until it is).
- Consumes existing retailer-adapter search-result price fields
  (`price`/`priceValue`/`comparePrice`) as one ingestion source for Willys; does not modify the
  adapter.
- Research findings land in `docs/research/swedish-price-data.md` and
  `docs/research/external-product-data.md`, per `PLAN.md`'s "Research Documents" list.
- Part of Epic F: Retailer, Pricing & Commerce (tracking issue #6).
