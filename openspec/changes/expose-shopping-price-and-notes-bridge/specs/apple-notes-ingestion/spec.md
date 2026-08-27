## ADDED Requirements

### Requirement: A named Apple Note can seed a spisordning shopping list
A Mac-local component SHALL be able to read a named Apple Note's checklist and submit its items to
spisordning's shopping-list API as a new (or updated) `shopping_list`, independent of and without
replacing the existing single-retailer Willys-only notes-bridge flow.

#### Scenario: A note's checklist becomes a spisordning shopping list
- **WHEN** the Mac-local reader runs against the "Köp Mat Andreas" note
- **THEN** it parses the checklist's unchecked items
- **AND** submits them to spisordning's shopping-list ingestion endpoint as line items

#### Scenario: Existing Willys-only note flow is unaffected
- **WHEN** a different note is still mapped through the existing `willys-adapter`-direct notes-sync
  flow (per the merged `retailer-adapter` spec's "Apple Notes checklists drive resolution through
  the adapter" requirement)
- **THEN** that flow continues to work unchanged

#### Scenario: The reader only runs where Notes.app is available
- **WHEN** the Mac-local reader is invoked
- **THEN** it runs on the user's Mac (not on the Proxmox-hosted spisordning server), since Apple
  provides no server-side API for reading Notes content
