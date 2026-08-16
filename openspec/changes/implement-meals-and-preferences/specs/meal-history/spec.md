# meal-history (delta)

## ADDED Requirements

### Requirement: Planned food and actual food remain distinct

The system SHALL keep planned food (`meal_plan`/`meal_plan_candidate`/`meal_plan_decision`)
and actual food (`meal_event`/`meal_participant`/`meal_reaction`/`meal_review`) as distinct
entities. A `meal_plan_decision` MAY produce a `meal_event`, but a `meal_event` SHALL be able
to exist without any corresponding plan (an ad-hoc, unplanned meal).

#### Scenario: An unplanned meal is recorded without a plan

- **WHEN** a household cooks something that was never in a `meal_plan`
- **THEN** a `meal_event` can still be created and reviewed
- **AND** no `meal_plan_decision` row is required to exist for it

#### Scenario: A planned dinner produces an actual meal

- **WHEN** a `meal_plan_decision` for a slot is acted on and the meal is cooked
- **THEN** a `meal_event` is created that can be linked back to that decision
- **AND** the `meal_plan_decision` row itself is not mutated to represent the actual outcome

### Requirement: Favorites are person/household-scoped, not global

A `Favorite` SHALL be scoped to a person or a household — never a global boolean on a recipe
independent of who holds the preference.

#### Scenario: Different household members favorite different recipes

- **WHEN** one person favorites a recipe and another does not
- **THEN** the recipe is a favorite for the first person only
- **AND** no single global "is favorite" flag exists on the recipe

### Requirement: Favorites are explicit and never derived from ratings

A `Favorite` SHALL only be created by an explicit action. The system SHALL NOT automatically
create, remove, or infer a `Favorite` from `MealReview` history or aggregate ratings.

#### Scenario: A high average rating does not create a favorite

- **WHEN** a recipe accumulates a high average `MealReview` score
- **THEN** no `Favorite` row is created automatically for any person as a result

#### Scenario: A favorite survives a run of poor reviews

- **WHEN** a person has favorited a recipe and then leaves several low `MealReview` scores for
  meals made from it
- **THEN** the `Favorite` row remains unless explicitly removed

### Requirement: A meal review is per-person, per-meal-instance

A `MealReview` SHALL record one person's review of one specific `meal_event`, not of a recipe
directly. Recipe-level rating is an aggregate derived from these instance-level reviews.

#### Scenario: Different people rate the same meal instance differently

- **WHEN** a meal is served to three people
- **THEN** each person may leave their own `MealReview` for that `meal_event`
- **AND** the reviews can differ (e.g. 5/5, 4/5, 2/5) without conflict

#### Scenario: Recipe-level rating is an aggregate, not a stored opinion

- **WHEN** a recipe's aggregate rating is requested
- **THEN** it is computed from the `MealReview` rows across that recipe's `meal_event`
  instances
- **AND** no single `MealReview` row is treated as the recipe's rating
