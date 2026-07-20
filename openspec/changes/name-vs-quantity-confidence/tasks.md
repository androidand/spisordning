# Tasks: name-vs-quantity-confidence

- [x] 1.1 `core.ts`: confidence = name score (uncapped); `quantityUncertain` field; review
      driven by name confidence only
- [x] 1.2 `pins.ts`: pinned resolutions carry `quantityUncertain` consistently
- [x] 1.3 Jest: perfect-name/unknown-size resolves with quantityUncertain; weak name still
      reviews; reconciled sizes report certain; existing tests updated
- [x] 1.4 Go `internal/retailer`: add QuantityUncertain field (optional, non-breaking)
- [x] 1.5 Live verified 2026-07-21: the 9 real note terms went 0/9 -> 7/9 resolved; Majskolvar (0.40) and Coca cola zero (0.40) correctly remain in review
      ones still review
