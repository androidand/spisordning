## ADDED Requirements

### Requirement: Retailer operations declare which auth tier they need
`internal/retailer` SHALL model a basic/elevated auth-tier distinction per retailer operation, with
ICA as the grounding case: search/resolve requires only basic (anonymous ecom) auth; wishlist
creation requires elevated (OAuth2 session) auth.

#### Scenario: A basic-tier operation doesn't require the elevated credential
- **WHEN** `internal/icaretailer` resolves shopping requirements (search)
- **THEN** it does not require the elevated (ecom-session) credential to be present or fresh

#### Scenario: An elevated-tier operation reports staleness distinctly
- **WHEN** `internal/icaretailer` attempts to push a wishlist and the elevated credential is stale
- **THEN** the failure is reported as an auth-tier issue (matching `expose-shopping-price-and-notes-
  bridge`'s D3 401-detection finding), not a generic HTTP error indistinguishable from a network
  fault

### Requirement: Config owns the elevated credential's location, not the retailer client
The elevated auth credential's file location SHALL be configured via `internal/config`, not
hardcoded or independently read via `os.Getenv` inside `internal/icaretailer`.

#### Scenario: The elevated credential path comes from Config
- **WHEN** `internal/icaretailer` needs the elevated credential
- **THEN** it receives the file path (or the loaded credential) from `Config`, injected at
  construction, not read independently
