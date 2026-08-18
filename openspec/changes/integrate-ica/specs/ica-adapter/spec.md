# ica-adapter (delta)

## ADDED Requirements

### Requirement: Adapter design and implementation are gated on ica-client stability

The system SHALL NOT begin `ica-adapter` design or implementation work while
`~/dev/willys/ica-client` is unstable (failing build, no passing tests, or unverified live
auth), per the snapshot in `docs/research/ica-current-api.md` §5. Gate status SHALL be
re-verified live before proceeding, not assumed from a prior snapshot.

#### Scenario: Stale readiness assumption is rejected

- **WHEN** a change proposes `ica-adapter` implementation work
- **AND** the only evidence of `ica-client` stability is an outdated research-document snapshot
- **THEN** the change SHALL re-verify current state before proceeding, not cite the stale
  snapshot as sufficient

### Requirement: ica-adapter inherits retailer-adapter's non-negotiable invariants

`ica-adapter` SHALL satisfy every invariant `openspec/specs/retailer-adapter/spec.md` already
establishes for Willys: no automated checkout, payment, or slot booking; retailer product
identity kept distinct from the canonical `ingredient`; low-confidence resolutions routed to a
human review queue rather than silently applied.

#### Scenario: ICA's cart API does not become a checkout path

- **WHEN** `ica-adapter` wraps `ica-client`'s cart service (item add/update/remove, delivery
  address)
- **THEN** no endpoint on `ica-adapter` places an order, initiates payment, or books a delivery
  slot
- **AND** this holds regardless of what `ica-client`'s underlying API technically permits

### Requirement: Home-Assistant-specific design is not inherited

`ica-adapter` SHALL NOT adopt `ha-ica-todo`'s Home-Assistant-coupled plumbing (config flows,
coordinators, HA entities/services/storage) — it follows `willys-adapter`'s standalone HTTP
service shape instead, per `research-and-integrate-ica`'s `ica-integration` capability.

#### Scenario: The adapter is a plain HTTP service, not an HA integration

- **WHEN** `ica-adapter` is implemented
- **THEN** it is a standalone HTTP service callable by `food-brain`, structurally parallel to
  `willys-adapter`
- **AND** it does not depend on Home Assistant being present or running
