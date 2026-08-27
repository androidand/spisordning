## ADDED Requirements

### Requirement: Resolution responses include a comparable price
A retailer adapter's `/resolve` response SHALL include a price for each resolved product (when the
retailer's own product data has one available), so callers can compare cost across retailers without
a second lookup.

#### Scenario: A resolved product's price is returned
- **WHEN** the willys-adapter resolves a requirement to a real product
- **THEN** the resolution includes that product's current price
- **AND** the price reflects the same value the adapter's own review-queue UI already displays for
  that product

#### Scenario: A resolution with no price available is still usable
- **WHEN** a resolved product has no price data (e.g. a needs-review item with no confirmed match)
- **THEN** the resolution's price field is absent/null rather than the resolution failing
