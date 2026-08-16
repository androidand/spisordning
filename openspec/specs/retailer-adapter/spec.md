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

### Requirement: Apple Notes checklists drive resolution through the adapter

An Apple Notes checklist SHALL be usable as a source of shopping terms that resolve and
create a wishlist through the willys-adapter, reusing the adapter's pins and single session.
The notes bridge SHALL NOT maintain its own Willys login, product search, or wishlist
creation — those go through the adapter.

#### Scenario: A note checklist becomes a wishlist via the adapter

- **WHEN** the notes bridge runs against a mapped note with checklist items
- **THEN** it parses the items and POSTs their terms to the adapter's resolve endpoint
- **AND** creates or extends the mapped wishlist through the adapter's shopping-list endpoint
- **AND** does not perform its own Willys login or product search

#### Scenario: Pinned terms from a note resolve to the household product

- **WHEN** a note item's term is pinned in the adapter (e.g. "handdiskmedel")
- **THEN** the bridge's resolution of that item uses the pin, not fuzzy search

#### Scenario: Dry-run makes no changes

- **WHEN** the bridge runs without the apply flag
- **THEN** it reports the resolved products and needs-review items
- **AND** creates no wishlist and modifies nothing

#### Scenario: Needs-review items are not silently added

- **WHEN** an item resolves below the review threshold (or a pin is broken)
- **THEN** the bridge reports it for review
- **AND** does not add it to the wishlist

### Requirement: Confidence reflects name match; quantity uncertainty is separate

Resolution `confidence` SHALL reflect name-match quality only. When the product's package
size cannot be reconciled with the requirement's unit, the resolution SHALL report
`quantityUncertain: true` with a safe package default of 1 — and this SHALL NOT by itself
mark the resolution as needing review.

#### Scenario: Perfect name match with unreconcilable units resolves

- **WHEN** a requirement for "1 st mjölk" resolves to a product named "Mjölk" sold as "1l"
- **THEN** the resolution's confidence reflects the strong name match (not capped)
- **AND** `needsReview` is false
- **AND** `quantityUncertain` is true with `packages` = 1

#### Scenario: Weak name match still goes to review

- **WHEN** a requirement's best candidate is only a weak lexical match
- **THEN** `needsReview` is true regardless of whether the package size reconciles

#### Scenario: Reconciled quantities are certain

- **WHEN** a requirement of 900 g resolves to a 650 g product
- **THEN** `quantityUncertain` is false
- **AND** `packages` is 2

### Requirement: Needs-review terms are queued and pickable

The adapter SHALL maintain a queue of terms whose latest resolution needed review, and SHALL
offer a review surface where each queued term is shown with live Willys search hits so a
person can pick the product the term means. The pick SHALL be persisted as the term's pin
(optionally with a backup product), after which the term leaves the queue.

#### Scenario: New keyword lands in the review queue

- **WHEN** a resolution for "majskolvar" is flagged needs-review
- **THEN** "majskolvar" appears in the review queue with its requested quantity

#### Scenario: Picking a hit pins it

- **WHEN** a person picks a product from the search hits shown for a queued term
- **THEN** a pin term → that product is persisted
- **AND** a subsequent resolution of the term resolves via the pin without review

#### Scenario: Confident resolution clears the queue

- **WHEN** a queued term later resolves confidently (e.g. after being pinned)
- **THEN** it is removed from the queue

#### Scenario: A term can be dismissed

- **WHEN** a person dismisses a queued term
- **THEN** it leaves the queue without creating a pin

### Requirement: Pin changes propagate to watched notes

The notes watcher SHALL detect pin-store changes and re-sync watched notes even when the
note content is unchanged, so a pick takes effect without editing the note.

#### Scenario: Pick re-syncs the note within a watch cycle

- **WHEN** a pin is added while a note watcher is running
- **THEN** the next watch cycle re-syncs the note
- **AND** the newly pinned term is added to the wishlist

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

