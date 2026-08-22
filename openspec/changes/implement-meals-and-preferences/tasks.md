## 1. Reconcile planned vs. actual naming

- [x] 1.1 Compare PLAN.md's candidate naming (`meal_plans`/`meal_plan_entries` for planned food;
      `meals`/`meal_participants`/`meal_reviews` for actual food) against what
      `migrations/0001_init.sql` already ships (`meal_plan`/`meal_plan_candidate`/
      `meal_plan_decision` for planned; `meal_event`/`meal_reaction` for actual) and decide,
      table by table, whether to keep, rename, or add alongside.
      → all existing tables kept; `meal_participant` and `meal_review` added alongside `meal_reaction`;
      full comparison in `design.md` §Decision.
- [x] 1.2 Confirm `meal_event` is kept as the "actual meal" entity (do not introduce a
      competing `meals` table) since `internal/scoring` and `cmd/food-brain/plan.go` already
      depend on `meal_event`/`meal_reaction` — document this decision explicitly since it
      diverges from PLAN.md's literal naming.
      → kept; documented in `design.md` §Actual food row and in `proposal.md` §What Changes.
- [x] 1.3 Confirm a planned dinner (`meal_plan_decision`) may produce an actual `meal_event`,
      per PLAN.md's "a planned dinner may produce an actual Meal" — decide whether that link is
      an explicit FK on `meal_event` or inferred by date+recipe match.
      → explicit nullable FK `meal_event.meal_plan_decision_plan_id`; documented in `design.md` §Plan→actual link.

## 2. Meal participants (actual attendance)

- [x] 2.1 Add `meal_participant`: who was actually present/ate at a given `meal_event`,
      distinct from `meal_reaction` (who reacted and how) — a person can attend without
      reacting, and (design question) can a reaction exist without recorded attendance?
      → `meal_participant` is a many-to-many attendance ledger; `meal_reaction` is a
      separate reaction ledger. A reaction MAY exist without a participant row
      (a person reacts about a meal they didn't attend — e.g. reviewing a meal they
      watched someone else cook), but the common case is both rows present.
      Schema in §5.2; domain type `MealParticipant` in §5.3.
- [x] 2.2 Decide participant capture UX/data source: explicit tally at serving time vs. implied
      from household membership at the time vs. inferred from who left a reaction.
      → explicit tally at serving time (CLI `food-brain serve` or HA tap). Implied-from-
      membership is wrong when someone is away; inferred-from-reaction misses non-
      reactors. The schema leaves the door open for future automation but the first-
      class path is explicit.

## 3. Ratings (per-person review of an actual meal)

- [x] 3.1 Design `MealReview` per PLAN.md's example ("Andreas 5/5, Vera 4/5, Valdemar 2/5") —
      decide whether this widens existing `meal_reaction.sentiment` (currently SMALLINT -2..2)
      to a richer scale, or is a genuinely separate table (quick reaction vs. considered
      review) — document the tradeoff either way.
      → genuinely separate table (see `design.md` §meal_reaction vs meal_review).
      `meal_reaction.sentiment` stays SMALLINT -2..2 (quick, directional).
      `meal_review.rating` is TINYINT 1..5 (considered, post-meal). Two scales for
      two concerns; widening would require migrating every existing reaction row.
- [x] 3.2 A person should be able to review the actual meal instance, not the recipe directly —
      confirm `MealReview` FKs to `meal_event`, not to `recipe_ref`/future recipe revision.
      → FK to `meal_event.id`. Recipe is reached transitively via `meal_event.mealie_recipe_id`.
      This is what PLAN.md intends: "one person's review of one specific `meal_event`".
