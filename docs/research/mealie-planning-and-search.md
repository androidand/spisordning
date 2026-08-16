# Mealie Investigation: Search, Planning, Shopping, Households, Users, Ratings (items 2.14–2.19)

Continuation of `mealie-recipe-model.md` (same live instance, same test data: household
"Family", one user, one imported recipe "Easy pancakes" with structured ingredients, tags
`breakfast`/`quick`, category `Baking`, a cookbook, a meal plan entry, a shopping list, a
favorite, and a 4.5 rating). See that document's header for methodology notes.

---

## 2.14 Search

**User behavior**: Free-text search across recipe name/description, filterable by
category/tag/tool/food.

**API behavior**: `GET /api/recipes?search=pancake` → `{"total": 1, "items": [...]}`. Also
accepts `categories=`, `tags=`, `tools=`, `foods=` as additional filter params, composable with
`search`. Live: `?search=pancake` correctly matched "Easy pancakes" (case-insensitive, partial
match).

**DB mutation**: None (read-only), but two **derived, materialized columns** exist purely to
make search fast: `recipes.name_normalized`, `recipes.description_normalized` — pre-lowercased
(and presumably accent-stripped/Unicode-normalized) copies of the display text, recomputed on
every write, that search filters against instead of the display columns directly.
`ingredient_foods.name_normalized`/`plural_name_normalized` and
`ingredient_units.*_normalized` follow the identical pattern (added later, per the 2023
`added_normalized_search_properties` and `added_normalized_unit_and_food_names` migrations —
this was **not** part of the original 2022 schema, meaning early Mealie either had no working
fuzzy/case-insensitive search or relied on slower `LOWER(name) LIKE ...` scans).

