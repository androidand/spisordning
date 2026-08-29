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

- [x] 1.1 Add `CreateRecipe(ctx, name string) (slug string, err error)` to `internal/mealie.Client`
      — `POST /api/recipes`, decode the bare-string response body (not a struct).

      ✅ Done 2026-08-29. `internal/mealie/client.go` `CreateRecipe`.
- [x] 1.2 Add `SetIngredients(ctx, slug string, lines []IngredientLine) error` — `PATCH
      /api/recipes/{slug}` with `{"recipeIngredient": [...]}` using the verified shape above.
      Reuses `parseNotes` (already added for the read-side fix) to resolve food/unit/quantity per
      line before writing, rather than a second parser call path.

      ✅ Done 2026-08-29. `internal/mealie/client.go` `SetIngredients` + `recipeIngredientPatch`.
      Calls the existing `parseUnstructured` (which itself calls `parseNotes`) before building the
      patch, so the write side gets the same batch-then-per-note-retry resilience as the read side.
- [x] 1.3 Add `SetInstructions(ctx, slug string, steps []string) error` — `PATCH
      /api/recipes/{slug}` with `{"recipeInstructions": [...]}` using the verified shape above.

      ✅ Done 2026-08-29. `internal/mealie/client.go` `SetInstructions` + `recipeInstructionPatch`.
- [x] 1.4 Update the package doc comment on `internal/mealie/client.go` to note the scoped write
      exception and why (link back to this change).

      ✅ Done 2026-08-29.
- [x] 1.5 Unit tests against a fake Mealie HTTP server for all three methods, including a case
      that reproduces the referenceId-corruption shape being avoided (assert the request body
      always carries a non-empty `referenceId`) and a case reproducing the instructions-500 bug
      (assert the request body always carries `id`/`title`/`summary`/`ingredientReferences`).

      ✅ Done 2026-08-29. `internal/mealie/client_test.go`:
      `TestCreateRecipe_DecodesBareStringResponse`,
      `TestSetIngredients_AlwaysSendsReferenceIDAndCleanNulls`,
      `TestSetInstructions_AlwaysSendsFullObjectShape`. Also added `PatchJSON` to
      `internal/httpclient` (mirroring the existing `PostJSON`) since no PATCH verb existed yet,
      with its own round-trip test. Full `go build ./... && go test ./...` green (471 tests, all
      packages).

## 2. Freeform text sectioning

- [x] 2.1 Implement a small sectioning function: title = first non-blank line; ingredients = lines
      after the title up to a "Gör så här"/"Instruktioner"-style marker (case-insensitive,
      configurable list of recognized markers); instructions = everything after the marker.
      Fall back to a blank-line heuristic (first blank-separated block after the title is
      ingredients, rest is instructions) when no marker line is found.

      ✅ Done 2026-08-29. `internal/service/recipe_structuring.go` `sectionRecipeText` +
      `sectionMarkers` (whole-line, case/colon-insensitive match — "1 msk metod-ost" doesn't
      falsely trigger on "method"). No-marker fallback: everything after the title becomes
      ingredients, no instructions (best-effort, not a hard failure).
- [x] 2.2 Instruction-line handling: one step per non-blank line after the marker; blank lines are
      pure separators (skipped), not merged into or splitting a step. (Earlier draft of this task
      proposed collapsing lines into blank-line-separated paragraphs — miscounted the worked
      example while drafting; corrected 2026-08-29. One-line-per-step is also the more useful
      shape for a step-by-step cooking view, and simpler to reason about.)

      ✅ Done 2026-08-29, same function as 2.1.
- [x] 2.3 Unit tests against the worked example from proposal.md end to end: title
      "Pasta och tacokyckling i ugn", 8 ingredient lines, 5 instruction steps (one per non-blank
      line: Koka pastan.../Strö över kycklingen.../Blanda såsen.../Strö över osten/Grädda i
      mitten... — the blank line before "Grädda" is a separator only, not a step boundary).

      ✅ Done 2026-08-29. `internal/service/recipe_structuring_test.go`
      `TestSectionRecipeText_WorkedExample` (plus `_NoMarker`, `_MarkerCaseAndColonInsensitive`,
      `_MarkerSubstringDoesNotTrigger` for the edge cases).

