## Why

Today the only recipe intake path is Mealie sync (`internal/mealie`), which assumes a recipe
already exists in Mealie. PLAN.md's Epic C "Automatic Cookbook Growth" loop
(external recipe → planned/cooked → save local version → review → household cookbook) needs an
actual intake mechanism from outside Mealie: named external recipe APIs
(TheMealDB/Edamam/Spoonacular/Foodie/Swedish sources), a generic `schema.org/Recipe` JSON-LD
web-import pipeline, and — because PLAN.md explicitly calls it out as worth studying even if
it can't be integrated — the Swedish inspiration site
`https://vadfanskajaglagatillmiddag.nu/`. None of these have been evaluated yet; this change is
the research-and-scaffold step, not a promise to integrate every source.

## What Changes

- Research and record, per external recipe source (TheMealDB, Edamam, Spoonacular, Foodie,
  Swedish sources), license, cost, rate limits, commercial rights, Swedish content coverage,
  ingredient structure, images, quality, and API stability — producing a decision (integrate
  now / defer / omit) per source, not blanket integration.
- Design a generic `schema.org/Recipe` JSON-LD web-import pipeline (fetch → find JSON-LD →
  parse structured recipe → parse ingredient strings → canonicalize against
  `establish-household-and-catalog`'s `Ingredient` vocabulary → review unresolved mappings →
  import), with per-site parsers added only where a site lacks usable JSON-LD.
- Investigate `https://vadfanskajaglagatillmiddag.nu/` via network inspection and source
  analysis: candidate recipe list mechanism, server vs. client-side randomness, source sites,
  possible hidden API, and legal/terms constraints — without assuming scraping is appropriate,
  and study its inspiration UX (a single "surprise me" affordance) even where the data
  mechanism can't be reused.
- Define the "external recipe → cookbook" provenance and review lifecycle: an externally
  sourced recipe is imported as a candidate, and only becomes part of the household cookbook
  (a `RecipeFamily`/`RecipeVariant`/`RecipeRevision`, per `implement-recipe-family-and-revisions`)
  after being planned/cooked and reviewed — not auto-imported wholesale.
- Record source provenance (source URL, license, import date, external id) on any imported
  recipe so attribution and later re-sync/removal remain possible.

## Capabilities

### New Capabilities

- `recipe-discovery`: external recipe source evaluation, the generic web-import pipeline,
  inspiration-site research findings, and the provenance/review model governing when an
  external recipe becomes a household cookbook entry.

### Modified Capabilities

<!-- none -->

## Impact

- Affected code: new `internal/recipeimport` (or similar) package for the JSON-LD pipeline;
  `docs/research/recipe-data-sources.md`, `docs/research/recipe-web-import.md`,
  `docs/research/inspiration-sites.md` (PLAN.md's "Expected Research Documents" for this Epic).
- Depends on `establish-household-and-catalog` (Ingredient canonicalization for parsed
  ingredient strings) and `implement-recipe-family-and-revisions` (the RecipeFamily/Variant/
  Revision structure a reviewed import ultimately lands in).
- No automated legal commitment: any source found to require a commercial license or whose
  terms prohibit storage/redistribution is recorded as OMIT/DEFER, not silently integrated.
- Out of scope: actually calling any paid/rate-limited external API in production; that is
  follow-up work once this change's research and pipeline design are reviewed.
