# Tasks: establish-clean-layered-architecture

## 1. Architecture foundation

- [x] 1.1 Update `internal/architecturetest/checker.go` to add the `Service`
      layer and classify `internal/service` into it. Rules:
        - service must not import persistence, httpapi, or cmd
        - service may import domain, client, and other service packages
        - httpapi may import domain and service (but not persistence or client)
        - cmd may import everything

      Verified: `Service` layer classifies `internal/service`;
      rules enforced for service→{httpapi,cmd} and httpapi→{persistence,client}
      (client rule added this pass; no existing violations). Deviation from the
      bullet list: service→persistence stays allowed by design — the proposal's
      `Store` interface is defined over persistence row types and tasks 2.2–9.2
      all specify services "backed by `persistence.Store`"; forbidding it would
      require moving persistence row types out of the layer. Added unit tests
      (service→httpapi violation, httpapi→client violation, clean service graph
      incl. subpackage). `go build ./... && go vet ./... && go test ./...` green
      (389 tests).
- [x] 1.2 Define service interfaces in `internal/httpapi/services.go` — these
      are the DI contracts. Each interface lists the methods the HTTP handlers
      and MCP server need.

      Deviation: the DI contracts live in `internal/dto` (the contract layer)
      rather than `httpapi/services.go`. They are consumed by both
      `httpapi.Dependencies` and the first-cut `internal/mcp` server (removed — see
      §10); placing them in `internal/dto` keeps httpapi depending on contracts
      without
      importing service implementations (the checker forbids httpapi→service).
      Eight interfaces: `PersonService`, `PreferencesService`,
      `RecipesService`, `MealsService`, `PantryService`, `PlanningService`,
      `IngredientsService`, `StoresService`.
- [x] 1.3 Create `internal/service/` package skeleton with a `Store` interface
      that every service implementation depends on (repository abstractions).

      `internal/service/service.go` defines the `Store` interface;
      `*persistence.Store` satisfies it. Services implemented against it:
      `People`, `Preferences`, `Recipes`, `Meals` (service.go), `Pantry`
      (pantry.go), `Planning` (planning.go), `Ingredients` (ingredients.go),
      `Stores` (stores.go).
- [x] 1.4 Update `api/openapi.yaml` `x-layer-policy` to include the service
      layer.

      `x-layer-policy` lists `service: internal/service`.

## 2. People service

- [x] 2.1 Define `PersonService` interface in `httpapi` (already exists partially)

      `PersonService` in `internal/dto/people.go` (ListPeople, GetPerson,
      CreatePerson).
- [x] 2.2 Implement `service.People` backed by `persistence.Store`

      `service.People` in `internal/service/service.go`, backed by `Store`.
- [x] 2.3 Wire into `cmd/food-brain/main.go` buildDependencies

      Wired in `cmd/food-brain/adapters.go` buildDependencies →
      `httpapi.Dependencies.People`.
- [x] 2.4 Add tests for service layer (in-memory or test DB)

      `TestPeopleList`/`TestPeopleCreate` (internal/service/service_test.go).

## 3. Preferences service

- [x] 3.1 Define `PreferencesService` interface in `httpapi` (already exists)

      `PreferencesService` in `internal/dto/preferences.go`.
- [x] 3.2 Implement `service.Preferences` backed by `persistence.Store`

      `service.Preferences` in `internal/service/service.go`.
- [x] 3.3 Wire into composition root

      buildDependencies → `httpapi.Dependencies.Preferences`.
- [x] 3.4 Add tests

      `TestPreferencesList` (service_test.go).

## 4. Recipes service

- [x] 4.1 Define `RecipesService` interface in `httpapi`

      `RecipesService` in `internal/dto/recipes.go` (ListRecipes; `GetRecipe`
      added this pass).
- [x] 4.2 Implement `service.Recipes` — wraps `mealie.Client` for fetching,
      `persistence.Store` for storing refs

      `service.Recipes{db, mealie}`: `SyncFromMealie` upserts refs, `GetRecipe`
      maps `pgx.ErrNoRows` → `dto.ErrNotFound`. `Store` gained `UpsertRecipeRef`.
- [x] 4.3 Add `GET /recipes/{id}` endpoint

      Handler in `httpapi/recipes.go`, route `GET /recipes/{id}`, spec path
      `/recipes/{recipeId}` + `RecipeId` param; `dto.ErrNotFound` sentinel added.
- [x] 4.4 Add sync command: `food-brain sync recipes`

      `cmd/food-brain/sync_recipes.go` + `sync` dispatcher in `main.go`; requires
      `MEALIE_BASE_URL`/`MEALIE_API_TOKEN` and a configured DB.
