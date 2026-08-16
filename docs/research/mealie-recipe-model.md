# Mealie Investigation: Recipe Model (items 2.1–2.13)

Live reference instance: Mealie v3.9.2, `http://192.168.1.22:9000` (see
`mealie-deployment.md`). Investigated by creating a real household ("Family"), seeding the
default unit/food catalogs (`POST /api/groups/seeders/{units,foods}` — 23 units, 1,394 foods),
importing a real recipe by URL (`https://www.bbcgoodfood.com/recipes/easy-pancakes`, a
schema.org/JSON-LD site), running it through Mealie's NLP ingredient parser, and exercising
tags, categories, a cookbook, a meal plan entry, a favorite/rating, and a shopping list against
it. Database findings are from direct SQLite introspection
(`ssh proxmox "pct exec 2319 -- ...sqlite3..."`, via the container's own Python
`sqlite3` module — no `sqlite3` CLI binary is installed in the image). Source references are
pinned to tag `v3.9.2` on `github.com/mealie-recipes/mealie`.

---

## 2.1 Recipe model

**User behavior**: A recipe is a single mutable document: name, description, times, servings,
a yield string, ingredients, instructions, nutrition, settings, images, assets, notes, tags,
categories, tools, and a rating. There is exactly one row per recipe — editing it in place
changes that row. There is no "draft vs. published" state and no version history beyond an
optional, manually-written timeline event log (2.2).

**API behavior**: `GET/PUT/PATCH/DELETE /api/recipes/{slug}` (slug or id both accepted for
`GET`). `Recipe` (~30 top-level fields) nests `recipeIngredient[]`, `recipeInstructions[]`,
`nutrition`, `settings`, `assets[]`, `notes[]`, `tags[]`, `recipeCategory[]`, `tools[]`,
`comments[]`. Response envelope for list endpoints is
`{page, per_page, total, total_pages, items, next, previous}`.

**DB mutation**: One row in `recipes`, fanning out via FK to `recipes_ingredients`,
`recipe_instructions`, `recipe_nutrition` (1:1), `recipe_settings` (1:1), `recipe_assets`,
`notes`, and join tables `recipes_to_tags`/`recipes_to_categories`/`recipes_to_tools`.
`recipes.rating` exists but — see 2.19 — is a legacy fallback column, not the number actually
shown to a user.

```sql
CREATE TABLE "recipes" (
    created_at DATETIME, update_at DATETIME, id CHAR(32) NOT NULL,
    slug VARCHAR, group_id CHAR(32) NOT NULL, user_id CHAR(32), name VARCHAR NOT NULL,
    description VARCHAR, image VARCHAR, total_time VARCHAR, prep_time VARCHAR,
    perform_time VARCHAR, cook_time VARCHAR, recipe_yield VARCHAR, "recipeCuisine" VARCHAR,
    rating FLOAT, org_url VARCHAR, date_added DATE, date_updated DATETIME,
    is_ocr_recipe BOOLEAN, last_made DATETIME, name_normalized VARCHAR NOT NULL,
    description_normalized VARCHAR, recipe_yield_quantity FLOAT DEFAULT '0' NOT NULL,
    recipe_servings FLOAT DEFAULT '0' NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT recipe_slug_group_id_key UNIQUE (slug, group_id),
    FOREIGN KEY(group_id) REFERENCES groups (id), FOREIGN KEY(user_id) REFERENCES users (id)
);
```

Note `recipes` is scoped to `group_id` (the tenant), **not** `household_id` — the slug
uniqueness constraint is `(slug, group_id)`. A recipe belongs to a household only loosely,
through `households_to_recipes` (last-made tracking) and the creating user's `household_id`;
recipes themselves are shared catalog data across every household inside one Mealie "group".

**Source**: `mealie/schema/recipe/recipe.py` (Pydantic schema, `create_recipe_slug()` uses
`python-slugify`, truncated at 250 chars); `mealie/db/models/recipe/recipe.py` (SQLAlchemy
model, `auto_init` pattern that builds nested children from raw dicts on `__init__`/`update`).

**Tests**: `tests/integration_tests/user_recipe_tests/` (full CRUD round-trips),
`tests/unit_tests/schema_tests/` (Pydantic validation), `tests/integration_tests/test_openapi.py`
(schema stays in sync with the generated spec).

**Strengths**: A genuinely normalized schema — ingredients, instructions, nutrition and
settings are real child rows with real FKs, not one JSON blob (contrast with the OpenAPI
`Recipe` object, which *looks* like one big nested document but is reconstructed from several
tables on every read). Slugs are stable, human-readable, and unique per tenant.

**Weaknesses**: `recipes.rating` is dead weight in the schema — a leftover from before
per-user ratings existed (2.19) that the query layer now overrides at read time rather than
being dropped. No `version`/`updated_by` column beyond `update_at`/`user_id` (the *creator*,
not necessarily the last editor).

**Spisordning lesson**: The recipe-as-one-mutable-row model is exactly what PLAN.md's
`RecipeFamily → RecipeVariant → RecipeRevision` hierarchy is designed to replace. Confirmed
directly: Mealie has **zero** revision/version tables anywhere in its 62-table schema. Editing
overwrites in place; `recipe_timeline_events` is an optional, user-authored annotation log
(2.2), not an automatic diff/undo mechanism. This is strong evidence *for*
`implement-recipe-family-and-revisions`'s premise — Mealie is not a counterexample to worry
about, it is the gap being filled. Also worth copying: real FKs for every child relation
(ingredients, instructions, nutrition) rather than a JSON blob — matches PLAN.md's "Do Not Use
Generic Polymorphism Carelessly" instinct.

---

## 2.2 Recipe editing

**User behavior**: Full edit (`PUT`) or partial edit (`PATCH`) of a recipe by slug.

