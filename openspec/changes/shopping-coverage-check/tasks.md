# Tasks: shopping-coverage-check

Implementation of the `check_shopping_coverage` tool. The coverage computation is a pure,
unit-tested function in `internal/coverage`; a thin service method wraps it; the MCP tool
is a thin adapter. MCP-only; no REST endpoint, no migrations.

## 1. Pure coverage aggregation (internal/coverage/coverage.go)

- [x] 1.1 Define the coverage model types: `Key{IngredientID string; Unit string}` (the
  `(ingredient_id, unit)` group key), `Requirement{Key, Quantity float64, Name string}`,
  `Supply{Key, Quantity float64}` (aggregated list-item quantities per key), `Line{
  Key, Name, Status, Required float64, Supplied float64, Shortfall float64}`, `Report{
  Lines []Line; ShortCount int; MissingCount int}`.
- [x] 1.2 Implement `Aggregate(items []Supply) map[Key]float64` — sums quantity per key.
- [x] 1.3 Implement `Check(reqs []Requirement, supplied map[Key]float64) Report` — for each
  requirement, set `Supplied` from the map (0 when absent), `Status`
  (`covered` when supplied ≥ required, else `short`), `Shortfall = required − supplied`
  (positive only), and count `short`/`missing` (a line is `missing` when supplied == 0 and
  required > 0). `MissingCount` counts `missing` lines; `ShortCount` counts `short` lines.
- [x] 1.4 Write unit tests in `internal/coverage/coverage_test.go`:
  - fully covered (every line `covered`, counts 0),
  - short on one ingredient (correct shortfall),
  - missing on one ingredient,
  - two items summed into one key (grouping),
  - a second unit for the same ingredient is a distinct key (not summed across units),
  - a staple-like requirement that is absent is still counted (coverage does not know about
    staples — that is the caller's job via persisted post-staple requirements),
  - empty requirements yields an empty report with 0 counts.

## 2. Application/service layer (internal/service/shopping.go)

- [x] 2.1 Define `CoverageService` with `CheckCoverage(ctx, listID, planID) (CoverageReport, error)`.
- [x] 2.2 Implement `Coverage.CheckCoverage`: parse `planID` and `listID`; call
  `db.ListShoppingRequirements(ctx, planID)` and `db.ListShoppingListItems(ctx, listID)`;
  build `[]Requirement` from the plan rows (key = `IngredientID.String()` + `Unit`,
  `Quantity` = `Quantity`, `Name` = `IngredientName`) and `map[Key]float64` from the list
  items that carry an `IngredientID` (skip label-only items, which are not plan-derived);
  call `coverage.Check`; return the report. Wrap persistence errors with a `service:` prefix.
- [x] 2.3 Add `ListShoppingListItems` to the `service.Store` interface in
  `internal/service/service.go` (it already exists on the persistence `Store` and is
  implemented; only the interface method is new).
- [x] 2.4 Ensure the concrete `storeAdapter` in `cmd/food-brain/adapters.go` and any
  persistence `Store` used by the MCP server satisfy the extended interface.

## 3. MCP tool surface (internal/mcptools + cmd/mcp-server)

- [x] 3.1 Define `CheckCoverageInput{shopping_list_id string, plan_id string}` with
  validation: both fields required and non-empty.
- [x] 3.2 Define `CoverageLine{ingredient_id string, ingredient_name string, unit string,
  status string, required float64, supplied float64, shortfall float64}` and
  `CoverageReport{short_count int, missing_count int, lines []CoverageLine}`.
- [x] 3.3 Define `CoverageService` MCP interface with `CheckCoverage(ctx, in) (CoverageReport, error)`.
- [x] 3.4 Register the `check_shopping_coverage` tool in `internal/mcptools/mcptools.go`
  inside the `deps.ShoppingList != nil` block, guarded by the same guard as
  `create_shopping_list`, with a description stating it compares a filled shopping list
  against a plan's requirements and returns covered/short/missing per ingredient.
- [x] 3.5 Implement the `check_shopping_coverage` handler — thin: validate input, call
  `CoverageService.CheckCoverage`, return the report.
- [x] 3.6 Add `CheckCoverage` to `mcpStoreAdapter` in `cmd/mcp-server/adapters.go`,
  delegating to `service.Coverage.CheckCoverage`.

## 4. Verification

- [x] 4.1 `go build ./...` succeeds.
- [x] 4.2 `go vet ./...` reports no issues.
- [x] 4.3 `go test ./...` — new `internal/coverage` unit tests pass; no existing test fails.
- [x] 4.4 A new MCP handler test asserts `check_shopping_coverage` returns `short` for a
  list short on one ingredient and `missing` for an absent ingredient, using a fake
  `CoverageService` (no DB required).

## 5. OpenSpec hygiene

- [x] 5.1 `openspec validate shopping-coverage-check` passes.
- [x] 5.2 `specsync -dry-run -change shopping-coverage-check` produces the expected issue delta.
- [x] 5.3 Assign the resulting issue to the appropriate Epic milestone.