- [x] 4.5 Add tests

      `TestRecipesGet`, `TestRecipesSync` (httptest Mealie, idempotent upsert),
      `TestRecipesSyncNoClient`; handler tests `TestGetRecipe_HappyPath/NotFound/Error`.
      395 tests green.

## 5. Meals service

- [x] 5.1 Define `MealsService` interface in `httpapi`

      `MealsService` in `internal/dto/meals.go`.
- [x] 5.2 Implement `service.Meals` — wraps `persistence.Store` for meal events
      and reactions, calls `service.Preferences` for reaction learning

      `service.Meals` in `internal/service/service.go` — meal events +
      reactions via `Store`; reaction learning goes through
      `dto.PreferencesService` (`NewMeals(db, prefs)`, nil to skip).
- [x] 5.3 Add `GET /meals/{id}` and `GET /meals` endpoints

      Both in `internal/httpapi/meals.go`.
- [x] 5.4 Add tests

      `TestMealsCreate`/`TestMealsGet`/`TestMealsList` (service_test.go).

## 6. Pantry service

- [x] 6.1 Define `PantryService` interface in `httpapi`

      `PantryService` in `internal/dto/pantry.go`.
- [x] 6.2 Implement `service.Pantry` — wraps `persistence.Store` for inventory
      locations, lots, purchases, consumption

      `service.Pantry` in `internal/service/pantry.go`.
- [x] 6.3 Add `GET /pantry/locations`, `POST /pantry/locations`,
      `GET /pantry/lots`, `POST /pantry/lots/purchase`,
      `POST /pantry/lots/{id}/consume` endpoints

      All five in `internal/httpapi/pantry.go`; `GET /pantry/lots` is
      implemented as the location-scoped superset
      `GET /pantry/locations/{id}/lots`.
- [x] 6.4 Add tests

      `TestPantry*` (internal/service/pantry_test.go).

## 7. Planning service

- [x] 7.1 Define `PlanningService` interface in `httpapi`

      `PlanningService` in `internal/dto/planning.go`.
- [x] 7.2 Implement `service.Planning` — orchestrates mealie (candidates),
      scoring, availability, and persistence for meal plans

      `PlanWeek` in `internal/service/planweek.go` runs Mealie sync →
      `planning.PlanWeek` → LLM explain/reorder → `persistWeek`. Persistence is
      non-fatal: failures land in `PlanWeekResult.PersistError`, success in
      `Persisted`.
- [x] 7.3 Add `GET /plans`, `POST /plans`, `GET /plans/{id}`,
      `PATCH /plans/{id}`, `POST /plans/{id}/decisions`,
      `GET /plans/{id}/shopping-requirements`

      All six in `internal/httpapi/plans.go`.
- [x] 7.4 Add `food-brain plan` command using the service

      `cmd/food-brain/plan.go` now calls `service.NewPlanning(db, mc).PlanWeek`;
      the inline Mealie/scoring/persist pipeline and `candidatesFromRefs` were
      removed.
- [x] 7.5 Add tests

      `TestPlanning*` (internal/service/planning_test.go).

## 8. Ingredients / nutrition service

- [x] 8.1 Define `IngredientsService` interface in `httpapi`

      `IngredientsService` in `internal/dto/ingredients.go`.
- [x] 8.2 Implement `service.Ingredients` — wraps `ingredients.Client` (SLV,
      Dabas) for food lookup and nutrition data

      `service.Ingredients` in `internal/service/ingredients.go` — wraps SLV
      (nutrition), Dabas, and MatPrisKollen clients.
- [x] 8.3 Add `GET /ingredients/search`, `GET /ingredients/{id}/nutrition`,
      `GET /ingredients/nutrition/{slvNummer}`

      `searchFood`/`lookupNutrition` handlers already existed; added
      `nutritionByID` (internal/httpapi/ingredients.go) + `GET
      /ingredients/{id}/nutrition` route (people.go). All three paths plus
      `Ingredient`/`IngredientNutrient` schemas added to api/openapi.yaml;
      types regenerated.
- [x] 8.4 Add `food-brain sync nutrition` command

      `runSyncNutrition` in cmd/food-brain/sync_nutrition.go — fetches SLV
      nutrition for one or more nummers and prints JSON (no nutrition table to
      persist into yet). Wired into the `sync` dispatcher (sync_recipes.go).
