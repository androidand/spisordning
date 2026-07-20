# Tasks: pinned-product-resolution

## 1. Pin store core (pure)

- [x] 1.1 `apps/willys-adapter/pins.ts`: store shape, term normalization, lookup
      (searchTerm then ingredientId), alias rewrite for unpinned terms
- [x] 1.2 Resolution-order logic: pinned-primary → pinned-backup → fuzzy(+forced review on
      broken pin), preserving package-count computation from product displayVolume

## 2. Server wiring

- [x] 2.1 Load pins file at startup (env `ADAPTER_PINS_FILE`; gitignored live file;
      committed `product-pins.example.json`)
- [x] 2.2 `/resolve` consults pins with availability check via product detail
- [x] 2.3 `GET /pins` + `POST /pins` (atomic file write; pin usable without restart)

## 3. Tests & verification

- [x] 3.1 Jest: normalization/lookup/alias; pinned/backup/broken-pin resolution order;
      review forced on broken pin
- [x] 3.2 Live: pin "handdiskmedel" → Yes Original 1,25l with Eldorado 1l backup; verify
      /resolve returns the Yes product with matchType "pinned"
- [x] 3.3 Docs: adapter README section + example pins file
