## ADDED Requirements

### Requirement: Quantities use numeric

Measurable quantities SHALL be stored as `numeric`, not IEEE `float` (`DOUBLE PRECISION`). This
applies to `shopping_requirement.quantity`, `shopping_list_item.quantity`,
`shopping_cart_item.quantity`, `order_item.quantity`, `inventory_lot.quantity`,
`inventory_event.quantity_delta`, `product_ingredient_mapping.quantity`, and
`recipe_import_candidate_ingredient.quantity`.

#### Scenario: An inventory lot quantity is numeric
- **WHEN** an `inventory_lot` row stores a quantity
- **THEN** the column is `numeric`, not `DOUBLE PRECISION`

#### Scenario: No quantity column is a float
- **WHEN** the schema is inspected after the re-baseline
- **THEN** no quantity column on a non-recipe table is `DOUBLE PRECISION`

### Requirement: Transaction money uses minor units and currency

Transaction amounts SHALL be stored as `amount_minor BIGINT NOT NULL` plus
`currency CHAR(3) NOT NULL DEFAULT 'SEK'`. This applies to `order.total_price`,
`order_item.total_price`, and `shopping_cart_item.resolved_price`.

#### Scenario: An order total is minor units with currency
- **WHEN** an `order` row stores its total
- **THEN** it is stored as an integer minor amount and a three-letter currency code

#### Scenario: A transaction amount is not a float
- **WHEN** the schema is inspected after the re-baseline
- **THEN** no transaction-amount column on a non-recipe table is `DOUBLE PRECISION`

### Requirement: Derived unit prices are not conflated with transaction amounts

A per-unit price that may need sub-minor precision (`order_item.unit_price`) SHALL be stored as
`numeric`, distinct from the minor-units representation used for transaction amounts.

#### Scenario: A unit price is numeric
- **WHEN** an `order_item` row stores its per-unit price
- **THEN** the column is `numeric`, while the line `total_price` is minor units + currency

### Requirement: Scores, confidence, and weight are bounded numerics

Stored scores, confidence, and weight SHALL be bounded `numeric` with a `CHECK` constraint,
reviewed individually rather than converted mechanically:
- `person.weight` to `numeric(6,3) CHECK (weight > 0)`
- `person_preference.confidence` to `numeric(5,4) CHECK (confidence >= 0 AND confidence <= 1)`
- `recipe_import_candidate.rating` to `numeric(3,1) CHECK (rating >= 0 AND rating <= 5)`

Transient computed scores (`meal_plan_candidate.score`) MAY remain `DOUBLE PRECISION`.

#### Scenario: A preference confidence is bounded
- **WHEN** a `person_preference` row stores a confidence of `1.5`
- **THEN** the insert is rejected by the `CHECK (confidence >= 0 AND confidence <= 1)` constraint

#### Scenario: A computed recommendation score may stay a float
- **WHEN** a `meal_plan_candidate` row stores its computed `score`
- **THEN** the column may be `DOUBLE PRECISION`

### Requirement: GTIN is a zero-padded string

GTIN values SHALL be stored as a zero-padded 14-digit `TEXT` string, not an integer. GTIN is one
`scheme` of `product_identifier`; scheme-specific format validation SHALL occur at the Go
boundary, not as a single shared database constraint.

#### Scenario: A GTIN keeps its leading zeros
- **WHEN** a GTIN `0731...` is stored on a product
- **THEN** it is stored as the full 14-character zero-padded string

#### Scenario: A product identifier is unique per scheme
- **WHEN** the same `(scheme, value)` is inserted twice into `product_identifier`
- **THEN** the second insert is rejected by the primary key
