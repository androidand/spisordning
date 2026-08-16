# recommendations (delta)

## ADDED Requirements

### Requirement: Recommendations are produced by deterministic, explainable logic, never merely an LLM response

The system SHALL produce recommendation candidates and their ranking through deterministic,
testable scoring logic. The system SHALL NOT treat an LLM's output as the source of a
recommendation's ranking, feasibility, or novelty classification.

#### Scenario: Recommendations are reproducible without the LLM

- **WHEN** recommendations are generated for a fixed input set with Olla unavailable
- **THEN** the system returns a deterministic ranked candidate list, including novelty/
  familiarity classification
- **AND** the ranking and classification are identical across repeated runs with the same input

#### Scenario: An LLM cannot reclassify a candidate's novelty

- **WHEN** the LLM proposes a variation or explanation for a candidate
- **THEN** the candidate's novelty/familiarity classification and score remain those computed by
  the deterministic scorer
- **AND** the LLM's output cannot override or replace that classification

### Requirement: A recommendation batch balances known favorites against discovery

The system SHALL classify each recommendation candidate as a known favorite, a discovery/novel
candidate, or neither, using a deterministic rule over meal history and preference data. A
default recommendation batch SHALL include both known-favorite and discovery candidates when the
candidate pool contains both.

#### Scenario: A batch is not all-favorites when novel candidates exist

- **WHEN** a household has both frequently-cooked favorites and never-cooked feasible recipes
  available as candidates
- **THEN** the default recommendation batch includes at least one candidate from each group
- **AND** each candidate's classification is explainable from meal history and preference data

#### Scenario: An all-favorites household still gets a valid batch

- **WHEN** no discovery/novel candidates are available in the pool
- **THEN** the batch is composed entirely of known favorites
- **AND** this is reported honestly rather than the system fabricating a novel candidate

### Requirement: Recommendation control modes are deterministic weight transformations

The system SHALL support the user-facing control modes `safe choice`, `something similar`,
`surprise me`, and `something completely new`, each implemented as a deterministic
transformation of scorer weights and/or candidate-pool filtering over the same underlying
scorer. The system SHALL NOT implement a mode as a separate, independent scoring algorithm.

#### Scenario: Each mode produces a distinct, reproducible transformation

- **WHEN** the same candidate pool is scored under two different control modes
- **THEN** the two modes produce different (or at least independently justifiable) rankings
- **AND** running the same mode twice against the same input produces the identical ranking

#### Scenario: Surprise me still respects hard feasibility constraints

- **WHEN** `surprise me` mode is selected
- **THEN** candidates that are infeasible under the existing scorer's feasibility rule are still
  ranked last or excluded
- **AND** novelty-seeking never overrides a hard feasibility constraint
