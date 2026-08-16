# recipe-discovery (delta)

## ADDED Requirements

### Requirement: External recipes are not auto-imported into the household cookbook

The system SHALL treat an externally sourced recipe (from an external API, a generic web
import, or any other discovery source) as a review candidate, not directly as a
`RecipeFamily`/`RecipeVariant`/`RecipeRevision` in the household cookbook.
Promotion to the cookbook SHALL require an explicit review action, and SHALL normally follow
the recipe having been planned or cooked.

#### Scenario: A web-imported recipe requires review before joining the cookbook

- **WHEN** a recipe is imported from a JSON-LD web page
- **THEN** it is stored as a review candidate
- **AND** no `RecipeVariant`/`RecipeRevision` is created in the household cookbook until an
  explicit review/save action occurs

#### Scenario: Bulk import does not bypass review

- **WHEN** multiple recipes are fetched from an external API in one operation
- **THEN** each one individually requires its own review action before becoming part of the
  cookbook
- **AND** none are promoted automatically as a batch

### Requirement: Imported recipes carry source provenance

Every recipe imported from an external source SHALL record its provenance: source URL or
source name, license note where known, an external identifier if the source provides one, and
the import timestamp.

#### Scenario: A web-imported recipe records its source URL

- **WHEN** a recipe is imported via the generic JSON-LD pipeline
- **THEN** the resulting candidate records the source URL it was fetched from
- **AND** the import timestamp

#### Scenario: An API-sourced recipe records its external id

- **WHEN** a recipe is imported from an external recipe API
- **THEN** the candidate records that source's external recipe id
- **AND** the source name

### Requirement: Ingredient strings are canonicalized, not stored as free text only

When an imported recipe's ingredient lines are parsed, each parsed ingredient SHALL be
resolved against the canonical `Ingredient` vocabulary where possible. An ingredient that
cannot be resolved SHALL be flagged for review rather than silently dropped or stored only as
unstructured text.

#### Scenario: A resolvable ingredient is canonicalized

- **WHEN** an imported ingredient line parses to a known ingredient name
- **THEN** it is linked to the corresponding canonical `Ingredient`

#### Scenario: An unresolvable ingredient is flagged, not dropped

- **WHEN** an imported ingredient line does not match any canonical `Ingredient`
- **THEN** the parsed line is retained and flagged as needing review
- **AND** it is not silently discarded from the imported recipe
