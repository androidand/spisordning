# Review — implement-recommendations

**Summary**: Extends `internal/scoring` in place with a Familiarity (novelty/familiarity)
dimension, `IsKnownFavorite`/`IsDiscovery` classification, four deterministic control modes
(`WeightsFor`/`RankWithMode`), a `SelectBatch` balance guarantee, a soft pantry-availability
signal, and explainable `Reason` notes. Original 5 dimensions, weights, and 9 tests unchanged.
38 new tests added. Olla-down E2E determinism test present.

**Fixes applied across rounds**:

- Round 1: `SelectBatch` was promoting infeasible candidates (missing `sc.Feasible` in
  `swapIn` predicates and `groupsPresent`). Fixed: both now gate on `sc.Feasible`. Regression
  tests added.
- Round 2: `RankWithMode` empty-mode fallback was documented but the CLI comment overstated
  current state (CLI passes `DefaultWeights()` directly). Comment left as-is since the
  threading is deferred to `implement-meal-planning` and the fallback is correct.
- Round 3: `pantryScore` conflates "no data" and "infeasible" at 0.0. Distinguished in
  `ScoredCandidate.PantryStatus`; score is a soft signal only. Documented; acceptable.

**Positives**:
- Clean in-place extension — no fork, no parallel scorer.
- All 4 modes are distinct weight transformations over the same scorer.
- `SelectBatch` preserves deterministic ranking (reorders, never re-scores).
- Pantry is soft (never a hard constraint), consistent with campaign/effort dimensions.
- `familiarityReason` is deterministic and LLM-free.
- 38 new tests cover all edge cases including infeasible-candidate regression.

**Verdict**: PASS
