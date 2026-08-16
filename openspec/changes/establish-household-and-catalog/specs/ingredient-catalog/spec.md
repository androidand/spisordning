# ingredient-catalog (delta)

## ADDED Requirements

### Requirement: Ingredient is canonical and product-independent

The system SHALL represent `Ingredient` as a canonical semantic foodstuff (e.g. "chicken
breast") distinct from `Product` (e.g. "Garant Kycklingfilé 900g"). An `Ingredient` SHALL NOT
carry a brand, package size, barcode, or retailer identifier.

#### Scenario: Two products map to one ingredient

- **WHEN** "Garant Kycklingfilé 900g" and "ICA Kycklingfilé 500g" are both registered as
  `Product` rows
- **THEN** both map via `ProductIngredientMapping` to the same canonical `Ingredient`
  ("chicken breast")
- **AND** neither product's brand or package size is written onto the `Ingredient` row

#### Scenario: A Product without a resolved mapping is still valid

- **WHEN** a new `Product` is registered before anyone has mapped it to an `Ingredient`
- **THEN** the `Product` row exists and is flagged for review
- **AND** no `Ingredient` row is invented or guessed automatically to satisfy the mapping

### Requirement: Ingredient substitution is directional and quantity-explicit

An `IngredientSubstitution` SHALL record a direction (from one ingredient/form to another), a
category (`EQUIVALENT`, `GOOD`, `ACCEPTABLE`, `FORM`, `DIETARY`, or `EMERGENCY`), and an
explicit quantity ratio. A substitution in one direction SHALL NOT imply the reverse
substitution exists.

#### Scenario: A one-directional substitution does not imply its reverse

- **WHEN** "dried basil" is defined as a FORM substitution for "fresh basil" with ratio 0.33
- **THEN** no substitution from "dried basil" to "fresh basil" is created automatically
- **AND** querying substitutes for "dried basil" returns nothing unless a separate row exists

#### Scenario: Substitution quantity is never assumed 1:1

- **WHEN** any `IngredientSubstitution` row is created
- **THEN** it carries an explicit ratio value
- **AND** the system never applies a substitution using an implicit 1:1 assumption

### Requirement: Units carry an explicit dimension; conversions never invent density

Every `Unit` SHALL declare a dimension (mass, volume, or count). A `UnitConversion` between
two units of the same dimension SHALL be universal (apply to any ingredient). A conversion
between a volume unit and a mass unit SHALL only exist scoped to a specific `Ingredient` (and
optionally `IngredientForm`); the system SHALL NOT provide or apply a global/default density
for such a conversion.

#### Scenario: A universal conversion applies to any ingredient

- **WHEN** a requirement is expressed in deciliters and needs converting to milliliters
- **THEN** the universal `dl → ml` conversion applies regardless of which ingredient is
  involved

#### Scenario: A mass/volume conversion requires an ingredient-specific row

- **WHEN** a recipe requirement of "2 dl flour" needs converting to grams
- **THEN** the system looks up an `ingredient_unit_conversion` row scoped to "flour"
- **AND** if no such row exists, the system does not invent a density to complete the
  conversion

### Requirement: An ingredient form belongs to exactly one ingredient

An `IngredientForm` (e.g. fresh, dried, canned, frozen) SHALL reference exactly one canonical
`Ingredient`.

#### Scenario: Fresh and canned tomato are forms of the same ingredient

- **WHEN** "fresh tomato" and "canned tomato" forms are both registered
- **THEN** both reference the same canonical `Ingredient` ("tomato")
- **AND** neither form row references more than one ingredient
