# retailer-adapter (delta)

## ADDED Requirements

### Requirement: Confidence reflects name match; quantity uncertainty is separate

Resolution `confidence` SHALL reflect name-match quality only. When the product's package
size cannot be reconciled with the requirement's unit, the resolution SHALL report
`quantityUncertain: true` with a safe package default of 1 — and this SHALL NOT by itself
mark the resolution as needing review.

#### Scenario: Perfect name match with unreconcilable units resolves

- **WHEN** a requirement for "1 st mjölk" resolves to a product named "Mjölk" sold as "1l"
- **THEN** the resolution's confidence reflects the strong name match (not capped)
- **AND** `needsReview` is false
- **AND** `quantityUncertain` is true with `packages` = 1

#### Scenario: Weak name match still goes to review

- **WHEN** a requirement's best candidate is only a weak lexical match
- **THEN** `needsReview` is true regardless of whether the package size reconciles

#### Scenario: Reconciled quantities are certain

- **WHEN** a requirement of 900 g resolves to a 650 g product
- **THEN** `quantityUncertain` is false
- **AND** `packages` is 2
