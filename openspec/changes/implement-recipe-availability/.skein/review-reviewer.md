# Review — implement-recipe-availability (round 2)

**Summary**: Fixed two blocking issues from the first review round.

**Fixes applied**:

1. **Substitution walk no longer short-circuits on shortfall** (`availability.go:186-238`).
   Previously: when a substitution's target lot existed but was quantity-insufficient,
   `evaluateLine` returned `missing` immediately, blocking lower-tier substitutions that
   would have fully covered the line. Fix: track `bestShortfall` across all tiers and
   all substitutions; `continue` past insufficient lots; only report `missing` after
   exhausting all tiers. The shortfall value is surfaced in the final reason
   (`missing-shortfall`).

2. **`ConsumedLotIDs` now populated** (`availability.go:170-171,217`).
   Previously: `ConsumedLotIDs` was never assigned — only `NearExpiryLotIDs` was set,
   making the field a dead letter. Fix: `ConsumedLotIDs` is set to `[lotID]` on both
   direct on-hand matches and substituted matches. `NearExpiryLotIDs` is now correctly
   a subset of `ConsumedLotIDs`.

3. **`ToForm` enforcement** (`availability.go:212-214`).
   Previously: `sub.ToForm` was stored but never checked when matching the target lot.
   Fix: when looking for a target lot, pass `PreferredForm: sub.ToForm` to
   `findBestDirectLot` so form-constrained substitutions only match lots of the
   specified form.

**New tests** (27 total, up from 22):
- `TestEvaluateRecipe_SubstitutionShortfallDoesNotBlockLowerTier` — verifies
  EQUIVALENT sub with short lot doesn't block GOOD sub that fully covers.
- `TestEvaluateRecipe_ConsumedLotIDsPopulated` — verifies direct match surfaces lot id.
- `TestEvaluateRecipe_ConsumedLotIDsForSubstitution` — verifies sub match surfaces lot id.
- `TestEvaluateRecipe_SubstitutionToFormEnforced` — verifies ToForm=frozen blocks fresh lot.
- `TestEvaluateRecipe_SubstitutionToFormAllowsMatch` — verifies ToForm=frozen matches frozen lot.

**Minor notes from round 1** (deferred):
- `LotInfo.Unit` unused — cross-unit normalization is out of scope (deferred to unit system work).
- Lots past best-before silently not surfaced as near-expiry — correct by design (only
  within 7-day window is "near"; expired lots are handled by pantry expiration logic).

**Verdict**: PASS (round 2)
