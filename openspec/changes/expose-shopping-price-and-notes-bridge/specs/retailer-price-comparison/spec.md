## ADDED Requirements

### Requirement: Shopping requirements resolve to a per-retailer price comparison
Given a set of canonical shopping requirements, spisordning SHALL resolve each against every
configured retailer adapter and report, per item, the cheapest available match along with each
retailer's individual resolution (name, price, confidence), so a household can see what they'd pay
at each store before choosing where to buy.

#### Scenario: Two retailers both resolve an item
- **WHEN** a shopping requirement for "mjölk 1l" is compared across Willys and ICA
- **THEN** the comparison result includes both retailers' resolutions and their prices
- **AND** identifies which one is cheaper

#### Scenario: A retailer session is stale
- **WHEN** the ICA adapter's session has expired (second-auth not refreshed)
- **THEN** the comparison result marks ICA as unavailable for that request
- **AND** still returns the Willys resolution rather than failing the whole comparison

#### Scenario: An item has no price from either retailer
- **WHEN** neither retailer returns a price for an item (e.g. both need review)
- **THEN** the comparison reports the item as unresolved rather than guessing a cheapest option
