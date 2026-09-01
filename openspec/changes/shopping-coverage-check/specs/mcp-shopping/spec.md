# mcp-shopping (delta)

## ADDED Requirements

### Requirement: An MCP client can check shopping coverage against a plan

The system SHALL expose a `check_shopping_coverage` MCP tool that compares the items of a
filled `shopping_list` against the persisted `shopping_requirement` rows of an `approved`
meal plan, and returns a per-ingredient covered/short/missing report.

The tool SHALL accept a `shopping_list_id` and a `plan_id`, read the list's items grouped
by `(ingredient_id, unit)` and the plan's requirements grouped by `(ingredient_id, unit)`,
and compute a status per required line.

The system SHALL match list items to plan requirements by `ingredient_id` (the canonical
ingredient id, derived identically on both sides by `IngredientIDForName(CanonicalIngredientID(...))`),
NOT by `shopping_requirement_id`, which is NULL on every plan-derived item.

The system SHALL compare against the persisted `shopping_requirement` rows only (already
post-staple, after `PartitionStaples`), so assumed-on-hand staples are never reported as
missing.

#### Scenario: A fully-covered list reports every line covered

- **WHEN** an MCP client calls `check_shopping_coverage` with `plan_id` of an approved plan
  whose requirements are {mjölk 1 l, flour 500 g} and `shopping_list_id` of a list whose
  items sum to {mjölk 1 l, flour 500 g}
- **THEN** the report contains one line per requirement
- **AND** every line has status `covered`
- **AND** `short_count` and `missing_count` are both 0

#### Scenario: A list short on one ingredient reports it as short with the shortfall amount

- **WHEN** an MCP client calls `check_shopping_coverage` for an approved plan requiring
  {mjölk 1 l} and a list that supplies {mjölk 0.5 l}
- **THEN** the report contains a line for mjölk with status `short`
- **AND** the line reports `required = 1`, `supplied = 0.5`, and `shortfall = 0.5`
- **AND** `short_count` is 1

#### Scenario: An ingredient not in the list at all is reported missing

- **WHEN** an MCP client calls `check_shopping_coverage` for an approved plan requiring
  {flour 500 g} and a list that contains no flour line
- **THEN** the report contains a line for flour with status `missing`
- **AND** the line reports `required = 500`, `supplied = 0`

#### Scenario: Items grouped by ingredient and unit are summed

- **WHEN** the plan requires {flour 500 g} and the list has two flour items, 200 g and
  300 g, on the same `(ingredient_id, unit)` key
- **THEN** the two items are summed to a `supplied` of 500
- **AND** the flour line has status `covered`

#### Scenario: Lines that differ only by unit are distinct requirements

- **WHEN** the plan requires {mjölk 1 l} and the list supplies {mjölk 1 kg} (a different unit)
- **THEN** the mjölk line has status `missing` for the required `l` key
- **AND** the tool does not attempt to convert kg to l before comparing

#### Scenario: Assumed-on-hand staples are not reported missing

- **WHEN** the plan's persisted `shopping_requirement` rows exclude staples because
  `PartitionStaples` dropped them (the persisted rows are already post-staple)
- **THEN** no staple appears as `missing` in the coverage report
- **AND** the report reflects only the requirements that were persisted as "buy" lines

#### Scenario: Free-text items are reported as not-plan-derived

- **WHEN** a list item has only a `label` and no `ingredient_id` (e.g. a checklist line
  like "paper towels")
- **THEN** that item is reported separately as `not-plan-derived`
- **AND** it does not satisfy or reduce the shortfall of any requirement

#### Scenario: Coverage is checked against a single explicit plan

- **WHEN** an MCP client calls `check_shopping_coverage` with an explicit `plan_id`
- **THEN** the report is computed against exactly that plan's `shopping_requirement` rows
- **AND** the response includes the plan's requirement count so the caller can tell whether
  the plan was populated

### Requirement: An MCP client gets a machine-readable coverage report

The system SHALL return from `check_shopping_coverage` a `CoverageReport` containing a
`short_count` and a `missing_count`, and a `lines` array in which each entry has an
`ingredient_id`, an `ingredient_name`, a `unit`, a `status` (`covered` / `short` /
`missing`), `required`, `supplied`, and `shortfall` (positive when supplied < required;
zero when covered).

#### Scenario: The report summary aggregates per statuses

- **WHEN** the per-line statuses are {covered, short, missing}
- **THEN** `short_count` is 1 and `missing_count` is 1
- **AND** `lines` has one entry per required line
