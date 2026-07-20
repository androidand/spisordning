# Tasks: review-and-pick

- [x] 1.1 `reviewQueue.ts` (pure, 6 tests): normalized upsert/clear/dismiss/list; fed by
      /resolve (needs-review enters, confident leaves)
- [x] 1.2 `GET /review` picker page (live hits: name/size/price/image; Välj = pin,
      Reserv = backup, Hoppa över = dismiss); `GET /review/queue` JSON;
      `DELETE /review/:term`
- [x] 1.3 Bridge: watch hash includes pins etag (GET /pins per cycle); pins-only change
      re-syncs applying ONLY newly-confident terms (per-mapping synced-term tracking — no
      additive double-adds)
- [x] 1.4 Live verified 2026-07-21: queue populated with Majskolvar + Coca cola zero 1,5 L;
      picker renders live hits incl. Coca-cola Zero Läsk Pet; 182 tests green
- [ ] 1.5 Owner picks their defaults on http://localhost:8402/review and confirms the terms
      sync to the wishlist within a watch cycle
