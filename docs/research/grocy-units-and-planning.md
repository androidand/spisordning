# Grocy research: units and planning (tasks.md 3.14–3.20)

Continuation of `grocy-inventory-and-stock.md` — same live instance, same methodology (direct
SQLite, REST API with a manually-inserted API key, and PHP source read from the running
container and GitHub). Test data continues from that document's products (Milk/Rice/Eggs/Frozen
Peas) plus a new recipe exercised live: **"Rice Pudding"** (`recipe_id=1`, `base_servings=4`,
`desired_servings=4`), positions Rice 500g, Milk 1L, Eggs 2 pieces, added to the meal plan for
today at `recipe_servings=2`, and actually cooked (`POST /api/recipes/1/consume`) — with a
specific, surprising discrepancy between the two "2" and "4" numbers documented in §3.19.

---

## 3.14 Units

**User behavior.** A quantity unit is a name + plural form ("Gram"/"Grams"). Products declare
*four* separate unit roles, not one: `qu_id_purchase` (bought in), `qu_id_stock` (tracked in),
`qu_id_consume` (defaults to stock, but can differ — e.g. buy a "Pack," stock in "Piece," but the
consume-form defaults to a different granularity), and `qu_id_price` (defaults to purchase — the
unit prices are quoted in). This is the single most consequential Grocy design decision in the
whole units cluster: **there is no single "the unit" for a product** — there are four,
independently settable, and independently convertible via `quantity_unit_conversions` (§3.15).

**API behavior.** `GET/POST/PUT/DELETE /api/objects/quantity_units`. Live-created:

```json
POST /api/objects/quantity_units {"name":"Gram","name_plural":"Grams"}     → id 4
POST /api/objects/quantity_units {"name":"Kilogram","name_plural":"Kilograms"} → id 5
POST /api/objects/quantity_units {"name":"Liter","name_plural":"Liters"}   → id 6
```
(Grocy's own defaults, `Piece`/`Pack`, already existed as ids 2/3.)

**DB mutation.** `quantity_units(id, name UNIQUE, description, name_plural, plural_forms, active)`.
`plural_forms` (a locale-aware pluralization rule string, unused in this test) exists for
languages with more than two grammatical number forms — a genuinely deep i18n detail most
inventory systems skip entirely.

**Source implementation.** `services/QuantityUnitsService.php` for CRUD; the four-role split
lives entirely in the `products` table's four `qu_id_*` columns and is consumed throughout
`StockService.php`/the resolved SQL views (§3.15).

**Tests.** None.

**Strengths.** Separating purchase/stock/consume/price units is real, hard-won generality:
"bought by the pack, tracked by the piece, but the consume form should still let me say 'I used
one pack's worth'" is a genuine household pattern (Eggs, in this test's own data, uses exactly
this: purchase=Pack, stock=Piece, and `qu_id_consume` defaulted to Piece too, but could
legitimately be set to Pack for a household that only ever consumes whole cartons at once).

**Weaknesses.** Four independently-settable unit roles is also four independent places for a
conversion to be missing or wrong (§3.15's `products_default_qu_conversions_INS` trigger exists
specifically to paper over exactly this by auto-inserting 1:1 stand-ins — see §3.16 for the
concrete way that auto-insert bit this research during test-data setup).

