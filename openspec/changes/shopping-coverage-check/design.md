## Context

Spisordning's plan→shopping flow is:

- `meal_plan_decision` — the chosen recipe per slot for a plan.
- `planning.BuildRequirements` — aggregates chosen recipes into `domain.ShoppingRequirement[]` keyed by `(ingredient_id, unit)`, unions acceptable forms.
- `internal/planning.staples.PartitionStaples` — splits requirements into a "buy" set and a "dropped" set (salt, pepper, oil, water, butter, flour, yeast, sugar, … assumed on hand).
- `persistWeek` — persists the buy set into `shopping_requirement(plan_id, ingredient_id, quantity, unit, acceptable_forms, preferred_form)`, deriving `ingredient_id` as
  `domain.IngredientIDForName(domain.CanonicalIngredientID(name))`. Staples are already dropped, so the persisted rows are the plan's actual "what to buy".
- `create_shopping_list` (MCP, `cmd/mcp-server/adapters.go`) — seeds `shopping_list_item` rows from requirements, setting `ingredient_id` (via `resolveShoppingIngredientID`) and **never** `shopping_requirement_id`.

So a filled list and the plan it was built from share only the canonical `ingredient_id`
space. Both sides derive that id through the identical `IngredientIDForName(CanonicalIngredientID(...))` path, so matching on it is exact.

## The simulation that decides the match strategy

Trace a single item through the real flow for "jölk 1 l":

1. Plan is approved → `persistWeek` inserts `shopping_requirement{ingredient_id: IngredientIDForName("jölk"), quantity: 1, unit: "l"}`.
2. Agent calls `create_shopping_list` with the requirement. `resolveShoppingIngredientID` computes
   `IngredientIDForName("jölk")` and inserts `shopping_list_item{ingredient_id: that, quantity: 1, unit: "l"}`.
3. `shopping_requirement_id` on the item is **NULL** — the seeding code (`adapters.go:688`)
   never sets it; `AddShoppingListItem` sets it only if a caller explicitly passes one.

Consequences:

- Matching on `shopping_requirement_id` → every plan-derived item is unlinked → the tool
  would report every plan ingredient as `missing`. Broken in the real data.
- Matching on `ingredient_id`, grouped by `(ingredient_id, unit)` → the ids match by
  construction. Correct.

This is not hypothetical: it is the actual shape of the data as it is written today.

## Decisions

### D1: Coverage lives in the service layer, surfaced by a thin MCP tool

The user's direction: MCP is one presentation module in the presentation layer; the
coverage functionality is an application-layer capability. So the aggregation is a
`service.Coverage.CheckCoverage` method, and the MCP `check_shopping_coverage` tool is a
thin adapter over it — the same shape as `create_shopping_list`.

### D2: Match on `ingredient_id`, grouped by `(ingredient_id, unit)`

Per the simulation (see above). No `shopping_requirement_id` join. No unit conversion — a
line in `kg` and a line in `g` are distinct requirements, exactly as `BuildRequirements`
treats them. This is conservative: it reports only what it can prove, never a false match.

### D3: Compare against persisted requirements (already post-staple)

`shopping_requirement` rows are stored *after* `PartitionStaples` has dropped the
assumed-on-hand staples, so comparing the list against them naturally excludes salt,
pepper, oil, etc. A naive comparison against raw recipe ingredients would flag those as
missing. Reading persisted requirements avoids that without duplicating staple logic.

### D4: One plan at a time, anchored by explicit `plan_id`

`check_shopping_coverage` takes an explicit `plan_id` (the caller reads it from
`get_shopping_requirements`/`get_plan`). This keeps the requirement set unambiguous — a
household with several approved plans sharing ingredients would otherwise need an
auto-derivation heuristic with its own edge cases. Free-text/checklist items (no
`ingredient_id`) are reported as `not-plan-derived` and excluded.

### D5: Coverage statuses reuse `internal/availability` semantics

The status vocabulary (`covered` / `short` / `missing`) and the positive `shortfall`
(when supplied < required) mirror `internal/availability.LineVerdict`. Coverage is
shopping-list-vs-plan; availability is recipe-vs-pantry. They are sibling capabilities with
the same shortfall notion — coverage reuses the notion, not the recipe code.

## Risks and mitigations

- **Free-text items can't be matched.** A checklist line like "paper towels" has only a
  `label`, no `ingredient_id`. It is reported as `not-plan-derived` and neither satisfies
  nor is counted against any requirement. Correct by construction — we do not guess a match.
- **A list built before the plan's requirements were persisted is empty of
  requirements.** Coverage against such a plan is all-`missing`; that is honest. The tool
  reports the plan's requirement count so the caller can see whether the plan was populated.
- **Unit mismatch.** Two lines for the same ingredient in different units are distinct
  requirements (D2). We do not attempt conversion, so a list that supplies the ingredient in
  a different unit shows as missing for the required unit — again conservative and honest.
