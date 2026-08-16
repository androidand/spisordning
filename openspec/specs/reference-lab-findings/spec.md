# reference-lab-findings Specification

## Purpose
TBD - created by archiving change establish-reference-lab. Update Purpose after archive.
## Requirements
### Requirement: Mealie's and Grocy's data models are documented before Spisordning's equivalents are designed

The project SHALL document Mealie's and Grocy's data models, APIs, and observed behavior in
`docs/research/mealie-*.md` and `docs/research/grocy-*.md` before any Spisordning OpenSpec
change proposes the equivalent domain tables (recipe model, ingredient/unit model, inventory
model, meal-planning model, etc.).

#### Scenario: Domain-model work cites reference-lab findings

- **WHEN** an OpenSpec change proposes a Spisordning table or entity that has a direct Mealie or
  Grocy analogue (e.g. recipe ingredients, inventory stock, meal plans)
- **THEN** that change's proposal references the corresponding `docs/research/mealie-*.md` or
  `docs/research/grocy-*.md` finding it is informed by, or explicitly states no reference-lab
  finding exists yet

#### Scenario: Reference-lab documents exist before Phase 3 domain design proceeds

- **WHEN** `docs/domain/` documents (per `PLAN.md`'s "Expected Domain Documents") are drafted
- **THEN** `docs/research/mealie-*.md` and `docs/research/grocy-*.md` already exist and cover at
  minimum recipe model, ingredients/units, inventory/stock, and meal planning

### Requirement: Overlapping capabilities carry an explicit disposition

For every capability both Mealie and Grocy address, the project SHALL record one explicit
disposition — MEALIE, GROCY, MERGE, REDESIGN, DEFER, or OMIT — with a stated concrete reason, in
the Feature Overlap Matrix produced by this change.

#### Scenario: A disposition is never left as "take the best parts"

- **WHEN** the Feature Overlap Matrix records a disposition for an overlapping capability (e.g.
  unit conversion, shopping lists, meal planning)
- **THEN** the entry states which system's approach informed the decision and why
- **AND** the entry does not read as a vague blend of "best parts" without a stated reason

