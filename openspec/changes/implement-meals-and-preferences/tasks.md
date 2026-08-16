## 1. Reconcile planned vs. actual naming

- [ ] 1.1 Compare PLAN.md's candidate naming (`meal_plans`/`meal_plan_entries` for planned food;
      `meals`/`meal_participants`/`meal_reviews` for actual food) against what
      `migrations/0001_init.sql` already ships (`meal_plan`/`meal_plan_candidate`/
      `meal_plan_decision` for planned; `meal_event`/`meal_reaction` for actual) and decide,
      table by table, whether to keep, rename, or add alongside.
- [ ] 1.2 Confirm `meal_event` is kept as the "actual meal" entity (do not introduce a
      competing `meals` table) since `internal/scoring` and `cmd/food-brain/plan.go` already
      depend on `meal_event`/`meal_reaction` — document this decision explicitly since it
      diverges from PLAN.md's literal naming.
- [ ] 1.3 Confirm a planned dinner (`meal_plan_decision`) may produce an actual `meal_event`,
      per PLAN.md's "a planned dinner may produce an actual Meal" — decide whether that link is
      an explicit FK on `meal_event` or inferred by date+recipe match.

## 2. Meal participants (actual attendance)

- [ ] 2.1 Add `meal_participant`: who was actually present/ate at a given `meal_event`,
      distinct from `meal_reaction` (who reacted and how) — a person can attend without
      reacting, and (design question) can a reaction exist without recorded attendance?
- [ ] 2.2 Decide participant capture UX/data source: explicit tally at serving time vs. implied
      from household membership at the time vs. inferred from who left a reaction.

## 3. Ratings (per-person review of an actual meal)

- [ ] 3.1 Design `MealReview` per PLAN.md's example ("Andreas 5/5, Vera 4/5, Valdemar 2/5") —
      decide whether this widens existing `meal_reaction.sentiment` (currently SMALLINT -2..2)
      to a richer scale, or is a genuinely separate table (quick reaction vs. considered
      review) — document the tradeoff either way.
- [ ] 3.2 A person should be able to review the actual meal instance, not the recipe directly —
      confirm `MealReview` FKs to `meal_event`, not to `recipe_ref`/future recipe revision.
- [ ] 3.3 Design recipe-level rating aggregation: use reviews to derive recipe-level aggregates
      per PLAN.md — decide the aggregation function (mean, weighted by `person.weight`,
      most-recent-N) and whether it's computed on read or cached, and if cached, its
      invalidation trigger.
- [ ] 3.4 Decide how aggregation interacts with recipe identity once
      `implement-recipe-family-and-revisions` lands (aggregate at variant level? family level?
      revision level?) — flag as a known follow-up rather than solving here.

## 4. Favorites

- [ ] 4.1 Do not use a global recipe boolean — favorites SHALL be person/household-specific per
      PLAN.md; design `Favorite` as a person-scoped (and/or household-scoped — decide which,
      or both) explicit marker on a recipe.
- [ ] 4.2 Investigate and confirm the distinction PLAN.md calls "desirable": favorites are
      explicit preferences (a deliberate act), ratings are observations from actual meals (a
      byproduct of eating) — ensure no code path derives a `Favorite` automatically from a high
      `MealReview` average.
- [ ] 4.3 Decide favorite scope semantics: can a household have a household-level favorite
      distinct from an individual's personal favorite, and if both exist, how do they interact
      in recommendation input (a later Epic's concern, but the schema must not foreclose it).

## 5. Persistence

- [ ] 5.1 For every new table (`meal_participant`, `meal_review`, `favorite`) answer PLAN.md's
      Database Review Questions: domain concept, owner, mutator, mutability, history
      requirement, lifecycle, deletion behavior, uniqueness constraints, indexing, FK-ability.
- [ ] 5.2 Write the additive migration extending `0001_init.sql`/`establish-household-and-catalog`'s
      migration — FK `meal_participant`/`meal_review` to `Person` (post-household model), not
      the bare `person` table's old shape if it changed underneath.
- [ ] 5.3 Add Go domain types in `internal/domain` for MealParticipant, MealReview, Favorite,
      and a recipe-level rating aggregate read model.

## 6. Verification

- [ ] 6.1 `openspec validate implement-meals-and-preferences` passes.
- [ ] 6.2 Confirm no regression to `internal/scoring`'s existing `meal_event`/`meal_reaction`
      consumption (no renames of those tables/columns in this change).
- [ ] 6.3 Unit tests for recipe-level aggregation and for the favorite/rating independence
      invariant (a favorite persists even if all reviews for that recipe are low).
