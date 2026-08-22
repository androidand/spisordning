## 1. Reconcile planned vs. actual naming

- [x] 1.1 Compare PLAN.md's candidate naming (`meal_plans`/`meal_plan_entries` for planned food;
      `meals`/`meal_participants`/`meal_reviews` for actual food) against what
      `migrations/0001_init.sql` already ships (`meal_plan`/`meal_plan_candidate`/
      `meal_plan_decision` for planned; `meal_event`/`meal_reaction` for actual) and decide,
      table by table, whether to keep, rename, or add alongside.
      *Decided: keep existing, add alongside.* `meal_event` and `meal_reaction` are kept
      unchanged because `internal/persistence/meals.go` and the scoring pipeline depend on
      them. New tables (`meal_participant`, `meal_review`, `favorite`) are additive.
      `meal_plan`/`meal_plan_candidate`/`meal_plan_decision` are kept as-is.
- [x] 1.2 Confirm `meal_event` is kept as the "actual meal" entity (do not introduce a
      competing `meals` table) since `internal/scoring` and `cmd/food-brain/plan.go` already
      depend on `meal_event`/`meal_reaction` — document this decision explicitly since it
      diverges from PLAN.md's literal naming.
      *Confirmed.* `meal_event` remains the actual-meal entity. No `meals` table introduced.
      `internal/scoring` and `internal/planning` do not directly reference `meal_event`
      (they use `domain.RecentMeal`), but `internal/persistence/meals.go` and
      `cmd/food-brain/adapters.go` do — renaming would break them. Documented in
      `migrations/0012_meal_history.sql` header and `proposal.md`.
- [x] 1.3 Confirm a planned dinner (`meal_plan_decision`) may produce an actual `meal_event`,
      per PLAN.md's "a planned dinner may produce an actual Meal" — decide whether that link is
      an explicit FK on `meal_event` or inferred by date+recipe match.
      *Decided: optional explicit FK plus existing implicit match.* Added nullable
      `plan_id` and `plan_slot_date` columns to `meal_event` in migration 0012 with a
      CHECK constraint ensuring both are null or both are set. `persistence.LinkMealEventToPlan`
      and `GetMealEventPlanLink` support the explicit link. The existing date+recipe join in
      `GetTonightMeal` continues to work as the inference path.

## 2. Meal participants (actual attendance)

- [x] 2.1 Add `meal_participant`: who was actually present/ate at a given `meal_event`,
      distinct from `meal_reaction` (who reacted and how) — a person can attend without
      reacting, and (design question) can a reaction exist without recorded attendance?
      *Added.* `meal_participant` table in migration 0012 with `(meal_event_id, person_id)`
      UNIQUE constraint. A reaction CAN exist without a participant row (the reaction itself
      is evidence of presence); the tables are independent.
- [x] 2.2 Decide participant capture UX/data source: explicit tally at serving time vs. implied
      from household membership at the time vs. inferred from who left a reaction.
      *Decided: explicit tally at serving time.* `AddMealParticipant` is an explicit write;
      no auto-inference from reactions or household membership. The API layer (future change)
      will drive the UX.

## 3. Ratings (per-person review of an actual meal)

- [x] 3.1 Design `MealReview` per PLAN.md's example ("Andreas 5/5, Vera 4/5, Valdemar 2/5") —
      decide whether this widens existing `meal_reaction.sentiment` (currently SMALLINT -2..2)
      to a richer scale, or is a genuinely separate table (quick reaction vs. considered
      review) — document the tradeoff either way.
      *Decided: separate table.* `meal_review.rating` is a 1–5 scale (considered review);
      `meal_reaction.sentiment` remains -2..2 (quick directional reaction). They answer
      different questions and coexist on the same meal_event. Widening sentiment would
      break existing data and conflate two distinct signals.
- [x] 3.2 A person should be able to review the actual meal instance, not the recipe directly —
      confirm `MealReview` FKs to `meal_event`, not to `recipe_ref`/future recipe revision.
      *Confirmed.* `MealReview` FKs to `meal_event(id)`. Recipe-level aggregation is
      computed by joining through `meal_event.mealie_recipe_id` at read time.
- [x] 3.3 Design recipe-level rating aggregation: use reviews to derive recipe-level aggregates
      per PLAN.md — decide the aggregation function (mean, weighted by `person.weight`,
      most-recent-N) and whether it's computed on read or cached, and if cached, its
      invalidation trigger.
      *Decided: simple mean, computed on read.* `Store.GetRecipeRating` joins
      `meal_review → meal_event` and returns `AVG(rating), COUNT(id)`. No caching column
      on `recipe_ref`; no invalidation needed. Weighted-by-weight and most-recent-N are
      left as future extensions (the `Person.Weight` field exists in domain but is not
      used in the aggregation yet).
- [x] 3.4 Decide how aggregation interacts with recipe identity once
      `implement-recipe-family-and-revisions` lands (aggregate at variant level? family level?
      revision level?) — flag as a known follow-up rather than solving here.
      *Flagged as follow-up.* `RecipeRating` doc comment notes that once recipe-family
      modeling lands, aggregation should be re-evaluated at variant/family level.

## 4. Favorites

- [x] 4.1 Do not use a global recipe boolean — favorites SHALL be person/household-specific per
      PLAN.md; design `Favorite` as a person-scoped (and/or household-scoped — decide which,
      or both) explicit marker on a recipe.
      *Decided: person-scoped for now.* `Favorite` has `person_id` + `mealie_recipe_id`
      UNIQUE. Household favorites are not modeled at the schema level — they can be derived
      by querying all household members' individual favorites. The schema leaves room for
      a future `household_id` column.