**API behavior — a real, reproduced bug**: `PUT /api/recipes/{slug}` with the complete
`Recipe` object (as returned by `GET`) works correctly, including structured
`quantity`/`unit`/`food` on ingredients. **`PATCH` does not.** Sending
`{"slug": "...", "recipeIngredient": [...]}` with fully-populated `quantity`/`unit`/`food`
objects returns `200 OK`, but the persisted ingredients come back with `quantity: 0`,
`unit: null`, `food: null` — only `note`/`display`/`referenceId` survive. Verified against the
raw DB (`recipes_ingredients.unit_id`/`food_id`/`quantity` all null/zero after the PATCH,
non-null after an equivalent `PUT`). Root cause not fully traced, but the divergence is
reproducible and specific to nested relation fields on `PATCH`; scalar fields (`tags`,
`recipeCategory`, `recipeServings`) patch correctly.

**A worse bug — non-atomic write vs. response validation**: On a first attempt, `PATCH` was
sent with ingredient dicts missing `referenceId` (a required UUID field). The request returned
`500 Internal Server Error` with a Pydantic `ValidationError` for `reference_id` — but the
underlying SQL write had already been committed with `reference_id = NULL` on six rows.
Confirmed via direct DB read (`recipes_ingredients.reference_id` was `NULL` for the whole
recipe). Worse: because the response-serialization step (`self.schema.model_validate(entry)`
in `mealie/repos/repository_generic.py`) now *also* fails on every subsequent read, the recipe
became **completely inaccessible through the API** — `GET`, and even `DELETE`, both returned
`500` (`DELETE` fetches-then-serializes the deleted row, hitting the same validation error).
Recovery required a direct SQL `UPDATE` via the container's Python `sqlite3` module to backfill
valid UUIDs — there was no API-level recovery path. Traceback (from `/app/data/mealie.log`):

```text
File ".../mealie/repos/repository_generic.py", line 226, in update
    return self.schema.model_validate(entry)
pydantic_core._pydantic_core.ValidationError: 6 validation errors for Recipe
recipe_ingredient.0.reference_id
  UUID input should be a string, bytes or UUID object [type=uuid_type, input_value=None, ...]
```

**DB mutation**: `UPDATE recipes SET ...`, full replace of `recipes_ingredients` rows (deleted
and reinserted, not diffed — evidenced by DB row `id`s changing on every `PUT` even for
unchanged ingredients), full replace of `recipe_instructions`, join-table rows for
tags/categories rewritten wholesale.

