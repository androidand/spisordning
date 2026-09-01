## Why

The shopping pipeline is complete but person-shaped inputs are not. Spisordning already resolves and
compares canonical shopping lines against every configured retailer adapter (`internal/retailer.Compare`,
well-tested), and does so over two existing surfaces — REST `POST /compare` and MCP
`compare_shopping_prices`, both backed by `PriceComparisonService.ComparePrices`. But those surfaces
accept a *canonical* requirement (an ingredient name + quantity + unit), which is exactly what a
planned meal's shopping requirements are. There is no surface where a person can drop a jotted list
("500g chicken, 2 onions, cream") and get back the same price-ranked recommendation. The household
writes shopping lists by hand (Apple Notes, paper, the Willys app); today none of those free-text
lines reach the price engine.

## What Changes

- Add a free-text → requirement mapping layer: accept a list of `{ item, quantity, unit }` lines,
  map each `item` onto a canonical ingredient name with `domain.CanonicalIngredientID` (the same
  normalization the plan pipeline uses when structuring recipe notes), and feed the mapped lines to
  the **existing** `PriceComparisonService.ComparePrices`.
- **No hard reject.** A jotted line that no retailer recognizes comes back `unresolved: true` with
  its label and quantity preserved — the person still sees what they asked for and which retailer (if
  any) has it. This differs from the recipe-import review path (which flags unmapped lines for review);
  a handwritten list degrades to "unresolved", not an error.
- Expose the mapped comparison over a new REST route (`POST /shopping/suggest`) and a new MCP tool
  (`resolve_jotted_list`). Both are thin adapters over `ComparePrices`: they only add the mapping,
  then return `PriceComparison` unchanged. No second comparison path, no new service type, no new
  `internal/dto` service.
- No new migrations: free-text lines are accepted in a single call and mapped on the fly; nothing is
  persisted (a recommendation, not a durable list — `shopping_list`/`shopping_list_item` are the
  durable objects, owned by `implement-shopping-and-commerce`).

## Capabilities

### New Capabilities

- `jotted-list-shopping-intelligence`: accept a free-text shopping list, map each item onto a
  canonical ingredient name, and return the existing price comparison across retailer adapters with
  per-item cheapest match and `unresolved` lines preserved. The capability the change exists for.
- `jotted-list-rest-exposure`: expose that comparison over REST (`POST /shopping/suggest`) — a thin
  HTTP surface over the shared `PriceComparisonService`.
- `jotted-list-mcp-exposure`: expose the same comparison over MCP (`resolve_jotted_list`) — a thin
  MCP adapter over the same service, so an AI chat session can price a handwritten list the same way
  it plans a week.

## Impact

- `internal/httpapi`: new `POST /shopping/suggest` handler + request DTO (mapping free text to
  `CompareRequirement`), registered in the mux. Returns the existing `PriceComparison`.
- `internal/mcptools`, `cmd/mcp-server/adapters.go`: new `resolve_jotted_list` tool (same mapping
  adapter), delegating to the same `PriceComparisonService.ComparePrices`.
- No change to `internal/retailer.Compare`, the retailer clients, the wishlist/checkout flow, the
  Apple Notes bridge, or deployment.
- No new migrations; no new `internal/dto` service.
- `api/openapi.yaml`: new request schema for `/shopping/suggest` (response reuses the `compare`
  response); regenerate `internal/openapi/types.gen.go`.
- Part of Epic F: Retailer, Pricing & Commerce.
