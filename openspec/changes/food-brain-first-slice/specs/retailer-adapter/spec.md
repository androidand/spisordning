# retailer-adapter (delta)

## ADDED Requirements

### Requirement: Retailer interface wrapping the Willys client

The system SHALL expose a retailer adapter as an HTTP service that wraps the existing
`willys-client` without porting it, offering at least `SearchProducts`, `GetProduct`,
`resolveRequirements`, and `CreateShoppingList`. The adapter SHALL own session, CSRF, retry,
rate-limiting, and home-store pinning so that callers never handle cookies or store state.

#### Scenario: Home store pinned before store-scoped queries

- **WHEN** the adapter serves a campaign or price query
- **THEN** it ensures the session's active store is the customer's own store first

#### Scenario: Caller never handles session state

- **WHEN** the Go planner calls the adapter over HTTP
- **THEN** it passes only domain data (queries, requirements)
- **AND** never sends or receives Willys cookies or CSRF tokens

### Requirement: Requirement-to-product resolution

The adapter SHALL resolve a canonical requirement
`{ ingredientId, quantity, unit, acceptableForms[], preferredForm }` into a retailer result
`{ retailerProductId, packages, resolvedQuantity, matchType, confidence }`, and SHALL flag
low-confidence matches for human review rather than committing them silently.

#### Scenario: Low-confidence match is flagged, not committed

- **WHEN** resolution confidence for a requirement is below the review threshold
- **THEN** the result is marked as needing review
- **AND** is not silently placed on the shopping list

### Requirement: Shopping list output as a durable wishlist

The adapter's primary output SHALL be a durable per-week Willys **wishlist**. Converting a
wishlist to the session cart SHALL be a separate, explicitly-triggered step. The adapter SHALL
NOT perform checkout, payment, or slot booking.

#### Scenario: Weekly requirements become a wishlist

- **WHEN** `CreateShoppingList` runs for an approved plan
- **THEN** a named per-week wishlist is created via the Willys client
- **AND** no cart is filled and no payment is initiated as part of that call

#### Scenario: Cart fill is opt-in and separate

- **WHEN** the family explicitly requests filling the cart from a wishlist
- **THEN** the adapter converts the wishlist to the session cart in a distinct operation
- **AND** payment and slot booking remain manual
