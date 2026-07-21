## Context

Live: search "coca cola zero 1,5 L" returns 50cl/33cl (Willys ignores size); the 1,5l Pet only
appears for the residual query "coca cola zero". And matchScore on the full term scores 0.40
because "1","5","l" tokens are not in the product name.

## Decisions

- D1: splitSizeHint(term) -> {name, hint} where hint reuses the parseDisplayVolume grammar
  (kg/g/hg/l/dl/cl/ml/st/pack, comma decimals) anywhere in the term; name is the term with the
  size token removed and whitespace collapsed. No size -> {name: term, hint: null}.
- D2: searchTermFor(req) returns the alias-resolved term with the size stripped; server + picker
  use it as the Willys query. resolveAgainstCandidates scores matchScore on the name part.
- D3: candidate ranking adds a size-match term: parseDisplayVolume(candidate.displayVolume)
  compared to the hint (same base unit + within 10%) gives a bonus; strong mismatch a small
  penalty. Name score still dominates so a size typo never picks a wrong product.
- D4: picker sorts hits by (size-match desc, original order) so the right size floats up; the
  variant-family expansion is unaffected.

## Risks / Trade-offs

- Aggressive size stripping could eat a real name token (rare: names are words, sizes are
  number+unit). Mitigated by requiring a numeric+unit shape.
- 10% tolerance is a heuristic; too tight misses "ca 900 g", too loose conflates 1l/1,5l — 10%
  separates the common Swedish pack sizes cleanly.
