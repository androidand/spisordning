# price-intelligence (delta)

## ADDED Requirements

### Requirement: RetailerProduct and StoreOffer stay distinct from the canonical food ontology

A `retailer_product` SHALL represent one retailer's SKU for a `Product` and SHALL NOT itself
define or modify canonical `Ingredient` or `Product` identity. A `store_product_offer` SHALL
represent that SKU's assortment fact at one specific store and MAY differ in availability across
stores of the same retailer.

#### Scenario: A retailer SKU cannot redefine an ingredient

- **WHEN** a `retailer_product` is created for a Willys SKU
- **THEN** it references an existing `product_id`
- **AND** creating it does not create, rename, or modify any `Ingredient` or `Product` row

#### Scenario: The same retailer product can be offered differently per store

- **WHEN** a `retailer_product` is carried by one store but not another store of the same
  retailer
- **THEN** a `store_product_offer` exists for the carrying store and none for the other
- **AND** this is a normal, expected state, not an error

### Requirement: Price is modeled as an append-only observation series

A `store_product_offer`'s price SHALL be represented as `price_observation` rows — append-only,
timestamped, sourced — and NOT as a single mutable price field on the offer itself. Reading "the
current price" SHALL be a query over the latest observation(s), never a stored value that is
updated in place.

#### Scenario: A price change creates a new observation, not an update

- **WHEN** a store's price for an offer changes from one ingestion to the next
- **THEN** a new `price_observation` row is inserted with the new price and a new `observed_at`
- **AND** the prior observation row is retained unmodified

#### Scenario: Current price is derived, not stored

- **WHEN** a caller asks for an offer's current price
- **THEN** the answer is computed as the latest `price_observation` for that offer (optionally
  per `price_kind`)
- **AND** no schema field holds "the" current price as a directly-updatable column

#### Scenario: Multiple price kinds coexist without overwriting each other

- **WHEN** an ingestion reports both a regular price and a member price for the same offer at the
  same time
- **THEN** two `price_observation` rows are inserted, distinguished by `price_kind`
- **AND** neither overwrites the other

### Requirement: External price/product sources are evaluated before ingestion is built

External price/product sources SHALL have their current API availability, license, rate limits,
and Swedish/store-level coverage verified and recorded in `docs/research/` before any ingestion
pipeline is built against them — this applies to Primat, Matpriskollen, Matmoms, Matpriser.nu,
Comparator, Open Prices, and Livsmedelsverket alike. The system SHALL NOT rely on a source's
terms as stated in `PLAN.md` without reverification, particularly for Primat, which `PLAN.md`
explicitly flags as needing its own notes reverified.

#### Scenario: An unverified source is not wired into ingestion

- **WHEN** a source's license or rate-limit terms have not been reverified since `PLAN.md` was
  written
- **THEN** no ingestion pipeline is built against it
- **AND** the source is recorded as "unverified" in the research document

#### Scenario: Swedish coverage gates Open Prices reliance

- **WHEN** Open Prices' Swedish retailer/store coverage has not been confirmed as adequate
- **THEN** the system does not rely on Open Prices as a price-observation source
- **AND** the coverage gap is recorded explicitly rather than silently worked around
