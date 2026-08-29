# Tasks: implement-recipe-structuring-from-text

## 1. Mealie write path

- [ ] 1.1 Add `CreateRecipe(ctx, name string) (recipeID string, err error)` to
      `internal/mealie.Client` — Mealie's create-by-name endpoint, returns the new (empty) recipe's
      id/slug for the follow-up PATCH calls.
- [ ] 1.2 Add `SetIngredients(ctx, recipeID string, lines []IngredientLine) error` — PATCHes
      `recipeIngredient` using the verified-safe shape: fresh UUID `referenceId` per line, clean
      `null` `food`/`unit` when unstructured, populated `food`/`unit`/`quantity` when the brute
      parser resolved them. Reuses `parseNotes` (already added for the read-side fix) rather than
      a second parser call path.
- [ ] 1.3 Add `SetInstructions(ctx, recipeID string, steps []string) error` — PATCHes
      `recipeInstructions` with the full required object shape (`id`, `title: ""`, `summary: ""`,
      `text`, `ingredientReferences: []`) per step.
- [ ] 1.4 Update the package doc comment on `internal/mealie/client.go` to note the scoped write
      exception and why (link back to this change).
- [ ] 1.5 Unit tests against a fake Mealie HTTP server for all three methods, including a case
      that reproduces the referenceId-corruption shape being avoided (assert the request body
      always carries a non-empty `referenceId`) and a case reproducing the instructions-500 bug
      (assert the request body always carries `id`/`title`/`summary`/`ingredientReferences`).

## 2. Freeform text sectioning

- [ ] 2.1 Implement a small sectioning function: title = first non-blank line; ingredients = lines
      after the title up to a "Gör så här"/"Instruktioner"-style marker (case-insensitive,
      configurable list of recognized markers); instructions = everything after the marker.
      Fall back to a blank-line heuristic (first blank-separated block after the title is
      ingredients, rest is instructions) when no marker line is found.
- [ ] 2.2 Instruction-line grouping: collapse consecutive non-blank lines after the marker into
      one step per originally-blank-line-separated paragraph (matches the worked example, where
      "Grädda i mitten..." is its own step despite the blank line before it) — verify against the
      worked example in proposal.md specifically, since that's the real shape this needs to handle.
- [ ] 2.3 Unit tests against the worked example from proposal.md end to end: title
      "Pasta och tacokyckling i ugn", 8 ingredient lines, 4 instruction steps (3 pre-blank-line +
      the oven step after it).

## 3. Ingredient confidence reporting

- [ ] 3.1 Extend the ingredient-parsing path to carry forward Mealie's own per-line `confidence`
      score (already present in `/api/parser/ingredients`'s response — see the raw response shape
      captured this session) rather than discarding it.
- [ ] 3.2 Surface low-confidence lines (bare `"salt"`, the `"(eller något annat?)"` aside) in the
      tool's response as a `low_confidence: string[]` list, not silently — so a chat session can
      tell the user "I structured this but wasn't sure about: salladskrydda" instead of presenting
      a guess as fact.

## 4. MCP tool

- [ ] 4.1 Design the tool schema: input `{raw_text: string}`, output `{recipe_id, title,
      ingredients: [...], instructions: [...], low_confidence: [...]}`.
- [ ] 4.2 Add a `Recipes`-service method (alongside the existing `SyncFromMealie`) that sections
      the text, calls `CreateRecipe`/`SetIngredients`/`SetInstructions`, and returns the structured
      result — keep composition-root (`cmd/mcp-server/adapters.go`) thin, matching how
      `ShoppingRequirements` already delegates to `internal/planning`/`internal/service` rather
      than doing real work itself.
- [ ] 4.3 Register the MCP tool in `internal/mcptools` following the existing pattern (schema +
      handler in `mcptools`, service interface implemented by the composition root).
- [ ] 4.4 Tag recipes created this way (e.g. `chat-import`) so they're distinguishable from the
      Middagsbank imports and this session's 7 hand-imported dinners.

## 5. Verification

- [ ] 5.1 `go build ./... && go test ./... && go vet ./...` green.
- [ ] 5.2 Live-verify against the real Mealie instance using the exact worked example from
      proposal.md: create the recipe via the new tool, then GET it back and confirm it reads
      cleanly (not corrupted — the same verification discipline used for this session's 7 manual
      imports) and that the ingredients/instructions match what was intended.
- [ ] 5.3 Live-verify a line the parser handles badly on purpose (feed it something odd) to
      confirm the low-confidence reporting actually surfaces it rather than silently guessing.
