# meal-planning (delta)

## ADDED Requirements

### Requirement: Durable family food model

The system SHALL persist, in its own PostgreSQL database, the domains no integrated app owns:
people, per-person preferences with confidence, preference observations, meal events, meal
reactions, effort profiles, planning constraints, plan candidates, plan decisions, ingredient
mappings, and canonical shopping requirements. It SHALL NOT store authoritative copies of
recipe content; recipes are referenced by Mealie id with an optional cached snapshot.

#### Scenario: Recipe stored as a reference, not a copy

- **WHEN** a Mealie recipe is synced into Food Brain
- **THEN** the stored record contains `mealie_recipe_id`, normalized ingredient references,
  `last_synced_at`, and a `raw_mealie_snapshot`
- **AND** Mealie remains the source of truth for the recipe's instructions and ingredients

#### Scenario: A meal reaction updates preference confidence

- **WHEN** a family member records a reaction to a served meal
- **THEN** a `meal_reaction` is persisted linked to the `meal_event` and person
- **AND** the affected person's preference confidence is updated from that observation

### Requirement: Deterministic, testable suggestion scoring

The system SHALL produce ranked dinner candidates from a deterministic scorer accounting for
per-person preferences, the day's effort budget, a repetition penalty over recent meal events,
Skolmaten school-lunch dedup, and Willys campaign bias. The scoring SHALL be reproducible and
assertable without any LLM available.

#### Scenario: Plan is reproducible without the LLM

- **WHEN** the scorer runs over a fixed input set with Olla unavailable
- **THEN** it returns a deterministic ranked candidate list
- **AND** the ranking is identical across repeated runs with the same input

#### Scenario: School lunch is not echoed at dinner

- **WHEN** Skolmaten reports a dish served at school on a given day
- **THEN** candidates equivalent to that dish are penalized for that day's dinner

#### Scenario: Campaign items rank up

- **WHEN** an ingredient of a candidate meal is on campaign at the family's active store
- **THEN** that candidate's score increases relative to an otherwise-equal non-campaign meal

### Requirement: LLM is additive, never load-bearing

The system SHALL treat the local LLM (Olla) as additive: it MAY use the LLM to vary candidates
within constraints and to generate human-readable explanations, but the LLM MUST NOT determine
feasibility or override the deterministic scorer.

#### Scenario: LLM output cannot introduce an infeasible meal

- **WHEN** the LLM proposes a variation that violates a hard planning constraint
- **THEN** the system rejects that variation before it enters the plan

### Requirement: Canonical shopping requirements

The system SHALL emit the plan's shopping needs as retailer-independent requirements of the
form `{ ingredientId, quantity, unit, acceptableForms[], preferredForm }`, with no retailer
product identifiers embedded.

#### Scenario: Requirement carries no retailer id

- **WHEN** an approved weekly plan is converted to shopping requirements
- **THEN** each requirement identifies the ingredient canonically
- **AND** contains no Willys (or other retailer) article number
