# retailer-adapter (delta)

## ADDED Requirements

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
