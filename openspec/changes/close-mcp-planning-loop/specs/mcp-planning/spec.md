# mcp-planning (delta)

## ADDED Requirements

### Requirement: An MCP client can persist a weekly meal plan

The system SHALL expose a `persist_plan` MCP tool that runs the weekly meal planner and persists the result to the `meal_plan` database tables (`meal_plan`, `meal_plan_candidate`, `meal_plan_decision`, `shopping_requirement`). The tool SHALL accept a `week_start` date and `days` count, and SHALL return the persisted plan's ID.

#### Scenario: Persisting a plan for next week succeeds

- **WHEN** an MCP client calls `persist_plan` with `week_start` set to next Monday and `days` set to 7
- **THEN** the planner generates candidates for each slot, scores them, and persists the result
- **AND** the response includes a non-empty `plan_id` and `week_start` matching the input

#### Scenario: Persisting a plan without an explicit week_start defaults to next Monday

- **WHEN** an MCP client calls `persist_plan` with `week_start` omitted
- **THEN** the system computes next Monday as the default week start
- **AND** the persisted plan's `week_start` is that computed date

#### Scenario: Persisting a plan with no recipes available fails gracefully

- **WHEN** the mealie client has zero recipes synced
- **THEN** `persist_plan` returns an error indicating that no recipes are available
- **AND** no partial plan is written to the database

### Requirement: An MCP client can read back a persisted plan

The system SHALL expose a `get_plan` MCP tool that returns the full view of a meal plan — its metadata, all candidates (ranked per slot), all decisions, and all shopping requirements — in a single response.

#### Scenario: Reading a plan returns candidates and decisions

- **WHEN** an MCP client calls `get_plan` with a valid `plan_id`
- **THEN** the response includes the plan's `week_start`, `status`, a list of candidates ordered by date and slot, and a list of decisions
- **AND** each candidate includes its `mealie_recipe_id`, `title`, `score`, `rank`, and `feasible` flag

#### Scenario: Reading a nonexistent plan returns an error

- **WHEN** an MCP client calls `get_plan` with a `plan_id` that does not exist
- **THEN** the tool returns an error indicating the plan was not found
- **AND** no partial or fabricated data is returned

### Requirement: An MCP client can record per-slot decisions on a plan

The system SHALL expose a `set_plan_decision` MCP tool that records accept/swap decisions for one or more slots on a plan. A decision is identified by `slot_date` and `slot_kind`, and specifies the chosen `mealie_recipe_id`.

#### Scenario: Accepting the planner's top choice for a slot

- **WHEN** an MCP client calls `set_plan_decision` with `slot_date`, `slot_kind`, and the `mealie_recipe_id` of the top-ranked candidate
- **THEN** the decision is persisted as an upsert on the (plan_id, slot_date, slot_kind) composite key
- **AND** the response includes the persisted decision with its `decided_at` timestamp

#### Scenario: Swapping a slot to a different recipe

- **WHEN** an MCP client calls `set_plan_decision` with `slot_date`, `slot_kind`, and a `mealie_recipe_id` that differs from the planner's top choice
- **THEN** the new recipe is recorded as the decision for that slot
- **AND** the decision replaces any previous decision for the same (plan_id, slot_date, slot_kind)

#### Scenario: Setting decisions on a draft plan fails

- **WHEN** an MCP client calls `set_plan_decision` for a plan whose status is `draft`
- **THEN** the tool returns an error indicating that decisions can only be set on an approved plan
- **AND** no decisions are written

### Requirement: An MCP client can record a meal event linked to a plan slot

The system SHALL expose a `record_meal_from_plan` MCP tool that records a meal event with a link to the originating plan slot. The tool accepts `plan_id` and `plan_slot_date` alongside the standard reaction fields (`recipe`, `served_on`, `person_id`, `sentiment`).

#### Scenario: Recording a meal linked to its plan slot

- **WHEN** an MCP client calls `record_meal_from_plan` with `plan_id`, `plan_slot_date`, `recipe`, `served_on`, `person_id`, and `sentiment`
- **THEN** a `meal_event` row is created with `meal_plan_id` and `meal_plan_slot_date` set to the plan link
- **AND** a `meal_reaction` row is created with the person's sentiment
- **AND** the response includes the created `meal_event_id`

#### Scenario: Recording a meal without a plan link works as before

- **WHEN** an MCP client calls `record_meal_from_plan` without `plan_id` or `plan_slot_date`
- **THEN** a `meal_event` row is created with `meal_plan_id` and `meal_plan_slot_date` as null (ad-hoc meal)
- **AND** the behavior is identical to the existing `record_meal_reaction` tool

#### Scenario: A plan_id without plan_slot_date is rejected

- **WHEN** an MCP client calls `record_meal_from_plan` with `plan_id` set but `plan_slot_date` omitted
- **THEN** the tool returns an error indicating that `plan_slot_date` is required when `plan_id` is set
- **AND** no meal event is created

### Requirement: An MCP client can list all meal plans

The system SHALL expose a `list_plans` MCP tool that returns a summary of all meal plans — their IDs, week starts, statuses, and creation dates — ordered by week start descending.

#### Scenario: Listing plans returns all plans

- **WHEN** an MCP client calls `list_plans`
- **THEN** the response is a list of plan summaries ordered by `week_start` descending
- **AND** each summary includes `id`, `week_start`, `status`, and `created_at`

#### Scenario: Listing plans when none exist returns an empty list

- **WHEN** no meal plans exist in the database
- **THEN** `list_plans` returns an empty array
- **AND** the tool call succeeds (does not error)
