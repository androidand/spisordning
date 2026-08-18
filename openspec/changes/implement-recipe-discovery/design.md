# Design — recipe discovery: cookbook-growth lifecycle & persistence

Design deliverable for `implement-recipe-discovery` Sections 4 (Automatic cookbook growth
lifecycle) and 5.1 (persistence / database review). Section 5.2/5.3 (the `internal/recipeimport`
scaffold and the `0002` migration) implement what this document specifies.

## Context

Today the only recipe intake path is Mealie sync (`internal/mealie`), which assumes a recipe
already exists in Mealie and is referenced via `recipe_ref`. PLAN.md's Epic C 'Automatic Cookbook
Growth' loop needs an intake path from *outside* Mealie: an external recipe is discovered,
imported, tried, and — only if the household accepts it — becomes part of the household cookbook.

Two constraints shape this design:

1. **The cookbook's content model is owned by another change.** `RecipeFamily → RecipeVariant →
   RecipeRevision` is designed in `implement-recipe-family-and-revisions` (see its `design.md`)
   and does **not** exist in the database yet. This change must not assume those tables; it must
   stage imported recipes *outside* them and hand off cleanly when they land.
2. **The cookbook is a real membership, not a saved filter.** `establish-reference-lab`'s Mealie
   findings (`docs/research/mealie-recipe-model.md`) confirmed Mealie's 'cookbook' is a saved
   search with no recipe-membership table — it can re-run a query but cannot record the durable
   fact 'this recipe was reviewed and accepted.' This change must provide that durable fact.

## Section 4 — Automatic cookbook growth lifecycle

### 4.1 — The lifecycle (and why it is not 'auto-import everything')

```text
 external recipe (URL / API)
        │  import (pipeline Stage 7)
        ▼
 recipe_import_candidate  (status = 'candidate')      ← NOT in the household cookbook
        │
        │  planned / cooked at least once  (first_served_at set)   [the 'normally' signal]
        │  + an EXPLICIT review action ('accept into cookbook')
        ▼
 PROMOTED → new RecipeFamily + RecipeVariant + first RecipeRevision
            (candidate.status = 'promoted', candidate.promoted_variant_id set)
        ▼
 household cookbook   (owned by implement-recipe-family-and-revisions)