**Spisordning lesson.** PLAN.md's Unit System section lists a flat set of units (`g, kg, ml, dl,
l, piece, tbsp, tsp, pinch, package, can`) without yet deciding how many *roles* a
product/ingredient needs. Grocy's four-role split (purchase/stock/consume/price) is worth
deliberately evaluating against Spisordning's simpler two-role split implied by design.md
(`RecordPurchase(..., unit, ...)` vs. an implicit stock unit) — a `Unit`-per-role design adds
real flexibility Grocy users clearly lean on, but each additional role is a source of the exact
silent-1:1-conversion footgun documented in §3.16, so this should be a conscious complexity
trade-off, not an accident of copying Grocy's column count.

---

## 3.15 Unit conversion

**User behavior.** Grocy supports two independent conversion mechanisms layered on top of each
other: **global/default conversions** (`kg → g`, defined once, apply to every product that uses
those units and has no product-specific override) and **product-specific conversions**
(§3.16). A product's actual purchase→stock (and consume→stock, price→stock) factor is resolved
by walking both together.

**API behavior.** `GET/POST/PUT/DELETE /api/objects/quantity_unit_conversions`
(`from_qu_id`, `to_qu_id`, `factor`, `product_id` nullable — NULL means global). Resolved
(computed) conversions are read via `GET /api/objects/quantity_unit_conversions_resolved`, a
read-only view, never written to directly. Live-created global conversion:

```json
POST /api/objects/quantity_unit_conversions {"from_qu_id":5,"to_qu_id":4,"factor":1000}
→ {"created_object_id":"1"}
```

Reading it back immediately showed **two** rows, not one:

```json
[{"id":1,"from_qu_id":5,"to_qu_id":4,"factor":1000,"product_id":null},
 {"id":2,"from_qu_id":4,"to_qu_id":5,"factor":0.001,"product_id":null}]