- [x] 3.3 Design recipe-level rating aggregation: use reviews to derive recipe-level aggregates
      per PLAN.md — decide the aggregation function (mean, weighted by `person.weight`,
      most-recent-N) and whether it's computed on read or cached, and if cached, its
      invalidation trigger.
      → read-side computed: `AVG(rating)` across all `MealReview` rows for events whose
      `mealie_recipe_id` matches, weighted by `person.weight`. No cached column.
      Invalidated implicitly (it's a view, not stored). The query is simple enough
      that read-time cost is negligible; caching would add invalidation complexity
      for no benefit at current data volumes.
- [x] 3.4 Decide how aggregation interacts with recipe identity once
      `implement-recipe-family-and-revisions` lands (aggregate at variant level? family level?
      revision level?) — flag as a known follow-up rather than solving here.
      → Follow-up for `implement-recipe-family-and-revisions`. Current `meal_event.mealie_recipe_id`
      is a flat reference; when recipe identity becomes variant/family/revision, the
      aggregation query's JOIN target moves one level down the hierarchy. No schema
      change needed here — the aggregate is computed, not stored, so the target is
      a query parameter, not a column.

## 4. Favorites

- [x] 4.1 Do not use a global recipe boolean — favorites SHALL be person/household-specific per
      PLAN.md; design `Favorite` as a person-scoped (and/or household-scoped — decide which,
      or both) explicit marker on a recipe.
      → dual-scope: `(person_id, mealie_recipe_id)` and `(household_id, mealie_recipe_id)`
      are separate rows in the same `favorite` table. A recipe can be both a household
      favorite and a personal favorite (or one without the other). See §5.2 schema.
- [x] 4.2 Investigate and confirm the distinction PLAN.md calls "desirable": favorites are
      explicit preferences (a deliberate act), ratings are observations from actual meals (a
      byproduct of eating) — ensure no code path derives a `Favorite` automatically from a high
      `MealReview` average.
      → confirmed invariant: no code path creates or removes a `Favorite` from `MealReview`
      history. The spec requirement "Favorites are explicit and never derived from ratings"
      enforces this. Domain types are separate; the persistence layer has no trigger or
      stored-procedure logic that could accidentally derive one from the other.
- [x] 4.3 Decide favorite scope semantics: can a household have a household-level favorite
      distinct from an individual's personal favorite, and if both exist, how do they interact
      in recommendation input (a later Epic's concern, but the schema must not foreclose it).
      → schema supports both scopes independently. Recommendation input (a later Epic)
      will decide how to combine them; the schema does not force a precedence rule.

## 5. Persistence

- [x] 5.1 For every new table (`meal_participant`, `meal_review`, `favorite`) answer PLAN.md's
      Database Review Questions: domain concept, owner, mutator, mutability, history
      requirement, lifecycle, deletion behavior, uniqueness constraints, indexing, FK-ability.
      → answered in `design.md` §Database Review Questions (3 tables × 11 questions).
- [x] 5.2 Write the additive migration extending `0001_init.sql`/`establish-household-and-catalog`'s
      migration — FK `meal_participant`/`meal_review` to `Person` (post-household model), not
      the bare `person` table's old shape if it changed underneath.
      → `migrations/0010_meals_and_preferences.sql`: `meal_participant`, `meal_review`,
      `favorite`, plus `meal_event.meal_plan_id` nullable FK. All FKs target existing
      tables (`person`, `household`, `meal_event`, `meal_plan`, `recipe_ref`).
- [x] 5.3 Add Go domain types in `internal/domain` for MealParticipant, MealReview, Favorite,
      and a recipe-level rating aggregate read model.
      → `internal/domain/meal_history.go`: `MealParticipant`, `MealReview`, `FavoriteRating`,
      `Favorite` with `IsPersonScoped`/`IsHouseholdScoped` helpers.
      Persistence types and methods in `internal/persistence/meals.go`:
      `AddMealParticipant`, `ListMealParticipants`, `UpsertMealReview`,
      `ListMealReviews`, `GetRecipeRating`, `UpsertFavorite`, `DeleteFavorite`,
      `ListFavoritesForRecipe`.

## 6. Verification

- [x] 6.1 `openspec validate implement-meals-and-preferences` passes.
      → validated; also `food-brain-first-slice` and `establish-enforced-go-architecture`
      still valid.
- [x] 6.2 Confirm no regression to `internal/scoring`'s existing `meal_event`/`meal_reaction`
      consumption (no renames of those tables/columns in this change).
      → no renames; `meal_event` and `meal_reaction` schemas unchanged. `meal_event`
      gained a nullable `meal_plan_id` column (additive, backward-compatible). Scorer
      continues to consume `domain.RecentMeal` (unchanged).
- [x] 6.3 Unit tests for recipe-level aggregation and for the favorite/rating independence
      invariant (a favorite persists even if all reviews for that recipe are low).
      → `internal/persistence/meals_test.go`:
      `TestMealsAndPreferences_ParticipantAndReviewRoundTrip` — attendance + review upsert;
      `TestMealsAndPreferences_RecipeRatingAggregation` — weighted avg across events/people;
      `TestMealsAndPreferences_FavoriteSurvivesLowReviews` — explicit favorite persists
      despite low aggregate rating;
      `TestMealsAndPreferences_FavoriteScopeInvariant` — dual scope (person + household)
      on same recipe, delete one without affecting the other.
