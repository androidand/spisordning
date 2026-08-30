# Tasks: activate-recipe-discovery

## 1. Migration: activate the existing discovery schema

- [ ] 1.1 Create `db/migrations/000019_recipe_discovery_activation.sql`.
- [ ] 1.2 Seed the default `web-jsonld` source in `external_recipe_source` with
  `kind = 'jsonld_web'`, `decision = 'integrate_now'`, and `enabled = true`.
- [ ] 1.3 Add the deferred foreign key from `recipe_import_candidate.promoted_variant_id` to
  `recipe_variant(id)` with `ON DELETE SET NULL`.
- [ ] 1.4 Add a `+goose Down` section that drops the foreign key and deletes the seeded source.

## 2. Persistence layer

- [ ] 2.1 Create `internal/persistence/recipe_discovery.go`.
- [ ] 2.2 Define persistence types for `ExternalRecipeSource`, `ImportCandidate`, and
  `ImportCandidateIngredient`.
- [ ] 2.3 Implement `GetExternalRecipeSource(ctx, id)` and `UpsertExternalRecipeSource(ctx, src)`.
- [ ] 2.4 Implement `SaveImportCandidate(ctx, c)` that upserts the candidate row using the
  existing unique indexes (`source_id + external_id` or `source_url`).
- [ ] 2.5 Implement `SaveCandidateIngredients(ctx, candidateID, lines)` that replaces the
  candidate's ingredient lines.
- [ ] 2.6 Implement `GetImportCandidate(ctx, id)` returning the candidate plus its ingredient
  lines.
- [ ] 2.7 Implement `ListImportCandidates(ctx, status)` returning candidates ordered by
  `imported_at DESC`.
- [ ] 2.8 Implement `SetCandidateStatus(ctx, id, status)`.
- [ ] 2.9 Implement `SetCandidatePromoted(ctx, id, variantID)` that sets status to `promoted` and
  sets `promoted_variant_id`.

## 3. DTOs and service interface

- [ ] 3.1 Create `internal/dto/recipe_discovery.go`.
- [ ] 3.2 Define `DiscoverRecipeInput` with `url` and optional `source_id`.
- [ ] 3.3 Define `ImportCandidateResponse` with id, source id, source URL, external id, title,
  description, image URL, servings, times, category, cuisine, attribution, rating, license note,
  imported at, status, promoted variant id, and ingredients.
- [ ] 3.4 Define `ImportCandidateIngredientResponse` with line number, raw text, quantity, unit,
  ingredient id, and `needs_review`.
- [ ] 3.5 Define `PromoteCandidateInput` with optional `family_id`.
- [ ] 3.6 Define `PromoteCandidateResponse` with family id, variant id, revision id, and candidate
  status.
- [ ] 3.7 Define the `DiscoveryService` interface: `DiscoverFromURL`, `ListCandidates`,
  `GetCandidate`, `RejectCandidate`, `PromoteCandidate`.

## 4. Discovery service

- [ ] 4.1 Create `internal/service/discovery.go`.
- [ ] 4.2 Implement `NewDiscovery(db Store, family dto.RecipeFamilyService, httpClient *http.Client)`.
- [ ] 4.3 Implement `DiscoverFromURL`:
  - validate the URL scheme is `http` or `https`,
  - fetch the page with a timeout,
  - call `recipeimport.ExtractRecipeJSONLD`,
  - call `recipeimport.ParseRecipe`,
  - resolve the source (default `web-jsonld`),
  - call `recipeimport.CandidateFromParsed`,
  - persist the candidate and its ingredient lines,
  - return the stored candidate.
- [ ] 4.4 Implement `ListCandidates` with optional status filtering.
- [ ] 4.5 Implement `GetCandidate` mapping persistence rows to DTOs.
- [ ] 4.6 Implement `RejectCandidate`, rejecting only candidates that are not already promoted.
- [ ] 4.7 Implement `PromoteCandidate`:
  - load the candidate,
  - if already promoted, return the existing linked content,
  - create or reuse the target family,
  - create a variant with source attribution from the candidate,
  - create the initial revision from the candidate's parsed content,
  - set the new variant as default when a new family was created,
  - mark the candidate promoted and link `promoted_variant_id`.
- [ ] 4.8 Ensure promotion preserves raw ingredient text for unresolved lines.

## 5. HTTP API

- [ ] 5.1 Create `internal/httpapi/recipe_discovery.go`.
- [ ] 5.2 Add a `Discovery` field to `httpapi.Dependencies`.
- [ ] 5.3 Register routes in `internal/httpapi/people.go` when `deps.Discovery != nil`:
  - `POST /recipes/discover`
  - `GET /recipes/discovery/candidates`
  - `GET /recipes/discovery/candidates/{id}`
  - `POST /recipes/discovery/candidates/{id}/reject`
  - `POST /recipes/discovery/candidates/{id}/promote`
- [ ] 5.4 Implement handlers that map `dto.ErrNotFound` to 404 and validation errors to 400.
- [ ] 5.5 Wire `service.NewDiscovery` in the composition root (`cmd/food-brain` or equivalent).

## 6. OpenAPI and codegen

- [ ] 6.1 Add the discovery paths and schemas to `api/openapi.yaml`.
- [ ] 6.2 Add schemas for `DiscoverRecipeRequest`, `ImportCandidate`, `ImportCandidateIngredient`,
  `PromoteCandidateRequest`, and `PromoteCandidateResponse`.
- [ ] 6.3 Run `make generate-openapi` and commit `internal/openapi/types.gen.go`.
- [ ] 6.4 Run the web client codegen and commit `web/src/generated/spisordning.ts`.

## 7. MCP tools

- [ ] 7.1 Add MCP input/output types for discovery in `internal/mcptools/mcptools.go`.
- [ ] 7.2 Add a `DiscoveryService` interface to `mcptools` matching the service surface needed by
  the tools.
- [ ] 7.3 Add `Discovery` to `mcptools.Dependencies`.
- [ ] 7.4 Register `discover_recipe` when `deps.Discovery != nil`.
- [ ] 7.5 Register `list_import_candidates` when `deps.Discovery != nil`.
- [ ] 7.6 Register `get_import_candidate` when `deps.Discovery != nil`.
- [ ] 7.7 Register `reject_import_candidate` when `deps.Discovery != nil`.
- [ ] 7.8 Register `promote_import_candidate` when `deps.Discovery != nil`.
- [ ] 7.9 Implement the `cmd/mcp-server` adapter methods that delegate to `service.Discovery`.

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
- [ ] 9.5 Add HTTP handler tests for the discovery routes.
- [ ] 9.6 Add MCP tool tests for input validation and service delegation.

## 10. Verification

- [ ] 10.1 `go build ./...` succeeds.
- [ ] 10.2 `go vet ./...` succeeds.
- [ ] 10.3 `go test ./...` succeeds.
- [ ] 10.4 `make verify-codegen` succeeds.
- [ ] 10.5 `openspec validate activate-recipe-discovery` succeeds.