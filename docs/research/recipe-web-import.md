# Generic web recipe import pipeline

Research/design deliverable for `implement-recipe-discovery` Section 2 (Generic web recipe
import pipeline). This documents the pipeline design (task 2.1), how ingredient canonicalization
hooks into the existing `Ingredient` vocabulary and review flow (task 2.2), the fallback trigger
for per-site parsers (task 2.3), and the sites actually tested against it (task 2.4). The
scaffolding that implements it lives in `internal/recipeimport` (Section 5).

The design is driven by one empirical fact from Section 1: **ICA.se, Köket.se, and Arla.se all
publish a complete, standard `schema.org/Recipe` JSON-LD node**, while Coop.se publishes none.
So the pipeline is built around the standard format first, with a per-site fallback only for the
sites that lack it.

## 2.1 — Pipeline stages

```text
 ┌─────────┐   ┌──────────────────┐   ┌────────────────────┐   ┌──────────────────────┐
 │ 1 FETCH │ → │ 2 FIND JSON-LD   │ → │ 3 PARSE RECIPE     │ → │ 4 PARSE INGREDIENTS  │
 │  URL    │   │  (@type:Recipe)  │   │  → ParsedRecipe    │   │  (qty/unit/food)     │
 └─────────┘   └──────────────────┘   └────────────────────┘   └──────────┬───────────┘
                                                                          │
 ┌─────────┐   ┌──────────────────┐   ┌────────────────────┐              │
 │ 7 IMPORT│ ← │ 6 REVIEW         │ ← │ 5 CANONICALIZE     │ ←────────────┘
 │ candidate│  │  unresolved      │   │  vs Ingredient     │
 │ +prov.  │   │  mappings        │   │  (needs_review)    │
 └─────────┘   └──────────────────┘   └────────────────────┘
```

**Stage 1 — Fetch URL.** `GET` the recipe page with a browser-like `User-Agent`, following
redirects (note: a site's link URL can differ from its canonical URL — Coop's does). Enforce a
timeout and a response-size cap. Where a `robots.txt` exists, respect it; where it does not
(ICA/Köket/Arla/Coop all lack one or are out of scope), the decision to import a *single,
user-requested* page for a *private, self-hosted* household is a deliberate, low-volume act —
not bulk scraping. The raw HTML is retained so the parse stages are testable offline (task 6.2
uses captured markup as fixtures, no live network in tests).

**Stage 2 — Find Recipe JSON-LD.** Parse the HTML and extract every `<script
type="application/ld+json">` block, JSON-decode each, and select the node(s) whose `@type` is
`Recipe`. This must handle all three shapes observed in the wild: a single top-level object, an
array of nodes (Köket emits `[Corporation, Recipe]`), and an `@graph` wrapper. This step is the
whole reason the pipeline is generic — it is what lets ICA/Köket/Arla import with **zero
site-specific code**. If no `Recipe` node is found, the pipeline hands off to a per-site parser
(Stage 2.3 below) or fails with a clear 'no Recipe JSON-LD' result.

**Stage 3 — Parse structured recipe.** Map the `schema.org/Recipe` fields onto an internal
`ParsedRecipe` value:

