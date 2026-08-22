# Review — implement-recipe-availability (round 4)

**Summary**: Round 3 issues fixed. Pure-domain `internal/availability` package now correctly handles
multi-lot aggregation, substitution shortfall in target units, and deduplication.

**Fixes from round 3**:

1. **Multi-lot aggregation** (`aggregateDirectLots`): Replaced `findBestDirectLot` (single-lot) with
   `aggregateDirectLots` that sums quantities across ALL matching lots. Direct path now aggregates
   all on-hand lots first; if total < required, walks substitutions for the residual. Test:
   `TestEvaluateRecipe_MultiLotAggregation` (500+500 >= 800 → feasible),
   `TestEvaluateRecipe_MultiLotAggregationStillShort` (300+400 < 800 → missing shortfall 100),
   `TestEvaluateRecipe_MultiLotSubstitutionForResidual` (300g direct + 600g sub covers 800g residual).

2. **Substitution shortfall in target units** (`walkSubstitutions`): `bestNeeded` now tracks the
   target-unit quantity (`needed * sub.Ratio`) instead of source-unit quantity. Shortfall is
   `bestNeeded - bestAvailable` in target units. Test:
   `TestEvaluateRecipe_SubstitutionShortfallCorrect` (100g fresh → 33g dried, 20g on-hand →
   shortfall 13, not 80).

3. **Short direct lot now walks substitutions for residual** (minor, round 3): When direct lots
   exist but are insufficient, the code now falls through to substitution walk for the residual
   before declaring missing.

4. **Recipe-level deduplication** (`EvaluateRecipe`): `ConsumedLotIDs` and `NearExpiryLotIDs` are
   deduplicated via `dedupInt64s`. Test: `TestEvaluateRecipe_MultiLotDeduplication`.

5. **`worstConf` initialization**: `aggregateDirectLots` now initializes `worstConf = EXACT` so
   UNKNOWN lots are correctly detected (was `""` which had rank 0, never updated).

6. **Near-expiry computation**: Moved into `aggregateDirectLots` with `now` parameter. Correctly
   surfaces lots within 7 days.

7. **Comments**: Removed references to non-existent `design.md`.

**Tests**: 32 total (up from 27). All pass. `openspec validate` passes. `go vet` clean.

**Verdict**: PASS (round 4)
