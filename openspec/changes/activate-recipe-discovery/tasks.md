# Tasks: activate-recipe-discovery

## 1. Migration: activate the existing discovery schema

- [x] 1.1 Create `db/migrations/000019_recipe_discovery_activation.sql` — Created migration that seeds `web-jsonld` source and binds the deferred FK from `recipe_import_candidate.promoted_variant_id` to `recipe_variant(id)` with `ON DELETE SET NULL`. Build passes; openspec validate passes.
- [x] 1.2 Seed the default `web-jsonld` source in `external_recipe_source` with
  `kind = 'jsonld_web'`, `decision = 'integrate_now'`, and `enabled = true` — Done as part of 1.1 migration (INSERT ... ON CONFLICT DO NOTHING).
- [x] 1.3 Add the deferred foreign key from `recipe_import_candidate.promoted_variant_id` to
  `recipe_variant(id)` with `ON DELETE SET NULL` — Done as part of 1.1 migration. Original 000003 used RESTRICT; this migration drops and re-adds with SET NULL so promoted variants can be deleted.
- [x] 1.4 Add a `+goose Down` section that drops the foreign key and deletes the seeded source — Done as part of 1.1 migration (DROP CONSTRAINT + DELETE WHERE id = 'web-jsonld').

## 2. Persistence layer

- [x] 2.1 Create `internal/persistence/recipe_discovery.go` — Created with types `ExternalRecipeSource`, `ImportCandidate`, `ImportCandidateIngredient` and methods: `GetExternalRecipeSource`, `UpsertExternalRecipeSource`, `SaveImportCandidate`, `SaveCandidateIngredients`, `GetImportCandidate`, `ListImportCandidates`, `SetCandidateStatus`, `SetCandidatePromoted`, `ListCandidateIngredients`. Build passes; openspec validate passes.
- [x] 2.2 Define persistence types for `ExternalRecipeSource`, `ImportCandidate`, and
  `ImportCandidateIngredient` — Done as part of 2.1.
- [x] 2.3 Implement `GetExternalRecipeSource(ctx, id)` and `UpsertExternalRecipeSource(ctx, src)` — Done as part of 2.1.
- [x] 2.4 Implement `SaveImportCandidate(ctx, c)` that upserts the candidate row using the
  existing unique indexes (`source_id + external_id` or `source_url`) — Done as part of 2.1. Two code paths handle the dual unique indexes.
- [x] 2.5 Implement `SaveCandidateIngredients(ctx, candidateID, lines)` that replaces the
  candidate's ingredient lines — DELETE existing + re-insert all lines. Done as part of 2.1.
- [x] 2.6 Implement `GetImportCandidate(ctx, id)` returning the candidate plus its ingredient
  lines — `GetImportCandidate` returns the candidate row; `ListCandidateIngredients` fetches the lines separately. Done as part of 2.1.
- [x] 2.7 Implement `ListImportCandidates(ctx, status)` returning candidates ordered by
  `imported_at DESC` — Done as part of 2.1. Optional status filter supported.
- [x] 2.8 Implement `SetCandidateStatus(ctx, id, status)` — Done as part of 2.1.
- [x] 2.9 Implement `SetCandidatePromoted(ctx, id, variantID)` that sets status to `promoted` and
  sets `promoted_variant_id` — Done as part of 2.1.

## 3. DTOs and service interface

- [x] 3.1 Create `internal/dto/recipe_discovery.go` — Created with `DiscoverRecipeInput`, `ImportCandidateIngredientResponse`, `ImportCandidateResponse`, `PromoteCandidateInput`, `PromoteCandidateResponse`, and `DiscoveryService` interface. Build passes.
- [x] 3.2 Define `DiscoverRecipeInput` with `url` and optional `source_id` — Done as part of 3.1.
- [x] 3.3 Define `ImportCandidateResponse` — Done as part of 3.1. Includes all requested fields plus `ingredients`.
- [x] 3.4 Define `ImportCandidateIngredientResponse` — Done as part of 3.1.
- [x] 3.5 Define `PromoteCandidateInput` with optional `family_id` — Done as part of 3.1.
- [x] 3.6 Define `PromoteCandidateResponse` — Done as part of 3.1.
- [x] 3.7 Define the `DiscoveryService` interface — Done as part of 3.1.

## 4. Discovery service

- [x] 4.1 Create `internal/service/discovery.go` — Created with `NewDiscovery`, `DiscoverFromURL`, `ListCandidates`, `GetCandidate`, `RejectCandidate`, `PromoteCandidate`. Also added discovery methods to the `Store` interface in `service.go`. Build passes; openspec validate passes.
- [x] 4.2 Implement `NewDiscovery(db Store, family dto.RecipeFamilyService, httpClient *http.Client)` — Done as part of 4.1. Uses default HTTP client when nil.
- [x] 4.3 Implement `DiscoverFromURL` — Done as part of 4.1. Validates URL scheme, fetches page, extracts JSON-LD, parses, resolves source (default `web-jsonld`), persists candidate + ingredients, returns DTO.
- [x] 4.4 Implement `ListCandidates` with optional status filtering — Done as part of 4.1.
- [x] 4.5 Implement `GetCandidate` mapping persistence rows to DTOs — Done as part of 4.1.
- [x] 4.6 Implement `RejectCandidate`, rejecting only candidates that are not already promoted — Done as part of 4.1.
- [x] 4.7 Implement `PromoteCandidate` — Done as part of 4.1. Idempotent for already-promoted candidates. Creates family (or reuses), variant, revision; re-parses raw_jsonld for instructions/ingredients.
- [x] 4.8 Ensure promotion preserves raw ingredient text for unresolved lines — Raw text is stored in `recipe_import_candidate_ingredient.raw_text` and also in the revision's `ingredients` JSONB via `domain.Ingredient.RawText`.