**Backend-dependent fuzzy search**: On Postgres, a real trigram index exists —
`CREATE EXTENSION pg_trgm; CREATE INDEX ... USING gin (name_normalized gin_trgm_ops)`
(migration `2023-04-13 postgres_fuzzy_search`) — giving genuine fuzzy/typo-tolerant matching.
**On SQLite (this deployment, and Mealie's default/small-scale option), no such index exists**
— search there falls back to a plain substring/`LIKE` scan over the normalized column. This is
a real, documented capability gap between the two supported databases, not just a performance
difference.

**Source**: `mealie/repos/repository_recipes.py` (query construction, category/tag/food join
filters), `mealie/db/init_db.py` (pg_trgm extension setup), `mealie/schema/_mealie
/mealie_model.py` (the `_normalize_search`/`_searchable_properties` mixin used by `Recipe`,
`IngredientFood`, `IngredientUnit`, `RecipeTag` alike — a shared, reusable normalization
pattern across every searchable entity).

**Tests**: `tests/integration_tests/` recipe search round-trips; no dedicated SQLite-vs-Postgres
fuzzy-search-parity test was found — a gap, since the two backends genuinely behave differently
for the same query.

**Strengths**: The `_normalize_search` mixin as a shared, declarative pattern
(`_searchable_properties: ClassVar[list[str]]`) applied uniformly to every searchable entity
is a clean piece of reusable design — new entities opt into normalized-search columns with one
class attribute rather than bespoke code per table.

**Weaknesses**: The SQLite/Postgres search-quality gap is real and undocumented at the
API/schema level — a client can't tell from the response whether it's getting fuzzy matching
or exact substring matching.

**Spisordning lesson**: Spisordning is Postgres-only by design (per PLAN.md/ADRs), so it can
adopt the *good* half of this pattern — `pg_trgm` GIN trigram indexes on normalized text
columns — from day one, with no SQLite-parity compromise to worry about. Copy the
normalized-column-plus-trigram-index approach directly for recipe/ingredient/product name
search. Also worth copying: the shared "any searchable entity declares its searchable columns"
pattern, implemented as a small reusable Go helper/interface rather than ad-hoc per-repository
query building.

---

## 2.15 Meal plans

**User behavior**: Assign a recipe (or free-text entry, e.g. "leftovers") to a date and a
meal slot (breakfast/lunch/dinner/side). A "randomize" button can auto-fill a slot from
matching recipes, optionally constrained by category/tag/day-of-week rules.

**API behavior**: `POST /api/households/mealplans {date, entryType, title, text, recipeId}`.
Live:

```json
{"date": "2026-08-17", "entryType": "dinner", "title": "", "text": "",
 "recipeId": "40c63a6e-...", "id": 1, "householdId": "54211088-...",
 "recipe": { ...full embedded recipe... }}
```

`POST /api/households/mealplans/random {date, entryType}` → picks a recipe automatically
(honoring any `group_meal_plan_rules` scoped to that day/category/tag/household) and creates
the entry in one call. `GET /api/households/mealplans/today`.

**DB mutation/schema**:

```sql
CREATE TABLE "group_meal_plans" (
    id INTEGER NOT NULL, date DATE NOT NULL, entry_type VARCHAR NOT NULL,
    title VARCHAR NOT NULL, text VARCHAR NOT NULL, group_id CHAR(32), recipe_id CHAR(32),
    user_id CHAR(32),
    FOREIGN KEY(recipe_id) REFERENCES recipes (id)
);
CREATE TABLE "group_meal_plan_rules" (
    ..., group_id CHAR(32) NOT NULL, day VARCHAR NOT NULL, entry_type VARCHAR NOT NULL,
    household_id CHAR(32), query_filter_string VARCHAR DEFAULT '' NOT NULL
);
-- plus join tables plan_rules_to_categories / plan_rules_to_tags / plan_rules_to_households
```

`recipe_id` is **nullable** — a meal plan entry can be a bare `title`/`text` freeform note with
no recipe at all (e.g. "eating out", "leftovers night"), correctly modeling that not every
planned meal traces back to a structured recipe.

**The critical finding — meal plans have no concept of "did this actually happen."** There is
no attendance/participant field, no link from a `group_meal_plans` row to anything recording
that the meal was actually cooked/eaten, no per-person reaction or review tied to a specific
planned night, and (confirmed in 2.19) recipe ratings are **recipe-scoped, never
meal-instance-scoped** — rating a recipe after eating it is indistinguishable from rating it
after just reading it. A meal plan entry is purely prospective; nothing in Mealie's schema
closes the loop PLAN.md describes ("a planned dinner may produce an actual Meal").

**Source**: `mealie/routes/household/controller_mealplan.py` (routes),
`mealie/services/household_services/` (random-pick logic, rule matching).

**Tests**: `tests/integration_tests/user_household_tests/` covers meal plan CRUD and rule
matching for the random-fill feature.

**Strengths**: Nullable `recipe_id` for freeform entries is a small but correct modeling
choice — forcing every meal plan entry to reference a recipe would be wrong. Rule-based
auto-fill (day-of-week + category/tag constraints) is a reasonable, explainable randomization
mechanism, not an opaque black box.

**Weaknesses**: The complete absence of a planned-vs-actual distinction is the single biggest
gap in Mealie's entire data model relative to PLAN.md's ambitions — there is no way, even in
principle, to answer "what did we actually eat last Tuesday and how did it go," only "what was
planned." This isn't a minor omission; it's the entire "Meals" domain PLAN.md wants
(`meal_event`/`meal_participant`/`meal_review`) simply not existing in Mealie at any level.

**Spisordning lesson**: **`implement-meals-and-preferences`'s premise is not just
un-contradicted by Mealie, it is essentially unaddressed by Mealie at all** — this is the
strongest confirmation across the whole investigation that the planned/actual split
(`meal_plan_decision` vs. `meal_event`) is genuinely necessary work, not a speculative
gold-plating exercise. There is no Mealie prior art to mine here beyond "this is what a
planning-only tool without an actual-meal-history concept looks like, and here's the ceiling
that hits" — Spisordning should treat this as validated, novel ground rather than expect to
find more nuance by digging deeper into Mealie's meal-plan code. One small pattern worth
copying regardless: nullable `recipe_id` on the planning side for non-recipe entries.

---

## 2.16 Shopping

**User behavior**: A named shopping list; items can come from manually typed text, from adding
a whole recipe's ingredients at a chosen quantity, or standalone (non-recipe) entries like
"paper towels". Items can be checked off, labeled (for aisle/category grouping), and carry
arbitrary extra key-value metadata.

**API behavior**: `POST /api/households/shopping/lists {name}`;
`POST /api/households/shopping/lists/{id}/recipe/{recipeId} {recipeIncrementQuantity}` adds
every ingredient of that recipe, scaled by the given increment, as list items in one call —
live-tested and confirmed: adding the 6-ingredient pancakes recipe produced 6 new
`shopping_list_items`, each with the recipe's `unit`/`food` copied over and `quantity` equal to
the recipe's ingredient quantity × the increment.

**DB mutation/schema**:

```sql
CREATE TABLE shopping_list_items (
    ..., shopping_list_id CHAR(32), is_ingredient BOOLEAN, position INTEGER NOT NULL,
    checked BOOLEAN, quantity FLOAT, note VARCHAR, is_food BOOLEAN, unit_id CHAR(32),
    food_id CHAR(32), label_id CHAR(32),
    FOREIGN KEY(food_id) REFERENCES ingredient_foods (id),
    FOREIGN KEY(label_id) REFERENCES multi_purpose_labels (id)
);
CREATE TABLE shopping_list_item_recipe_reference (
    ..., shopping_list_item_id CHAR(32) NOT NULL, recipe_id CHAR(32),
    recipe_quantity FLOAT NOT NULL, recipe_scale FLOAT NOT NULL, recipe_note VARCHAR
);
CREATE TABLE shopping_list_recipe_reference (
    ..., shopping_list_id CHAR(32) NOT NULL, recipe_id CHAR(32), recipe_quantity FLOAT NOT NULL
);
```

`is_food`/`unit_id`/`food_id` are all **nullable** on `shopping_list_items` — a single table
serves both structured (from-a-recipe) items and bare freeform-text items
(`is_food: false, note: "paper towels"`), distinguished by a boolean flag rather than two
separate tables/subtypes. `shopping_list_item_recipe_reference` is the audit trail: which
recipe (and at what `recipe_scale`) produced this specific list item — but once created, the
item's own `quantity`/`checked`/`note` are independently mutable and **do not sync back** if
the source recipe changes later (confirmed conceptually from the schema: no trigger/FK
`ON UPDATE CASCADE` relationship exists that would propagate a recipe edit into already-created
shopping items — the reference is historical record, not a live binding).

**Source**: `mealie/routes/household/controller_shopping_lists.py`,
`mealie/services/household_services/` (recipe-to-shopping-list materialization logic).

**Tests**: `tests/integration_tests/user_household_tests/` covers list CRUD, item CRUD, and
recipe-add materialization.

**Strengths**: The scale/quantity snapshot on `shopping_list_item_recipe_reference` (2.9) is a
clean, well-thought-out decoupling — shopping intent, once generated, is independent of the
recipe that spawned it. Supporting both structured and freeform items in one flexible list is
pragmatic — real shopping lists genuinely mix "food from a recipe" and "toilet paper."

**Weaknesses**: `is_food`/`is_ingredient` as two separate nullable booleans on the same row
(rather than one clear discriminant, or genuinely separate tables) is exactly the kind of
"nullable relationship serving two purposes" PLAN.md's Database Review Questions ask about —
it works, but it's a soft version of the polymorphic-shortcut pattern PLAN.md warns against, not
a real subtype split.

**Spisordning lesson**: The "materialize a snapshot when adding a recipe to a shopping list,
then let it live independently" pattern is exactly right and matches PLAN.md's own "Local
Shopping Intent" section (distinguishing "Need 500g chicken breast" from a specific retailer
product) — Spisordning's `shopping_list_items` should snapshot the resolved
Ingredient+quantity+unit at add-time the same way. Consider a cleaner split than Mealie's
nullable-boolean-on-one-table for structured-vs-freeform items — e.g. always require a
`food_id`, and represent a genuinely freeform item as a manually-created, un-mapped `Product`
or `Ingredient` stub rather than a null-everything row; this keeps `shopping_list_items` a
single, real-FK-backed shape rather than a row whose meaning depends on which nullable columns
happen to be populated.

