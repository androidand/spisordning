## Why

`migrations/0001_init.sql` already has `meal_event`/`meal_reaction` (an actual served meal and
per-person sentiment reaction to it) and `meal_plan`/`meal_plan_candidate`/`meal_plan_decision`
(the planning side). That's a reasonable start on PLAN.md's planned-vs-actual split, but it
predates `establish-household-and-catalog`'s `Household`/`Person`/`PersonRestriction` model, has
no notion of **who was actually present** at a meal (`meal_reaction` records a reaction, not
attendance), no **favorites** at all (PLAN.md is explicit: favorites must be person/household
scoped, never a global recipe boolean), and only a single `sentiment SMALLINT` per reaction
rather than PLAN.md's richer "Andreas 5/5, Vera 4/5, Valdemar 2/5" review-with-aggregation
picture. This change reconciles PLAN.md's `meals`/`meal_participants`/`meal_reviews` naming
against what's already shipped, adds what's missing (participants, favorites, a richer
per-person rating separate from the existing coarse reaction), and defines how ratings
aggregate to the recipe level — without renaming or breaking the existing scorer's use of
`meal_event`/`meal_reaction`.

**Validated by `establish-reference-lab`'s Mealie findings (2026-08-16):** this is not a
hypothetical design preference. `docs/research/mealie-planning-and-search.md` found that Mealie
originally shipped a group-wide `recipes.rating` column and had to retrofit per-user scoping
later (migration `2024-03-18_migrate_favorites_and_ratings_to_user_`), with the migration's own
committed comment reading *"Since we don't know who rated the recipe initially, we copy the
rating to all users"* — a real, permanent, fabricated-data incident baked into every Mealie
instance that existed before that migration ran. Person-scoped `MealReview` from day one (this
change) is specifically the thing that incident would have prevented; it's cited here as the
concrete reason this isn't over-engineering.

## What Changes

- Reconcile naming: keep `meal_event` (already exists, already consumed by
  `internal/scoring`) as the "actual meal" entity rather than introducing a competing `meals`
  table; add `meal_participant` (who was actually present/ate) alongside the existing
  `meal_reaction` (who reacted, and how) rather than overloading one table with both concerns.
- Add `MealReview`: a richer per-person, per-meal-instance rating (PLAN.md's "Andreas 5/5, Vera
  4/5, Valdemar 2/5" example) as a sibling of `meal_reaction`, not a replacement — decide during
  design whether `meal_reaction`'s `sentiment` and a new numeric rating are the same field
  widened, or genuinely two concerns (quick reaction vs. considered review).
- Add recipe-level rating aggregation, computed from `MealReview` history, exposed
  read-side (not stored as a mutable denormalized column unless design work justifies caching).
- Add `Favorite`: an explicit, person/household-scoped preference over a recipe (Mealie
  recipe id, consistent with `recipe_ref`), distinct from and never derived automatically from
  ratings/reactions. A recipe with a low average rating can still be a favorite (e.g. a child's
  comfort food); a recipe with a high average rating is not automatically a favorite.
- Keep `meal_plan`/`meal_plan_candidate`/`meal_plan_decision` (planned food) as-is; this change
  only adds the actual-meal-side entities (participants, reviews, favorites) that were missing,
  and wires `meal_event`/`meal_plan_decision` to `Household`/`Person` from
  `establish-household-and-catalog` instead of the flat `person` table.

## Capabilities

### New Capabilities

- `meal-history`: actual meal attendance, per-person reviews, recipe-level rating aggregation,
  and person/household-scoped favorites — the "actual food" and "explicit preference" side of
  PLAN.md's Meals/Favorites/Ratings sections, kept distinct from `meal-planning`'s "planned
  food" and suggestion-scoring concerns (owned by `food-brain-first-slice`).

### Modified Capabilities

<!-- none — `meal-planning` (food-brain-first-slice) is not modified; this change is additive
     and depends on establish-household-and-catalog's Household/Person, not a replacement of
     the existing meal_plan tables -->

## Impact

- Affected code: `migrations/` (additive migration on top of `0001_init.sql`), `internal/domain`
  (new MealParticipant/MealReview/Favorite types), no change to `internal/scoring`'s existing
  consumption of `meal_event`/`meal_reaction`.
- Depends on `establish-household-and-catalog` for `Household`/`Person`/`HouseholdMembership`;
  this change should land after (or alongside, with FK targets adjusted) that one.
- Does not touch `recipe_ref` beyond referencing its `mealie_recipe_id`; recipe-family modeling
  (`RecipeFamily`/`RecipeVariant`/`RecipeRevision`) is `implement-recipe-family-and-revisions`'s
  concern, not this change's — `Favorite`/`MealReview` here reference whatever recipe identity
  exists at the time (currently `mealie_recipe_id`), reconciled later if recipe identity moves.
