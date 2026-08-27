## ADDED Requirements

### Requirement: MCP tools cover shopping-list creation through wishlist push, and no further
An MCP client SHALL be able to create a shopping list from requirements, compare price across
retailers, and push a chosen set of resolutions to a retailer wishlist — entirely through MCP tools.
No MCP tool SHALL trigger cart conversion, checkout, payment, or delivery-slot booking.

#### Scenario: Creating and pushing a shopping list via MCP
- **WHEN** an MCP client calls a tool to create a shopping list from a set of requirements, then a
  tool to compare price, then a tool to push the cheapest resolutions to Willys
- **THEN** a durable wishlist is created on Willys via the existing adapter flow
- **AND** no cart, checkout, or payment step is triggered

#### Scenario: No MCP tool exposes cart conversion
- **WHEN** the MCP server's tool list is inspected
- **THEN** it contains no tool that calls the existing `ToCart` adapter endpoint

#### Scenario: A stale ICA session doesn't block pushing to Willys
- **WHEN** an MCP client pushes a shopping list to Willys while ICA's session is stale
- **THEN** the Willys push succeeds independent of ICA's session state