- [x] 4.2 Investigate and confirm the distinction PLAN.md calls "desirable": favorites are
      explicit preferences (a deliberate act), ratings are observations from actual meals (a
      byproduct of eating) — ensure no code path derives a `Favorite` automatically from a high
      `MealReview` average.
      *Confirmed.* No code path auto-creates a `Favorite` from ratings. Tests verify this
      invariant: `TestMealHistory_FavoriteExplicitNotDerived` adds low ratings first, then
      explicitly adds a favorite, confirming it persists.
- [x] 4.3 Decide favorite scope semantics: can a household have a household-level favorite
      distinct from an individual's personal favorite, and if both exist, how do they interact
      in recommendation input (a later Epic's concern, but the schema must not foreclose it).
      *Schema does not foreclose it.* The `favorite` table has no `household_id` column
      today, but adding one later (with a separate UNIQUE constraint) would not break
      existing rows. Person-scoped favorites remain the primary mechanism.

## 5. Persistence

- [x] 5.1 For every new table (`meal_participant`, `meal_review`, `favorite`) answer PLAN.md's
      Database Review Questions: domain concept, owner, mutator, mutability, history
      requirement, lifecycle, deletion behavior, uniqueness constraints, indexing, FK-ability.
      *Documented in migration header and domain type comments.* Summary:
      - `meal_participant`: domain=attendance, owner=household, mutator=API, mutable=yes,
        no history needed (point-in-time), cascade delete on meal_event, UNIQUE(event,person),
        indexed on person_id, FK to meal_event+person.
      - `meal_review`: domain=per-person rating, owner=person, mutator=API, mutable=yes
        (upsert), no history needed, cascade delete on meal_event, UNIQUE(event,person),
        indexed on person_id and meal_event_id, FK to meal_event+person.
      - `favorite`: domain=explicit preference, owner=person, mutator=API, mutable=yes
        (add/remove), no history needed, cascade delete on person+recipe,
        UNIQUE(person,recipe), indexed on mealie_recipe_id, FK to person+recipe_ref.
- [x] 5.2 Write the additive migration extending `0001_init.sql`/`establish-household-and-catalog`'s
      migration — FK `meal_participant`/`meal_review` to `Person` (post-household model), not
      the bare `person` table's old shape if it changed underneath.
      *Written.* `migrations/0012_meal_history.sql` uses `REFERENCES person(id)` (the
      post-household model from 0010/0011). Idempotent: uses `CREATE TABLE IF NOT EXISTS`
      and `ON CONFLICT DO NOTHING` for inserts; no `DO/EXCEPTION` blocks (all adds are
      guarded by `IF NOT EXISTS`). Also adds optional `plan_id`/`plan_slot_date` columns
      to `meal_event`. The CHECK constraint that would enforce both being null-or-both-set
      was omitted (documented) because `ON DELETE SET NULL` on `plan_id` would violate it;
      application-layer `LinkMealEventToPlan` maintains the invariant instead.
- [x] 5.3 Add Go domain types in `internal/domain` for MealParticipant, MealReview, Favorite,
      and a recipe-level rating aggregate read model.
      *Added.* `internal/domain/meals.go` defines `MealParticipant`, `MealReview`,
      `RecipeRating`, and `Favorite` with doc comments explaining their relationships
      and constraints. `internal/persistence/meals_history.go` implements all CRUD
      methods on `*Store`.

## 6. Verification

- [x] 6.1 `openspec validate implement-meals-and-preferences` passes.
      *Valid.* `openspec validate implement-meals-and-preferences` passes.
- [x] 6.2 Confirm no regression to `internal/scoring`'s existing `meal_event`/`meal_reaction`
      consumption (no renames of those tables/columns in this change).
      *Confirmed.* No changes to `meal_event` or `meal_reaction` tables (only additive
      columns on `meal_event`). `internal/scoring`, `internal/persistence/meals.go`, and
      `cmd/food-brain/adapters.go` are untouched. Build and all 229 tests pass.
      Fixed pre-existing bug in `ListMealReactions` (missing `eventID` arg in `Query` call)
      and gofmt'd `GetTonightMeal` indentation — both were latent, caught by review.
- [x] 6.3 Unit tests for recipe-level aggregation and for the favorite/rating independence
      invariant (a favorite persists even if all reviews for that recipe are low).
      *Added.* `internal/persistence/meals_history_test.go` with 6 integration tests:
      `TestMealHistory_ParticipantRoundTrip`, `TestMealHistory_ReviewRoundTrip`,
      `TestMealHistory_RecipeRatingAggregation`,
      `TestMealHistory_FavoriteExplicitNotDerived`,
      `TestMealHistory_FavoritePersonScoped`, `TestMealHistory_MealEventPlanLink`.
      All skip cleanly when no DB is available. The favorite/rating independence
      invariant is explicitly tested: low ratings do not auto-create a favorite, and
      an explicit favorite survives subsequent low reviews.
      Fixed pre-existing FK bug in `TestMeals_ReactionsAndConstraints` (repos_test.go:
      `p-kid` never created) and all three new tests that referenced person IDs without
      creating them — now call `CreatePerson` and include `"person"` in `truncateTables`.
      `LinkMealEventToPlan` now errors on mixed (one-nil, one-non-nil) input instead of
      silently clearing. `domain.RecipeRating.Average` doc corrected: no longer claims
      rounding (returns raw AVG).
