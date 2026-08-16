## 1. External recipe sources evaluation

- [ ] 1.1 Evaluate TheMealDB: license, cost, rate limits, commercial rights, Swedish content,
      ingredient structure, images, quality, API stability.
- [ ] 1.2 Evaluate Edamam: license, cost, rate limits, commercial rights, Swedish content,
      ingredient structure, images, quality, API stability.
- [ ] 1.3 Evaluate Spoonacular: license, cost, rate limits, commercial rights, Swedish content,
      ingredient structure, images, quality, API stability.
- [ ] 1.4 Evaluate Foodie: license, cost, rate limits, commercial rights, Swedish content,
      ingredient structure, images, quality, API stability.
- [ ] 1.5 Identify and evaluate Swedish recipe sources (e.g. ICA.se, Köket.se, Arla, Coop
      Recept) for the same criteria, noting which expose `schema.org/Recipe` JSON-LD (feeding
      into Section 2) vs. requiring a dedicated per-site parser.
- [ ] 1.6 Record a MEALIE / GROCY / MERGE / REDESIGN / DEFER / OMIT-style decision per source
      (per PLAN.md's Feature Overlap Matrix convention) — no "take the best parts" hand-waving.
- [ ] 1.7 Produce `docs/research/recipe-data-sources.md` capturing the above.

## 2. Generic web recipe import pipeline

- [ ] 2.1 Design the pipeline stages per PLAN.md: fetch URL → find Recipe JSON-LD → parse
      structured recipe → parse ingredient strings → canonicalize ingredients → review
      unresolved mappings → import.
- [ ] 2.2 Decide how "canonicalize ingredients" hooks into `establish-household-and-catalog`'s
      `Ingredient` vocabulary and `Product`/mapping review flow — reuse the existing
      needs-review pattern from `ingredient_mapping` rather than inventing a parallel one.
- [ ] 2.3 Add per-site parsers only where necessary (a site lacking usable JSON-LD or with
      structured data that doesn't map cleanly) — document the fallback trigger condition.
- [ ] 2.4 Produce `docs/research/recipe-web-import.md` documenting the pipeline design and any
      sites tested against it.

## 3. Inspiration site investigation

- [ ] 3.1 Investigate `https://vadfanskajaglagatillmiddag.nu/` via network inspection and
      source analysis (not assuming scraping is appropriate).
- [ ] 3.2 Determine the candidate recipe list mechanism and selection mechanism (how "surprise
      me" picks a recipe).
- [ ] 3.3 Determine whether randomness is server-side or client-side.
- [ ] 3.4 Identify source sites the inspiration site draws from, and any categories exposed.
- [ ] 3.5 Determine whether a hidden API exists and, if so, its shape.
- [ ] 3.6 Research legal/terms constraints before concluding any integration is viable —
      explicitly do not assume scraping is appropriate.
- [ ] 3.7 Study the inspiration UX (its extremely simple "surprise me" interaction) even if the
      underlying data mechanism cannot legally/technically be integrated, and record UX lessons
      for Spisordning's own discovery surface.
- [ ] 3.8 Produce `docs/research/inspiration-sites.md` capturing findings and a clear
      integrate/defer/omit recommendation with rationale.

## 4. Automatic cookbook growth lifecycle

- [ ] 4.1 Design the provenance/review lifecycle: external recipe → planned/cooked → save local
      version → review → household cookbook, per PLAN.md — confirm this is not "auto-import
      everything without review."
- [ ] 4.2 Decide what "save local version" means against
      `implement-recipe-family-and-revisions`'s model: does an imported external recipe become
      a new `RecipeFamily`+`RecipeVariant`+`RecipeRevision`, or a candidate staged separately
      until reviewed?
- [ ] 4.3 Define provenance fields recorded on any imported recipe (source URL, source name,
      license note, external id, imported-at) so attribution and later re-sync/removal remain
      possible.
- [ ] 4.4 Decide the trigger for promotion to "household cookbook": explicit review action
      after being planned/cooked at least once, per PLAN.md — not on import alone.

## 5. Persistence & scaffolding

- [ ] 5.1 For any new tables (e.g. `external_recipe_source`, `recipe_import_candidate`),
      answer PLAN.md's Database Review Questions: domain concept, owner, mutator, mutability,
      history requirement, lifecycle, deletion behavior, uniqueness constraints, external ids,
      indexing, FK-ability.
- [ ] 5.2 Scaffold `internal/recipeimport` (or equivalent) for the JSON-LD fetch/parse pipeline,
      with the actual external-API calls deferred until a source is approved for integration.
- [ ] 5.3 Write the additive migration for provenance/candidate tables.

## 6. Verification

- [ ] 6.1 `openspec validate implement-recipe-discovery` passes.
- [ ] 6.2 Unit tests for the JSON-LD parsing stage against a handful of real recipe pages'
      captured markup (fixture-based, no live network calls in tests).
- [ ] 6.3 Confirm the review-before-cookbook invariant is enforced: no code path adds an
      imported candidate directly to a household's `RecipeFamily`/`RecipeVariant` without an
      explicit review action.
