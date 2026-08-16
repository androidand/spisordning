# retailer-adapter (delta)

## ADDED Requirements

### Requirement: A size in a term is parsed out and honoured

When a search term contains a size (e.g. "1,5 l", "500 g", "15-pack"), the adapter SHALL
split it into a name part and a size hint. The Willys search query SHALL use the name part,
name-match confidence SHALL be scored on the name part, and a candidate whose package size
matches the hint SHALL be preferred over one that does not.

#### Scenario: Sized soft-drink term resolves to the matching size

- **WHEN** "Coca cola zero 1,5 L" is resolved
- **THEN** the Willys search uses "coca cola zero"
- **AND** the chosen product is a 1,5 l Coca-Cola Zero, not a 50 cl can
- **AND** confidence reflects the strong name match (not lowered by the size tokens)

#### Scenario: Size hint with no matching pack still resolves on name

- **WHEN** a term's size hint matches no candidate's package size
- **THEN** resolution falls back to the best name match with quantityUncertain
- **AND** is no worse than resolution without a size hint

#### Scenario: Terms without a size are unaffected

- **WHEN** a term contains no size (e.g. "blomkål")
- **THEN** the search query and name scoring are the whole term, as before