```

**DB mutation — the inverse-conversion trigger.** `quantity_unit_conversions_INS` (an `AFTER
INSERT` trigger on the table itself, confirmed by reading `.schema quantity_unit_conversions`
directly) automatically inserts the mathematical inverse (`1/factor`) as a second row every time
a conversion is created, and a matching `_UPD` trigger keeps the inverse in sync on edit (verified
live in §3.16: updating the Eggs Pack→Piece factor from 1 to 6 automatically flipped the
existing inverse row's factor from `1` to `0.16666666666666666` with no second API call). A third
trigger pair (`qu_conversions_custom_constraint_INS`/`_UPD`) enforces "no duplicate
`(from,to,product)` conversion" as a hand-rolled check (SQLite `UNIQUE` constraints don't cover
NULL-inclusive tuples, hence the trigger instead of a real constraint).

**Resolution algorithm — read from `quantity_unit_conversions_resolved`'s view definition
directly**: the *closure* over all conversions is computed via a **recursive CTE**, twice —
once for the global/default conversion graph (any unit reachable from any other via chained
default conversions), and once for the product-specific graph, with product-specific
conversions taking priority and falling through to the default graph only where no
product-specific path exists (an explicit `product_reachable`/`product_reachable_distinct` CTE
stage literally joins the product-specific closure onto the default closure's *edges*, not
just its endpoints — i.e. a product-specific override on one leg of a multi-hop conversion can
still ride on default conversions for the rest of the path). Cycle prevention is done via a
`path` string column (`'/from/to/'`) checked with `NOT LIKE` in the recursive step.

**Source implementation.** `.schema quantity_unit_conversions_resolved` on the live DB (the full
~150-line view, quoted in this research's working notes); consumed everywhere a
`from_qu_id`/`to_qu_id`/`product_id` factor lookup is needed
(`ConsumeProduct`'s subproduct-substitution branch, `recipes_pos_resolved`, `products` triggers
on `qu_id_stock` change, `OpenProduct`'s subproduct branch).

**Tests.** None.

**Strengths.** The recursive-closure approach is genuinely more capable than a flat lookup table
— defining `kg→g` once and `g→mg` once correctly yields `kg→mg` (and all inverses) without an
explicit third row, and the same machinery serves both the global unit graph and every
individual product's override graph uniformly. Automatic inverse-row generation removes an
entire class of "I defined A→B but forgot B→A" bugs.

**Weaknesses.** This is powerful but genuinely hard to reason about by inspection — a `factor`
for a given `(product, from, to)` pair can originate from a direct override, a chained override,
a pure-default conversion, or a chained default-plus-override mix, and the *only* way to know
which is to run the view (or read all ~150 lines of recursive SQL). There is no `source`/
`explanation` column anywhere telling a caller *why* a resolved factor is what it is. Critically
(see §3.8), **the stock-mutation API never calls this resolution logic itself** — `AddProduct`/
`ConsumeProduct` take a raw `amount` already assumed to be in the stock unit; unit conversion is
a client-side (or recipe-view-SQL-side) concern only, never applied automatically by the write
path. A REST client integrating against Grocy must replicate (or separately query) this
resolution logic itself before ever calling `/add`.

**Spisordning lesson.** This is the concrete evidence base for PLAN.md's Unit System section
("Universal dimensions must remain distinct from ingredient-specific conversions. Do not invent
density values universally") — Grocy's two-tier (global default graph + product-specific
override graph, product-specific always wins) is a validated, real-world-proven shape for
exactly that distinction and is worth adopting close to verbatim: a `unit_conversion` table with
a nullable `product_id`/`ingredient_id` scoping column, global rows for true universal
dimensions (mass↔mass, volume↔volume — never mass↔volume without a per-product/per-ingredient
density), product-specific rows for count-based or density-dependent conversions (§3.16). The
recursive-closure resolution is worth adopting in spirit but should ship with an explainability
requirement Grocy lacks — `implement-recipe-availability`'s stated goal ("every verdict is
explainable... machine-readable reason, not an opaque score") should extend to unit resolution
too: a resolved factor should be able to say *which* conversion(s) it was built from. Whether
Spisordning's stock/inventory write path performs conversion server-side (recommended, given
§3.8's finding) or requires the caller to pre-convert (Grocy's actual, riskier choice) is a
concrete design decision this research surfaces, not one to inherit by default.

---

## 3.16 Product-specific conversion

**User behavior.** A product can override the global conversion graph for its own purchase/
stock/consume/price units — "for Eggs specifically, 1 Pack = 6 Piece" — even when no global
`Pack→Piece` conversion exists (Pack and Piece are otherwise unrelated *count* units with no
universal ratio, unlike mass or volume).

**API behavior.** Same `POST /api/objects/quantity_unit_conversions` endpoint as §3.15, with
`product_id` set.

**DB mutation — a real footgun, hit live in this research.** Creating the Eggs product
(`qu_id_purchase=Pack, qu_id_stock=Piece`, before any conversion existed) silently triggered
`products_default_qu_conversions_INS` (an `AFTER INSERT ON products` trigger), which inserted a
**1:1** `quantity_unit_conversions` row `(from_qu_id=Pack, to_qu_id=Piece, factor=1,
product_id=3)` automatically, on the reasoning "some conversion must exist wherever
`qu_id_stock != qu_id_purchase`, default to identity if none is given yet." The subsequent
attempt to *insert* the real 6:1 conversion failed outright:

```json
POST /api/objects/quantity_unit_conversions {"from_qu_id":3,"to_qu_id":2,"factor":6,"product_id":3}
→ {"error_message":"SQLSTATE[23000]: Integrity constraint violation: 19 QU conversion already exists"}
```

because the `(from_qu_id, to_qu_id, product_id)` tuple already existed (as the auto-inserted
1:1 stub) and the custom uniqueness trigger (§3.15) rejected the duplicate. The correct fix —
discovered by reading `quantity_unit_conversions` back and editing the existing row —
was a `PUT` on the auto-created row's `id`, not a `POST`:

```json
PUT /api/objects/quantity_unit_conversions/3 {"from_qu_id":3,"to_qu_id":2,"factor":6,"product_id":3}
→ 200; inverse row (id 4) automatically flipped from factor 1 to 0.16666666666666666
```

Had the (very natural) `POST`-then-error been accepted at face value without checking existing
rows, Eggs would have silently stayed at a **1:1 Pack:Piece conversion** — meaning every future
purchase entered as "1 Pack" would only add 1 Piece to stock, six times less than reality, with
no error, ever, to indicate the mistake.

**Source implementation.** The `products_default_qu_conversions_INS`/`_UPD` triggers (`.schema
products`, quoted in `grocy-inventory-and-stock.md`'s §3.1) fire for `qu_id_purchase`,
`qu_id_consume`, and `qu_id_price` independently — a product with all four unit roles distinct
from `qu_id_stock` gets **three** auto-inserted 1:1 stub conversions on creation, any or all of
which may need to be corrected afterward.

**Tests.** None.

**Strengths.** Guaranteeing *some* conversion always exists (rather than leaving a product in a
state where purchase-to-stock math is undefined) avoids a worse failure mode — a `NULL`/missing
conversion causing an exception mid-purchase. Editing in place (rather than needing a
delete-then-insert) once you know the row exists is a one-call fix.

**Weaknesses. This is a genuine, discoverable UX/API footgun with real inventory-accuracy
consequences**, not a hypothetical one — it happened during this research's own test-data setup
on the very first product where purchase and stock units genuinely differed by more than 1:1.
Nothing in the product-creation response, the API, or the UI's product-edit form makes clear that
a *wrong* placeholder conversion now silently exists until a human notices stock levels are off
by the true factor.

**Spisordning lesson.** This is the strongest concrete evidence in the whole Grocy investigation
for a specific implementation requirement, not just a design principle: **creating a
`Product`/`Ingredient` whose purchase/stock (or any other role-pair) units differ MUST NOT
silently default to a 1:1 conversion.** Either (a) require the real conversion factor as part of
the same creation call/transaction (reject creation without it), or (b) if a placeholder must
exist to satisfy referential/computation completeness, make it loudly, unmissably distinct from
a confirmed conversion — e.g. a `confirmed: bool` column on `unit_conversion`, surfaced
prominently anywhere the product/ingredient is shown, and blocking `RecordPurchase` in that unit
pair until confirmed. Silent identity-factor defaults are exactly the kind of "accumulated
edge case" PLAN.md asks this research to surface, and this is as concrete as it gets.

---

## 3.17 Shopping

**User behavior.** One (or more — `shopping_lists` supports multiple named lists) running list
of "things to buy," each line a product + amount (+ optional free-text note for non-product
items). Populated manually, or automatically from three distinct triggers: below-minimum-stock,
overdue products, and expired products — each its own explicit action, not implicit background
behavior.

**API behavior.**

```
POST /api/stock/shoppinglist/add-missing-products   {"list_id": 1}
POST /api/stock/shoppinglist/add-overdue-products    {"list_id": 1}
POST /api/stock/shoppinglist/add-expired-products     {"list_id": 1}
POST /api/stock/shoppinglist/add-product              {"product_id","product_amount","qu_id","note","list_id"}
POST /api/stock/shoppinglist/remove-product
POST /api/stock/shoppinglist/clear                    {"list_id","done_only"}
POST /api/recipes/{recipeId}/add-not-fulfilled-products-to-shoppinglist
```

Live-exercised: `POST /api/recipes/1/add-not-fulfilled-products-to-shoppinglist` against the
Rice Pudding recipe (fully in stock at the time) correctly returned an **empty** result — nothing
added, because nothing was missing (`need_fulfilled=1` from the fulfillment check, §3.18).

**DB mutation.** `shopping_list(id, product_id NULLABLE, note, amount, shopping_list_id,
done, qu_id)`. `product_id` is nullable specifically so a shopping list can hold **free-text
items with no product mapping at all** ("napkins," "something for Vera's lunch") — a real,
deliberate design choice, not an oversight. `AddMissingProductsToShoppingList`
(`StockService.php` line 21) is *merge-not-duplicate*-aware: if a product is already on the
list, it only bumps the existing row's amount **up** (never down, never adds a second row) when
the newly-computed missing amount exceeds what's already listed.

**Source implementation.** `StockService::AddMissingProductsToShoppingList`/
`AddOverdueProductsToShoppingList`/`AddExpiredProductsToShoppingList` (lines 21–111);
`GetMissingProducts` (a query against `stock_missing_products`, itself driven by
`min_stock_amount` vs. current aggregated stock).

**Tests.** None.

**Strengths.** Nullable `product_id` for free-text items is a real, correct design decision —
PLAN.md's own "Local Shopping Intent" section wants exactly this kind of "Need 500g chicken
breast" (generic, not necessarily product-mapped) distinguished from a specific retailer SKU, and
Grocy's shopping list already lives partway toward that distinction (a list line can be a
product reference *or* free text, though notably not "an ingredient reference distinct from a
product," since Grocy has no Ingredient/Product split at all — see §3.1). Merge-not-duplicate
auto-add logic avoids the obvious annoyance of running "add missing products" twice and getting
doubled quantities.

**Weaknesses.** A shopping list line references a `Product` directly, never anything looser like
"any product that satisfies this ingredient" — because Grocy has no Ingredient concept, "I need
milk, brand doesn't matter" isn't representable; every shopping list line is already resolved to
one specific product.

**Spisordning lesson.** `PLAN.md`'s "Local Shopping Intent" section's distinction ("Need 500g
chicken breast" vs. "Willys Garant Chicken 900g") is *not* fully realized in Grocy — Grocy's
shopping list is Product-scoped (or free-text), never Ingredient-scoped, which is direct evidence
that Spisordning's own `shopping_requirement`/`shopping_list_item` split (Ingredient-level need
vs. Product-level or RetailerProduct-level line item) is going *further* than Grocy's reference
behavior, deliberately, not something Grocy validates or contradicts — Spisordning is filling a
real gap here, not copying a solved problem. The merge-not-duplicate auto-add pattern is worth
adopting directly for any automatic "generate shopping list from unmet requirements" feature.

---

## 3.18 Recipes

**User behavior.** A recipe is a name, serving count (`base_servings` the recipe was written for,
`desired_servings` the household currently wants — defaulting equal, adjustable independently to
trigger automatic scaling), and an ordered list of product+amount+unit positions
(`recipes_pos`), each optionally flagged `only_check_single_unit_in_stock` (treat "is any amount
in stock" as sufficient — e.g. spices) or `not_check_stock_fulfillment` (ignore entirely for
fulfillment purposes — e.g. "salt to taste," water).

**API behavior.** `GET /api/recipes/{id}/fulfillment` / `GET /api/recipes/fulfillment` (all
recipes at once) — live-exercised:

```json
GET /api/recipes/1/fulfillment
→ {"recipe_id":1,"need_fulfilled":1,"need_fulfilled_with_shopping_list":1,
   "missing_products_count":0,"costs":7580,"costs_per_serving":1895,
   "calories":0,"due_score":10,"product_names_comma_separated":"Rice,Milk,Eggs",
   "prices_incomplete":0}
