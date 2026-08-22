# Review — implement-recipe-availability

**Summary**: Adds `internal/domain` types for `IngredientForm` and
`IngredientSubstitution` (consumed from `establish-household-and-catalog`,
not redefined), plus a pure-domain `internal/availability` package with
`EvaluateRecipe` — per-ingredient and per-recipe feasibility against
current household inventory, with explainable substitution walks.

**Design decisions documented in code/comments**:
- Partial quantity = unmet with shortfall noted (task 3.5). Feeds
  shopping-gap cleanly; no "partially satisfied" ambiguity.
- UNKNOWN-confidence lot = satisfied-but-flagged as
  `on-hand-uncertain`; recipe verdict downgrades from `feasible` to
  `feasible-with-substitution` (task 3.6).
- Form mismatch triggers `FORM`-tier substitution walk, not auto-match
  (task 3.2). `acceptable_forms` on the recipe line allows mismatched
  form as direct match.
- Expiry: `NearExpiryLotIDs` surfaces lots with best-before within 7
  days of `Now`; zero `Now` = no expiry surface. Scoring is left to
  `implement-recommendations` (task 6.1).
- Pure domain: `EvaluateRecipe` takes pre-fetched `RecipeLine`/`LotInfo`/
  `IngredientSubstitution`; no DB, no new tables (task 7.1).

**Positives**:
- Clean separation: domain types in `domain.go`, computation in
  `availability.go`, tests in `availability_test.go`. No persistence
  leak.
- `SubstitutionTierOrder()` is a single source of truth for walk order.
- `findBestDirectLot` sorts by form preference → confidence →
  best-before, which is the right priority for consumption ordering.
- Shortfall is computed and surfaced on the `missing` line, not silently
  ignored.
- 22 tests cover all spec scenarios plus edge cases (retired sub,
  acceptable forms, near-expiry gating).

**Issues**: None blocking. One minor note: `LotInfo` mirrors
`persistence.InventoryLot` fields but is a separate struct — the caller
must map between them. This is intentional (pure domain), but the
mapping layer is not part of this change (deferred to the planning or
persistence consumer).

**Verdict**: PASS
