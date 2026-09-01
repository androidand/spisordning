## MODIFIED Requirements

### Requirement: Apple Notes checklists drive resolution through food-brain

An Apple Notes checklist SHALL be usable as a source of shopping terms that resolve and
create a wishlist through food-brain's own retailer-resolution component, reusing its pins and
its single embedded Willys client session. The notes bridge SHALL NOT maintain its own Willys
login, product search, or wishlist creation, and SHALL NOT call a separately-deployed adapter
service — those go through food-brain directly.

#### Scenario: A note checklist becomes a wishlist via food-brain

- **WHEN** the notes bridge runs against a mapped note with checklist items
- **THEN** it parses the items and POSTs their terms to food-brain's resolve endpoint
- **AND** creates or extends the mapped wishlist through food-brain's shopping-list endpoint
- **AND** does not perform its own Willys login or product search
- **AND** does not call a separately-deployed willys-adapter service

#### Scenario: Pinned terms from a note resolve to the household product

- **WHEN** a note item's term is pinned in food-brain (e.g. "handdiskmedel")
- **THEN** the bridge's resolution of that item uses the pin, not fuzzy search
