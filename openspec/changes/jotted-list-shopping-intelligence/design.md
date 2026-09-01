# Design: jotted-list-shopping-intelligence

## Context

Spisordning already resolves and compares canonical shopping lines across every configured retailer
adapter. Two existing surfaces do this over identical plumbing:

- **REST** `POST /compare` (`internal/httpapi/compare.go`) → `PriceComparisonService.ComparePrices(reqs []CompareRequirement)`.
- **MCP** `compare_shopping_prices` (`internal/mcptools/shopping.go`) → same method over
  `[]mcptools.ShoppingRequirement`.

Both take a **canonical requirement** — an `Ingredient` name, `Quantity`, `Unit`, optional forms —
and the composition root (`cmd/food-brain/adapters.go:163`, `cmd/mcp-server/adapters.go:727`)
converts each to a `domain.ShoppingRequirement` and calls the already-tested `internal/retailer.Compare`.
`Compare` resolves each requirement against every adapter in parallel, marks a retailer unavailable on
error (stale ICA session), and computes per-item `Cheapest`. None of that is in question.

What is missing is the **input**: a person jots "500g kycklingfilé, 2 lök, mjöl" and has no recipe to
derive the lines from. There is no surface that accepts a free-text list. This change adds only that
input layer on top of the existing comparison service — it does not reimplement resolution.

## Free-text → requirement mapping

Free-text normalization already exists. `domain.CanonicalIngredientID(foodName)` (`internal/domain/domain.go:110`)
lowercases, trims, and collapses whitespace runs to single spaces — the same normalization the plan
pipeline uses when structuring recipe notes (`internal/service/recipe_structuring.go:108`). The
compare path's `Ingredient` field *is* a canonical ingredient name, so a jotted line maps by
normalizing the item to that name.

The mapping is therefore a **thin adapter**, not a new service:

```
jotted item { item:"kycklingfilé", quantity:500, unit:"g" }
  ──► CompareRequirement{ Ingredient: CanonicalIngredientID("kycklingfilé"), Quantity:500, Unit:"g" }
```

- **No hard reject.** Unlike the recipe-import review path, a jotted line is *not* rejected when the
  canonical name is absent from any mapping table — `CompareRequirement.Ingredient` is just a string
  that the adapter searches for, and `Compare` marks it `unresolved` if no retailer finds it. This is
  the right behavior for a handwritten list: the person can jot "bokhyllsmjölk" and see which
  retailers (if any) recognize it, rather than being told the word is unknown. An `unresolved: true`
  line is still returned with its label and quantity so the caller can eyeball it.
- **Quantity/unit come straight through.** The DTO carries `quantity`/`unit` separately; the adapter
  passes them to `Compare`'s `ShoppingRequirement`. A line like "2 lök" with unit "st" carries the
  amount into comparison instead of being swallowed into the name.
- **The free-text label is preserved** in the response as `label` so the caller can match the result
  back to what the person wrote.

## Reused surfaces

The mapping adapter feeds the **existing** `PriceComparisonService`, which already has both a REST and
an MCP implementation. So the new surface is genuinely thin:

- **REST** (`POST /shopping/suggest`): the handler decodes `{ items: [{ item, quantity, unit }] }`,
  maps each item to a `CompareRequirement` via `CanonicalIngredientID`, and calls the existing
  `PriceComparisonService.ComparePrices`, returning its `PriceComparison` unchanged. No new service,
  no new comparison, no new response mapping.
- **MCP** (`resolve_jotted_list`): identical adapter — decode the jotted items, map to the same
  `CompareRequirement` shape, call the same `ComparePrices`, return the same comparison. The handler
  delegates to the same underlying method as `compare_shopping_prices`.

Both surfaces are documented as thin adapters over `ComparePrices`; a shared unit test asserts
identical output for identical input across REST and MCP (the adapter is the only thing being tested
here, and it is identical code).

## Response shape

`POST /shopping/suggest` returns the **same** `PriceComparison` that `POST /compare` already returns
(`ItemComparison[]` with `ingredient`, `results`, `cheapest`, `unresolved`). The only difference is
the input side. The `ingredient` field carries the canonical name; the caller also sees the original
`label` it sent. No per-item `total_minor` is added — `compare` never computed a list total, and the
jotted list reuses `compare`'s shape verbatim. If a future change wants a grand total, it sums the
per-item `cheapest` values that both surfaces already return.

## What this does NOT do (out of scope, by design)

- **No new comparison/resolution logic.** `internal/retailer.Compare` and the retailer adapter clients
  are unchanged. No new service type, no new `internal/dto` service.
- **No fuzzy matching, synonym graph, or diacritic folding.** Mapping is `CanonicalIngredientID`
  only. If a jotted "bokhyllsmjölk" isn't a real retailer product, it comes back `unresolved: true`,
  not silently corrected.
- **No persistence.** The jotted list is a recommendation, not a durable object.
- **No cart/checkout/wishlist.** Pushing to a Willys wishlist is the adjacent capability.
- **No second retailer client.** Spisordning keeps talking to adapters, never the generated
  store-clients.

## Risks and mitigations

- **Free text that maps to a name but isn't a real product.** Returns `unresolved: true` per retailer,
  not an error — the person still sees what they asked for and which retailer (if any) has it.
- **Quantity/semantics in the name.** Because the label is normalized into `Ingredient`, "500g
  kycklingfilé" sent as a single string would normalize the whole string as the name. The DTO splits
  `item`/`quantity`/`unit` so the caller sends the quantity separately; we document that the name and
  amount are separate fields. We do not attempt to extract a quantity from free text (out of scope).