**Source**: `mealie/routes/recipe/recipe_crud_routes.py` (`patch_one`/`update_one`),
`mealie/services/recipe/recipe_service.py`, `mealie/repos/repository_generic.py` (`update`),
`mealie/db/models/recipe/recipe.py` (`__init__` rebuilds `recipe_ingredient` from raw dicts
every time — this is where the `PATCH` merge silently drops nested fields, since a `PATCH`'s
`exclude_unset=True` payload is passed through the same "delete all children, reinsert from
dicts" path built for `PUT`, not a true partial merge of ingredient sub-fields).

**Tests**: `tests/integration_tests/user_recipe_tests/` exercises `PUT` round-trips
extensively; no test found that exercises `PATCH` with nested `recipeIngredient` containing
populated `unit`/`food` objects, nor one that intentionally sends a malformed nested payload to
`PATCH` and asserts the write is rolled back. This gap matches exactly the bug found live.

**Strengths**: Full-replace (`PUT`) editing is simple, predictable, and correctly round-trips
every structured field including nested food/unit creation-by-name.

**Weaknesses**: `PATCH`'s "partial update" contract is broken for nested relations — a
significant trap for any client that assumes REST `PATCH` semantics (merge changed fields
only). The non-atomic write-then-validate-response ordering means a schema/validation failure
*after* the DB write leaves committed-but-unreadable rows, with no recovery through the public
API. This is a genuine, live-reproduced production-quality bug in a mature, widely-deployed
(13k GitHub stars) project.

**Spisordning lesson**: **Validate before you persist, never after.** Spisordning's Go/Postgres
stack should validate the full domain object (including all nested structures) *before*
opening a write transaction, and any response-serialization step must not be capable of
leaving a transaction half-committed — wrap persistence and response-shape validation so a
failure anywhere rolls the whole operation back, never partial-writes-then-500s. If Spisordning
ever offers a `PATCH`-shaped partial-update endpoint over nested structures (e.g., editing one
`RecipeRevision`'s ingredient list), define its merge semantics for nested objects explicitly
and test them — "PATCH replaces siblings but silently drops nested object fields" is exactly
the kind of surprising, hard-to-notice bug that erodes trust in an editing API. Given
`RecipeRevision` is immutable by design (`implement-recipe-family-and-revisions`), Spisordning
mostly sidesteps this entire class of bug for revision content — a further point in that
design's favor: immutable-create-only avoids the update-semantics ambiguity Mealie fell into.

---

## 2.3 Recipe import

**User behavior**: Four import paths: paste a URL, paste raw HTML/JSON, upload an image (OCR),
upload a `.zip` export. Bulk URL import exists too.

**API behavior**: `POST /api/recipes/create/url {url, includeTags}` → `201` with the new
recipe's slug (a bare JSON string, not an object). `POST /api/recipes/test-scrape-url
{url}` returns the raw scraped JSON-LD/data without creating anything (useful for diagnosing
scrape failures). `POST /api/recipes/create/html-or-json`, `POST /api/recipes/create/image`,
`POST /api/recipes/create/zip`, `POST /api/recipes/create/url/bulk`.

**Live result — scraping is unreliable and site-dependent**: Tried against four real URLs:

| Site | Result |
|---|---|
| `allrecipes.com/recipe/158140/...` | Created a recipe named `"No Recipe Name Found - <uuid>"` with ingredient `"Could not detect ingredients"` — scrape silently failed but a broken recipe was still created |
| `cooking.nytimes.com/recipes/...` | `"recipe_scrapers was unable to scrape this URL"` (no recipe created) |
| `seriouseats.com/basic-pancakes-recipe` | Same failure |
| `bbcgoodfood.com/recipes/easy-pancakes` | **Succeeded** — real JSON-LD extracted cleanly |
| `ica.se/recept/...` (Swedish site, sanity check) | JSON-LD present and scrapeable per `test-scrape-url` |

Container egress itself was verified working (`urllib.request` to `google.com` returned `200`),
so the NYT/Serious Eats failures are the scraper/site's fault (bot-blocking or an
unsupported/changed page structure), not a network problem.

**DB mutation**: A new `recipes` row plus children, exactly as a manual create — import and
manual creation converge on the same persistence path. Critically: **`create/url` does not
structure the ingredients.** The pancakes recipe, immediately after import, had six
`recipeIngredient` rows all with `quantity: 0, unit: null, food: null` and the raw scraped
line in `note`/`display` (e.g. `"100g plain flour"`). Structuring requires a *separate*
explicit call to the parser (2.4) and then a save — it is not automatic.

**Source**: `mealie/routes/recipe/recipe_crud_routes.py` (`create_from_url` etc.),
`mealie/services/scraper/recipe_scraper.py` — a three-strategy chain, **in this fixed order**:

```python
DEFAULT_SCRAPER_STRATEGIES = [RecipeScraperPackage, RecipeScraperOpenAI, RecipeScraperOpenGraph]
```

`RecipeScraperPackage` wraps the third-party `recipe-scrapers==15.11.0` PyPI package (which
itself has per-site scraper classes plus a generic "wild mode" schema.org fallback for
unlisted sites — this is why some unlisted sites still work and others don't: it depends on
whether `recipe_scrapers`' own fallback can parse that site's markup). `RecipeScraperOpenAI`
only fires if an OpenAI-compatible key is configured (not configured on this instance).
`RecipeScraperOpenGraph` is the last-resort fallback using bare `og:` meta tags, which explains
the Allrecipes result — it produced *a* recipe object (from OG tags) but with no ingredients or
instructions, since OG tags don't carry structured recipe data.

**Tests**: `tests/unit_tests/services_tests/scraper_tests/` — largely tests the cleaner
(`cleaner.py`, normalizes scraped duration/yield strings etc.), not live-site scraping (which
would be flaky by nature and isn't asserted against real network calls in CI).

**Strengths**: `test-scrape-url` as a dry-run diagnostic endpoint is a good pattern — lets a
client show the user what would be imported before committing a write. Falling through
multiple strategies (package → AI → OG) rather than hard-failing is a reasonable resilience
choice, *if* the caller is warned when it degrades to a low-information fallback.

**Weaknesses**: The `RecipeScraperOpenGraph` fallback silently produces a nearly-empty,
misleadingly-named recipe (`"No Recipe Name Found - <uuid>"`) rather than failing loudly and
telling the user to try `test-scrape-url` or a manual entry. A client naively trusting the
`201`+slug response has no signal that the import essentially failed.

**Spisordning lesson**: PLAN.md's own priority — schema.org/JSON-LD extraction as the generic
import path — is correct and matches what `recipe-scrapers` does under the hood; Spisordning
should evaluate reusing or porting the JSON-LD-extraction logic (MIT-licensed, well-tested
against hundreds of real sites) rather than reinventing it, while writing the *cleaning* step
(duration/yield normalization) itself since that's small and domain-specific. Two concrete
process improvements over Mealie: (1) never silently create a "recipe" from a failed/degraded
scrape — surface a clear "scrape produced low-confidence/partial data, review before saving"
state, consistent with PLAN.md's "review unresolved mappings" step in the generic import
pipeline; (2) treat "scraped but not yet ingredient-parsed" as an explicit pipeline stage
(Mealie's import/parse split is right in spirit — do it, but make the intermediate state
visible in the UI/API rather than a recipe that silently has zero quantities until someone
notices and re-parses it).

---

## 2.4 Recipe parsing

**User behavior**: Paste or scrape raw ingredient lines; the system extracts quantity, unit,
food, and note per line, each with a confidence score.

**API behavior**: `POST /api/parser/ingredients {parser: "nlp"|"brute"|"openai", ingredients:
[...]}` → `ParsedIngredient[]`, each `{input, confidence: {average, comment, name, unit,
quantity, food}, ingredient: RecipeIngredient}`. Live example (`"100g plain flour"`):

```json
{
  "input": "100g plain flour",
  "confidence": {"average": 0.999, "unit": 0.9998, "quantity": 1.0, "food": 0.998},
  "ingredient": {"quantity": 100.0, "unit": {"name": "gram", "abbreviation": "g"},
                 "food": {"name": "plain flour", "id": null}, "note": "",
                 "display": "100 gram plain flour"}
}
```

`"1 tbsp sunflower or vegetable oil plus a little extra for frying"` parsed to
`quantity: 1, unit: tablespoon, food: "sunflower vegetable oil" (conf 0.787), note: "plus a
little extra for frying"` — a realistic, messy real-world ingredient line handled reasonably,
with the lower food-confidence (0.787 vs. ~0.999 for the clean lines) correctly flagging that
this is the field most worth a human glance.

**DB mutation**: None by itself — parsing is a pure function over input strings; only saving
the resulting recipe persists anything. A parsed `food`/`unit` with `id: null` (no match in the
group's existing catalog) is **not** auto-created — attempting to `PATCH`/`PUT` a recipe with
such a food object fails server-side (`ValueError: Expected 'id' to be provided for food`)
until the client explicitly `POST`s a new `/api/foods` row first and substitutes its id. This
means the client is responsible for the "review and resolve unmatched food/unit" step PLAN.md
describes for external recipe import — Mealie's API surfaces the *need* for that step (via the
null id and low confidence) but does not orchestrate it; the Vue frontend presumably does this
resolution UI, but nothing enforces or automates it at the API layer.

**Source**: `mealie/services/parser_services/ingredient_parser.py`. Three real, pluggable
parser strategies:
- **`nlp`** (default) — wraps the third-party `ingredient-parser-nlp==2.4.0` PyPI package, a
  CRF (conditional random field) ML model trained on annotated ingredient-line datasets
  (NYT Cooking, Cookstr, etc.), independent of Mealie itself.
- **`brute`** — a hand-written regex/heuristic parser (`parser_services/brute/`), the fallback
  with no ML dependency.
- **`openai`** — delegates to an OpenAI-compatible chat model for extraction, only usable if
  configured.

Each strategy converges on the same `ParsedIngredient` shape and calls a shared
`find_ingredient_match()` to try matching the extracted food/unit text against the group's
existing `ingredient_foods`/`ingredient_units` (including their alias tables) before falling
back to a bare create-by-name object.

**Tests**: `tests/unit_tests/test_ingredient_parser.py` and `test_recipe_parser.py` — build a
seeded food/unit catalog (including deliberately-adversarial cases: `"ñör̃m̈ãl̈ĩz̈ẽm̈ẽ"` for
Unicode normalization, `"green onion"` vs `"frozen pearl onions"` as near-duplicate-substring
foods, explicit alias and plural-name test fixtures) and assert the parser correctly resolves
against existing catalog entries rather than creating duplicates. This is exactly the kind of
edge-case coverage PLAN.md wants ("edge cases they've hit").

**Strengths**: Confidence-per-field is genuinely useful and directly informs a review UI
(low-confidence fields get flagged, high-confidence ones can be silently accepted). Three
swappable parser backends (no-ML/heuristic, ML, LLM) is a sound resilience/upgrade path.
Existing-catalog matching before create-by-name reduces duplicate food/unit proliferation.

**Weaknesses**: Confidence is heuristic and per-*component*, not calibrated/probabilistic in a
rigorous sense (e.g. `average` is a plain mean of whichever component confidences are present,
not a joint probability). No confidence threshold is enforced anywhere in the API — a
low-confidence parse and a high-confidence parse are returned and can be saved identically; the
"should a human review this" decision is entirely a client-side choice.

**Spisordning lesson**: This is close prior art for the already-designed
`name-vs-quantity-confidence` work (per the git log, already implemented) — Mealie's
`IngredientConfidence{quantity, unit, food, comment}` split independently arrived at the same
idea (separate confidence per semantic component rather than one blended score), which is
validating. Consider: (1) evaluate whether `ingredient-parser-nlp` (MIT-ish licensed CRF
model, Python-only) is usable from Go via a small sidecar/subprocess, versus porting the
simpler `brute` heuristic approach directly in Go for v1 and treating ML parsing as a later
upgrade; (2) unlike Mealie, make "needs review" a first-class, queryable state on the
imported/parsed ingredient (not just a confidence number a client may or may not check) —
directly matching PLAN.md's "review unresolved mappings" pipeline step and closing the gap
where Mealie lets a low-confidence, unreviewed parse ship silently.

---

## 2.5 Structured ingredients

**User behavior**: Each ingredient line has a quantity, an optional unit, an optional food, a
free-text note, an auto-generated (or overridable) display string, and can reference an
*entire other recipe* as a "sub-recipe" ingredient (e.g. "1 batch pizza dough").

**API/schema**: `RecipeIngredient{quantity, unit, food, referencedRecipe, note, display, title,
originalText, referenceId}`. `display` is computed by a `@model_validator` if not explicitly
set — `_format_display()` composes quantity (as a fraction or decimal depending on
`unit.fraction`), pluralized unit, pluralized food, and note into one human string, e.g.
`"100 gram plain flour"`, generated fresh on every construction unless the caller has already
supplied a `display` value (import/parse flows do supply their own).

**DB mutation** (schema, already shown in 2.1's companion table):

```sql
CREATE TABLE "recipes_ingredients" (
    ..., position INTEGER, recipe_id CHAR(32), title VARCHAR, note VARCHAR,
    unit_id CHAR(32), food_id CHAR(32), quantity INTEGER, reference_id CHAR(32),
    original_text VARCHAR, note_normalized VARCHAR, original_text_normalized VARCHAR,
    referenced_recipe_id CHAR(32),
    FOREIGN KEY(referenced_recipe_id) REFERENCES recipes (id),
    FOREIGN KEY(unit_id) REFERENCES ingredient_units (id),
    FOREIGN KEY(food_id) REFERENCES ingredient_foods (id)
);
```

Note `quantity` is declared `INTEGER` in the live SQLite schema — a fossil from Mealie's
original 2022 design (2.22); SQLite's loose type affinity has silently tolerated float values
(`100.0` stores fine) since the migration `convert_quantity_from_integer_to_float` explicitly
skipped SQLite ("SQLite doesn't require migration as types are not enforced... Postgres
Specific Migration") — so this repo's own dev database schema string is stale/misleading
relative to what it actually stores, and only Postgres deployments got a real `ALTER COLUMN`.

**Source**: `mealie/schema/recipe/recipe_ingredient.py` (see 2.4 excerpt above) — `quantity`
is rounded to `INGREDIENT_QTY_PRECISION = 3` decimal places on every validation pass; fraction
display uses `Fraction(...).limit_denominator(32)`.

**Tests**: Covered together with 2.4's parser tests; also `tests/unit_tests/schema_tests/` for
the `RecipeIngredient` display-formatting logic (fraction vs. decimal rendering).

**Strengths**: `unit`/`food` as optional nested objects (not required) correctly models
"1 egg" (no unit) and "a pinch of salt, to taste" (unit but arbitrary/no strict quantity).
`referencedRecipe` for sub-recipes (composability — a recipe made of recipes) is a real,
useful modeling choice added relatively recently (2.25's migration, `2025-09-10`) and is
directly relevant to Spisordning's own recipe-composition ambitions. `originalText` preserves
the pre-parse source string alongside the structured result — good for auditability/re-parsing.

**Weaknesses**: No `quantity_max` for ranges ("1–2 cups") — Mealie's model is a single point
value only; a "1-2 cups" ingredient must be squeezed into a free-text note or accepted as
imprecise. No first-class ingredient-form concept (fresh vs. dried vs. canned) — that
distinction, if captured at all, lives inside the free-text `food.name` string (e.g. a user
would create two separate `ingredient_foods` rows, `"basil"` and `"dried basil"`, with no
structural relationship between them — see 2.6).

**Spisordning lesson**: Confirms PLAN.md's Ingredient Forms concern is a real, unaddressed gap
in Mealie — worth Spisordning solving properly (`ingredient_form` as a genuine sub-entity of
`Ingredient`, per `establish-household-and-catalog`'s design) rather than the flat-string
workaround Mealie users are left with. Also: seriously consider a `quantity_max` (nullable,
defaults to `quantity` when absent) for range support from day one — real recipes routinely say
"1-2 cloves garlic" and Mealie's single-point model cannot express that without lying (picking
one end) or falling back to unstructured text, defeating half the purpose of structuring
ingredients at all.

---

## 2.6 Foods

**User behavior**: Foods are the "what" of an ingredient (flour, egg, milk) — created inline
during parsing/editing, or seeded in bulk (`POST /api/groups/seeders/foods`, which populated
1,394 entries from a locale-specific USDA-derived list on this instance), or browsed/merged via
`/api/foods`.

**API behavior**: Full CRUD at `/api/foods`, plus `PUT /api/foods/merge {fromFood, toFood}` —
an explicit "these are duplicates" merge operation, implying duplicate accumulation is an
expected, normal occurrence that the system provides tooling to clean up after the fact rather
than prevent.

**DB mutation/schema**:

```sql
CREATE TABLE "ingredient_foods" (
    ..., id CHAR(32) NOT NULL, group_id CHAR(32) NOT NULL, name VARCHAR, description VARCHAR,
    label_id CHAR(32), name_normalized VARCHAR, plural_name VARCHAR,
    plural_name_normalized VARCHAR, on_hand BOOLEAN NOT NULL,
    CONSTRAINT ingredient_foods_name_group_id_key UNIQUE (name, group_id),
    FOREIGN KEY(label_id) REFERENCES multi_purpose_labels (id)
);
CREATE TABLE ingredient_foods_aliases (
    id CHAR(32) NOT NULL, food_id CHAR(32) NOT NULL, name VARCHAR NOT NULL,
    name_normalized VARCHAR, PRIMARY KEY (id, food_id),
    FOREIGN KEY(food_id) REFERENCES ingredient_foods (id)
);
```

`on_hand` (the migration that added it is literally titled `add_staple_flag_to_foods`, but the
column it actually adds is named `on_hand` — a naming drift between the migration's own comment
and its schema effect, worth flagging as an archaeology curiosity) is a single **global boolean
per food per group** — "we generally have this" — with no quantity, no expiry, no location, no
per-household distinction. `households_to_ingredient_foods` exists as a join table
(presumably scoping *which* households currently have that food "on hand"), a thin retrofit
over the same shallow boolean idea.

**Source**: `mealie/schema/recipe/recipe_ingredient.py` (`IngredientFood`/`CreateIngredientFood`
— see 2.4 excerpt).

**Tests**: `tests/unit_tests/test_ingredient_parser.py` fixtures build deliberately
near-duplicate/confusable foods (`"onion"` vs `"green onion"` vs `"frozen pearl onions"`) to
test that parsing correctly disambiguates against the existing catalog rather than creating
new near-duplicates blindly.

**Strengths**: `group_id`-scoped (shared catalog across all households in one Mealie tenant,
not siloed per household) is a reasonable choice for a shared family food vocabulary. Alias
table lets multiple spellings/names resolve to one canonical food without duplicating rows.

**Weaknesses**: **`ingredient_foods.name` is free-text with no canonical taxonomy behind it at
all.** There is no distinction anywhere in the schema between "chicken breast" (canonical
ingredient) and "Garant Kycklingfilé 900g" (a specific commercial product) — both would just be
rows in the same `ingredient_foods` table if a user typed them, with identical structure and no
flag distinguishing one from the other. `on_hand` is the closest thing to a
pantry/inventory concept and is a single crude boolean, nothing like Grocy's lot/expiry model.

**Spisordning lesson**: This is the single clearest piece of evidence for PLAN.md's
**Ingredient vs. Product distinction being non-negotiable** — Mealie's "Food" is not a
canonical ingredient in any enforced sense; it is *whatever string a user or the parser
produced*, with the merge tool existing precisely because the model allows semantic duplicates
and near-duplicates (a generic ingredient and a specific product) to coexist indistinguishably.
`establish-household-and-catalog`'s `Ingredient`/`Product`/`ProductIngredientMapping` split is
directly validated by watching what happens when that split doesn't exist: an
`ingredient_foods` table that is simultaneously trying to be a controlled vocabulary and a
free-text bucket, needing a manual merge tool as damage control. `on_hand`'s shallowness
likewise validates PLAN.md's explicit warning against `products.current_quantity` as "the
complete inventory model" — Mealie's single boolean is exactly that anti-pattern, just on
`ingredient_foods` instead of `products`.

---

## 2.7 Units

**User behavior**: Units (gram, milliliter, tablespoon, pinch, ...) are managed the same way as
foods: seeded in bulk, created inline, mergeable.

**API/DB**: `/api/units` CRUD + `PUT /api/units/merge`. Schema:

```sql
CREATE TABLE "ingredient_units" (
    ..., group_id CHAR(32) NOT NULL, name VARCHAR, description VARCHAR, abbreviation VARCHAR,
    fraction BOOLEAN, use_abbreviation BOOLEAN, name_normalized VARCHAR,
    abbreviation_normalized VARCHAR, plural_name VARCHAR, plural_name_normalized VARCHAR,
    plural_abbreviation VARCHAR, plural_abbreviation_normalized VARCHAR,
    CONSTRAINT ingredient_units_name_group_id_key UNIQUE (name, group_id)
);
CREATE TABLE ingredient_units_aliases ( ... same shape as food aliases ... );
```

Live seed produced 23 units (pinch, pack, bunch, gram, milliliter, tablespoon, teaspoon,
piece, can, ... — a superset of PLAN.md's candidate list). `fraction: bool` controls whether a
quantity in this unit displays as a fraction (`¾ cup`) or a decimal (`100 g`) — a
display/formatting concern living directly on the unit row.

**The critical finding: there is no unit *conversion* table anywhere in the schema.** No
`unit_conversion`, no dimension/kind column (mass vs. volume vs. count), nothing relating
`gram` to `kilogram` or `milliliter` to `deciliter`, let alone anything cross-dimensional
(volume→mass, i.e. density). Scaling (2.9) multiplies the stored `quantity` directly; it never
converts units. A user who wants "500g" to display as "0.5kg" gets no help from the system —
units are purely independent, unrelated catalog entries with a display-formatting flag, not a
dimensioned measurement system.

**Source**: same `recipe_ingredient.py` (`IngredientUnit`/`CreateIngredientUnit`).

**Tests**: No conversion logic exists to test; formatting tests only (fraction vs. decimal
rendering, abbreviation vs. full-name pluralization).

**Strengths**: Simple, and it's honest about what it is — a labeled, formattable tag on a
quantity, not a unit-conversion engine. No universal density is invented anywhere (accidentally
aligned with PLAN.md's "Do not invent density values universally" instinct, simply because
Mealie never attempts conversion of any kind, universal or ingredient-specific).

**Weaknesses**: This is a significant, user-visible gap in a mature recipe manager — no
"convert this recipe to metric/imperial," no consolidating "300ml" and "0.3L" as the same
quantity on a shopping list, no scaling a unit sensibly (e.g. "1500g" doesn't become "1.5kg").
It's a real limitation, not a deliberate simplicity choice documented anywhere in the product.

**Spisordning lesson**: `establish-household-and-catalog`'s two-tier design — a universal
`UnitConversion` table for same-dimension conversions (g↔kg, ml↔dl↔l) plus a separate
`ingredient_unit_conversion` for ingredient-specific cross-dimension conversions (dl flour →
g) — is **not something Mealie validates by precedent, because Mealie doesn't attempt any of
it.** This is a place Spisordning should do *more* than Mealie, deliberately: real unit
conversion (at least same-dimension) is a legitimate, expected feature for a system this
ambitious, and Mealie's absence of it is a gap to fill, not a pattern to follow. Do keep
Mealie's `fraction`/display-style flag on `Unit` — that part is a genuinely nice, low-cost UX
touch worth copying regardless of the conversion question.

---

## 2.8 Servings

**User behavior**: A recipe carries both a serving count and a yield (e.g. "4 servings" and
"12 items" simultaneously, for a pancake recipe where "servings" and "individual pancakes" are
both meaningful units of output).

**API/DB**: `recipeServings: float`, `recipeYieldQuantity: float`, `recipeYield: string` (e.g.
`"items"`). Live: the imported pancakes recipe scraped to `recipeYieldQuantity: 12,
recipeYield: "items"` with `recipeServings: 0` (servings wasn't present in the source's
schema.org markup and wasn't inferred) — I set `recipeServings: 4.0` manually via `PATCH`.

**Source**: `recipe.py` schema; `recipe_yield_quantity`/`recipe_servings` are separate FLOAT
columns added later than the original schema (per 2.22's migration
`2024-10-23-...add_recipe_yield_quantity`), meaning "yield" as a *structured* quantity+unit-ish
concept (distinct from bare `recipeServings`) is itself a relatively recent addition —
originally there was probably just a free-text yield string.

**Strengths**: Separating "servings" (a portioning concept driving scaling) from "yield" (a
literal output count, e.g. "12 cookies") is the right call — they're genuinely different axes
and conflating them (as many recipe formats do) loses information.

**Weaknesses**: Having three related-but-distinct fields (`recipeServings`,
`recipeYieldQuantity`, `recipeYield`) with no explicit link stating which one (if any) scaling
should be driven by is a soft ambiguity; in practice `recipeServings` drives the frontend's
scale control while yield is display-only, but that convention isn't enforced by the schema.

**Spisordning lesson**: Keep the servings/yield split — it's correct — but make the
scaling-driver relationship explicit in the domain model (e.g. `RecipeRevision.servings` is
the one number a scale factor is ever computed against; yield is purely informational,
never itself scaled or scalable) rather than leaving it an implicit frontend convention as
Mealie does.

---

## 2.9 Scaling

**User behavior**: The recipe view offers a scale control (e.g. "×2") that recalculates
displayed ingredient quantities.

**API behavior — scaling is not a server concept at all.** `GET /api/recipes/{slug}` has no
`scale` query parameter (checked the full OpenAPI parameter list — absent). Scaling is a pure
**client-side** computation: `displayed_quantity = stored_quantity * (desired_servings /
recipe.recipeServings)`. Nothing is ever written back; scaling a recipe for viewing leaves the
DB completely untouched. `RecipeSettings.disableAmount` / `HouseholdPreferences
.recipeDisableAmount` (`recipe_settings.disable_amount`, `household_preferences
.recipe_disable_amount`) exists as an escape hatch for recipes where scaling quantities doesn't
make sense (e.g. "salt to taste") — it's a household-default plus a per-recipe override that
suppresses showing/scaling numeric amounts at all, rather than a per-ingredient flag.

**Where scaling *is* persisted**: only at the point a scaled recipe's ingredients are
materialized onto a shopping list. `shopping_list_item_recipe_reference.recipe_scale FLOAT
NOT NULL` stores the scale factor **as of the moment the recipe was added to that list** — the
shopping list item itself then holds an already-multiplied `quantity`, fully decoupled from the
recipe from that point on (editing the recipe later, or re-scaling it, does not touch
previously-generated shopping list items).

**DB mutation**: None on recipe view/scale. On "add recipe to shopping list", new
`shopping_list_items` rows are inserted with `quantity = ingredient.quantity *
recipeIncrementQuantity` and a `shopping_list_item_recipe_reference` row recording
`recipe_id`, `recipe_quantity`, `recipe_scale`.

**Source**: `mealie/schema/recipe/recipe_ingredient.py` computes display formatting but not
scaling itself; scaling math lives in the Vue frontend (not fetched in depth here — out of
scope per PLAN.md's "prefer retrieval from actual source over guessing" for the *backend*, and
the frontend behavior was directly observed instead via the shopping-list materialization
above, which is the authoritative record of what scale was applied and when).

**Tests**: `shopping_list_recipe_reference`/`shopping_list_item_recipe_reference` round-trips
are covered in the household shopping integration tests.

**Strengths**: Not persisting a "scaled recipe" as mutable state is exactly right — a recipe's
canonical quantities should not be overwritten just because someone viewed it at 2x once.
Materializing the *scale actually used* onto the shopping list reference, decoupled from the
live recipe afterward, is a sound, deliberate design — the shopping list is a point-in-time
snapshot of intent, not a live view.

**Weaknesses**: `disableAmount` is recipe-wide, not per-ingredient — a recipe with one
"to taste" ingredient among ten precisely-measured ones has no way to suppress scaling on just
that line; it's all-or-nothing.

**Spisordning lesson**: Directly validates the design instinct already embedded in
`implement-recipe-family-and-revisions`: **scaling must be a read-time computation, never
stored on `RecipeRevision`** — Mealie proves this pattern works in production at real scale
(pun intended). When Spisordning's shopping/planning capability materializes a shopping
requirement from a scaled recipe (a future Epic), copy Mealie's decoupling: snapshot the
resolved quantity and the scale factor used at materialization time, and never let later
recipe edits retroactively change an already-generated shopping list item. Improve on Mealie by
making "suppress scaling" ingredient-scoped (a boolean or a `quantity: null`/"to taste" sentinel
on the ingredient itself) rather than recipe-wide only.

---

## 2.10 Images

**User behavior**: Upload or scrape a recipe's hero image; it's shown at multiple sizes.

**API behavior**: `PUT/POST /api/recipes/{slug}/image` (upload or scrape-by-URL),
`DELETE /api/recipes/{slug}/image`, served via
`GET /api/media/recipes/{recipe_id}/images/{file_name}`.

**Storage — filesystem, not DB blobs**: Confirmed on disk
(`/app/data/recipes/<recipe-uuid>/images/`):

```text
images/
  original.webp
  min-original.webp
  tiny-original.webp
  timeline/
assets/
```

Every image is normalized to **WebP** regardless of source format, at three fixed sizes. The
`recipes.image` DB column is not a path — it's a short opaque cache-busting token
(`"lhv3"` for the pancakes recipe); the actual file path is deterministic from the recipe's
UUID, so the DB only needs to signal "the image changed" to bust client caches.

**Source**: `mealie/routes/recipe/recipe_crud_routes.py` (image endpoints),
`mealie/services/recipe/recipe_data_service.py` (image processing/resizing, WebP conversion).

**Strengths**: Filesystem storage for binary assets (not DB blobs) is the right call for a
self-hosted app — keeps the DB small and backup-friendly, and standardizing to WebP at fixed
sizes keeps frontend rendering predictable and reasonably fast without per-client resizing.

**Weaknesses**: Deterministic path-from-UUID means image storage and recipe identity are
tightly coupled — there's no independent `RecipeAsset`-style content-addressed image entity
that could be shared/deduped across recipes (e.g. the same stock photo used on two recipes is
stored twice, once per recipe folder).

**Spisordning lesson**: Copy the pattern — filesystem/object-storage for images, DB holds only
a reference/cache-buster, normalize to one format at a few fixed sizes on ingest. Given
`RecipeRevision` is immutable, decide explicitly whether an image belongs to the `RecipeFamily`
(shared across variants/revisions, the common case — a photo of "Korvstroganoff" applies
regardless of which variant) or to a specific `RecipeRevision` (if a revision meaningfully
changed what the dish looks like) — Mealie's single-recipe model never had to make this call,
so it offers no precedent either way; this is genuinely new ground for the
`RecipeFamily`/`Variant`/`Revision` design to settle explicitly.

---

## 2.11 Tags

**User behavior**: Free-form labels attached to recipes for filtering (e.g. "quick",
"breakfast").

**API/DB**: `/api/organizers/tags` CRUD; `tags(id, group_id, name, slug)` with
`UNIQUE(slug, group_id)`; join via `recipes_to_tags`. Live: created `breakfast` and `quick`
tags, attached both to the pancakes recipe via `PATCH /api/recipes/{slug}
{"tags":[{...}]}` — this worked correctly (unlike nested `recipeIngredient`, confirming the
`PATCH` bug in 2.2 is specific to relations carrying nested sub-objects like `unit`/`food`, not
to many-to-many joins in general).

**Source**: `mealie/schema/recipe/recipe.py` (`RecipeTag`).

**Strengths**: Simple, correctly group-scoped, slug-unique. `GET /api/organizers/tags/empty`
(tags with zero recipes) is a nice small utility for catalog hygiene.

**Weaknesses**: No hierarchy, no synonym/alias table (unlike foods/units, which do have
aliases) — a "breakfast" and a "brunch" tag can't be related to each other structurally.

**Spisordning lesson**: Fine to copy as-is for a simple free-text label system; not a
priority area for redesign. If Spisordning wants tag-driven cookbook-style filters (2.13), keep
tags this lightweight and let filter expressiveness live in the query layer, not the tag model.

---

## 2.12 Categories

**User behavior**: A second, structurally-identical labeling axis to tags, conventionally used
for a recipe's primary classification (e.g. "Baking", "Main Course") vs. tags' more granular,
multi-valued labeling.

**API/DB**: `/api/organizers/categories`; schema is **byte-for-byte identical in shape** to
`tags` (`id, group_id, name, slug`, same unique constraint pattern), joined via
`recipes_to_categories`. Live: created a "Baking" category, attached via the same `PATCH`
mechanism as tags.

**Weaknesses**: Categories and tags are the same data structure with two different names and
two different tables/join-tables — the only distinction is convention-of-use (documented
nowhere in the schema itself), not anything the system enforces. A user could just as easily
use categories the way they use tags and vice versa; nothing differentiates them mechanically.

**Spisordning lesson**: This is a real "avoid this" signal — two identically-shaped tables
distinguished only by naming convention is schema duplication without a schema reason. If
Spisordning wants both a primary-classification axis and a free-multi-label axis, consider
either (a) one `tag` table with a `kind` enum (`category`/`tag`) if the two are genuinely
never combined in a query, or (b) keep them separate only if there's a real behavioral
difference (e.g. categories are single-select per recipe, tags are multi-select) — Mealie's
categories are *not* single-select (a recipe can have many), so there is no such behavioral
difference here, undermining the case for two tables at all. Don't copy this duplication
without a concrete reason Mealie itself doesn't have.

---

## 2.13 Cookbooks

**User behavior**: A named, curated-feeling recipe collection (e.g. "Weeknight favorites").

**API behavior**: `POST /api/households/cookbooks {name, description, categories: [...]}`.

**The critical finding: a cookbook is a *saved filter*, not a curated list of specific
recipes.** Confirmed both via the live response and the schema — creating a cookbook with
`categories: [{"id": "...", "name": "Baking"}]` produced:

```json
{"name": "Weeknight favorites", "slug": "weeknight-favorites", "queryFilterString": "",
 "requireAllCategories": null, "queryFilter": {"parts": []}, ...}
```

```sql
CREATE TABLE "cookbooks" (
    ..., position INTEGER NOT NULL, name VARCHAR NOT NULL, slug VARCHAR NOT NULL,
    description VARCHAR, group_id CHAR(32), public BOOLEAN, require_all_categories BOOLEAN,
    require_all_tags BOOLEAN, require_all_tools BOOLEAN, household_id CHAR(32),
    query_filter_string VARCHAR DEFAULT '' NOT NULL,
    CONSTRAINT fk_cookbooks_household_id FOREIGN KEY(household_id) REFERENCES households (id)
);
```

There is **no join table between cookbooks and specific recipes at all.** A cookbook's
contents are computed at read time from `cookbooks_to_categories`/`cookbooks_to_tags`
/`cookbooks_to_tools` (which recipes it should match) plus `require_all_*` (AND vs. OR
semantics) plus an arbitrary `query_filter_string` (a more powerful ad-hoc filter expression,
added later per the `2024-10-08 added_query_filter_string_to_cookbook` migration). Membership is
therefore entirely dynamic — add a category to a recipe and it silently joins every cookbook
requiring that category; remove it and the recipe silently leaves. There is no way to pin one
specific recipe into a cookbook independent of its tags/categories.

**Source**: `mealie/schema/recipe/recipe.py` region for `Cookbook` schema; query resolution
happens in the recipe repository layer (`repository_recipes.py`) by translating a cookbook's
filter fields into a SQL/ORM `WHERE` clause at list-time, not by joining a membership table.

**Strengths**: Zero-maintenance "collections" — a cookbook stays current automatically as
recipes are tagged/categorized, with no risk of a cookbook going stale because someone forgot
to add a new matching recipe to it. `household_id`-scoping (a cookbook is one household's view,
even though the underlying recipes/tags/categories are group-shared) is a sensible
where-does-this-live decision — my cookbook curation choices shouldn't be visible/editable by
another household sharing the same Mealie tenant.

**Weaknesses**: A household **cannot** curate an explicit, hand-picked list of recipes as a
cookbook — only a filter. If a user wants "these exact 5 recipes, regardless of their
tags/categories," Mealie's cookbook model cannot express it at all. This is a real, surprising
limitation for a feature whose name ("cookbook") strongly implies manual curation.

**Spisordning lesson**: Dynamic, filter-based collections are a genuinely good idea and worth
having *as one kind of collection* — but Spisordning should not copy this as the *only* kind.
Support both: a `saved_filter`-style dynamic collection (Mealie's cookbook, useful for "all
quick breakfasts") **and** an explicit membership table for a hand-curated collection (a real
`cookbook_recipe(cookbook_id, recipe_id)` join, useful for "the five recipes we're actually
committing to cook this month"). This also connects directly to PLAN.md's "Automatic Cookbook
Growth" vision (external recipe → cooked → reviewed → saved to household cookbook) — that
flow fundamentally requires *explicit* membership (a specific recipe was cooked and is being
retained), which a pure filter-based model like Mealie's cannot represent at all.