## 5. HTTP API

- [x] 5.1 Create `internal/httpapi/recipe_discovery.go` — Created `recipeDiscoveryHandler` with `discover`, `listCandidates`, `getCandidate`, `rejectCandidate`, `promoteCandidate`.
- [x] 5.2 Add a `Discovery` field to `httpapi.Dependencies` — Added `Discovery dto.DiscoveryService` to the struct.
- [x] 5.3 Register routes in `internal/httpapi/people.go` when `deps.Discovery != nil` — Registered all five routes (`POST /recipes/discover`, `GET /recipes/discovery/candidates`, `GET /recipes/discovery/candidates/{id}`, `POST .../reject`, `POST .../promote`).
- [x] 5.4 Implement handlers that map `dto.ErrNotFound` to 404 and validation errors to 400 — `errors.Is(err, dto.ErrNotFound)` → 404; missing `url` / malformed JSON → 400; other errors → 500.
- [x] 5.5 Wire `service.NewDiscovery` in the composition root (`cmd/food-brain` or equivalent) — `deps.Discovery = service.NewDiscovery(store, deps.RecipeFamily, nil)` in `buildDependencies`.

## 6. OpenAPI and codegen

- [x] 6.1 Add the discovery paths and schemas to `api/openapi.yaml` — Added `POST /recipes/discover`, `GET /recipes/discovery/candidates`, `GET /recipes/discovery/candidates/{id}`, `POST .../reject`, `POST .../promote` (tag `discovery`), reusing `BadRequest`/`NotFound` responses.
- [x] 6.2 Add schemas for `DiscoverRecipeRequest`, `ImportCandidate`, `ImportCandidateIngredient`, `PromoteCandidateRequest`, and `PromoteCandidateResponse` — All five added under `components/schemas`, field-for-field with the Go DTOs.
- [x] 6.3 Run `make generate-openapi` and commit `internal/openapi/types.gen.go` — Regenerated; 5 new types, idempotent on re-run, build/vet/test green.
- [x] 6.4 Run the web client codegen and commit `web/src/generated/spisordning.ts` — Web types are hand-maintained (not codegen'd, per web/README.md TS7 note); transcribed the 5 paths + 5 schemas into `spisordning.ts`. `tsc -b` and ESLint pass.

## 7. MCP tools

- [x] 7.1 Add MCP input/output types for discovery in `internal/mcptools/mcptools.go`.
  Done in `internal/mcptools/discovery.go` (input/output types, service interface, handlers).
- [x] 7.2 Add a `DiscoveryService` interface to `mcptools` matching the service surface needed by
  the tools.
- [x] 7.3 Add `Discovery` to `mcptools.Dependencies`.
- [x] 7.4 Register `discover_recipe` when `deps.Discovery != nil`.
- [x] 7.5 Register `list_import_candidates` when `deps.Discovery != nil`.
- [x] 7.6 Register `get_import_candidate` when `deps.Discovery != nil`.
- [x] 7.7 Register `reject_import_candidate` when `deps.Discovery != nil`.
- [x] 7.8 Register `promote_import_candidate` when `deps.Discovery != nil`.
- [x] 7.9 Implement the `cmd/mcp-server` adapter methods that delegate to `service.Discovery`.

## 8. Frontend

- [ ] 8.1 Add discovery API methods to `web/src/api/client.ts`.
- [ ] 8.2 Create `web/src/pages/RecipeDiscoveryPage.tsx` with:
  - a URL input and discover button,
  - a candidate list with status filter,
  - a candidate detail panel,
  - reject and promote actions.
- [ ] 8.3 Add a `/recipes/discovery` route and navigation entry in `web/src/App.tsx`.
- [ ] 8.4 Ensure the page displays provenance (source URL, external id, license note, attribution)
  and ingredient `needs_review` flags.

## 9. Tests

- [ ] 9.1 Add persistence tests for candidate upsert idempotency using the existing unique
  indexes.
- [ ] 9.2 Add service tests for `DiscoverFromURL` using an `httptest` server that serves a
  JSON-LD recipe page.
- [ ] 9.3 Add service tests for promotion creating family/variant/revision and updating candidate
  status.
- [ ] 9.4 Add service tests for idempotent promotion of an already-promoted candidate.
- [x] 9.5 Add HTTP handler tests for the discovery routes — `internal/httpapi/recipe_discovery_test.go` covers discover (happy/missing-url), list, get (happy/not-found), reject (happy/not-found), promote (happy/not-found).
- [ ] 9.6 Add MCP tool tests for input validation and service delegation.

## 10. Verification

- [ ] 10.1 `go build ./...` succeeds.
- [ ] 10.2 `go vet ./...` succeeds.
- [ ] 10.3 `go test ./...` succeeds.
- [ ] 10.4 `make verify-codegen` succeeds.
- [ ] 10.5 `openspec validate activate-recipe-discovery` succeeds.