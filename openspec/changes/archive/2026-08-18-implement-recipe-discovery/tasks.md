## 1. External recipe sources evaluation

- [x] 1.1 Evaluate TheMealDB: license, cost, rate limits, commercial rights, Swedish content,
      ingredient structure, images, quality, API stability. — Recorded in
      `docs/research/recipe-data-sources.md` (TheMealDB section, verified 2026-08-18 against
      dev key `1` + site docs/terms): proprietary ToS (not an open license; attribution
      required), free dev key `1` + $10 lifetime premium, rate limit referenced but not
      quantified, appstore release requires paid tier, **zero Swedish recipes**
      (`filter.php?a=Swedish` → `{"meals":null}`), unstructured `strIngredientN`/`strMeasureN`
      pairs in US-imperial units, per-meal + per-ingredient images, 792 meals / 992
      ingredients (crowd-sourced), V1 stable / V2 beta. Leaning: **DEFER** (no Swedish content).
- [x] 1.2 Evaluate Edamam: license, cost, rate limits, commercial rights, Swedish content,
      ingredient structure, images, quality, API stability. — Recorded in
      `docs/research/recipe-data-sources.md` (Edamam section, from source docs/pricing/terms,
      no live call): proprietary; web recipes not storable on free tier (no instructions — a
      paid licensed-recipe agreement is required); free dev tier (exact limit unconfirmed);
      international, not Swedish-first (only a `nordic` cuisine tag); structured ingredients
      for licensed recipes; ~900K foods / 1.5M+ web recipes; stable `api.edamam.com` + OpenAPI.
      Decision: **DEFER**.
- [x] 1.3 Evaluate Spoonacular: license, cost, rate limits, commercial rights, Swedish content,
      ingredient structure, images, quality, API stability. — Recorded in
      `docs/research/recipe-data-sources.md` (Spoonacular section, from source docs/pricing/
      terms, no live call): proprietary ToS that **prohibits storing** data (only id/title/
      image URL may be kept, 1h cache max); Free $0 (50 pts/day, 1 req/s) up to Chef $149
      (10k pts/day); English-only; structured USDA-based ingredients; 5k+ recipes; 99–99.9%
      SLA on paid; has a URL-extraction endpoint (cited as pipeline prior art). Decision:
      **DEFER**.
- [x] 1.4 Evaluate Foodie: license, cost, rate limits, commercial rights, Swedish content,
      ingredient structure, images, quality, API stability. — Recorded in
      `docs/research/recipe-data-sources.md` (Foodie section): no distinct recipe-database API
      under this name — the one real Foodie API (`foodapi.devco.solutions`) is an ML
      food-recognition/nutrition service (no Swedish content); the rest are hobby frontends
      wrapping TheMealDB/Spoonacular. Nothing to integrate. Decision: **OMIT**.
- [x] 1.5 Identify and evaluate Swedish recipe sources (e.g. ICA.se, Köket.se, Arla, Coop
      Recept) for the same criteria, noting which expose `schema.org/Recipe` JSON-LD (feeding
      into Section 2) vs. requiring a dedicated per-site parser. — Recorded in
      `docs/research/recipe-data-sources.md` (Swedish publisher sites section, from live page
      fetches 2026-08-18): **ICA.se, Köket.se, and Arla.se all expose full `schema.org/Recipe`
      JSON-LD** (standard import via Section 2); **Coop.se has none** (OG + `dataLayer` +
      undocumented internal API → dedicated per-site parser, the Section 2.3 fallback).
