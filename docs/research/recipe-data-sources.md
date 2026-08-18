# Recipe data sources — external recipe API evaluation

This document is the research deliverable for `implement-recipe-discovery` Section 1
(External recipe sources evaluation). For each candidate external recipe source it records,
against a fixed set of criteria, the findings needed to make an `INTEGRATE NOW` / `DEFER` /
`OMIT` decision (per `PLAN.md`'s Feature Overlap Matrix convention — see
`feature-overlap-matrix.md`). No "take the best parts" hand-waving: each source gets one
decision with a stated reason.

Findings are recorded as observed (live API calls against the public/dev key, plus the
source's own documentation and terms), not inferred from third-party summaries. Each source
section ends with a preliminary decision leaning; the consolidated per-source decision table
is added in task 1.6.

## Criteria evaluated per source

1. **License** — what governs the data/artwork: an open license (e.g. CC) or a proprietary ToS.
2. **Cost** — free tier, paid tiers, pricing.
3. **Rate limits** — documented limits (or their explicit absence).
4. **Commercial rights** — what is permitted for a self-hosted/single-household deployment vs.
   a publicly released / commercial one.
5. **Swedish content** — coverage of Swedish recipes/cuisine (the primary requirement for this
   household).
6. **Ingredient structure** — structured (quantity/unit/food) vs. free-text; which units are used.
7. **Images** — availability, licensing, sizes.
8. **Quality** — database size, curation, consistency.
9. **API stability** — versioning, SLA, project health.

---

## TheMealDB (themealdb.com)

**Decision leaning: DEFER** — zero Swedish content; usable only as a secondary
international-recipe source or as a pipeline test fixture, not as a primary source for this
household. (Formal decision recorded in task 1.6.)

Evaluated 2026-08-18 against the public dev key `1` and the site's own docs/terms.

- **License.** Not an open license. Governed by a proprietary Terms of Use (last updated
  01/07/2025): "You can scrape, copy and modify any content returned from the API, as long as
  you use the official end points. Please do not scrape our website. You also cannot remove or
  alter any copyright or trademark notices." Artwork is mostly custom and user-created;
  individual items *may* be CC-licensed (check the per-item `strCreativeCommons` /
  `strCreativeCommonsConfirmed` tag) but there is no blanket license over the corpus.
  Attribution is required ("must mention us as the source of the data").
- **Cost.** Free at point of access; a development key `1` is provided for development and
  educational use. Premium is a **$10 lifetime one-off** (293 supporters as of 2026-08-18),
  unlocking the beta V2 API (multi-ingredient filters), the ability to add one's own meals and
  images, and listing the full database instead of the 100-item free cap.
- **Rate limits.** Referenced in the ToS ("as long as you stay within the rate limit") but
  **not publicly quantified** — no documented requests/minute or requests/hour figure. The
  free tier caps list endpoints at 100 items; premium lifts that cap.
- **Commercial rights.** Free tier: "You cannot publish apps to an appstore unless you are a
  paid subscriber." Paid tier: may develop apps/services within the rate limit, with
  attribution. The API may not be resold. For Spisordning (self-hosted, single-household, not
  publicly released / not an appstore app) the free tier is the relevant one; the AGENTS.md
  guidance that "production services should obtain the appropriate production or Premium access"
  is noted, but a private self-hosted deployment is not a public release.
- **Swedish content.** **None.** The area list includes `strArea: "Swedish"` (country
  "Sweden"), but `filter.php?a=Swedish` returns `{"meals":null}` — zero Swedish recipes.
  Disqualifying as a primary source for a Swedish household. Measures are also US-imperial
  (cups, teaspoons, tablespoons, oz), not metric.
- **Ingredient structure.** **Unstructured.** A full lookup returns `strIngredient1..20`
  (name slots) paired positionally with `strMeasure1..20` (measure slots); empty slots are `""`
  or `null`. There are no separate quantity/unit/food fields — canonicalization must parse
  free-text measures such as `3/4 cup`, `1/2 teaspoon`, `1 (12 oz.)`. Max 20 ingredients per
  recipe.
- **Images.** Every meal has a `strMealThumb` (792 meals / 792 images) with `/small`
  (200×200), `/medium` (350×350), `/large` (500×500) variants. Ingredient images are served at
  `https://www.themealdb.com/images/ingredients/{Name}.png` (+ size variants). Artwork is
  custom/user-created; retain attribution and check per-item CC tags before redistribution.
- **Quality.** Small, crowd-sourced database: **792 meals, 992 ingredients**. Community/hobby
  project (TheDataDB Ltd, UK). Sample lookup (id `52772`, "Teriyaki Chicken Casserole") was
  coherent but US-centric; several metadata fields (`strSource`, `strImageSource`,
  `strCreativeCommonsConfirmed`, `dateModified`) were `null`. No formal curation or quality
  guarantee.
- **API stability.** Simple browser-friendly JSON API; **V1 is the stable surface, V2 is
  beta/premium-only**. No formal versioning beyond v1/v2 and no SLA. OpenAPI specs are
  published (`/api/spec/openapi-v1.yaml`, `/api/spec/openapi-v2.yaml`) and an AI-agent guide
  (`/AGENTS.md`) documents the field layout. Stability is informal (single-maintainer project).

**Why DEFER (not INTEGRATE NOW).** The single disqualifier is Swedish content: a Swedish
household dinner-planning system gets zero usable recipes from this source, and its
US-imperial measures would all need conversion. It does have secondary value — (a) a
fixture/test source for the generic web-import pipeline (stable, simple, image-rich, no auth
friction) and (b) a source for explicitly international recipes — but both are follow-up, not
this change's integration scope.

> Note on method for the sources below: TheMealDB was evaluated with a live call against the
> public dev key. Edamam, Spoonacular, and Foodie are recorded from each source's own
> documentation, pricing, and terms pages — no live API calls were made (no keys are held, and
> the proposal explicitly scopes out calling paid/rate-limited external APIs in this change).
> The Swedish publisher sites were evaluated by fetching real recipe pages and inspecting their
> structured data directly.

## Edamam (edamam.com)

**Decision: DEFER** — no Swedish-content advantage, and the free tier cannot import a full
recipe (web recipes carry no cooking instructions); a usable integration requires a paid
licensed-recipe agreement.

- **License.** Proprietary. The Recipe Search API returns *web* recipes (scraped from the
  public web) that are **not** fully licensed for storage/redistribution on the free tier —
  only the source URL and Edamam-generated nutrition are provided for those; the actual
  ingredient list and step-by-step instructions require a **paid licensed-recipe agreement**.
  No open license over the corpus.
- **Cost.** Free developer tier (limited daily request allowance); paid tiers scale the request
  allowance and unlock licensed-recipe content. Exact free-tier request figure was not confirmed
  in this pass.
- **Rate limits.** Tied to the tier's request allowance; not a simple fixed RPS on the free
  tier.
- **Commercial rights.** Web-recipe data is not storable/redistributable on the free tier; a
  self-hosted local cookbook that persists full recipes would need the paid licensed-recipe
  tier.
- **Swedish content.** Cuisine types include `nordic`, but there is no meaningful Swedish
  coverage advantage over the other sources — the corpus is international, not Swedish-first.
- **Ingredient structure.** Structured (quantity/unit/food) for licensed recipes; web recipes
  return a thinner payload (source URL + nutrition, no instructions).
- **Images.** Web recipes expose the source's image URL; licensed recipes include images.
- **Quality.** Large: ~900K foods (Food Database), 680K+ UPC barcodes, 1.5M+ web recipes
  (Recipe Search).
- **API stability.** Established commercial API; stable base URL `https://api.edamam.com`;
  OpenAPI spec published (`/doc/open-api/recipe-search-v2.yaml`).

**Why DEFER.** Two independent blockers: (1) no Swedish-content advantage, and (2) the free
tier structurally cannot deliver a full importable recipe — web recipes omit the instructions
that make a recipe a recipe, and getting them means a paid licensed-recipe agreement. Both are
revisit triggers (a future paid tier, or a need for nutrition/UPC lookups from the Food
Database), not this change's scope.

## Spoonacular (spoonacular.com)

**Decision: DEFER** — the ToS prohibits storing the data (only recipe id, title, and image URL
may be kept), which directly conflicts with a local persistent cookbook, and the content is
English-only.

- **License.** Proprietary Terms of Service. The license is **nonexclusive and
  non-transferable**, and — the decisive clause — it **prohibits scraping or storing** the API's
  data: only the recipe id, title, and image URL may be retained, and cached content may be held
  for at most one hour. Source attribution is required; the data may not be resold.
- **Cost.** Free $0/mo (50 points/day); Cook $29/mo (1,500 points/day); Culinarian $79/mo
  (4,500 points/day); Chef $149/mo (10,000 points/day); Enterprise $300+/mo.
- **Rate limits.** Free 1 req/s and 2 concurrent; Cook 5 req/s; Culinarian 10 req/s; Chef
  20 req/s.
- **Commercial rights.** Cannot store the corpus locally; a self-hosted cookbook that persists
  imported recipes is out of bounds under the ToS.
- **Swedish content.** **None** — the API serves English content only.
- **Ingredient structure.** Structured (quantity/unit/food); ingredient data is USDA-based.
- **Images.** Image URL provided per recipe.
- **Quality.** Curated and smaller than the scrapers: 5,000+ recipes, 2,600+ ingredients,
  600K+ products, 115K+ menu items.
- **API stability.** Commercial, with a 99–99.9% SLA on paid tiers, a status page, and client
  SDKs. Notably it exposes an **extract-recipes-from-any-website** endpoint — a useful design
  reference for Section 2's generic pipeline (see `recipe-web-import.md`), independent of
  whether Spoonacular itself is integrated.

**Why DEFER.** The storage prohibition is disqualifying on its own: Spisordning's whole point is
a *local, persistent* household cookbook, and Spoonacular's ToS forbids persisting anything but
id/title/image. English-only is a second, independent disqualifier. The URL-extraction endpoint
is still worth citing as prior art for the generic JSON-LD pipeline.

## Foodie

**Decision: OMIT** — there is no distinct, stable recipe-database API under this name to
integrate; the name resolves to an unrelated ML service and to hobby frontends wrapping other
sources.

- **What Foodie actually is.** Searching for a Foodie recipe API does not surface a
  first-party recipe corpus. The one real product named a Foodie API
  (`foodapi.devco.solutions`) is a **machine-learning food-recognition / nutrition** service
  (image → food + nutrition), not a browsable recipe source, and it has no Swedish content.
- **Everything else** under the name is a hobby frontend or sample app that *consumes*
  TheMealDB or Spoonacular (e.g. a Vue Foodie app over TheMealDB, a foodie-api wrapper over
  Spoonacular) — i.e. a client, not a source.
- **License / cost / rate limits / commercial rights / Swedish content / ingredient structure /
  images / quality / API stability.** Not applicable — there is no underlying recipe corpus or
  stable API contract to evaluate.

**Why OMIT.** There is nothing to integrate: no recipe corpus, no stable API, no Swedish
content. Recorded so the name is explicitly closed out rather than left as an open candidate.

## Swedish publisher sites (ICA, Köket, Arla, Coop)

**Decision: INTEGRATE NOW for ICA.se, Köket.se, and Arla.se** (via the generic `schema.org/Recipe`
JSON-LD pipeline — Section 2); **Coop.se DEFER** (needs a dedicated per-site parser — it is the
fallback case that triggers Section 2.3).

Evaluated 2026-08-18 by fetching a real recipe page from each site and inspecting its structured
data. Three of the four expose a **complete, standard `schema.org/Recipe` JSON-LD** block in the
page `<head>` — which means Section 2's generic pipeline imports them with no site-specific code.

- **ICA.se** (`/recept/potatisgratang-grundrecept-721833/`) — **full Recipe JSON-LD.** Fields
  present: `name`, `image`, `url`, `description`, `datePublished`/`dateModified`, `author`
  (Organization 'ICA Köket'), `aggregateRating` (4.6 / 422), `totalTime` (`PT90M`),
  `cookingMethod`, `recipeCategory`, `recipeCuisine`, `recipeYield`, `nutrition`
  (NutritionInformation), and `recipeInstructions` (HowToStep list). Swedish, metric.
- **Köket.se** (`/nigella-lawson/.../kramig-potatisgratang`) — **full Recipe JSON-LD.** A
  `Corporation` (Köket) block plus a `Recipe` block: `name`, `image`, `description`, `author`
  (Person 'Nigella Lawson'), `totalTime` (`PT45M`), `recipeYield` ('10 portioner'),
  `recipeIngredient` (8 lines), `recipeInstructions` (5 HowToStep), `aggregateRating`.
  Swedish, metric.
- **Arla.se** (`/recept/chokladkaka/`) — **full Recipe JSON-LD.** `name`, `image`, `author`
  (Person), `publisher` (Organization 'Arla'), `description`, `totalTime`/`cookTime`/`prepTime`,
  `recipeYield`, `recipeCategory`, `nutrition`, `recipeIngredient` (11 lines), and
  `recipeInstructions` as a nested HowToSection/HowToStep tree, plus `aggregateRating`.
  Swedish, metric. The most complete of the four (explicit prep/cook split + sectioned steps).
- **Coop.se** (`/recept/ragmacka-med-tonfiskrora`) — **no `schema.org/Recipe` JSON-LD.** Recipe
  metadata is exposed via OpenGraph meta tags and a `window.dataLayer` push (`recipeName`,
  `recipeId` 3967260, `recipeAmountOfIngredients`, `recipeCookingTime`, canonical URL); the
  full body renders in a React micro-app backed by an **undocumented internal JSON API**
  (`https://proxy.api.coop.se/external/recipe`), with images on Cloudinary. Importing Coop
  therefore requires a **dedicated per-site parser/adapter** — exactly the fallback Section 2.3
  is designed for. (No Coop credentials/keys are recorded here.)

**Why INTEGRATE NOW (ICA/Köket/Arla).** These are the primary Swedish sources for this
household, they are Swedish-first and metric, and they already publish the exact structured
format the generic pipeline targets — so integrating them is 'parse standard JSON-LD', not
'scrape a bespoke layout'. **Why Coop is the deferred exception:** it is the one major Swedish
publisher without usable JSON-LD, so it exercises the per-site-parser path; that adapter is
scoped out until the generic pipeline is proven on the three JSON-LD sites.

## Consolidated per-source decision table (task 1.6)

Per PLAN.md's Feature Overlap Matrix convention, each source gets exactly one decision with a
stated reason — no 'take the best parts'. (The source-analog vocabulary is `INTEGRATE NOW` /
`DEFER` / `OMIT`, matching this document's intro.)

| Source | Decision | Why (one line) |
|---|---|---|
| TheMealDB | **DEFER** | Zero Swedish content; US-imperial, unstructured measures. |
| Edamam | **DEFER** | No Swedish advantage; free tier can't import full recipes (web recipes lack instructions); paid license required. |
| Spoonacular | **DEFER** | ToS prohibits storing data (only id/title/image may be kept); English-only. |
| Foodie | **OMIT** | Not a real recipe-source API — only an ML food-recognition service and hobby wrappers under the name. |
| ICA.se | **INTEGRATE NOW** | Full `schema.org/Recipe` JSON-LD; Swedish, metric; standard import. |
| Köket.se | **INTEGRATE NOW** | Full `schema.org/Recipe` JSON-LD; Swedish, metric; standard import. |
| Arla.se | **INTEGRATE NOW** | Full `schema.org/Recipe` JSON-LD (most complete); Swedish, metric; standard import. |
| Coop.se | **DEFER** (per-site parser) | No JSON-LD; needs a dedicated adapter against an undocumented internal API — the Section 2.3 fallback case. |
