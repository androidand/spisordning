# Tasks: food-brain-first-slice

## 1. Repo & infra scaffolding

- [x] 1.1 Go module `github.com/androidand/spisordning`, standard layout
      (`cmd/food-brain`, `internal/domain`, `internal/scoring`, `internal/planning`)
- [x] 1.2 docker-compose with Postgres (auto-applies migrations) + willys-adapter (built from
      the willys-client repo via `Dockerfile.adapter`); `.env.example`. food-brain joins the
      compose file when it grows an HTTP server (5.1)
- [x] 1.3 PostgreSQL migrations for the durable schema (`migrations/0001_init.sql`; people,
      person_preferences,
      preference_observations, meal_events, meal_reactions, effort_profiles,
      planning_constraints, meal_plan_candidates, meal_plan_decisions, ingredient_mappings,
      shopping_requirements, retailer_products, product_resolution_rules)

## 2. Mealie sync (read-only)

- [ ] 2.1 Mealie client: fetch recipes + foods/units via REST API
- [ ] 2.2 Store recipe references (`mealie_recipe_id`, normalized ingredients,
      `last_synced_at`, `raw_mealie_snapshot`); no authoritative copies
- [ ] 2.3 Seed `ingredient_mappings` for a small curated recipe set (Swedish units → grams →
      package sizes) + a minimal review surface (CLI or endpoint)

## 3. Suggestion engine (Go, tested)

- [x] 3.1 Deterministic scorer: preferences+confidence, effort vs. day energy, repetition
      penalty, Skolmaten dedup, Willys campaign bias (`internal/scoring`)
- [x] 3.2 Unit tests asserting ranking is deterministic and reproducible with no LLM present
      (9 scorer tests incl. reproducibility across repeated runs)
- [ ] 3.3 Olla integration: vary top-N candidates within constraints + generate explanations;
      reject any variation that violates a hard constraint
- [x] 3.4 Emit canonical shopping requirements (no retailer ids) (`internal/planning`, 5 tests)

## 4. willys-adapter service

- [x] 4.1 HTTP service wrapping `willys-client`; owns session/CSRF/retry/rate-limit +
      `ensureHomeStore()` (lives at `willys-client/apps/willys-adapter`, `npm run adapter`;
      lazy login, serial rate limiter)
- [x] 4.2 `/search`, `/products/:code`, `/campaigns`, `POST /resolve` with confidence +
      needsReview flag (pure resolution core unit-tested: 14 jest tests — display-volume
      parsing, package counts, match scoring, review flagging)
- [x] 4.3 `POST /shopping-lists` → per-week Willys wishlist; separate opt-in
      `POST /shopping-lists/:id/to-cart`; no checkout/payment/slot endpoints exist
- [ ] 4.4 Skolmaten client (read-only) feeding the scorer's dedup input
- [x] 4.5 Go-side `internal/retailer` client for the adapter (`ResolveRequirements`,
      `CreateShoppingList`; 4 httptest tests incl. no-retailer-id-in-payload guard)

## 5. End-to-end slice

- [ ] 5.1 Wire the pipe: Mealie recipe → plan → approve → shopping requirements → resolve →
      wishlist; a `spisordning plan` command or endpoint that runs it
- [ ] 5.2 Surface tonight's meal + one-tap reactions via Home Assistant (through homeops MCP /
      HA API)
- [ ] 5.3 Demote the n8n `weekly-meal-planner` workflow to scheduler/webhook (or retire) once
      the Go pipe is verified

## 6. Verification & docs

- [x] 6.1 `go test ./...` green (14 tests); scorer + requirements covered; `go vet` clean.
      In-memory demo of the pure pipe: `go run ./cmd/food-brain`
- [ ] 6.2 Integration smoke: run the slice against real Mealie/Olla/Skolmaten/Willys with a
      test wishlist; confirm no cart/payment side effects
- [x] 6.3 README updated with run instructions; architecture decisions reflected
- [ ] 6.4 Manual verification by owner: generate a week's plan and review the wishlist in the
      Willys app