- [x] 1.6 Record a MEALIE / GROCY / MERGE / REDESIGN / DEFER / OMIT-style decision per source
      (per PLAN.md's Feature Overlap Matrix convention) — no 'take the best parts' hand-waving.
      — Consolidated one-decision-per-source table recorded in
      `docs/research/recipe-data-sources.md` (task 1.6 table): TheMealDB DEFER, Edamam DEFER,
      Spoonacular DEFER, Foodie OMIT, ICA/Köket/Arla INTEGRATE NOW, Coop DEFER (per-site parser).
- [x] 1.7 Produce `docs/research/recipe-data-sources.md` capturing the above. — Done: the file
      now covers all six sources (TheMealDB, Edamam, Spoonacular, Foodie, and the four Swedish
      publisher sites) against the fixed 9 criteria, each with a single decision + reason, plus
      the consolidated decision table.

## 2. Generic web recipe import pipeline

- [x] 2.1 Design the pipeline stages per PLAN.md: fetch URL → find Recipe JSON-LD → parse
      structured recipe → parse ingredient strings → canonicalize ingredients → review
      unresolved mappings → import. — All seven stages designed with a stage diagram, the
      schema.org→ParsedRecipe field map, and the ingredient-split rules in
      `docs/research/recipe-web-import.md` (2.1).
- [x] 2.2 Decide how 'canonicalize ingredients' hooks into `establish-household-and-catalog`'s
      `Ingredient` vocabulary and `Product`/mapping review flow — reuse the existing
      needs-review pattern from `ingredient_mapping` rather than inventing a parallel one. —
      Decided (recipe-web-import.md, 2.2): an imported line reuses the
      `ingredient_mapping.needs_review` pattern verbatim — a source identifier → canonical
      `ingredient.id` + `needs_review BOOLEAN NOT NULL DEFAULT true`; resolved clears the flag,
      unresolved keeps the raw line with the flag set. No parallel mechanism; resolved lines flow
      into the existing `Product`/shopping-requirement path.
- [x] 2.3 Add per-site parsers only where necessary (a site lacking usable JSON-LD or with
      structured data that doesn't map cleanly) — document the fallback trigger condition. —
      Fallback trigger documented (recipe-web-import.md, 2.3): no `@type:Recipe` node, or a node
      missing `recipeIngredient`/`recipeInstructions`. A per-site adapter emits the same
      `ParsedRecipe` so downstream stages stay source-agnostic. Coop.se is the concrete deferred
      case that trips the trigger.
- [x] 2.4 Produce `docs/research/recipe-web-import.md` documenting the pipeline design and any
      sites tested against it. — Done: full pipeline design + tested-sites table (ICA/Köket/Arla
      via the generic path, each exercising a distinct JSON-LD branch; Coop via the fallback
      trigger). The three captured JSON-LD blocks become the task 6.2 fixtures.

## 3. Inspiration site investigation

- [x] 3.1 Investigate `https://vadfanskajaglagatillmiddag.nu/` via network inspection and
      source analysis (not assuming scraping is appropriate). — Done: fetched home,
      `/vegetariskt`, and `/om` and read the raw server HTML directly (see
      `docs/research/inspiration-sites.md`).
- [x] 3.2 Determine the candidate recipe list mechanism and selection mechanism (how 'surprise
      me' picks a recipe). — Server-rendered: a curated server-side list of `(title,
      source-URL)` pairs; one is picked at request time and baked into the returned HTML
      (`docs/research/inspiration-sites.md`, 3.1/3.2).
- [x] 3.3 Determine whether randomness is server-side or client-side. — **Server-side**: two
      fetches returned different recipes (coop.se then ica.se) and the HTML has zero `<script>`
      tags, so no client-side randomness exists.
- [x] 3.4 Identify source sites the inspiration site draws from, and any categories exposed. —
      Links out to `ica.se` and `coop.se`; two categories: `/` ('VANLIG JÄVLA MAT') and
      `/vegetariskt` ('VEGETARISKT?').
- [x] 3.5 Determine whether a hidden API exists and, if so, its shape. — **None**: no `<script>`
      tags, no JSON endpoint; the 'roll again' control is a plain `href=""` reload that
      re-triggers the server-side pick.
- [x] 3.6 Research legal/terms constraints before concluding any integration is viable —
      explicitly do not assume scraping is appropriate. — No `robots.txt` (404), no ToS found;
      the site is a thin wrapper whose content is a curated link list to `ica.se`/`coop.se`
      (copyright belongs to those publishers). Absence of robots/ToS is not permission to
      scrape; data integration concluded not appropriate.
- [x] 3.7 Study the inspiration UX (its extremely simple 'surprise me' interaction) even if the
      underlying data mechanism cannot legally/technically be integrated, and record UX lessons
      for Spisordning's own discovery surface. — UX lessons recorded in
      `docs/research/inspiration-sites.md` (3.7): one tap → one recipe, trivial re-roll, at
      most one category axis; maps to a 'surprise me' affordance on Spisordning's own
      recommendation surface (a separate capability).
- [x] 3.8 Produce `docs/research/inspiration-sites.md` capturing findings and a clear
      integrate/defer/omit recommendation with rationale. — Done: findings for 3.1–3.7 plus a
      clear recommendation — **OMIT the data mechanism, ADOPT the UX lesson** — with rationale.

## 4. Automatic cookbook growth lifecycle

- [x] 4.1 Design the provenance/review lifecycle: external recipe → planned/cooked → save local
      version → review → household cookbook, per PLAN.md — confirm this is not 'auto-import
      everything without review.' — Lifecycle designed in `design.md` (4.1) with a diagram:
      import → candidate (not in cookbook) → [planned/cooked + explicit review] → promoted to
      Family/Variant/Revision. Import and promotion are two separate gated steps; the gate is a
      human action, not a batch op. Confirmed NOT auto-import.
- [x] 4.2 Decide what 'save local version' means against
      `implement-recipe-family-and-revisions`'s model: does an imported external recipe become
      a new `RecipeFamily`+`RecipeVariant`+`RecipeRevision`, or a candidate staged separately
      until reviewed? — Decided (design.md, 4.2): staged separately as a candidate until
      reviewed; on the explicit review action it is materialized into a NEW
      Family+Variant+first Revision (imported content = immutable first revision). The durable
      membership fact is candidate.status='promoted' + the pointed-to variant (a real row, not a
      saved filter).
- [x] 4.3 Define provenance fields recorded on any imported recipe (source URL, source name,
      license note, external id, imported-at) so attribution and later re-sync/removal remain
      possible. — Defined (design.md, 4.3): source_url, source_id→source name, external_id,
      license_note, imported_at, plus raw_jsonld retained for re-sync/audit; carried into the
      promoted variant's sourceAttribution.
- [x] 4.4 Decide the trigger for promotion to 'household cookbook': explicit review action
      after being planned/cooked at least once, per PLAN.md — not on import alone. — Decided
      (design.md, 4.4): hard gate = explicit review action (the only path to cookbook content);
      planned/cooked-at-least-once tracked as first_served_at and surfaced as a nudge (the
      'normally' signal), not a block; import alone never promotes.

## 5. Persistence & scaffolding

- [x] 5.1 For any new tables (e.g. `external_recipe_source`, `recipe_import_candidate`),
      answer PLAN.md's Database Review Questions: domain concept, owner, mutator, mutability,
      history requirement, lifecycle, deletion behavior, uniqueness constraints, external ids,
      indexing, FK-ability. — All three new tables (external_recipe_source,
      recipe_import_candidate, recipe_import_candidate_ingredient) answered against all 11
      review questions in `design.md` (5.1), plus the invariants enforced/tested in 6.3.
- [x] 5.2 Scaffold `internal/recipeimport` (or equivalent) for the JSON-LD fetch/parse pipeline,
      with the actual external-API calls deferred until a source is approved for integration. —
      Done: `internal/recipeimport/recipeimport.go` (476 lines, stdlib-only, no network) provides
      `ExtractRecipeJSONLD` (scans every JSON-LD script block, handles a single object / node
      array / `@graph`), `ParseRecipe` (schema.org→ParsedRecipe field map incl. ISO-8601
      durations, yield, nested HowToSection/HowToStep, aggregateRating, nutrition),
      `ParseIngredientLine` (conservative quantity/unit/food split, always `NeedsReview`),
      `CandidateFromParsed` (builds a Stage-7 candidate, not cookbook content), and `TrailingID`.
      `go build`/`go vet` clean.
- [x] 5.3 Write the additive migration for provenance/candidate tables. — Done:
      `migrations/0002_recipe_discovery.sql` (additive, `BEGIN`/`COMMIT`, matches
      0001 conventions) creates `external_recipe_source`, `recipe_import_candidate`,
      and `recipe_import_candidate_ingredient` per design.md 5.1. Verified by applying
      0001+0002 to a throwaway `postgres:16-alpine` (docker, no persistent volume): all
      three tables created, partial unique indexes `(source_id, external_id) WHERE
      external_id IS NOT NULL` and `(source_url) WHERE external_id IS NULL` present,
      `promoted_variant_id` left as a deferred TEXT (no FK to the not-yet-existing
      `recipe_variant`).

## 6. Verification

- [x] 6.1 `openspec validate implement-recipe-discovery` passes. — Passes:
      `openspec validate implement-recipe-discovery` → "Change 'implement-recipe-discovery' is
      valid"; `go build ./...` success; `go vet ./...` clean; `go test ./...` → 85 passed in 11
      packages.
- [x] 6.2 Unit tests for the JSON-LD parsing stage against a handful of real recipe pages'
      captured markup (fixture-based, no live network calls in tests). — Done:
      `internal/recipeimport/recipeimport_test.go` (287 lines) with three captured fixtures in
      `internal/recipeimport/testdata/` (ica.html, koket.html, arls.html — real JSON-LD extracted
      from live pages, no network in tests). `TestExtractAndParseFixtures` covers the three
      distinct JSON-LD branches (single node / [Corporation,Recipe] array / nested
      HowToSection+string-typed ratings), plus `TestExtractRecipeJSONLD_NoRecipe` (fallback
      trigger), `TestParseIngredientLine`, `TestParseDuration`, `TestParseYield`,
      `TestTrailingID`, and `TestCandidateFromParsed`. 44 tests, all pass.
- [x] 6.3 Confirm the review-before-cookbook invariant is enforced: no code path adds an
      imported candidate directly to a household's `RecipeFamily`/`RecipeVariant` without an
      explicit review action. — Confirmed two ways: (1) `TestImportProducesCandidateNotCookbookContent`
      asserts the package's only creation entry point (`CandidateFromParsed`) always returns
      `StatusCandidate` with an empty `PromotedVariantID`; (2) grep of `internal/recipeimport/`
      shows no `database/sql`/`pgx`/`INSERT`/`.Exec`/`.Query` and no `RecipeFamily`/`RecipeVariant`/
      `RecipeRevision` write target (only doc comments). The package is pure parsing; promotion is
      a separate explicit review action owned by the recipe-family-and-revisions change.
