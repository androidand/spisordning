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

- [x] 2.1 Mealie client: fetch recipes (paged list + full detail) via REST API with token auth
      (`internal/mealie`, 2 tests; effort derived from totalTime — ≤25 min low, ≤50 medium,
      else high)
- [x] 2.2 Recipe references normalized in-memory per plan run (`mealie_recipe_id`, lowercased
      tags, ingredient lines, `Raw` snapshot); Postgres persistence lands with the food-brain
      HTTP server — schema is ready in `recipe_ref`/`recipe_ingredient`
- [x] 2.3 Seed `ingredient_mappings` for a small curated recipe set (Swedish units → grams →
      package sizes) + a minimal review surface (CLI or endpoint). Interim: plan uses
      lowercase Mealie food names as canonical ids and the adapter flags low-confidence
      matches for review — seed at `migrations/seed/ingredient_mappings.sql` (4 curated
      dl/msk/tsk/förp → g → package rows, `PLACEHOLDER-*` mealie_food_ids, all needs_review),
      mirrored in-memory at `internal/ingredients/seed.go`; review surface is the
      `food-brain ingredients` CLI command (2 tests; `go build`/`go vet`/`go test` green)

## 3. Suggestion engine (Go, tested)

- [x] 3.1 Deterministic scorer: preferences+confidence, effort vs. day energy, repetition
      penalty, Skolmaten dedup, Willys campaign bias (`internal/scoring`)
- [x] 3.2 Unit tests asserting ranking is deterministic and reproducible with no LLM present
      (9 scorer tests incl. reproducibility across repeated runs)
- [x] 3.3 Olla integration (`internal/llm`, 4 tests): `Explain` (Swedish one-liner grounded in
      the score breakdown) + `ProposeOrder` (reorder within the feasible set only — invented
      ids and infeasible candidates are rejected; unparseable output falls back to scorer
      order, so the LLM is never load-bearing)
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
- [x] 4.4 Skolmaten client (read-only) feeding the scorer's dedup input (`internal/skolmaten`,
      3 tests; mirrors the observed /api/4 shape incl. Client-Token header; meal names
      tokenized into tags with Swedish stopword filtering)
- [x] 4.5 Go-side `internal/retailer` client for the adapter (`ResolveRequirements`,
      `CreateShoppingList`; 4 httptest tests incl. no-retailer-id-in-payload guard)

## 5. End-to-end slice

- [x] 5.1 Wire the pipe: `food-brain plan` (cmd/food-brain/plan.go) — Mealie sync → per-day
      scoring with Skolmaten dedup and in-week repetition avoidance → optional Olla
      explanations → shopping requirements → adapter resolve → wishlist. Dry-run by default;
      `--create-wishlist` applies; needs-review items are never silently added. Covered by an
      end-to-end test against fake Mealie/Skolmaten/Olla/adapter services
      (cmd/food-brain/plan_test.go)
- [ ] 5.2 Surface tonight's meal + one-tap reactions via Home Assistant (through homeops MCP /
      HA API)
- [ ] 5.3 Demote the n8n `weekly-meal-planner` workflow to scheduler/webhook (or retire) once
      the Go pipe is verified

## 6. Verification & docs

- [x] 6.1 `go test ./...` green (28 tests across 8 packages incl. the end-to-end plan test);
      `go vet` clean. In-memory demo: `go run ./cmd/food-brain demo`
- [x] 6.2 Integration smoke: ran `food-brain plan --create-wishlist` against real Mealie (4 ICA recipes) + real Willys; created wishlist "Vecka 30", no cart/payment side effects
- [x] 6.3 README updated with run instructions; architecture decisions reflected
- [x] 6.4 Generated a real week plan and created the Willys wishlist (2026-07-19); owner to eyeball "Vecka 30" in the app. FOUND: confidence model caps correct-but-piece/weight-mismatched matches at 0.65, under the 0.7 review threshold -> wishlist under-filled; see live-findings memory
