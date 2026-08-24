# Review — implement-pantry-inventory

**Summary**: Adds the pantry-inventory schema (migration 0009), domain vocabulary (confidence tiers, event kinds, GTIN normalization, location cycle check), and a transactional persistence layer (ledger-plus-projection) with integration tests.

**Issues (all fixed in follow-up commit eb730ac)**:

- `RecordAdjust` (internal/persistence/pantry.go:339) did not validate `newQuantity >= 0`; a negative adjustment silently wrote a negative lot quantity. Fixed: added explicit `newQuantity < 0` guard at the command boundary. `applyLotDelta` already guards `quantity + $1 >= 0` and `RecordTransfer` validates its quantity — this was the one mutating path missing a guard.
- `RecordPurchase` similarly accepted `quantity <= 0`. Fixed: added explicit `quantity <= 0` guard.
- tasks.md 9.4 claimed `TestNormalizeGTIN` covered "GTIN-8/12/13/14 canonicalization"; the test only carried a GTIN-13 vector. Fixed: added GTIN-8 (`96385074` → `00000096385074`) and GTIN-12 (`012345678905` → `00012345678905`) vectors. GTIN-14 is covered implicitly by the GTIN-13 vector (the padding path).
- `Store.ListProducts` (catalog.go:81) was dead code with a doc comment wrongly claiming it was used by `ListCandidateProductsForIngredient`'s name-match fallback (that fallback runs its own SQL). Fixed: corrected the doc comment.
- `RefineLotProduct` dropped the `source` parameter from design.md Step 5's command signature without documenting the deviation. Fixed: added a doc comment explaining the intentional deviation — a refinement is not an inventory event and carries no delta, so there is no event row to attach a source to. Documented in tasks.md 4.5's note.
- No test for `LookupBarcode` (normalize + lookup). Fixed: added `TestPantry_LookupBarcode` covering valid GTIN-14, GTIN-13 resolution, miss (returns "", nil), and invalid GTIN (returns error).
- No test for the over-consume guard. Fixed: added `TestPantry_OverConsumeGuard` (consume 2 from a 1-unit lot fails, lot stays at 1) and `TestPantry_RecordAdjustNegativeTarget` (negative target rejected, lot unchanged) and `TestPantry_RecordPurchaseZeroQuantity` (zero quantity rejected).

**Positives**: ledger-plus-projection writes are atomic per command; the GTIN check-digit algorithm is correct (verified against the 7300400176354 vector); location cycle check is correct; spec scenarios are covered; deferrals (5.8, 7.3–7.5, 9.6) are honestly documented.

**Verdict**: PASS
