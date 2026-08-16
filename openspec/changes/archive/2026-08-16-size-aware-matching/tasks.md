# Tasks: size-aware-matching

- [x] 1.1 `core.ts`: splitSizeHint (name + size hint) + searchTermFor (size-stripped query)
- [x] 1.2 `core.ts`: score confidence on the name part; sizeMatchBonus prefers the
      size-matching candidate (name score still dominates)
- [x] 1.3 `server.ts`: fuzzy search + picker use the size-stripped query; picker orders hits
      by size match
- [x] 1.4 Jest: split/no-size/size-preference/no-match-fallback; existing tests updated (188 green)
- [x] 1.5 Live verified 2026-07-21: "Coca cola zero 1,5 L" -> 1,5l Pet (was 50cl@0.40 review);
      "mjölk 1,5l" -> Mjölk 3% @ 0.95
