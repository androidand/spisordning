# recipe-availability (delta)

## ADDED Requirements

### Requirement: Recipe feasibility is a tri-state verdict computed against current inventory

The system SHALL classify a recipe's feasibility against a household's current inventory as one
of `feasible`, `feasible-with-substitution`, or `infeasible`, computed per ingredient line from
`InventoryLot` state and `IngredientSubstitution` rules, and aggregated to a recipe-level
verdict.

#### Scenario: All ingredients on hand in the required form

- **WHEN** every ingredient line of a recipe matches an on-hand `InventoryLot` in the required
  form with sufficient quantity
- **THEN** the recipe-level verdict is `feasible`

#### Scenario: A missing ingredient has an acceptable substitute on hand

- **WHEN** one ingredient line has no direct on-hand match, but an `IngredientSubstitution`
  resolves it to a product that is on hand in sufficient (ratio-adjusted) quantity
- **THEN** that line's verdict is satisfied-via-substitution, naming the substitution's category
- **AND** the recipe-level verdict is `feasible-with-substitution`

#### Scenario: A missing ingredient has no substitute

- **WHEN** an ingredient line has no on-hand match and no `IngredientSubstitution` resolves it
- **THEN** that line's verdict is unmet
- **AND** the recipe-level verdict is `infeasible`

### Requirement: Feasibility is explainable per ingredient line

The system SHALL report, for every ingredient line, a machine-readable reason: satisfied
on-hand, satisfied via substitution (naming the tier used), or unmet. The system SHALL NOT
report a recipe-level verdict without the per-line reasons that produced it.

#### Scenario: A verdict is traceable to its per-line reasons

- **WHEN** a recipe's feasibility is computed
- **THEN** the result includes, for every ingredient line, whether it was on-hand, substituted
  (and which category), or unmet
- **AND** the recipe-level verdict is derivable from those per-line results by the stated
  aggregation rule

### Requirement: Substitution tiers are consumed, not redefined

This capability SHALL reuse the `EQUIVALENT`/`GOOD`/`ACCEPTABLE`/`FORM`/`DIETARY`/`EMERGENCY`
substitution model and explicit quantity ratios owned by the ingredient-catalog capability. It
SHALL NOT define a parallel substitution taxonomy or assume a substitution ratio of 1:1.

#### Scenario: A substitution's explicit ratio is applied, not assumed

- **WHEN** a recipe requires 100g of an ingredient and the closest substitute is defined with a
  0.33 ratio
- **THEN** the system computes the substitute quantity needed using that ratio, not a 1:1
  assumption
- **AND** if the on-hand substitute quantity is insufficient at that ratio, the line is reported
  as unmet or partially satisfied, not silently satisfied

### Requirement: Low-confidence inventory does not silently count as available

An `InventoryLot` with `UNKNOWN` confidence SHALL NOT be treated as satisfying an ingredient
requirement without that fact being surfaced in the per-line reason.

#### Scenario: An UNKNOWN-confidence lot is flagged, not silently trusted

- **WHEN** the only on-hand match for an ingredient line is a lot with `UNKNOWN` confidence
- **THEN** the per-line reason states that the match is uncertain
- **AND** the recipe-level verdict does not present this as equivalent to a confidently
  on-hand match