- [x] 8.5 Add tests

      `TestIngredientsGetMapping`, `TestIngredientsNutritionByID`
      (internal/service/service_test.go), `TestGetIngredientNutrition_*`
      (internal/httpapi/ingredients_test.go).

## 9. Stores / pricing service

- [x] 9.1 Define `StoresService` interface in `httpapi`

      `StoresService` in `internal/dto/stores.go`.
- [x] 9.2 Implement `service.Stores` — wraps `matpriskollen.Client` for store
      search, product search, and offer fetching

      `service.Stores` in `internal/service/stores.go`: product search via the
      MPK client; store/offer reads via the price tables (deviation: MPK exposes
      no store/offer endpoints). Persistence gained `UpsertRetailer`,
      `UpsertStore`, `ListAllStores` in `internal/persistence/price.go`.
- [x] 9.3 Add `GET /stores`, `GET /stores/{id}/offers`,
      `GET /products/search`, `GET /products/by-gtin`

      Handlers in `internal/httpapi/stores.go`, routes in `people.go`, OpenAPI
      paths + `Store`/`StoreOffer`/`IngredientProduct` schemas in
      `api/openapi.yaml` (types regenerated, codegen idempotent).
- [x] 9.4 Add `food-brain sync prices` command

      `cmd/food-brain/sync_prices.go` (`sync prices -retailer ica|willys -store
      <id>`): fetches offers via `retailer.Client.SyncOffers`, upserts
      retailer/store/retailer_product/store_product_offer and appends a
      `price_observation` per offer. Wired into the `sync` dispatcher.
- [x] 9.5 Add tests

      Service: `TestStoresListStores`, `TestStoresListStoreOffers`,
      `TestStoresSearchProductsNoClient` in `service_test.go`. Handlers:
      `TestListStores_HappyPath`, `TestListStoreOffers_HappyPath`,
      `TestListStores_Error` in `handlers_test.go`.

## 10. MCP server v2

- [x] 10.1 Create `internal/mcp/` package with MCP v2 server

      `internal/mcp/server.go` on `modelcontextprotocol/go-sdk`.

      Superseded: the dedicated `implement-mcp-server` change (ADR
      `docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md`) replaced this first-cut
      server with the `internal/mcptools` adapter + `cmd/mcp-server` binary. The
      orphaned `internal/mcp` package (imported nowhere, no tests) was removed in
      that reconciliation, and `internal/architecturetest` no longer classifies it.
- [x] 10.2 Wire the same service interfaces from `httpapi` into the MCP server

      `NewServer` took the same `dto.*Service` DI contracts that
      `httpapi.Dependencies` uses. (Superseded — see 10.1: the production MCP
      server is `internal/mcptools`, which defines its own tool-facing service
      interfaces implemented by the `cmd/mcp-server` composition root.)
- [x] 10.3 Expose tools: `plan_week`, `get_nutrition`, `search_products`,
      `get_store_offers`, `record_reaction`

      Superseded by `implement-mcp-server` / `internal/mcptools` — the names above
      were a draft. The production tool surface (`mcptools.RegisterTools`) is:
      `list_recipe_candidates`, `record_meal_reaction`,
      `get_shopping_requirements`, `create_shopping_list`,
      `compare_shopping_prices`, `push_shopping_wishlist`, `structure_recipe`.
      The application-layer intent (planning, reaction recording, shopping) is met
      by that richer set; nutrition / product-search / store-offer lookups are
      served over the REST API and are not yet MCP tools (future work, not part of
      this superseded change).
- [x] 10.4 Add `food-brain mcp` CLI command

      Superseded by ADR Decision 3 (`docs/adr/mcp-protocol-2026-07-28-and-go-sdk.md`):
      the MCP server is a separate `cmd/mcp-server` binary (Streamable HTTP + stdio),
      not a `food-brain` subcommand. The capability exists as the `mcp-server`
      binary; no `food-brain mcp` subcommand is added.

## 11. Integration tests

- [x] 11.1 Add integration tests that exercise the full stack (HTTP → service
      → persistence) against a test Postgres instance

      `internal/httpapi/integration_test.go` — full-stack HTTP → service →
      persistence against real Postgres (skips cleanly without
      DATABASE_URL/POSTGRES_PASSWORD; CI provides a DB).
- [x] 11.2 Ensure `go test ./...` passes with zero violations

      Gate: `go build ./... && go vet ./... && go test ./...`;
      `internal/architecturetest` enforces zero import violations in the same
      run.

## 12. Documentation

- [x] 12.1 Update `docs/infrastructure/deployment-and-access.md` if needed
- [x] 12.2 Add architecture diagram to `docs/architecture.md`
