# architecture-foundation (delta)

## ADDED Requirements

### Requirement: Domain state is persisted, not in-memory

The system SHALL persist people, preferences, preference observations, meal events, meal
reactions, effort profiles, planning constraints, plan candidates, plan decisions, ingredient
mappings, and shopping requirements in PostgreSQL through a repository layer, rather than
holding them only in memory for the lifetime of a single CLI invocation.

#### Scenario: A meal reaction survives a process restart

- **WHEN** a meal reaction is recorded through the system
- **AND** the `food-brain` process is restarted
- **THEN** the reaction is still readable from PostgreSQL afterward

#### Scenario: Plan candidates and decisions are queryable after the run that produced them

- **WHEN** a weekly plan run produces candidates and a decision is recorded against one
- **THEN** both the candidates and the decision are persisted and retrievable independently of
  the CLI process that produced them

### Requirement: A versioned, contract-first OpenAPI surface

The system SHALL expose its HTTP API through a hand-authored OpenAPI contract (`api/openapi.yaml`)
that is the source of truth for the API shape. Server code SHALL be generated from that
contract. Generated code SHALL NOT be hand-edited; changes to server behavior SHALL be made by
editing the contract and regenerating.

#### Scenario: The API is discoverable from its contract

- **WHEN** a developer or client inspects `api/openapi.yaml`
- **THEN** every implemented HTTP endpoint of `food-brain` is described there, including request
  and response shapes

#### Scenario: Generated code is regenerated, not patched

- **WHEN** an endpoint's request or response shape needs to change
- **THEN** the change is made in `api/openapi.yaml` first
- **AND** server code is regenerated from it, not edited directly

### Requirement: Layer boundaries are mechanically enforced

The system SHALL separate domain, application, persistence, and HTTP concerns into distinct Go
packages with a declared allowed-dependency direction (httpapi → application → domain;
persistence → domain), and SHALL enforce that direction with an automated check that fails the
build on violation — not by code-review convention alone.

#### Scenario: A domain package importing persistence fails CI

- **WHEN** a change adds an import from a domain-layer package to a persistence-layer package
- **THEN** the architecture-enforcement check fails
- **AND** CI reports the violation before the change can be merged

#### Scenario: A conforming change passes the check

- **WHEN** a change only adds imports consistent with the declared layer directions
- **THEN** the architecture-enforcement check passes

### Requirement: CI builds, tests, and lints every change

The system SHALL run an automated CI pipeline on every push and pull request that builds the Go
module, runs its test suite, runs `go vet`, and runs the architecture-enforcement check.

#### Scenario: A broken build is caught before merge

- **WHEN** a pull request introduces a compile error or a failing test
- **THEN** CI fails on that pull request
- **AND** the failure is visible before merge

### Requirement: A container image and Compose integration exist

The system SHALL build as a Docker image and SHALL be runnable as a service within
`docker-compose.yml` alongside `postgres` and `willys-adapter`, applying migrations and serving
its OpenAPI-described API once running.

#### Scenario: Compose brings up the full local stack

- **WHEN** `docker compose up -d` is run
- **THEN** `postgres`, `willys-adapter`, and `food-brain` all start successfully
- **AND** `food-brain` successfully connects to `postgres` and serves its API

### Requirement: Optional inspection tools remain non-load-bearing

The system SHALL function correctly whether or not an optional database-inspection tool (e.g. a
future Directus instance) is running. Stopping such a tool SHALL NOT affect `food-brain`'s
availability or correctness.

#### Scenario: Stopping an optional inspection tool has no effect

- **WHEN** an optional, read-only database-inspection tool that was running is stopped
- **THEN** `food-brain` continues to serve requests and persist data without degradation