| schema.org field | Spisordning field | Notes |
|---|---|---|
| `name` | `title` | required |
| `description` | `description` | |
| `image` | `image_url` | may be a string or an array — take the first |
| `recipeYield` | `servings` | parse `'10 portioner'` / `'6'` / `'12 bitar'` → number |
| `totalTime` / `prepTime` / `cookTime` | `total/prep/cook` | ISO-8601 durations (`PT45M`, `PT90M`) |
| `recipeCategory` | `category` | |
| `recipeCuisine` | `cuisine` | |
| `author` / `publisher` | `attribution` | Person or Organization name |
| `recipeIngredient` | `ingredients []string` | raw lines, order preserved |
| `recipeInstructions` | `steps []string` | flatten HowToStep / HowToSection trees to ordered text |
| `aggregateRating` | `rating` (optional) | `ratingValue` + `reviewCount` |
| `nutrition` | `nutrition` (optional) | NutritionInformation block |
| (the fetched URL) | `source_url` | provenance |
| (source's id, if any) | `external_id` | provenance |

`recipeInstructions` needs care: ICA/Köket emit a flat `HowToStep` list, but Arla nests
`HowToSection` → `HowToStep`. The parser walks the tree depth-first and emits each step's `text`
in document order, so both shapes produce a clean ordered step list.

**Stage 4 — Parse ingredient strings.** Each raw line is free text and is split into
`(quantity, unit, food, modifiers)`. Swedish units dominate: `dl`, `msk`, `tsk`, `förp`, `st`,
`kg`, `g`, `kl`, `klyfta`, `klyftor`. Examples from the tested pages:
- `'2 kg fast potatis'` → qty `2`, unit `kg`, food `fast potatis`
- `'5 dl standardmjölk'` → qty `5`, unit `dl`, food `standardmjölk`
- `'1 gul lök, skalad och strimlad'` → qty `1`, unit (st), food `gul lök`, modifiers `skalad, strimlad`
- `'2 klyftor vitlök, finhackade'` → qty `2`, unit `klyftor`, food `vitlök`, modifier `finhackad`

Lines that are **not** ingredients (section headers / notes) are detected and excluded — e.g.
Köket's `'Till en form (10 personer)'`. The parser is deliberately conservative: when it cannot
confidently split a line, it keeps the whole line as the food text and flags it for review rather
than guessing.

**Stage 5 — Canonicalize ingredients.** Each parsed food name is resolved against the canonical
`Ingredient` vocabulary owned by `establish-household-and-catalog`. Resolved → link the canonical
`ingredient.id`; unresolved → keep the raw line and set `needs_review = true` (see 2.2). Nothing
is ever silently dropped.

**Stage 6 — Review unresolved mappings.** The review surface lists every candidate ingredient
with `needs_review = true`; the human resolves each by picking a canonical ingredient, adding a
new one, or marking the line as a non-ingredient. This reuses the existing `food-brain
ingredients` review interaction (see 2.2).

**Stage 7 — Import.** The recipe is stored as a **candidate** with full provenance (`source_url`,
`source_name`, `license_note`, `external_id`, `imported_at`) and its parsed ingredients. It is
*not* yet part of the household cookbook — that promotion is Section 4's lifecycle, gated on an
explicit review action.

## 2.2 — How canonicalization hooks into the existing vocabulary

This does **not** invent a parallel mechanism. It reuses the exact `needs_review` pattern already
established by `ingredient_mapping` (see `migrations/0001_init.sql`):

```sql
CREATE TABLE ingredient_mapping (
    mealie_food_id  TEXT PRIMARY KEY,
    ingredient_id   TEXT NOT NULL REFERENCES ingredient(id),
    ...
    needs_review    BOOLEAN NOT NULL DEFAULT true,   -- ← the pattern
    ...
);
```

An imported ingredient line is the same shape: a *source identifier* (here the parsed food name /
raw line, instead of a `mealie_food_id`) that maps to a canonical `ingredient.id`, with a
`needs_review BOOLEAN NOT NULL DEFAULT true` flag. The candidate-ingredient table
(`recipe_import_candidate_ingredient`, designed in Section 5 / `design.md`) carries
`ingredient_id` (nullable — NULL while unresolved) plus `needs_review`, so:

- **Resolved** line → `ingredient_id` set, `needs_review` cleared.
- **Unresolved** line → `ingredient_id` NULL, `needs_review = true`, raw line retained.

The review flow is the same human-in-the-loop surface the household already uses for Mealie
mappings: list the `needs_review` rows, resolve each, clear the flag. Because the canonical
`Ingredient` and the `Ingredient → Product` mapping both live in `establish-household-and-catalog`,
an imported line that resolves to a canonical ingredient automatically participates in the
existing `Product`/shopping-requirement flow — no new mapping layer is introduced.

## 2.3 — Per-site parsers: the fallback trigger

The generic JSON-LD path (Stages 2–7) handles **any** site that emits a valid `schema.org/Recipe`
node. A dedicated per-site parser is added **only** when the fallback trigger fires:

> **Fallback trigger:** the fetched page has **no** `@type: Recipe` JSON-LD node, **or** the node
> is present but is missing a required field (no `recipeIngredient`, or no `recipeInstructions`).

When the trigger fires, a small site-specific adapter is written that produces the **same**
`ParsedRecipe` struct the generic path outputs — so Stages 4–7 stay source-agnostic and the
parser is the only site-coupled code. Parsers are added lazily, one at a time, only when a
specific source is actually needed.

**Concrete case that trips the trigger: Coop.se.** It emits no `Recipe` JSON-LD at all — its
metadata lives in OpenGraph tags + a `window.dataLayer` push, and its body in a React micro-app
backed by an undocumented internal API (`proxy.api.coop.se/external/recipe`). A Coop adapter would
read the `dataLayer`/internal API and emit a `ParsedRecipe`. It is **deferred** (Section 1
decision) until the generic path is proven on the three JSON-LD sites; the trigger condition and
the adapter's output contract are documented here so it can be added without redesign.

## 2.4 — Sites tested against the pipeline

| Site | JSON-LD present? | Path used | Result |
|---|---|---|---|
| **ICA.se** | Yes — full `Recipe` node | Generic (Stage 2) | Parses cleanly: title, image, description, author (Org), rating, `totalTime` `PT90M`, category, yield, nutrition, 5 HowToStep. |
| **Köket.se** | Yes — `Recipe` node in a 2-node array | Generic (Stage 2, array shape) | Parses cleanly: title, image, description, author (Person), `totalTime` `PT45M`, yield `'10 portioner'`, 8 ingredients, 5 HowToStep. Exercises the array-of-nodes branch. |
| **Arla.se** | Yes — full `Recipe` node (most complete) | Generic (Stage 2) | Parses cleanly: title, image, author (Person), publisher (Org), prep/cook/total split, yield, category, nutrition, 11 ingredients, nested HowToSection/HowToStep. Exercises the nested-instructions branch. |
| **Coop.se** | **No** | Fallback trigger fires | No `Recipe` node → per-site parser required (deferred). Documented as the fallback case. |

The captured JSON-LD from the three successful pages (ICA, Köket, Arla) becomes the fixture set
for the parsing unit tests (task 6.2) — one fixture deliberately exercises each structural branch
(single object, node array, nested instructions).
