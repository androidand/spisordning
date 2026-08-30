## Context

`internal/recipeimport` already implements the parsing half of discovery:

- `ExtractRecipeJSONLD(html)` finds the schema.org/Recipe node.
- `ParseRecipe(node)` normalizes it into `ParsedRecipe`.
- `CandidateFromParsed(source, url, parsed)` builds a staged candidate.

The database already has the staging tables from `000002_recipe_discovery.sql`:

- `external_recipe_source`
- `recipe_import_candidate`
- `recipe_import_candidate_ingredient`

The native recipe hierarchy already exists and is service-backed:

- `service.RecipeFamily.CreateFamily`
- `service.RecipeFamily.CreateVariant`
- `service.RecipeFamily.CreateRevision`
- `service.RecipeFamily.SetDefaultVariant`

What is missing is the activation layer: persistence for the staging tables, a service that owns
fetch + parse + stage + review + promotion, HTTP/MCP surfaces, and a UI.

## Goals / Non-Goals

**Goals:**
- Make discovery reachable from HTTP, MCP, and the web UI.
- Preserve the review gate: discovery stages a candidate; promotion is explicit.
- Preserve provenance: source URL, source name, external id, license note, attribution, and import
  timestamp remain visible on the candidate and are carried into the promoted variant.
- Make re-discovery idempotent: importing the same URL again does not create a duplicate.
- Keep raw ingredient lines even when they cannot be canonicalized.

**Non-Goals:**
- Not adding site-specific parsers for sources that do not expose schema.org/Recipe JSON-LD.
- Not adding bulk API imports from external recipe providers.
- Not auto-resolving candidate ingredients to canonical ingredients (that remains a review step).
- Not making promoted recipes immediately visible to the Mealie-backed planner (that is
  `unify-recipe-source`).
- Not adding edit-in-place candidate mutation; a candidate is either re-imported, rejected, or
  promoted.

## Decisions

### D1: The service owns fetch + parse + persistence

`service.Discovery` is the only component that calls `http.Get`, `recipeimport.ExtractRecipeJSONLD`,
and `recipeimport.ParseRecipe`. Persistence only stores and reads rows. This keeps the parser and
the HTTP client out of the transport layer and makes the flow testable with an injected
`http.Client`.

### D2: A default `web-jsonld` source is seeded

The migration seeds one source:

```sql
INSERT INTO external_recipe_source (id, name, kind, base_url, license_note, decision, enabled)
VALUES ('web-jsonld', 'Web JSON-LD', 'jsonld_web', NULL, '', 'integrate_now', true)
ON CONFLICT (id) DO NOTHING;
```

`DiscoverFromURL` accepts an optional `source_id`. When omitted, it uses `web-jsonld`. This keeps
the common case a single URL while leaving room for registered API sources later.

### D3: Candidate identity is source-scoped

The existing schema already enforces:

- unique `(source_id, external_id)` when `external_id` is present,
- unique `source_url` when `external_id` is null.

`recipeimport.TrailingID(url)` provides a conservative external id heuristic. Re-discovering the
same URL therefore upserts the same candidate. If the candidate is already `promoted`, re-discovery
returns the existing candidate without overwriting it. If it is `rejected`, re-discovery refreshes
the parsed content and resets status to `candidate` so the user can review it again.

### D4: Promotion creates native recipe content, not Mealie content

`PromoteCandidate` uses the existing `dto.RecipeFamilyService` surface:

1. If the input supplies an existing `family_id`, use it; otherwise create a family from the
   candidate title.
2. Create a variant titled after the candidate, with `source_attribution` set to the source URL
   (and author attribution when present).
3. Create the first revision from the candidate's parsed content:
   - servings,
   - description,
   - steps,
   - ingredient lines (raw text preserved; unresolved lines carry no `ingredient_id`).
4. Set the new variant as the family's default variant when a new family was created.
5. Update the candidate to `status = 'promoted'` and `promoted_variant_id = <variant id>`.

Promotion is idempotent: if the candidate is already promoted, the service returns the existing
family/variant/revision instead of creating duplicates.

### D5: HTTP routes live under `/recipes/discovery`

Using a dedicated prefix avoids colliding with `/recipes/{id}` and makes the surface easy to find:

- `POST /recipes/discover` — body `{ url, source_id? }`
- `GET /recipes/discovery/candidates?status=candidate|promoted|rejected`
- `GET /recipes/discovery/candidates/{id}`
- `POST /recipes/discovery/candidates/{id}/reject`
- `POST /recipes/discovery/candidates/{id}/promote` — body `{ family_id? }`

### D6: MCP tools mirror the HTTP surface

The MCP surface adds five tools:

| Tool | Purpose |
|---|---|
| `discover_recipe` | Fetch + parse + stage a recipe from a URL. |
| `list_import_candidates` | List staged candidates, optionally filtered by status. |
| `get_import_candidate` | Read one candidate with its ingredient lines. |
| `reject_import_candidate` | Reject a candidate. |
| `promote_import_candidate` | Promote a candidate into the native cookbook. |

The tools are thin adapters over `service.Discovery`, matching the existing `mcptools` pattern.

### D7: The UI is a review queue, not an editor

The frontend page shows:

- a URL input to discover a new recipe,
- a candidate list filtered by status,
- a candidate detail view with provenance, parsed fields, and ingredient lines,
- reject and promote actions.

It does not allow free-form editing of candidate content. If the user wants changes, they promote
and then edit the native revision through the existing recipe-family surface.

## Risks / Trade-offs

- **Arbitrary URL fetching is a mild SSRF surface.** This is a self-hosted household tool, but the
  service still validates that the URL scheme is `http` or `https` and uses a short timeout.
  Blocking private IP ranges is a possible hardening follow-up, not a launch blocker.
- **JSON-LD-only discovery.** Sites without a schema.org/Recipe node will fail with a clear error.
  Site-specific parsers are explicitly out of scope.
- **Promoted recipes are not immediately plannable.** They exist in `recipe_family`, but the live
  planner still resolves recipes through Mealie until `unify-recipe-source` lands. This is
  intentional: it keeps discovery activation independent of the larger source unification.
- **Re-discovery of a rejected candidate resets it to `candidate`.** This is a deliberate
  review-queue behavior: the user can change their mind without manual SQL.