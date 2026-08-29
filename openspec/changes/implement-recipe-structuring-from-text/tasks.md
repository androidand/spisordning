# Tasks: implement-recipe-structuring-from-text

## 1. Mealie write path

Verified 2026-08-29 against the live Mealie instance end to end (create → patch ingredients →
patch instructions → readback → delete a throwaway recipe) — every shape below is confirmed
working, not guessed:

- **`POST /api/recipes`** body `{"name": "<title>"}` → `201`, response body is a **bare JSON
  string** (the new recipe's slug), not an object: `"scratch-test-delete-me-..."`.
- **`GET /api/recipes/{slug}`** → the full recipe object, including its real `id` (UUID) —
  needed because the create call only gives you the slug.
- **`PATCH /api/recipes/{slug}`** (keyed by slug, not id) body `{"recipeIngredient": [...]}` →
  `200`. Each entry: `{referenceId: <fresh uuid>, note, display, quantity: 0, unit: null,
  food: null, title: null, originalText: null, referencedRecipe: null}` when unstructured, or
  with `unit`/`food`/`quantity` populated when the brute parser resolved them — `referenceId`
  must always be a fresh UUID (the corruption bug from earlier this session).
- **`PATCH /api/recipes/{slug}`** body `{"recipeInstructions": [...]}` → `200`. Each entry:
  `{id: <fresh uuid>, title: "", summary: "", text: "<step>", ingredientReferences: []}` — the
  500-causing bug from earlier this session was posting `{text}` alone.

- [ ] 1.1 Add `CreateRecipe(ctx, name string) (slug string, err error)` to `internal/mealie.Client`
      — `POST /api/recipes`, decode the bare-string response body (not a struct).
- [ ] 1.2 Add `SetIngredients(ctx, slug string, lines []IngredientLine) error` — `PATCH
      /api/recipes/{slug}` with `{"recipeIngredient": [...]}` using the verified shape above.
      Reuses `parseNotes` (already added for the read-side fix) to resolve food/unit/quantity per
      line before writing, rather than a second parser call path.
- [ ] 1.3 Add `SetInstructions(ctx, slug string, steps []string) error` — `PATCH
      /api/recipes/{slug}` with `{"recipeInstructions": [...]}` using the verified shape above.
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
