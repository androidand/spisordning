# recipe-discovery (delta)

## ADDED Requirements

### Requirement: External recipes can be discovered from a URL

The system SHALL expose a discovery operation that fetches a recipe URL, extracts the
schema.org/Recipe JSON-LD node, parses it, and stores the result as a review candidate. The
operation SHALL be reachable through the application service, the HTTP API, and the MCP tool
surface.

#### Scenario: Discovering a JSON-LD recipe stages a candidate

- **WHEN** a client calls `POST /recipes/discover` with a URL whose page contains a valid
  schema.org/Recipe JSON-LD node
- **THEN** the system stores a `recipe_import_candidate` row with the parsed title, description,
  servings, times, instructions, and ingredient lines
- **AND** the candidate's status is `candidate`
- **AND** no `recipe_family`, `recipe_variant`, or `recipe_revision` row is created

#### Scenario: Discovering a page without a Recipe node fails clearly

- **WHEN** a client calls `POST /recipes/discover` with a URL whose page has no schema.org/Recipe
  JSON-LD node
- **THEN** the system returns an error indicating that no recipe node was found
- **AND** no candidate row is created

#### Scenario: Re-discovering the same URL is idempotent

- **WHEN** a client discovers a URL that has already been imported as a candidate
- **THEN** the system does not create a second candidate row
- **AND** the response refers to the existing candidate

### Requirement: Import candidates are reviewable through a queue

The system SHALL expose operations to list and inspect import candidates, including their status,
source provenance, parsed content, and ingredient lines. A candidate SHALL be rejectable through an
explicit review action.

#### Scenario: Listing candidates returns the review queue

- **WHEN** a client calls `GET /recipes/discovery/candidates`
- **THEN** the response includes each candidate's id, title, source URL, status, and import
  timestamp

#### Scenario: Reading a candidate includes its ingredient lines

- **WHEN** a client calls `GET /recipes/discovery/candidates/{id}`
- **THEN** the response includes the candidate's parsed fields and its ingredient lines
- **AND** each ingredient line includes its raw text, quantity, unit, and `needs_review` flag

#### Scenario: Rejecting a candidate marks it rejected

- **WHEN** a client calls `POST /recipes/discovery/candidates/{id}/reject`
- **THEN** the candidate's status becomes `rejected`
- **AND** no cookbook content is created

### Requirement: Candidate promotion creates native recipe content

The system SHALL expose an explicit promotion operation that creates native `recipe_family`
content from a candidate. Promotion SHALL create a family (or use an existing one), a variant, and
an initial revision, and SHALL link the candidate back to the created variant.

#### Scenario: Promoting a candidate creates a family, variant, and revision

- **WHEN** a client calls `POST /recipes/discovery/candidates/{id}/promote` for a candidate whose
  status is `candidate`
- **THEN** the system creates a recipe family, a variant, and an initial revision from the
  candidate's parsed content
- **AND** the variant's source attribution records the candidate's source URL
- **AND** the candidate's status becomes `promoted`
- **AND** the candidate's `promoted_variant_id` is set to the created variant

#### Scenario: Promoting into an existing family is allowed

- **WHEN** a client calls `POST /recipes/discovery/candidates/{id}/promote` with a `family_id`
- **THEN** the system creates the new variant and revision under that existing family
- **AND** the candidate is linked to the created variant

#### Scenario: Promoting an already-promoted candidate is idempotent

- **WHEN** a client promotes a candidate whose status is already `promoted`
- **THEN** the system returns the existing family, variant, and revision
- **AND** it does not create duplicate cookbook content

### Requirement: Candidate ingredient lines preserve raw source text

The system SHALL store every parsed ingredient line's raw source text. An ingredient line that
cannot be resolved to a canonical ingredient SHALL remain visible and flagged for review rather
than being dropped.

#### Scenario: An unresolvable ingredient line is retained

- **WHEN** a discovered recipe contains an ingredient line that does not match a canonical
  ingredient
- **THEN** the candidate's ingredient line stores the raw text
- **AND** the line's `needs_review` flag is true
- **AND** the line is not omitted from the candidate

### Requirement: MCP clients can discover and review recipes

The system SHALL expose MCP tools for discovering a recipe from a URL, listing import candidates,
reading one candidate, rejecting a candidate, and promoting a candidate. The tools SHALL delegate to
the same application service used by the HTTP API.

#### Scenario: Discovering a recipe over MCP

- **WHEN** an MCP client calls `discover_recipe` with a valid recipe URL
- **THEN** the tool returns the staged candidate's id, title, source URL, and status

#### Scenario: Promoting a candidate over MCP

- **WHEN** an MCP client calls `promote_import_candidate` with a candidate id
- **THEN** the tool returns the created or existing family, variant, and revision identifiers
- **AND** the candidate's status becomes `promoted`