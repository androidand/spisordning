## Context

Both live runs (weekly plan 2026-07-19; Apple Notes bridge 2026-07-21, 0/9 resolved) showed
resolveAgainstCandidates capping confidence at 0.65 when pkg.unit != requirement unit, which
puts perfect name matches under the 0.7 review threshold. Count-based requirements are the
normal grocery case.

## Goals / Non-Goals

**Goals:** confidence = name quality; quantity uncertainty visible, not punitive; review
queue contains only dubious names and broken pins.
**Non-Goals:** unit conversion via ingredient_mapping/grams_per_unit (first-slice task 2.3);
changing the review threshold.

## Decisions

- D1: confidence := min(1, bestScore); drop the size-based Math.min(bestScore, 0.65) cap.
- D2: quantityUncertain := !sizeKnown on fuzzy and pinned resolutions; packages default 1,
  resolvedQuantity null when uncertain (unchanged behaviour, now labelled).
- D3: Consumers unchanged: needsReview remains callers''' only gate; Go type gains the field
  optionally.

## Risks / Trade-offs

- Slight over-trust: a strong name with wildly wrong size buys 1 package instead of asking;
  acceptable for groceries, correctable via pins and later grams_per_unit.
