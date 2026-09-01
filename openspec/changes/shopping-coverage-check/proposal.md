# Check shopping coverage against a meal plan

## Why

Today Spisordning can *generate* a meal plan, derive that plan's canonical shopping
requirements, and build a shopping list from those requirements — but nothing checks
that a filled list actually covers the week's meals. An agent (or a person with the
Apple-Notes reader) can check off items, push the list, or walk out the door, and have no
signal that the basket is short a litre of milk, missing the flour, or missing a third of
the plan entirely.

The `create_shopping_list` MCP tool seeds items with an `ingredient_id` (resolved from the
plan's requirements) but **never** a `shopping_requirement_id` — that column is NULL on
every plan-derived item. The list and the plan therefore share only the canonical
`ingredient_id` space, which both sides derive identically. Coverage validation must join
on that id, not on a requirement link that does not exist.

## What Changes

- A **`check_shopping_coverage`** MCP tool that compares a filled `shopping_list` against
  the `shopping_requirement` rows of an `approved` plan, and reports, per
  `(ingredient_id, unit)`, whether the basket is covered, short, or missing.
- The computation lives in the **application/service layer** (`internal/service`), as a
  coverage method on the existing shopping service, and is surfaced to MCP by a thin
  handler. MCP is one presentation module; the functionality is a service capability.
- A new `CoverageLine` + `CoverageReport` shape returned to the MCP client.

### The coverage computation

For the given `plan_id`, read its persisted `shopping_requirement` rows (the plan's
post-staple "what to buy" — staples such as salt, pepper, and oil are already dropped by
`PartitionStaples` before requirements are persisted, so they never appear and are never
falsely reported as missing). For the given `shopping_list_id`, read its items and group
their `quantity` by `(ingredient_id, unit)`. Then, for each required line:

- **covered** — the list supplies ≥ the required quantity on that `(ingredient_id, unit)`.
- **short** — the list supplies some but less than required; `shortfall = required − supplied`.
- **missing** — the list supplies nothing on that line.

Lines are grouped by `(ingredient_id, unit)` — mirroring `planning.BuildRequirements` — and
no unit conversion is attempted. Two lines that differ only in unit are treated as distinct
requirements, exactly as they are during planning.

Free-text and checklist items (`label` set, no `ingredient_id`) cannot be traced to a plan
meal and are reported separately as `not-plan-derived`; they neither satisfy nor are
counted against any requirement.

## Capabilities

### New Capabilities

- `mcp-shopping`: an MCP client can ask whether a filled shopping list actually covers the
  ingredient needs of a specific meal plan, and get a per-ingredient covered/short/missing
  report with shortfall quantities.

### Modified Capabilities

- `mcp-server`: the existing shopping tool set (currently 9 tools — create_shopping_list,
  compare_shopping_prices, resolve_jotted_list, push_shopping_wishlist, list shopping
  carts, etc.) gains one new tool, `check_shopping_coverage`. No existing tool changes.

## Impact

- **Affected code:**
  - `internal/service/shopping.go` (new) — `CoverageService` with `CheckCoverage(ctx, listID, planID) (CoverageReport, error)`, the application-layer coverage computation.
  - `internal/service/service.go` — `Store` interface gains `ListShoppingListItems` (already implemented on the persistence `Store`, only read by the new service method).
  - `internal/mcptools/shopping.go` — `CheckCoverageInput`, `CoverageLine`, `CoverageReport` DTOs; `CoverageService` interface; `check_shopping_coverage` handler.
  - `internal/mcptools/mcptools.go` — register `check_shopping_coverage` under the `ShoppingList` dependency block, guarded by `deps.ShoppingList != nil` (same guard as `create_shopping_list`).
  - `cmd/mcp-server/adapters.go` — `mcpStoreAdapter.CheckCoverage` delegates to `service.Coverage.CheckCoverage`.
  - `internal/coverage/coverage.go` (new) — pure coverage aggregation: given `[]Requirement` and `[]ListItem` keyed by `(ingredient_id, unit)`, emit `CoverageLine`s. Unit-free of persistence and MCP — pure and unit-tested.
  - `api/openapi.yaml`, `internal/openapi/types.gen.go` — add `CoverageLine`/`CoverageReport` schemas if a REST sibling is added later (this change is MCP-only by design).

- **No new migrations.** The coverage report reads `shopping_list_item` and
  `shopping_requirement`, both existing. No schema changes.
