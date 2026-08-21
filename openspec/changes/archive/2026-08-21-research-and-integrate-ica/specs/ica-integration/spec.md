# ica-integration (delta)

## ADDED Requirements

### Requirement: ICA capability claims are verified before design

Any statement about current ICA API capabilities used to justify an ICA adapter design SHALL be
backed by inspection of the seed repositories (`ica-api`, `ha-ica-todo`) or direct testing, and
SHALL be recorded in `docs/research/ica-current-api.md`. The system SHALL NOT proceed to
implementing an ICA adapter based on unverified assumptions carried over from `PLAN.md`'s initial
observations alone.

#### Scenario: An unverified claim blocks adapter design

- **WHEN** a claim about ICA API behavior (e.g. "the April 2024 changes broke `ica-api`") has not
  been independently confirmed
- **THEN** it is recorded in `docs/research/ica-current-api.md` as an open question, not as a
  settled fact
- **AND** no ICA adapter implementation change may cite it as justification until verified

#### Scenario: A verified capability map exists before adapter design starts

- **WHEN** a future change proposes implementing an ICA adapter
- **THEN** it can point to a capability map in `docs/research/ica-current-api.md` in the same
  shape as `docs/research/willys-capabilities.md`
- **AND** the map distinguishes confirmed-supported capabilities from unsupported or unverified
  ones

### Requirement: Home-Assistant-specific design is not inherited by default

Ideas extracted from `ha-ica-todo`'s `ICA+Grocy.md` SHALL be evaluated on their own merits for
Spisordning's domain model, not adopted wholesale (source: any other Home-Assistant-oriented
material is held to the same standard). A design element that depends on Home Assistant
infrastructure (entities, services, HA-specific storage or auth patterns) SHALL NOT be adopted
as-is; it SHALL be re-expressed in Spisordning's own terms or explicitly rejected with reasoning.

#### Scenario: An HA-coupled idea is re-expressed, not copied

- **WHEN** `ICA+Grocy.md` describes an inventory lifecycle step implemented as an HA service call
- **THEN** the extracted idea is documented as a domain concept (event type, transition, trigger)
  independent of any HA API
- **AND** the HA-specific implementation detail is noted as not carried over

### Requirement: A future ICA adapter SHALL follow the retailer-adapter capability's invariants

Should ICA integration proceed, any future ICA adapter SHALL satisfy the same non-negotiable
invariants already established by the `retailer-adapter` capability for Willys: no automated
checkout, payment, or slot booking; retailer product identity kept distinct from canonical
ingredients; and low-confidence resolutions routed to human review rather than silently applied.

#### Scenario: A future ICA adapter design cites the existing invariants

- **WHEN** a future `integrate-ica` change proposes an ICA adapter
- **THEN** its proposal explicitly states it inherits the no-automated-checkout invariant from
  `retailer-adapter`
- **AND** does not re-derive or weaken it independently

#### Scenario: Checkout automation is out of scope regardless of ICA API capability

- **WHEN** research finds that the current ICA API technically permits placing an order
  programmatically
- **THEN** this finding does not change the invariant — automated checkout remains excluded
