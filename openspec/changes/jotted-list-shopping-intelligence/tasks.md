# Tasks: jotted-list-shopping-intelligence

Accept a free-text shopping list, map each item onto a canonical ingredient name with
`domain.CanonicalIngredientID`, and return the **existing** `PriceComparisonService.ComparePrices`
result across retailer adapters. The two new surfaces (`POST /shopping/suggest` + MCP
`resolve_jotted_list`) are thin adapters over that one method — they only add the mapping and pass
through the response. No new service type, no new comparison logic, no new migrations.

Test throughout: a shared unit test asserts identical output for identical input across the REST and
MCP surfaces (the mapping adapter is the only thing being tested, and it is the same code on both).

## 1. REST surface: POST /shopping/suggest

- [ ] 1.1 Add `internal/httpapi/jotted_list.go` with a `jottedListHandler` holding a
       `PriceComparisonService` and a `suggest` method.
- [ ] 1.2 Define the request DTO `JottedListInput { Items []JottedListItem }`,
       `JottedListItem { Item string; Quantity float64; Unit string }`, and register `POST /shopping/suggest`
       to decode it, map each item to a `CompareRequirement{ Ingredient: CanonicalIngredientID(item), Quantity, Unit }`,
       call `ComparePrices`, and write the `PriceComparison`.
- [ ] 1.3 Return `400` when `items` is empty/absent; map the mapping + `ComparePrices` straight through
       otherwise (including `unresolved` lines).
- [ ] 1.4 Register the handler in the mux in `internal/httpapi/people.go` (guarded by
       `deps.PriceComparison != nil`, alongside `POST /compare`).
- [ ] 1.5 Handler test: `POST /shopping/suggest` returns the comparison from a fake
       `PriceComparisonService`; an empty list returns 400; a `unresolved` line is preserved in the response.

## 2. MCP surface: resolve_jotted_list

- [ ] 2.1 Add the `resolve_jotted_list` tool input DTO (`JottedListInput`, reused) and a handler in
       `internal/mcptools` that maps items to the same `CompareRequirement` shape and calls
       `PriceComparisonService.ComparePrices`.
- [ ] 2.2 Wire the tool in `internal/mcptools/mcptools.go` (register the handler with the same
       `PriceComparisonService` the `compare_shopping_prices` tool already uses).
- [ ] 2.3 Implement `ComparePrices` on `cmd/mcp-server/adapters.go`'s store adapter is already present
       (verify the tool's `PriceComparisonService` resolves — no new method there).
- [ ] 2.4 MCP test: identical input to the REST handler yields an identical `PriceComparison`
       (shared assertion with task 1's test).

## 3. OpenAPI

- [ ] 3.1 Add the `POST /shopping/suggest` request schema to `api/openapi.yaml` (response reuses the
         existing `compare` response shape).
- [ ] 3.2 Regenerate `internal/openapi/types.gen.go` via the repo's `go generate` for openapi.

## 4. Verification

- [ ] 4.1 Run the architecture test (`go test ./...`) — the new handler/service stays in its layer and
         never imports the generated store-clients.
- [ ] 4.2 `go build ./...` and `go vet ./...`.
- [ ] 4.3 Run the full test suite (`go test ./...`); confirm no regressions in
         `internal/httpapi`/`internal/mcptools`.
- [ ] 4.4 `openspec validate --change jotted-list-shopping-intelligence`; each requirement scenario
         maps to a test in tasks 1–2.