```

- An externally sourced recipe **always** enters as a `recipe_import_candidate`. It is never
  written directly as a `RecipeFamily`/`RecipeVariant`/`RecipeRevision`.
- Promotion to the cookbook requires an **explicit review action**. There is no code path that
  creates cookbook content from an import without that action (enforced and tested — task 6.3).
- This is explicitly **not** 'auto-import everything without review': import and promotion are
  two separate, gated steps, and the gate is a human decision, not a batch operation. Bulk import
  (many recipes in one operation) still yields one candidate per recipe, each needing its own
  review action (spec scenario 'Bulk import does not bypass review').

### 4.2 — What 'save local version' means

An imported recipe does **not** immediately become a `RecipeFamily`/`RecipeVariant`/
`RecipeRevision`. It is **staged separately as a candidate until reviewed**. On the explicit
review action, the candidate's parsed content is *materialized* into the family/variant/revision
model:

- A **new `RecipeFamily`** is created (the imported dish is, by default, its own family — merging
  it into an existing family is a later curation decision, not an import side-effect).
- A **new `RecipeVariant`** is created in that family, titled from the candidate, with the
  candidate's provenance carried into the variant's `sourceAttribution`.
- The **first `RecipeRevision`** is created from the candidate's parsed ingredients + steps.
  Because a revision is immutable-once-created (that change's invariant 1), the imported content
  is a clean first revision — a later household edit is a *new* revision, never a mutation of the
  imported one.

Why stage rather than mint directly: (a) the family/variant/revision model is the household's
*curated* cookbook and is immutable-once-created — unreviewed external content should not mint
families/variants; (b) staging makes 'review' a real gate with a durable before/after; (c) it
matches the proposal exactly ('imported as a candidate, and only becomes part of the household
cookbook after being planned/cooked and reviewed').

**The membership fact.** The durable 'accepted into the cookbook' fact is `candidate.status =
'promoted'` **plus** the `RecipeVariant` row it points to (`promoted_variant_id`). That is a real
row with a status and a pointer — a genuine membership relationship, not a re-runnable filter.
This directly answers the proposal's concern that a saved-search filter cannot represent
acceptance.

**Cross-change handoff.** Promotion calls into `implement-recipe-family-and-revisions`'s commands
(`CreateRecipeFamily` / `CreateRecipeVariant` / `CreateRecipeRevision`). Because those tables do
not exist yet, the candidate table is fully self-contained, and `promoted_variant_id` is a plain
nullable column whose foreign key is **deferred** until that change lands (see 5.1).

### 4.3 — Provenance fields

Every candidate records, at import time:

| Field | Meaning |
|---|---|
| `source_url` | The exact URL the recipe was fetched from (web) or the API record locator. |
| `source_id` → `external_recipe_source.name` | Which registered source it came from (e.g. 'ICA.se'). |
| `external_id` | The source's own recipe id, if it provides one (e.g. ICA `721833`, Coop `3967260`). |
| `license_note` | A short note on the source's terms, where known (attribution requirement, etc.). |
| `imported_at` | The import timestamp. |
| `raw_jsonld` | The parsed `schema.org/Recipe` node, retained for re-sync/audit/re-import. |

On promotion, `source_url` + `source name` + `license_note` + `external_id` are carried into the
new `RecipeVariant.sourceAttribution`, so the provenance travels with the recipe into the
cookbook and attribution / later re-sync / removal remain possible.

### 4.4 — Promotion trigger

- **Hard gate:** an explicit review action (a human 'accept into cookbook' command). This is the
  only way a candidate becomes cookbook content.
- **The 'normally' signal:** promotion is *expected* to follow the recipe having been planned or
  cooked at least once. That is tracked as `first_served_at` on the candidate (set when the
  candidate is first planned/cooked). The review surface surfaces it as context — 'not yet cooked
  — cook it before promoting?' — but it is a **nudge, not a block**, matching the spec's 'SHALL
  *normally* follow' (the absolute requirement is the explicit review action, not the cook).
- **Not on import alone:** importing a recipe sets `status = 'candidate'` and nothing else; no
  import path touches the cookbook.

## Section 5.1 — Persistence: database review questions

Three new tables, all owned by `recipe-discovery`, added by the additive `0002` migration. They
are self-contained (no dependency on the not-yet-existing family/variant/revision tables).

### Table: `external_recipe_source`

A registry of known external recipe sources, giving each a stable identity for attribution.

| Question | Answer |
|---|---|
| Domain concept | A known external recipe source (publisher site or API) that recipes can be imported from. |
| Owner | `recipe-discovery` (this change). |
| Mutator | Manual/admin only — a human registers a source. The import pipeline reads it, never writes it. |
| Mutability | Mutable (name, base_url, license_note, decision, enabled) but low-frequency. |
| History requirement | None — the source's current attributes suffice; no history of a source's decision is needed. |
| Lifecycle | Created on registration; **disabled** (`enabled=false`) rather than deleted once it has candidates. |
| Deletion behavior | Not deletable while any candidate references it (`ON DELETE RESTRICT`); disable instead. |
| Uniqueness | `id` (slug) is the PK; `name` is `UNIQUE`. |
| External ids | None — this is our registry, not an external id. |
| Indexing | PK on `id`; unique on `name`. |
| FK-ability | Referenced by `recipe_import_candidate.source_id`. |

Columns: `id TEXT PK` (slug, e.g. `ica-se`), `name TEXT NOT NULL UNIQUE`, `kind TEXT NOT NULL`
(`jsonld_web` | `api` | `manual`), `base_url TEXT`, `license_note TEXT`, `decision TEXT NOT NULL
DEFAULT 'defer'` (`integrate_now` | `defer` | `omit`), `enabled BOOLEAN NOT NULL DEFAULT true`,
`created_at TIMESTAMPTZ NOT NULL DEFAULT now()`.

### Table: `recipe_import_candidate`

A staged, not-yet-reviewed imported recipe, carrying provenance + parsed content.

| Question | Answer |
|---|---|
| Domain concept | A recipe imported from an external source, staged for review, not yet in the cookbook. |
| Owner | `recipe-discovery`. |
| Mutator | The import pipeline (creates it); the review flow (promotes/rejects it). |
| Mutability | Parsed content is a snapshot set at import and **not edited in place**; only `status` and `promoted_variant_id` change on review. |
| History requirement | The parsed `Recipe` node is retained in `raw_jsonld` so the import is reproducible/auditable; no edit history needed (content isn't edited). |
| Lifecycle | `candidate` → `promoted` \| `rejected`. |
| Deletion behavior | Hard-deletable while still a `candidate` (it is staging). Once `promoted`, retained as the provenance record for the promoted variant — not deleted. |
| Uniqueness | Idempotent re-import: partial unique `(source_id, external_id) WHERE external_id IS NOT NULL` (API sources) and partial unique `(source_url) WHERE external_id IS NULL` (web sources). |
| External ids | `external_id TEXT` (nullable) — the source's own recipe id. |
| Indexing | PK `id`; the two partial uniques above; index on `status` (the review queue); index on `source_id`. |
| FK-ability | `source_id → external_recipe_source(id) ON DELETE RESTRICT`; `promoted_variant_id` is a plain nullable column whose FK to `recipe_variant(id)` is **deferred** to `implement-recipe-family-and-revisions`. |

Columns: `id BIGSERIAL PK`, `source_id TEXT NOT NULL REF external_recipe_source(id) ON DELETE
RESTRICT`, `source_url TEXT NOT NULL`, `external_id TEXT`, `title TEXT NOT NULL`, `description
TEXT`, `image_url TEXT`, `servings INT`, `prep_time_sec INT`, `cook_time_sec INT`, `total_time_sec
INT`, `category TEXT`, `cuisine TEXT`, `attribution TEXT`, `rating DOUBLE PRECISION`,
`rating_count INT`, `nutrition JSONB`, `raw_jsonld JSONB`, `license_note TEXT`, `imported_at
TIMESTAMPTZ NOT NULL DEFAULT now()`, `first_served_at TIMESTAMPTZ`, `status TEXT NOT NULL DEFAULT
'candidate' CHECK (status IN ('candidate','promoted','rejected'))`, `promoted_variant_id TEXT`.

### Table: `recipe_import_candidate_ingredient`

One parsed ingredient line of a candidate, with its canonicalization status. This is where the
`ingredient_mapping.needs_review` pattern is reused (Section 2.2).

| Question | Answer |
|---|---|
| Domain concept | A single parsed ingredient line of a candidate, and whether it resolved to a canonical `Ingredient`. |
| Owner | `recipe-discovery`. |
| Mutator | The import pipeline (creates, `needs_review=true`); the review flow (resolves → sets `ingredient_id`, clears `needs_review`). |
| Mutability | `raw_text` is immutable (it is the source line); `ingredient_id` / `needs_review` change during review. |
| History requirement | None — resolution is a current fact; no per-resolution audit history is needed at scaffold scope. |
| Lifecycle | Created with the candidate; lives and dies with it. |
| Deletion behavior | `ON DELETE CASCADE` from the candidate. |
| Uniqueness | `(candidate_id, line_no)` unique — one row per ingredient line, in document order. |
| External ids | None. |
| Indexing | PK `id`; unique `(candidate_id, line_no)`; index `(candidate_id, needs_review)` for the per-candidate review queue. |
| FK-ability | `candidate_id → recipe_import_candidate(id) ON DELETE CASCADE`; `ingredient_id → ingredient(id) ON DELETE SET NULL` (if a canonical ingredient is ever removed, the line keeps its raw text and goes back to needing review). |

Columns: `id BIGSERIAL PK`, `candidate_id BIGINT NOT NULL REF recipe_import_candidate(id) ON
DELETE CASCADE`, `line_no INT NOT NULL`, `raw_text TEXT NOT NULL`, `ingredient_id TEXT REF
ingredient(id) ON DELETE SET NULL`, `quantity DOUBLE PRECISION`, `unit TEXT`, `needs_review
BOOLEAN NOT NULL DEFAULT true`, `UNIQUE (candidate_id, line_no)`.

### Invariants (enforced in code, tested in 6.3)

1. **No import path writes cookbook content.** The only writer of `RecipeFamily`/`RecipeVariant`/
   `RecipeRevision` from an import is the explicit promotion action.
2. **A candidate's parsed content is not mutated in place** after import; corrections happen
   post-promotion as new revisions (that change's model).
3. **An unresolved ingredient is never dropped** — it is retained with `needs_review = true`.
4. **`promoted_variant_id` is set only when `status = 'promoted'`**, and never before.
