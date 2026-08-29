# Implement recipe structuring from freeform text

## Why

Andreas writes recipes the way most home cooks actually think — a title, a loose ingredient
list, a "Gör så här:" block of steps, shorthand quantities, an occasional aside ("eller något
annat?"). That's real input, not malformed input, and today there is no path from it to a usable
Mealie recipe: `internal/mealie.Client` is explicitly read-only (see its package doc), and every
recipe imported into Mealie so far in this project happened via ad-hoc raw HTTP calls made
directly against Mealie's API from an agent session — not through spisordning, not repeatable,
not exposed as a tool a chat session can call on the user's behalf.

Worked example (Andreas's own input, 2026-08-29), to ground the design against real shorthand
rather than a clean synthetic case:

```
Pasta och tacokyckling i ugn
500g kokt pasta
1 påse fryst tacokyckling (Ica basic) 600g
1 burk tacosås
1 stor burk Creme fraiche
salt
svartpeppar
salladskrydda (eller något annat?)
Riven ost

Gör så här:
Koka pastan och lägg i en ungnsfast form
Strö över kycklingen över pastan
Blanda såsen: creme fraiche, tacosås och kryddor
Strö över osten

Grädda i mitten av ugnen på 200 grader C, ca 20-32 min
```

Note what makes this a real test, not a toy: quantities are inconsistent in form ("500g" vs "1
burk" vs bare "salt" with none at all), one line has a parenthetical brand aside that isn't part
of the ingredient name, and the last instruction line reads as a step even though it comes after
a blank line that could be mistaken for a section break.

Two hard-won facts from this session directly shape the design, not guesses:

- **Ingredient-row corruption bug** (found and fixed manually earlier this session): PATCHing a
  `recipeIngredient` with `referenceId` omitted/null permanently corrupts the recipe (the row's
  `reference_id` ends up `NULL` server-side, and the whole recipe becomes unreadable via GET
  forever after). Every ingredient row this change writes must carry a freshly generated UUID
  `referenceId`, and `food`/`unit` must be clean `null` when unstructured — never a partial
  object like `{id: null}`.
- **`recipeInstructions` needs the full object shape**, not just `{text}` — found while importing
  the 7 dinners this session: `{text}}` alone throws a 500. Needs `id` (fresh UUID), `title: ""`,
  `summary: ""`, `ingredientReferences: []` alongside `text`.
- Mealie's own brute ingredient parser (`POST /api/parser/ingredients`) is what
  `internal/mealie/client.go`'s `parseUnstructured` already uses on the *read* side, and this
  session found and fixed a real bug in it (500s on a comma in the note, isolated via per-note
  retry — see `internal/mealie/client.go`). The same endpoint, and the same per-note-retry
  resilience, should be reused for the *write* side rather than writing a second ingredient
  parser — Mealie's parser already understands Swedish shorthand quantities/units reasonably
  well; the gap is that nothing calls it as part of *creating* a recipe.

## What Changes

- Add a write path to `internal/mealie.Client` (currently read-only): `CreateRecipe`,
  `SetIngredients`, `SetInstructions` (or one combined `PublishStructuredRecipe` call) — using the
  exact safe shapes above. This is a deliberate, scoped exception to the read-only design, not a
  general opening — document why on the package doc.
- Add a text-sectioning step (new, small — title / ingredients / instructions split on a
  "Gör så här"-style marker, falling back to a blank-line heuristic when no marker is present) and
  reuse the existing brute-parser call (extend `parseNotes`'s per-note-retry pattern, don't
  duplicate it) to structure each ingredient line's food/unit/quantity before writing it.
- Add an MCP tool (name TBD, e.g. `structure_recipe`) taking raw freeform text and returning the
  created Mealie recipe's id, title, structured ingredients, and instructions — so a chat session
  can hand it exactly the kind of text in the worked example above and get back a real recipe,
  not a manual multi-step process.
- Tag structured-from-text recipes distinctly (e.g. `chat-import`) so they're identifiable later,
  same spirit as the `lågeffekt`/`matlåda` tagging already used for this session's 7 dinners.

## Explicit scope boundaries

- Best-effort structuring, not guaranteed-correct: a line the brute parser can't confidently parse
  (bare `"salt"` with no quantity, the `"(eller något annat?)"` aside) should be written with
  whatever it recovers, not blocked or dropped — same "leave it unstructured rather than crash"
  philosophy already used on the read side. Report back to the caller which lines were low-
  confidence, so a chat session can flag it to the user rather than presenting silently-wrong data
  as fact.
- Does not attempt to resolve structured ingredients to retailer products in the same call — that
  stays a separate step through the existing shopping-requirement pipeline, keeping this change
  about recipe authoring only.
- Does not touch recipe *editing* (updating an existing Mealie recipe from new text) — creation
  only, editing is a natural follow-up once this lands.

## Impact

- `internal/mealie/client.go`: gains write methods; package doc comment needs updating to reflect
  the (scoped) exception to read-only.
- `internal/mcptools`, `cmd/mcp-server/adapters.go`: new tool + composition-root wiring, following
  the existing `mcptools` pattern (schema + handler in `mcptools`, real implementation in the
  composition root against `internal/service`/`internal/mealie`).
- Likely a new small `internal/service` method (e.g. on a `Recipes` service already used by
  `sync recipes`) rather than calling `internal/mealie` directly from the composition root, to
  keep the layering consistent with how `SyncFromMealie` already works.
