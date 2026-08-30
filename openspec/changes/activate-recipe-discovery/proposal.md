# Activate recipe discovery

## Why

The repository already contains the two hard pieces of external recipe discovery:

- `internal/recipeimport` can fetch a page's schema.org/Recipe JSON-LD, normalize it into a
  source-agnostic `ParsedRecipe`, and split ingredient lines conservatively.
- `db/migrations/000002_recipe_discovery.sql` already creates `external_recipe_source`,
  `recipe_import_candidate`, and `recipe_import_candidate_ingredient`.

But none of it is reachable. There is no persistence layer for the staging tables, no service,
no HTTP route, no MCP tool, and no UI. A user cannot paste a recipe URL, review the parsed
candidate, or promote it into the native `recipe_family` hierarchy. The discovery capability is
therefore dormant.

This change activates the pipeline end to end while preserving the existing invariant that
external recipes are never auto-imported into the household cookbook.

## What Changes

- Add a migration that activates the existing discovery schema:
  - seeds a default `web-jsonld` source,
  - adds the deferred foreign key from `recipe_import_candidate.promoted_variant_id` to
    `recipe_variant(id)`.
- Add persistence for discovery sources, import candidates, and candidate ingredient lines.
- Add a `Discovery` application service with:
  - `DiscoverFromURL(ctx, url, sourceID)` — fetch, extract JSON-LD, parse, and stage a candidate.
  - `ListCandidates(ctx, status)` — list staged candidates.
  - `GetCandidate(ctx, id)` — read one candidate with its ingredient lines.
  - `RejectCandidate(ctx, id)` — mark a candidate rejected.
  - `PromoteCandidate(ctx, id, in)` — explicitly create the native recipe content and link it
    back to the candidate.
- Add HTTP routes:
  - `POST /recipes/discover`
  - `GET /recipes/discovery/candidates`
  - `GET /recipes/discovery/candidates/{id}`
  - `POST /recipes/discovery/candidates/{id}/reject`
  - `POST /recipes/discovery/candidates/{id}/promote`
- Add OpenAPI schemas and regenerate Go + web client types.
- Add MCP tools:
  - `discover_recipe`
  - `list_import_candidates`
  - `get_import_candidate`
  - `reject_import_candidate`
  - `promote_import_candidate`
- Add a frontend discovery page where a user can submit a URL, inspect candidates, reject them,
  or promote them into the cookbook.

## Capabilities

### Modified Capabilities

- `recipe-discovery`: the existing review-gate capability becomes operational. It gains concrete
  service, HTTP, MCP, and UI surfaces, plus an explicit promotion path into `recipe_family`.

## Impact

- **Affected code:**
  - `db/migrations/000019_recipe_discovery_activation.sql`
  - `internal/persistence/recipe_discovery.go`
  - `internal/dto/recipe_discovery.go`
  - `internal/service/discovery.go`
  - `internal/httpapi/recipe_discovery.go`
  - `internal/httpapi/people.go`
  - `internal/mcptools/mcptools.go`
  - `cmd/mcp-server/adapters.go`
  - `api/openapi.yaml`
  - `internal/openapi/types.gen.go`
  - `web/src/generated/spisordning.ts`
  - `web/src/api/client.ts`
  - `web/src/pages/RecipeDiscoveryPage.tsx`
  - `web/src/App.tsx`
- **No planner changes:** promoted recipes join the native `recipe_family` hierarchy. They do not
  immediately become Mealie planning candidates; that is `unify-recipe-source`'s concern.
- **Depends on:** `rebaseline-recipe-domain` (native recipe family/variant/revision service).
- **Orthogonal to:** `unify-recipe-source` (this change creates native content; the other change
  makes native content the planning source of truth).
- **No changes to:** Mealie sync, retailer adapters, pricing, shopping lists, or deployment.