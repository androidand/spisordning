# recipe-family (delta)

## ADDED Requirements

### Requirement: A RecipeRevision is immutable once created

The system SHALL NOT provide any operation that mutates a `RecipeRevision`'s stored content
after creation. A correction or evolution of a variant's recipe SHALL be represented as a new
`RecipeRevision`.

#### Scenario: Correcting a recipe creates a new revision

- **WHEN** a household corrects an ingredient quantity in a variant's latest revision
- **THEN** a new `RecipeRevision` is created with the corrected content
- **AND** the original `RecipeRevision`'s stored content is unchanged

#### Scenario: No update path exists for revision content

- **WHEN** any client attempts to modify an existing `RecipeRevision`'s ingredients or steps
- **THEN** the system provides no such operation — only creation of a new revision

### Requirement: A RecipeVariant belongs to exactly one RecipeFamily

Every `RecipeVariant` SHALL reference exactly one `RecipeFamily`. A variant SHALL NOT be
shared across multiple families.

#### Scenario: A fork stays within its source family unless explicitly moved

- **WHEN** a new `RecipeVariant` is forked from a revision belonging to a variant in family
  "Korvstroganoff"
- **THEN** the new variant belongs to the "Korvstroganoff" family by default
- **AND** it does not simultaneously belong to any other family

### Requirement: Revision parentage never cycles

The system SHALL reject any `RecipeRevisionParent` edge that would create a cycle in the
revision lineage graph.

#### Scenario: A revision cannot become its own ancestor

- **WHEN** an attempt is made to record revision A as a parent of revision B, where B is
  already an ancestor of A
- **THEN** the system rejects the edge
- **AND** no cycle is created in `recipe_revision_parent`

### Requirement: The default variant is a stored, manually-set choice

A `RecipeFamily`'s `default_variant_id` SHALL be an explicitly stored reference, set only by an
explicit command, and SHALL NOT be silently overwritten by a computed rating or popularity
signal.

#### Scenario: A computed suggestion does not overwrite the stored default

- **WHEN** a variant other than the currently pinned default accumulates a higher aggregate
  rating
- **THEN** `default_variant_id` remains unchanged unless an explicit command sets it
- **AND** the higher-rated variant may be surfaced only as a suggestion, not applied
  automatically

#### Scenario: The default variant belongs to its own family

- **WHEN** `default_variant_id` is set on a `RecipeFamily`
- **THEN** the referenced `RecipeVariant` belongs to that same `RecipeFamily`
