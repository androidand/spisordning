# retailer-adapter (delta)

## ADDED Requirements

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