```

`POST /api/recipes/{id}/consume` "cooks" it (§3.19's live exercise); `POST
/api/recipes/{id}/copy` clones a recipe (fork-without-lineage — no relationship is recorded
between the original and the copy beyond a fresh, independent row); `POST
/api/recipes/{recipeId}/add-not-fulfilled-products-to-shoppinglist` (§3.17).

**DB mutation / the whole fulfillment+cost+calorie computation is a SQL view, not application
code.** `recipes_pos_resolved` (~100 lines, read in full from the live DB) computes, per recipe
position, in one `SELECT`: serving-scaled required amount (`recipe_amount`, with an explicit
`CEIL()` branch for `round_up`-flagged positions like "you need whole eggs, not 2.3 of one"),
current aggregated stock for that product, whether it's fulfilled from stock alone vs. fulfilled
including the shopping list, the missing amount, per-position cost (`price × amount`, price
sourced from `products_current_price` — itself "whichever price the *next lot to consume* was
purchased at" — see §3.20), per-position calories, and a numeric `due_score` (0/1/10/20 for
ok/due_soon/overdue/expired) used to rank which recipes should be cooked soonest to use up
aging stock. `recipes_resolved` aggregates positions up to one row per recipe
(`MIN(need_fulfilled)` across positions — one missing ingredient fails the whole recipe;
`SUM(costs)`, `SUM(due_score)`).

**Recipe nesting.** `recipes_nestings(recipe_id, includes_recipe_id, servings)` +
`recipes_nestings_resolved` (a **recursive CTE**, `WITH RECURSIVE r1(...)`, computing transitive
`includes_servings` products across arbitrary nesting depth) lets one recipe *include* another
(a "Pizza Dough" sub-recipe nested inside a "Pizza" recipe) — the same recursive-closure pattern
as unit conversion (§3.15), reused for a structurally different problem. Two trigger pairs
(`prevent_self_nested_recipes_*`, `prevent_infinite_nested_recipes_*`) guard against direct and
transitive cycles.

**Source implementation.** `.schema recipes_pos_resolved`, `.schema recipes_resolved`, `.schema
recipes_nestings_resolved` on the live DB; `services/RecipesService.php` is comparatively thin
(174 lines total) because the heavy computation lives in SQL, not PHP.

**Tests.** None.

**Strengths.** Nested/composable recipes with correct transitive serving-ratio math is a real,
non-trivial capability, and reusing recursive-CTE closures for both unit conversion and recipe
nesting shows a consistent (if dense) architectural idea repeated on purpose.
`only_check_single_unit_in_stock`/`not_check_stock_fulfillment` per-position flags are a real,
useful escape hatch for "ingredients too trivial to track" (salt, water, garnish).

**Weaknesses.** Pushing this much business logic into ~100-line SQL views is a genuine
maintainability and testability concern — the fulfillment/cost/calorie computation cannot be
unit-tested in isolation from a running SQLite instance with the full schema loaded, cannot be
stepped through, and any change requires understanding one dense `SELECT` doing five unrelated
jobs (unit conversion + substitution + price + shopping-list-awareness + due-status scoring) at
once. `recipes.type` conflates four semantically different row kinds in one table + one column
(`normal` — a real household recipe; `mealplan-day`/`mealplan-week`/`mealplan-shadow` — synthetic
rows the meal-plan trigger machinery auto-generates purely for reusing the recipe-fulfillment
computation, see §3.19) — a real recipe and an internal-bookkeeping pseudo-recipe are
indistinguishable at the schema level without checking `type`.

**Spisordning lesson.** Grocy's fulfillment logic is the single most direct, evidence-rich prior
art for `implement-recipe-availability`, and it validates that capability's scope almost exactly
(per-ingredient-line fulfillment, aggregated to a recipe-level verdict, substitution-aware,
shopping-list-aware) — but its *implementation shape* is close to the opposite of what
`implement-recipe-availability`'s proposal.md asks for ("every verdict is explainable... a
machine-readable reason, not an opaque score"): Grocy's `need_fulfilled` is a single bit computed
deep inside a wall of SQL, with no way to ask "why is this 0" without re-deriving the same
`CASE` expression by hand. Spisordning's stated plan (`internal/availability`, pure Go domain
logic, no new tables unless caching proves necessary) is the right divergence — implement the
per-line reasoning as inspectable, testable domain code, using Grocy's *rules* (serving-ratio
scaling, `round_up`, "only check any amount" escape hatches, shopping-list-inclusive fulfillment
as a secondary verdict) as the requirements list, not its SQL as an implementation template. The
synthetic-recipe-row overloading (`recipes.type`) is a concrete anti-pattern to avoid: reusing
one entity's machinery for an unrelated bookkeeping purpose by adding a `type` discriminator
column is exactly the kind of blurred-aggregate-boundary PLAN.md's "Database Design Process"
Step 2 (identify domain aggregates and boundaries) is meant to prevent.

---

## 3.19 Meal planning

**User behavior.** A meal plan entry is a `(day, recipe-or-product, servings-or-amount)` tuple,
optionally grouped into a named section (breakfast/lunch/dinner — `meal_plan_sections`). A
"cook this" action on a meal plan entry should consume the right ingredients for the right
serving count and mark the entry done.

**API behavior.** Generic CRUD via `/api/objects/meal_plan`. Live-created:

```json
POST /api/objects/meal_plan {"day":"2026-08-16","type":"recipe","recipe_id":1,"recipe_servings":2}
→ {"created_object_id":"1"}
```

**DB mutation — the auto-generated shadow-recipe mechanism, confirmed live.** Inserting that one
`meal_plan` row triggered `create_internal_recipe` (an `AFTER INSERT` trigger on `meal_plan`
itself), which synthesized **three additional rows in the `recipes` table**, none requested
directly, all with **negative IDs** (via `(SELECT MIN(id) - 1 FROM recipes)` — an explicit
ID-space-collision-avoidance hack, since `id` is `AUTOINCREMENT` and would otherwise never
produce a negative value):

```
id=-6  name='2026-08-16'    type='mealplan-day'     (aggregates every meal-plan entry for that day)
id=-7  name='2026-32'       type='mealplan-week'     (aggregates every entry that ISO-week)
id=-8  name='2026-08-16#1'  type='mealplan-shadow'   (this specific meal-plan entry, at its own recipe_servings)
```

each wired into `recipes_nestings`/`recipes_pos` so the *existing* `recipes_resolved`/
`recipes_pos_resolved` machinery (§3.18) can compute "is everything needed for today fulfilled,"
"is everything needed this week fulfilled," and per-entry cost/calories **for free**, reusing
recipe-fulfillment SQL for a structurally different question, without writing any new
computation. The trigger is re-run (delete-and-recreate the day/week/shadow rows) on every
`meal_plan` insert/update/delete for that day, an expensive but correctness-preserving approach.

**The serving-count discrepancy, found live.** `POST /api/recipes/1/consume` was called directly
against the *real* recipe id (1, `desired_servings=4`) — not its shadow recipe (id -8, which
would have looked up the meal plan entry and used its `recipe_servings=2` instead, per
`RecipesService::ConsumeRecipe`'s explicit `RECIPE_TYPE_MEALPLAN_SHADOW` branch, read directly
from source). The result: Rice -500g, Milk -1L, Eggs -2pc consumed — **exactly the recipe's own
4-serving amounts, not the meal plan's 2-serving amount.** Grocy's shadow-recipe design *does*
correctly solve "consume the meal-plan-specific quantity" — but only if the caller knows to
target the shadow recipe's id, not the "real" recipe's id; calling the wrong (more obvious,
more discoverable) id silently uses the wrong quantity, with no error, exactly as it did in this
test.

**Source implementation.** `.schema meal_plan` (the `create_internal_recipe`/
`remove_internal_recipe`/`update_internal_recipe` triggers, ~150 lines of near-duplicated SQL
across all three, quoted in full in this research's working notes);
`RecipesService::ConsumeRecipe` lines 114–124 for the shadow-recipe serving-count substitution.

**Tests.** None.

**Strengths.** Reusing one computation engine (recipe fulfillment) for three distinct questions
(single recipe / whole day / whole week) via synthetic recipes is a clever way to avoid writing
and maintaining parallel aggregation logic — genuinely elegant *as SQL engineering*.

**Weaknesses.** This is dense, hard-to-explain machinery (negative-id synthetic rows,
near-duplicated trigger bodies for insert/update/delete, a `recipes.type` discriminator carrying
real semantic weight) purely in service of computation reuse — and it has a real, demonstrated
caller-facing footgun (the serving-count discrepancy above) baked directly into its design:
"which id do I call `/consume` on" is not obvious from the API surface, and picking wrong fails
silently rather than erroring.

**Spisordning lesson.** This is the clearest evidence in the whole investigation that **Meal
(actual, servings-specific) and MealPlanEntry (planned) must be distinguishable at the type/API
level, not merely by which numeric id a caller happens to pass** — PLAN.md's own "Meals" section
already separates `meal_plan`/`meal_plan_entries` from `meals`/`meal_participants`/
`meal_reviews`, and this finding is direct validation that the separation needs to be structural,
not just conceptual: a "cook this specific meal-plan entry" operation should take a
`mealPlanEntryId` directly (never a bare `recipeId`, which is ambiguous between "the recipe as
generically defined" and "the recipe as scaled for a specific planned occasion") — exactly what
design.md's future `implement-meal-planning`/`implement-recipe-availability` boundary should
enforce by construction (distinct handler signatures, not a shared endpoint with an easy-to-miss
distinction). Do not reuse recipe-fulfillment machinery to answer meal-plan questions via
synthetic recipe rows — instead give `MealPlanEntry` its own explicit fulfillment/cost
computation that reads the same underlying `InventoryLot`/`IngredientSubstitution` data
directly, at the entry's own specified serving count, with no id-aliasing possible.

---

## 3.20 Cost tracking

**User behavior.** Every stock lot carries the price it was actually purchased at (or was set to
during an inventory correction). "What did this recipe cost" and "what is my average/current
price for this product" are both derived, not separately entered.

**API behavior.** `GET /api/stock/products/{id}/price-history`; cost fields
(`costs`/`costs_per_serving`/`prices_incomplete`) come back as part of
`GET /api/recipes/{id}/fulfillment` (§3.18, live example above: `costs: 7580` /
`costs_per_serving: 1895` for a 4-serving recipe — Grocy stores/returns money as a decimal
number, not integer cents, despite the round-number appearance here).

**DB mutation.** No separate "price" or "cost" table at all — cost tracking is entirely computed
from `stock`/`stock_log.price`, via three layered SQL views read in full from the live DB:

- `products_current_price`: the price of the **specific lot `stock_next_use` would consume
  next** (i.e. "current price" means "what the next thing I'll actually use cost," not a
  separate quoted/list price), falling back to `cache__products_last_purchased.price` if nothing
  is currently in stock.
- `products_average_price`: a **stock-log-derived weighted average**, `SUM(amount × price) /
  SUM(amount)` across all non-undone `purchase`/`inventory-correction`/`self-production` rows —
  explicitly excluding `stock-edit-old` rows and using only the newest `stock-edit-new` row per
  edited entry (via a `stock_edited_entries` helper), so a manually-corrected historical price
  doesn't get double-counted in the average.
- `recipes_pos_resolved.costs`: `resolved_amount × products_current_price.price`, per position,
  summed by `recipes_resolved` into per-recipe and per-serving costs.

`cache__products_average_price`/`cache__products_last_purchased` are materialized caches,
explicitly refreshed by `stock_log_INS`/`_UPD`/`_DEL` triggers (§3.4/§3.5) — cost data is
therefore always at most one write-transaction stale, never independently recomputed on a
schedule.

**Source implementation.** `.schema products_current_price`, `.schema products_average_price`
on the live DB (both quoted above); `recipes_pos_resolved`'s `costs`/`calories` expressions
(§3.18) for recipe-level rollup.

**Tests.** None.

**Strengths.** Deriving cost entirely from actual recorded purchase prices (never a separately
maintained "current price" field a human could forget to update) means cost data can never drift
from reality the way a manually-maintained price field would — it is a direct, structural
consequence of the same `stock`/`stock_log` data everything else already writes. Excluding
`undone` transactions and correctly de-duplicating edited entries from the average shows real
care for correctness under correction.

**Weaknesses.** "Current price" meaning "whatever the next lot to be consumed cost, whenever it
was bought" (not "the most recent purchase price," and not any external/live retailer price) is
a genuinely surprising semantic that would mislead anyone assuming otherwise — a household that
bought milk for 15kr last week and 20kr yesterday will see "current price" as whichever lot FEFO/
FIFO happens to consume next, not necessarily the most recent one. `prices_incomplete` (present
in the live fulfillment response as `0`, meaning complete) signals when any position has no
known price, but the recipe-level `costs`/`costs_per_serving` numbers say nothing about *which*
positions are estimated versus known even when incomplete.

**Spisordning lesson.** This is strong validation for PLAN.md's Price Model section's instinct
("Research whether current price should be mutable or represented as observations... Likely
model: retailer_products, store_product_offers, price_observations") — Grocy shows the
observations-not-a-mutable-field approach working well *for cost-of-goods-actually-purchased*,
which is a genuinely different concern from *current retailer shelf price* (Spisordning's
`price_observations` is explicitly about the latter, driven by retailer/store scraping, not
purchase history). Spisordning should keep these two "price" concepts explicitly distinct rather
than converging on one derived-price view the way Grocy does — Grocy has no retailer-price
concept at all, so "current price" can only ever mean "cost basis of what's in the pantry," and
conflating that with "what would it cost to buy more today" would be a real regression for a
system that (unlike Grocy) has actual live retailer price data available.