---

## 2.17 Households

**User behavior**: A household is a named sub-group within a Mealie "group" (tenant) —
multiple households can share one Mealie server/tenant (e.g. a family running one Mealie
instance for several related households) while each keeps separate meal plans, shopping lists,
and cookbooks, but shares the same recipe/food/unit/tag catalog.

**API behavior**: `GET /api/households/self` → the live "Family" household:

```json
{"groupId": "5a1f5ab1-...", "name": "Family", "id": "54211088-...", "slug": "family",
 "preferences": {"privateHousehold": true, "lockRecipeEditsFromOtherHouseholds": true,
                  "firstDayOfWeek": 0, ...}, "users": [{"id": "77b53cb8-...", "fullName": "Change Me"}]}
```

**DB mutation/schema**:

```sql
CREATE TABLE households (
    id CHAR(32) NOT NULL, name VARCHAR NOT NULL, slug VARCHAR, group_id CHAR(32) NOT NULL,
    CONSTRAINT household_name_group_id_key UNIQUE (group_id, name),
    CONSTRAINT household_slug_group_id_key UNIQUE (group_id, slug)
);
CREATE TABLE household_preferences (
    id CHAR(32) NOT NULL, household_id CHAR(32) NOT NULL, private_household BOOLEAN,
    first_day_of_week INTEGER, recipe_public BOOLEAN, recipe_show_nutrition BOOLEAN,
    recipe_show_assets BOOLEAN, recipe_landscape_view BOOLEAN, recipe_disable_comments BOOLEAN,
    recipe_disable_amount BOOLEAN, lock_recipe_edits_from_other_households BOOLEAN
);
```

