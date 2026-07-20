# retailer-adapter

## Purpose

Resolve canonical, retailer-independent shopping requirements to concrete Willys products, and
turn an approved plan into a durable per-week wishlist — wrapping the Willys client behind a
stable HTTP interface so the Food Brain never handles retailer session state. Full details of
the base interface, requirement→product resolution, and wishlist output live in the
`food-brain-first-slice` change until it is archived; this spec currently carries the
pinned-resolution requirements.

## Requirements

### Requirement: Pinned products resolve before fuzzy search

The adapter SHALL consult a household pin store (term → primary product code, optional
backup product code) before fuzzy search. A pinned term whose primary product is available
SHALL resolve to that product with full confidence and `matchType: "pinned"`, without
consulting search ranking.

#### Scenario: Pinned term resolves to the household's product

- **WHEN** "handdiskmedel" is pinned to the Yes Original 1,25l product code
- **AND** a requirement for "handdiskmedel" is resolved
- **THEN** the resolution is that exact product code with `matchType: "pinned"`
- **AND** `needsReview` is false

#### Scenario: Unavailable primary falls back to the pinned backup

- **WHEN** the pinned primary product is unavailable
- **AND** a backup product code is pinned and available
- **THEN** the resolution is the backup product with `matchType: "pinned-backup"`

#### Scenario: Broken pin is surfaced, not silently fuzzy-matched

- **WHEN** both the pinned primary and backup are unavailable
- **THEN** resolution falls back to fuzzy search
- **AND** the result has `needsReview` true regardless of fuzzy confidence

### Requirement: Pins are curated and growable

The pin store SHALL be a human-editable file, and the adapter SHALL expose endpoints to list
pins and to add or update a pin, so reviewed fuzzy matches can be promoted into pins.
Aliases in the same store SHALL rewrite search terms for unpinned requirements.

#### Scenario: Promoting a reviewed match to a pin

- **WHEN** a pin for a term is added via the adapter's pin endpoint
- **THEN** subsequent resolutions of that term use the pin
- **AND** the pin survives an adapter restart

#### Scenario: Alias rewrites fuzzy search for unpinned terms

- **WHEN** "gurka" is aliased to "slanggurka" and has no pin
- **THEN** fuzzy resolution for "gurka" searches Willys for "slanggurka"
