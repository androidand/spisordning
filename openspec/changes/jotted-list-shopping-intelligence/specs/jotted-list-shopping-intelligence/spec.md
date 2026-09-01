# jotted-list-shopping-intelligence (delta)

## ADDED Requirements

### Requirement: A jotted free-text shopping list is priced across retailers

The system SHALL accept a list of free-text shopping lines and return the existing price
comparison across retailer adapters for those lines. Each free-text item is mapped to a canonical
ingredient name with `domain.CanonicalIngredientID`, then the mapped lines are passed to the shared
`PriceComparisonService.ComparePrices`, whose result is returned unchanged.

#### Scenario: A jotted list is mapped to canonical requirements and compared

- **WHEN** a client POSTs `POST /shopping/suggest` (or calls the MCP `resolve_jotted_list` tool) with
  free-text items `{ items: [ { item: "kycklingfilé", quantity: 500, unit: "g" }, { item: "lök", quantity: 2, unit: "st" } ] }`
- **AND** the underlying `PriceComparisonService` resolves each requirement across the configured
  retailer adapters
- **THEN** the response is the standard `PriceComparison` (`items[]`, each with `ingredient`, per-retailer
  `results`, `cheapest`, and `unresolved`), computed by `ComparePrices`

#### Scenario: Quantity and unit are carried through to the comparison

- **WHEN** a jotted line is submitted with a `quantity` and `unit`
- **THEN** the comparison uses that quantity and unit (the mapped `CompareRequirement.Quantity` and
  `Unit` are forwarded to `ComparePrices`)

#### Scenario: An unresolvable jotted line is reported, not rejected

- **WHEN** a jotted item is submitted that no retailer resolves (e.g. a made-up product name)
- **THEN** the line is returned in `items[]` with `unresolved: true` and its original `item` label
  preserved, rather than being rejected by the mapping step or returning a 4xx

#### Scenario: REST and MCP produce identical output

- **WHEN** the same free-text items are submitted to `POST /shopping/suggest` and to the
  MCP `resolve_jotted_list` tool
- **THEN** the returned `PriceComparison` values are identical (both surfaces are adapters over the
  same `ComparePrices` call)

### Requirement: The jotted list is exposed over REST

The system SHALL expose the jotted-list comparison through a new `POST /shopping/suggest` route
that accepts free-text items and returns the shared `PriceComparison`, with no new comparison logic.

#### Scenario: POST /shopping/suggest accepts free-text items

- **WHEN** a client POSTs `{ items: [ { item: string, quantity: number, unit: string } ] }` to
  `/shopping/suggest`
- **THEN** the server accepts it and returns `200 OK` with a `PriceComparison`

#### Scenario: An empty jotted list is rejected

- **WHEN** a client POSTs `{ items: [] }` (or omits `items`)
- **THEN** the server returns `400 Bad Request`

#### Scenario: The response body matches the compare endpoint

- **WHEN** `POST /shopping/suggest` returns successfully
- **THEN** the response shape is identical to `POST /compare`'s response (`items` with
  `ingredient`, `results`, `cheapest`, `unresolved`)

### Requirement: The jotted list is exposed over MCP

The system SHALL expose the jotted-list comparison through a new `resolve_jotted_list` MCP tool
that accepts free-text items and returns the shared `PriceComparison` from the same
`PriceComparisonService`.

#### Scenario: resolve_jotted_list returns the price comparison

- **WHEN** an AI chat session calls the MCP `resolve_jotted_list` tool with free-text items
- **THEN** the tool returns the standard `PriceComparison` from `ComparePrices`

#### Scenario: resolve_jotted_list maps the free-text items before comparing

- **WHEN** the tool is called with `{ items: [{ item: "kycklingfilé", quantity: 500, unit: "g" }] }`
- **THEN** the returned comparison's `ingredient` for that line is the canonical name
  (`canonicalingredientid("kycklingfilé")`), not the raw jotted string