**Membership is a bare FK, not a join table with history.** `users.household_id` is a single
nullable FK column directly on `users` — one user belongs to exactly one household at a time,
with **no membership history** (no "joined on"/"left on" timestamps, no record of a user having
previously belonged to a different household). Confirmed: `households` itself has no
`member_count`/roster beyond querying `users WHERE household_id = ...`.

**Archaeological context — households were retrofitted, not designed in from the start.** The
`households` table did not exist in Mealie's original schema; it was added by migration
`2024-07-12 add_households`, nearly 2.5 years after the initial 2022 schema, as a level
inserted *between* the existing "group" (tenant) and "user" tables. The migration script itself
is revealing: it copies `group_preferences` values onto newly-created default households
(`create_household()`), and mechanically reassigns `household_id` on `users`,
`cookbooks`, `webhook_urls`, etc. by matching `group_id` — a real multi-table backfill
migration, not just a schema change. Before this, "group" *was* effectively "household" (one
undifferentiated tenant = one household), and multi-household-per-tenant was simply
unsupported.

**Source**: `mealie/routes/household_services/`, `mealie/schema/household/household.py`
(not read in full — inferred from live responses and DB schema, consistent with the rest of
the investigation's source-grounded findings).

**Tests**: `tests/integration_tests/user_household_tests/`,
`tests/multitenant_tests/` — the latter specifically tests **cross-group isolation** (that one
tenant's data is never visible to another), a security-relevant boundary directly analogous to
Spisordning's household-isolation requirement.

**Strengths**: `lockRecipeEditsFromOtherHouseholds` is a nice small feature — even though
recipes are shared catalog data across households in one tenant, a household can protect its
own recipes from being edited by another household sharing the same server. The
group→household retrofit, while late, was done with a real data-preserving migration rather
than a breaking change — worth noting as a *process* strength even though the *design* arrived
late.

**Weaknesses**: No membership history is a real, structural gap — Mealie cannot answer "who
was in this household when this meal was rated," matching a concern PLAN.md explicitly raises
for `HouseholdMembership`. A user can only ever belong to one household at a time with no
join/leave audit trail.

**Spisordning lesson**: Two distinct, validated lessons. **(1)** Mealie's own history —
starting with only a flat "group" tenant and having to retrofit a household layer 2.5 years
later, with a genuinely nontrivial backfill migration — is a strong argument for
`establish-household-and-catalog` building `Household` (and the `HouseholdMembership` join
table with real join/leave lifecycle, which Mealie still lacks even after its retrofit) in from
day one rather than deferring it. **(2)** Mealie's bare `users.household_id` FK (no membership
history) is precisely the shortcut PLAN.md's `HouseholdMembership` design already avoids by
being a genuine append-and-close join entity, not a single mutable FK — this is validated as
the correct call, directly informed by watching Mealie hit exactly this limitation.

---

## 2.18 Users

**User behavior**: A user is a login (`username`/`email`/`password`) that also *is* the
household member — there is no other way to represent a person in Mealie's data model.

**DB mutation/schema**:

```sql
CREATE TABLE "users" (
    ..., full_name VARCHAR, username VARCHAR, email VARCHAR, password VARCHAR,
    admin BOOLEAN, advanced BOOLEAN, group_id CHAR(32) NOT NULL, cache_key VARCHAR,
    can_manage BOOLEAN, can_invite BOOLEAN, can_organize BOOLEAN, can_manage_household BOOLEAN,
    owned_recipes_id CHAR(32), login_attemps INTEGER, locked_at DATETIME,
    auth_method VARCHAR(6) DEFAULT 'MEALIE' NOT NULL, household_id CHAR(32),
    FOREIGN KEY(household_id) REFERENCES households (id)
);
```

**The critical finding: login identity and food-domain person are the same row, with no way to
separate them.** `users` conflates authentication (`password`, `auth_method`,
`login_attemps`, `locked_at` — supports local password and OIDC) with household-member
identity (`household_id`, `full_name`) and with every user-scoped food-preference/behavior
(`users_to_recipes.rating`/`is_favorite`, per 2.19). There is no lower-privilege,
no-login "child" or "guest" concept — every household member who is to have a favorite, a
rating, or appear as a meal-plan creator **must** be a full login account with a password (or
OIDC identity), `auth_method` notwithstanding. A household with young children who should have
their own favorites/preferences tracked, but no login credentials of their own, cannot be
modeled without giving them an account.

**Source**: `mealie/db/models/users/users.py`, `mealie/schema/user/user.py`.

**Tests**: `tests/integration_tests/user_tests/`, `tests/integration_tests/user_group_tests/`,
`tests/integration_tests/admin_tests/`.

**Strengths**: `auth_method` supporting both local password and OIDC in one table with a
discriminant column, rather than two parallel user tables, is a reasonable pragmatic choice
for an app whose primary job is not identity management.

**Weaknesses**: This is the second-clearest (after Ingredient-vs-Product) structural gap found
in the whole investigation. Conflating `Account` and `Person` means Mealie **cannot** model a
household member who isn't a login — precisely the scenario PLAN.md calls out by name ("Do not
conflate login identity with household Person"). It also means every rating/favorite is
inescapably tied to a real authenticated login, forever ruling out tracking a child's food
preferences unless that child is issued credentials.

**Spisordning lesson**: This is the strongest single validation found anywhere in this
investigation for a specific already-written design decision:
`establish-household-and-catalog`'s `Account`/`Person` split (Account = login identity, Person
= food-domain subject, optional FK between them, a Person may exist with no Account) is
**directly proven necessary** by watching what Mealie cannot do as a result of not having it.
This isn't a hypothetical PLAN.md concern — it's Mealie's actual, load-bearing limitation today.
No reconsideration needed here; if anything, this raises the design's priority.

