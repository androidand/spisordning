# Tasks: establish-clean-layered-architecture

## 1. Architecture foundation

- [ ] 1.1 Update `internal/architecturetest/checker.go` to add the `Service`
      layer and classify `internal/service` into it. Rules:
        - service must not import persistence, httpapi, or cmd
        - service may import domain, client, and other service packages
        - httpapi may import domain and service (but not persistence or client)
        - cmd may import everything
- [ ] 1.2 Define service interfaces in `internal/httpapi/services.go` — these
      are the DI contracts. Each interface lists the methods the HTTP handlers
      and MCP server need.
- [ ] 1.3 Create `internal/service/` package skeleton with a `Store` interface
      that every service implementation depends on (repository abstractions).
- [ ] 1.4 Update `api/openapi.yaml` `x-layer-policy` to include the service
      layer.

## 2. People service

- [ ] 2.1 Define `PersonService` interface in `httpapi` (already exists partially)
- [ ] 2.2 Implement `service.People` backed by `persistence.Store`
- [ ] 2.3 Wire into `cmd/food-brain/main.go` buildDependencies
- [ ] 2.4 Add tests for service layer (in-memory or test DB)

## 3. Preferences service

- [ ] 3.1 Define `PreferencesService` interface in `httpapi` (already exists)
- [ ] 3.2 Implement `service.Preferences` backed by `persistence.Store`
- [ ] 3.3 Wire into composition root
- [ ] 3.4 Add tests

## 4. Recipes service

- [ ] 4.1 Define `RecipesService` interface in `httpapi`
- [ ] 4.2 Implement `service.Recipes` — wraps `mealie.Client` for fetching,
      `persistence.Store` for storing refs
- [ ] 4.3 Add `GET /recipes/{id}` endpoint
- [ ] 4.4 Add sync command: `food-brain sync recipes`
- [ ] 4.5 Add tests

## 5. Meals service

- [ ] 5.1 Define `MealsService` interface in `httpapi`
- [ ] 5.2 Implement `service.Meals` — wraps `persistence.Store` for meal events
      and reactions, calls `service.Preferences` for reaction learning
- [ ] 5.3 Add `GET /meals/{id}` and `GET /meals` endpoints
- [ ] 5.4 Add tests

## 6. Pantry service

- [ ] 6.1 Define `PantryService` interface in `httpapi`
- [ ] 6.2 Implement `service.Pantry` — wraps `persistence.Store` for inventory
      locations, lots, purchases, consumption
- [ ] 6.3 Add `GET /pantry/locations`, `POST /pantry/locations`,
      `GET /pantry/lots`, `POST /pantry/lots/purchase`,
      `POST /pantry/lots/{id}/consume` endpoints
- [ ] 6.4 Add tests

## 7. Planning service

- [ ] 7.1 Define `PlanningService` interface in `httpapi`
- [ ] 7.2 Implement `service.Planning` — orchestrates mealie (candidates),
      scoring, availability, and persistence for meal plans
- [ ] 7.3 Add `GET /plans`, `POST /plans`, `GET /plans/{id}`,
      `PATCH /plans/{id}`, `POST /plans/{id}/decisions`,
      `GET /plans/{id}/shopping-requirements`
- [ ] 7.4 Add `food-brain plan` command using the service
- [ ] 7.5 Add tests

## 8. Ingredients / nutrition service

- [ ] 8.1 Define `IngredientsService` interface in `httpapi`
- [ ] 8.2 Implement `service.Ingredients` — wraps `ingredients.Client` (SLV,
      Dabas) for food lookup and nutrition data
- [ ] 8.3 Add `GET /ingredients/search`, `GET /ingredients/{id}/nutrition`,
      `GET /ingredients/nutrition/{slvNummer}`
- [ ] 8.4 Add `food-brain sync nutrition` command
- [ ] 8.5 Add tests

## 9. Stores / pricing service

- [ ] 9.1 Define `StoresService` interface in `httpapi`
- [ ] 9.2 Implement `service.Stores` — wraps `matpriskollen.Client` for store
      search, product search, and offer fetching
- [ ] 9.3 Add `GET /stores`, `GET /stores/{id}/offers`,
      `GET /products/search`, `GET /products/by-gtin`
- [ ] 9.4 Add `food-brain sync prices` command
- [ ] 9.5 Add tests

## 10. MCP server v2

- [ ] 10.1 Create `internal/mcp/` package with MCP v2 server
- [ ] 10.2 Wire the same service interfaces from `httpapi` into the MCP server
- [ ] 10.3 Expose tools: `plan_week`, `get_nutrition`, `search_products`,
      `get_store_offers`, `record_reaction`
- [ ] 10.4 Add `food-brain mcp` CLI command

## 11. Integration tests

- [ ] 11.1 Add integration tests that exercise the full stack (HTTP → service
      → persistence) against a test Postgres instance
- [ ] 11.2 Ensure `go test ./...` passes with zero violations

## 12. Documentation

- [ ] 12.1 Update `docs/infrastructure/deployment-and-access.md` if needed
- [ ] 12.2 Add architecture diagram to `docs/architecture.md`
