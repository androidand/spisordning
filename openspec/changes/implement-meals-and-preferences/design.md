# Naming reconciliation: PLAN.md vs migrations/0001_init.sql

## Context

`PLAN.md`'s Candidate section proposes the following table names:

```
meal_plans
meal_plan_entries

meals
meal_participants
meal_reviews
```

`migrations/0001_init.sql` (shipped with `food-brain-first-slice`) already implements
a planned-vs-actual split under these names:

```
meal_plan
meal_plan_candidate
meal_plan_decision
meal_event
meal_reaction
```

This change reconciles the two without breaking the existing scorer's consumption of
`meal_event`/`meal_reaction`.

## Decision: table by table

### Planned food

| PLAN.md name       | Existing name             | Decision | Rationale                                                                 |
|--------------------|---------------------------|----------|---------------------------------------------------------------------------|
| `meal_plans`       | `meal_plan`               | **Keep** | Singular matches the existing schema; `meal_plan` is already consumed by `cmd/food-brain/plan.go` and `internal/persistence/meal_plan.go`. Renaming would require a migration, a domain-type rename, and a persistence rewrite for no gain. |
| `meal_plan_entries`| `meal_plan_candidate`     | **Keep** | `candidate` is more accurate — these rows are ranked candidates the scorer considers, not the entries that become decisions. `meal_plan_decision` already captures the "chosen entry" semantics. |
| (none proposed)    | `meal_plan_decision`      | **Keep** | PLAN.md omits this table; the existing one fills the gap between "considered" and "chosen" and is already wired into the planner. |

### Actual food

| PLAN.md name       | Existing name       | Decision | Rationale                                                                 |
|--------------------|---------------------|----------|---------------------------------------------------------------------------|
| `meals`            | `meal_event`        | **Keep** | `meal_event` is already consumed by `internal/scoring` (repetition penalty, `RecentMeal`) and `cmd/food-brain/adapters.go` (insert). Renaming to `meals` would be a breaking schema change with no semantic improvement — an event is a one-time occurrence, not a recurring entity. |
| (none proposed)    | `meal_reaction`     | **Keep** | Existing coarse sentiment (-2..2) with note. This change adds `meal_review` alongside it (see below) rather than replacing it. |
| `meal_participants`| (none)              | **Add**  | New table for attendance. Distinct from `meal_reaction` — a person can attend without reacting, and (see §2.1) a reaction may exist without recorded attendance. |
| `meal_reviews`     | (none)              | **Add**  | New table for per-person, per-meal-instance reviews (e.g. "Andreas 5/5, Vera 4/5, Valdemar 2/5"). Sibling of `meal_reaction`, not a replacement. |

## Decision: link between plan and actual meal (§1.3)

Per PLAN.md: "a planned dinner may produce an actual Meal."

The link is modelled as a **composite explicit FK**
`meal_event.(meal_plan_id, meal_plan_slot_date)` →
`meal_plan_decision(plan_id, slot_date)`, nullable (unplanned meals have no
link). Both columns must be nil together; PostgreSQL skips the FK check when
any column of a composite FK is NULL. Date+recipe match alone is too fragile
— two different plans could decide on the same recipe on the same day — so
the composite FK to the decision's PK is the right model.

## Decision: meal_reaction vs meal_review (§3.1)

Two separate tables, not a widened column:

- `meal_reaction.sentiment` (SMALLINT -2..2): a quick, directional reaction at serving
  time. Low cognitive load, captured in the moment. Already consumed by
  `preference_observation` seeding and `internal/scoring`'s repetition penalty.
- `meal_review.rating` (TINYINT 1..5): a considered, post-meal review. Higher cognitive
  load, captured after the meal. Drives recipe-level aggregate rating.

The scales are deliberately different so they capture genuinely different concerns.
Widening `sentiment` to 1..5 would conflate the two and force a data migration on the
existing rows.

## Database Review Questions (§5.1)

### meal_participant

| Question | Answer |
|---|---|
| What domain concept? | Attendance: who was actually present/ate at a `meal_event`. |
| Who owns it? | The `meal_event` aggregate (side-car, not the root). |
| Who may mutate it? | The person serving/cooking the meal, or any household member logging attendance. |
| Is it mutable? | Yes — people change their minds about whether they ate something. |
| Does it require history? | No. A current attendance ledger is sufficient; no need to track "was present yesterday but not today" as separate rows. |
| Lifecycle? | Created when the meal is logged; can be deleted if the meal is removed. |
| Deletion behavior? | Cascading: if the `meal_event` is deleted, participant rows go with it. If a `person` is deleted, their participant rows are orphaned (handled by ON DELETE SET NULL or CASCADE on the FK). |
| Uniqueness constraints? | UNIQUE `(meal_event_id, person_id)` — one attendance record per person per meal. Mirrors the constraint on `meal_reaction`. |
| Indexing? | Index on `(meal_event_id)` for listing attendees; index on `(person_id)` for "what meals did this person attend". |
| FK-ability? | FK to `meal_event.id` (CASCADE), FK to `person.id` (CASCADE). |
| JSON? | No — two clean FK columns. |

### meal_review

| Question | Answer |
|---|---|
| What domain concept? | A person's considered rating of a specific `meal_event` instance (1–5 stars). |
| Who owns it? | The `person` aggregate (their opinion about a meal). |
| Who may mutate it? | The person who left the review (or a household admin on their behalf). |
| Is it mutable? | Yes — ratings can be updated. |
| Does it require history? | No — the current rating is what matters for aggregation. Old ratings are overwritten, not archived. |
| Lifecycle? | Created when the person submits a review; deleted when the meal is removed. |
| Deletion behavior? | Cascading on `meal_event` delete. Person delete cascades. |
| Uniqueness constraints? | UNIQUE `(meal_event_id, person_id)` — one review per person per meal. |
| Indexing? | Index on `(mealie_recipe_id)` via a computed/joined query; index on `(person_id)` for "this person's review history". |
| FK-ability? | FK to `meal_event.id` (CASCADE), FK to `person.id` (CASCADE). No direct FK to `recipe_ref` — recipe is reached transitively. |
| JSON? | No — scalar rating + optional note. |

### favorite

| Question | Answer |
|---|---|
| What domain concept? | An explicit, person- or household-scoped preference marker on a recipe. |
| Who owns it? | The scoping aggregate: `person` or `household`. |
| Who may mutate it? | The person (for person-scoped) or a household admin (for household-scoped). |
| Is it mutable? | Yes — can be added or removed at any time. |
| Does it require history? | No — current favorite state is all that matters. |
| Lifecycle? | Created by explicit action; removed by explicit action. |
| Deletion behavior? | Person delete cascades. Household delete cascades. Recipe delete is RESTRICT (default FK behavior) — a favorite referencing a deleted recipe blocks the delete; cleanup is a separate concern. |
| Uniqueness constraints? | UNIQUE `(person_id, mealie_recipe_id)` and UNIQUE `(household_id, mealie_recipe_id)` — a person can't favorite the same recipe twice; same for household. |
| Indexing? | Index on `(mealie_recipe_id)` for "who favorited this recipe". |
| FK-ability? | Nullable FK to `person.id` (CASCADE), nullable FK to `household.id` (CASCADE), RESTRICT FK to `recipe_ref.mealie_recipe_id` (default). CHECK constraint: exactly one of `person_id`, `household_id` is non-NULL. |
| JSON? | No — two nullable FK columns with a CHECK constraint is the right model here. |