---

## 2.19 Ratings (and favorites)

**User behavior**: A user can favorite a recipe and/or give it a 0–5 star rating, independent
of any specific meal or cooking event.

**API behavior**: `POST /api/users/{id}/favorites/{slug}`,
`POST /api/users/{id}/ratings/{slug} {rating, isFavorite}` (note: keyed by *user id*, path
param `slug` actually accepts a slug for most calls but the read-side
`GET /api/users/self/ratings/{recipe_id}` requires a **UUID, not a slug** — a real, minor
API inconsistency, confirmed live: passing the slug to that specific endpoint raised a
`uuid_parsing` `422`). Both favorite and rating write to the same underlying resource.

**DB mutation/schema — one row per (user, recipe), not per meal instance**:

```sql
CREATE TABLE users_to_recipes (
    user_id CHAR(32) NOT NULL, recipe_id CHAR(32) NOT NULL, rating FLOAT,
    is_favorite BOOLEAN NOT NULL, id CHAR(32) NOT NULL,
    PRIMARY KEY (user_id, recipe_id, id),
    CONSTRAINT user_id_recipe_id_rating_key UNIQUE (user_id, recipe_id)
);
```

Live, after setting a 4.5 rating: `('77b53cb8...', '40c63a6e...', 4.5, 0, ...)` — exactly one
mutable row, unique per `(user, recipe)`. **Rating and favorite share one row/table**, scoped
per-user per-recipe — confirming Mealie already avoids PLAN.md's specific warning against a
*global* recipe-level favorite boolean (it's user-scoped, not global) — but there is **no
concept of a meal instance at all**: rating "Easy pancakes" today and rating it again after
cooking it three more times over the next month is the same UPDATE to the same row — the
previous rating is simply overwritten, with no history.

**The `recipes.rating` legacy column, and how it's actually resolved on read** — this took
source inspection to fully explain (the live behavior alone was ambiguous with only one test
user). `mealie/repos/repository_recipes.py`:

```python
"""Computed rating which uses the user's rating if it exists, otherwise falling back to
the recipe's rating"""
effective_rating = sa.case(
    (sa.and_(UserToRecipe.rating != None, UserToRecipe.rating > 0),
     sa.select(sa.func.max(UserToRecipe.rating))...),
    (self.model.rating == 0, None),
    else_=self.model.rating,
)
```

**This means the `rating` field returned on any `GET /api/recipes/{slug}` is the *requesting
user's own* rating (or a legacy fallback), never a cross-household/cross-user average.** Mealie
exposes **no aggregate rating across multiple people anywhere in its API** — there is no
"average rating," no "3 people rated this 4.6 average," nothing. Every user sees only their own
opinion reflected back.

**Archaeological context — this is the single most important migration finding in the whole
investigation.** `users_to_recipes` did not exist until migration
`2024-03-18 migrate_favorites_and_ratings_to_user_ratings`. Before it, Mealie had exactly the
anti-pattern PLAN.md explicitly warns against: a **group-tenant-wide `recipes.rating INTEGER`
column** (one rating value for the whole tenant, not per-user) plus a separate
`users_to_favorites` boolean join table. The migration's own code comment, verbatim:

```python
# Convert recipe ratings to user ratings. Since we don't know who
# rated the recipe initially, we copy the rating to all users.
```

Faced with retrofitting per-user scoping onto a design that never captured *who* gave the
original rating, the migration's only option was to **fabricate** an identical rating for
every user in the group — data that never actually reflected those individual users' opinions,
silently written into their history as if it had. This is real, permanent, unrecoverable
information loss caused directly by not scoping ratings to a person from the start.

**Source**: `mealie/routes/users/ratings.py` (verbatim excerpt above),
`mealie/repos/repository_recipes.py` (`_get_rating_col_alias`),
`mealie/alembic/versions/2024-03-18-...migrate_favorites_and_ratings_to_user_.py`.

**Tests**: `tests/integration_tests/user_recipe_tests/` covers rating/favorite CRUD;
`tests/unit_tests/test_alembic.py` runs migrations forward (verifying they don't crash) but
does not appear to assert anything about the *fidelity* of the favorites/ratings data migration
specifically (i.e., no test catching "did we just silently fabricate individual ratings").

**Strengths**: The current (post-migration) `users_to_recipes` design — one row per
(user, recipe), never per meal instance — is at least honestly a "current opinion," not
pretending to be more than it is. The read-time fallback logic (`effective_rating`) is a
reasonable non-breaking way to keep old aggregate-rating data visible to users who haven't yet
rated something themselves personally.

**Weaknesses**: No meal-instance scoping at all — a rating today about a dish read online and a
rating next month after actually cooking it are indistinguishable and mutually overwriting.
No cross-user aggregate rating is exposed anywhere, meaning even today's *better* design still
doesn't give a household "the household liked this 4.2/5 on average" — every person's opinion
is siloed and only ever shown to themselves.

**Spisordning lesson**: This is the strongest and most concrete evidence found in this entire
investigation, and it validates two separate already-written decisions simultaneously.
**(1)** `implement-meals-and-preferences`'s insistence that a `MealReview` is a rating of a
specific `meal_event` (a specific cooked instance), not of the recipe directly, is proven
correct by watching Mealie's own migration team be forced to fabricate data because their
original design didn't capture *who* rated *what instance* — Spisordning should treat this as
settled, not speculative. **(2)** Person-scoped ratings from day one (never a group/tenant-wide
column) avoids the exact lossy-migration scenario Mealie lived through — this is not a
theoretical risk PLAN.md is being cautious about, it is a documented, real incident in a
widely-deployed piece of software. Additionally: Mealie's complete absence of any *cross-person
aggregate* rating (each user only ever sees their own) is a genuine gap Spisordning should not
inherit — `implement-meals-and-preferences`'s task 3.3 ("design recipe-level rating
aggregation... decide the aggregation function") is doing necessary, unprecedented work here,
not validated-but-also-not-contradicted by Mealie; there is simply no prior art to draw on for
*how* to aggregate multiple people's `MealReview`s, only strong evidence that failing to keep
per-person attribution from the start is unrecoverable later.