## 3. Ingredient confidence reporting

- [x] 3.1 Extend the ingredient-parsing path to carry forward Mealie's own per-line `confidence`
      score (already present in `/api/parser/ingredients`'s response — see the raw response shape
      captured this session) rather than discarding it.

      ✅ Done 2026-08-29. `internal/mealie/client.go`: `parsedIngredient` gains `Confidence.Average`;
      `IngredientLine` gains a `Confidence float64` field, set by `applyParsed`. Since
      `SetIngredients` mutates its `lines` argument's backing array in place (parseUnstructured
      already worked this way), no signature change was needed for the confidence to reach the
      caller.
- [x] 3.2 Surface low-confidence lines (bare `"salt"`, the `"(eller något annat?)"` aside) in the
      tool's response as a `low_confidence: string[]` list, not silently — so a chat session can
      tell the user "I structured this but wasn't sure about: salladskrydda" instead of presenting
      a guess as fact.

      ✅ Done 2026-08-29. `internal/service/recipe_structuring.go` `StructureFromText` builds
      `StructuredRecipe.LowConfidence` from any line with `Confidence < 0.5` or an unresolved
      `FoodName` (`lowConfidenceThreshold`). Verified in
      `TestStructureFromText_WorkedExample`: salt/svartpeppar/the "(eller något annat?)" line are
      flagged, the confidently-parsed pasta line is not.

## 4. MCP tool

- [x] 4.1 Design the tool schema: input `{raw_text: string}`, output `{recipe_id, title,
      ingredients: [...], instructions: [...], low_confidence: [...]}`.

      ✅ Done 2026-08-29. `internal/mcptools/recipestructure.go`.
- [x] 4.2 Add a `Recipes`-service method (alongside the existing `SyncFromMealie`) that sections
      the text, calls `CreateRecipe`/`SetIngredients`/`SetInstructions`, and returns the structured
      result — keep composition-root (`cmd/mcp-server/adapters.go`) thin, matching how
      `ShoppingRequirements` already delegates to `internal/planning`/`internal/service` rather
      than doing real work itself.

      ✅ Done 2026-08-29. `Recipes.StructureFromText` (internal/service/recipe_structuring.go).
      `mcpStoreAdapter.StructureRecipe` (cmd/mcp-server/adapters.go) is a thin DTO-mapping shim.
- [x] 4.3 Register the MCP tool in `internal/mcptools` following the existing pattern (schema +
      handler in `mcptools`, service interface implemented by the composition root).

      ✅ Done 2026-08-29. `structure_recipe` tool registered in `RegisterTools`; wired in
      `cmd/mcp-server/main.go`'s `buildMCPDeps`, gated on `appCfg.MealieEnabled()` (degrades
      gracefully — no Mealie configured means the tool isn't registered, same pattern as the rest
      of this file).
- [x] 4.4 Tag recipes created this way (e.g. `chat-import`) so they're distinguishable from the
      Middagsbank imports and this session's 7 hand-imported dinners.

      ✅ Done 2026-08-29. `internal/mealie/client.go` `SetTags`/`getOrCreateTag`. Verified live
      against the real Mealie instance first (not guessed): `POST /api/organizers/tags` 500s if
      the tag already exists (not idempotent), so `getOrCreateTag` always lists first. Best-effort:
      a tagging failure doesn't fail the whole `StructureFromText` call, since the recipe already
      exists and is usable without the tag.

## 5. Verification

- [ ] 5.1 `go build ./... && go test ./... && go vet ./...` green.
- [ ] 5.2 Live-verify against the real Mealie instance using the exact worked example from
      proposal.md: create the recipe via the new tool, then GET it back and confirm it reads
      cleanly (not corrupted — the same verification discipline used for this session's 7 manual
      imports) and that the ingredients/instructions match what was intended.
- [ ] 5.3 Live-verify a line the parser handles badly on purpose (feed it something odd) to
      confirm the low-confidence reporting actually surfaces it rather than silently guessing.
