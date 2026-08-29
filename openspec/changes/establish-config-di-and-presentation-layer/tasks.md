## 1. Config layer

- [x] 1.1 Create `internal/config` with a `Config` struct covering every env var currently read in
      `cmd/food-brain/adapters.go` and `cmd/mcp-server/adapters.go` (`DATABASE_URL`, `MEALIE_BASE_URL`,
      `MEALIE_API_TOKEN`, `SKOLMATEN_*`, `ADAPTER_URL`, `ICA_ADAPTER_URL`, `DABAS_ENABLED`,
      `SLV_BASE_URL`, `MPK_ENABLED`, `OLLA_*`, `SPISORNING_ADDR`, `SPISORNING_MCP_ADDR`).
      **Done:** `internal/config/config.go` defines a `Config` struct with fields for every env var
      read in both composition roots (including `GROCY_BASE_URL`/`GROCY_API_KEY` and
      `HEMKOP_ADAPTER_URL`, which post-date the task's parenthetical). `Load()` reads all env vars
      via `os.Getenv` into one struct; optional integrations are left at zero values when unset.
      Convenience predicates (`HasDatabase`, `HasMealie`, `HasGrocy`, `HasSLV`, `HasSkolmaten`,
      `HasOllama`, `HasWillys`) report presence without failing. `internal/config` is classified as
      a new `Config` layer in `internal/architecturetest/checker.go` (importable by cmd + any layer
      that needs config values; persistence is the only layer that can't import it, which is fine —
      it receives config from the composition root). Verified: `go build ./...` green,
      `go vet ./internal/config/` clean, architecture test passes (13 tests).
- [x] 1.2 `config.Load()` validates required-for-this-command combinations (e.g. `DATABASE_URL`
      required for `serve`/`migrate`) and fails fast with a clear message naming the missing
      variable, before any client constructor runs.
      **Done:** `Config.Validate(command string) error` checks the required-var combinations for
      each food-brain sub-command (`serve`/`migrate`/`tonight`/`ingredients` → database; `plan` →
      Mealie base URL + token; `sync` → database or SLV; `sync-offers` → at least one retailer
      adapter; `demo` → nothing). Returns a clear error naming the specific missing variable(s).
      Also added `MissingVars(command) []string` and `FormatMissing(command) string` helpers for
      actionable error messages and test assertions. The composition root calls `cfg.Validate(cmd)`
      after `Load()` and before any client constructor (wired in tasks 1.4/1.5). Verified:
      `go build ./internal/config/` green, `go vet ./internal/config/` clean.
- [x] 1.3 Unit tests: `Load()` with a full valid env, with required vars missing (asserts the
      specific error), and with optional integrations unset (asserts they report disabled, not an
      error).
      **Done:** `internal/config/config_test.go` — 19 tests covering: `Load()` with a full valid env
      (all fields + all predicates + `Validate` for every command), with optional integrations unset
      (zero values, predicates report false, `Validate(serve)` passes, `Validate(plan)` fails naming
      `MEALIE_BASE_URL`), default values for retailer adapters + server addresses, `Validate` for
      each command with missing required vars (serve/migrate → `DATABASE_URL`, plan →
      `MEALIE_BASE_URL`/`MEALIE_API_TOKEN`, sync → database or SLV, sync-offers → at least one
      adapter, demo → nothing, unknown → nil), `MissingVars`/`FormatMissing` for serve/plan/none,
      and `HasDatabase` via `POSTGRES_*` fields vs. not configured. Verified: `go test
      ./internal/config/` — 19 passed; full `go build ./... && go vet ./... && go test ./...` green
      (526 tests, 28 packages); `cd web && npm run build && npm run lint` green.
- [x] 1.4 Migrate `cmd/food-brain/adapters.go` to build one `Config` and pass its fields into
      existing client constructors — no constructor signatures change, only where their arguments
      come from. `go build ./... && go test ./...` green.
      **Done:** `buildDependencies()` now calls `config.Load()` once at the top and uses `cfg` fields
      (via `HasMealie()`, `HasGrocy()`, `HasSLV()`, `DabasEnabled`, `MPKEnabled`, `HasWillys()`) for
      all optional-client wiring. The `storeAdapter` struct gained an `adapterURL` field (set from
      `cfg.AdapterURL`) so `PushShoppingList` no longer reads env. `persistence.FromEnv(os.Getenv)`
      is retained for the Postgres DSN parsing (it returns a `persistence.Config`, not a
      `config.Config`). No constructor signatures changed. Verified: `go build ./...`, `go vet
      ./...`, `go test ./...` all green (526 tests, 28 packages).
- [x] 1.5 Migrate `cmd/mcp-server/adapters.go` the same way.
      **Done:** `buildMCPDeps()` now calls `config.Load()` once and passes `appCfg.AdapterURL` /
      `appCfg.ICAAdapterURL` / `appCfg.HemkopAdapterURL` into `mcpStoreAdapter` (plus the whole
      `config.Config` as a `cfg` field). `loadSchoolTagsFor` now reads `a.cfg.SkolmatenSchool` /
      `a.cfg.SkolmatenBaseURL` / `a.cfg.SkolmatenClientToken` instead of `os.Getenv`. `main.go`
      reads `cfg.SpisordningMCPAddr` for the listen address instead of `envDefault`, and the now-unused
      `envDefault` helper was removed. `persistence.FromEnv(os.Getenv)` is retained for the Postgres
      DSN parse (it returns a `persistence.Config`). No constructor signatures changed. Verified:
      `go build ./...`, `go vet ./...`, `go test ./...` all green (526 tests, 28 packages); no
      application-config `os.Getenv` remains in the mcp-server composition root (only the
      `persistence.FromEnv(os.Getenv)` DSN parse and a test-file check).
- [x] 1.6 Confirm `internal/architecturetest` passes with `internal/config` added — it should be
      importable by both composition roots and by any layer that needs config values, without
      creating a new forbidden edge.
      **Done:** `internal/config` is classified as the `Config` layer in
      `internal/architecturetest/checker.go` (line 50 const, line 66 prefix). The layer rules allow
      `config` imports from every layer except `domain` and `persistence` (both by design — they
      don't need config values; persistence receives its `persistence.Config` from the composition
      root). Both composition roots (`cmd/food-brain`, `cmd/mcp-server`) now import `internal/config`
      and the real import-graph test (`TestLayeredArchitecture`, which runs `go list -deps` and
      checks every edge) passes — confirming the package is classified and no forbidden edge was
      created. Verified: `go test ./internal/architecturetest/` — 13 passed; full `go build ./... &&
      go vet ./... && go test ./...` green (526 tests, 28 packages).

## 2. Retailer auth tiers

- [x] 2.1 Add an `AuthTier` type (`AuthBasic`/`AuthElevated`) to `internal/retailer`.
      **Done:** `internal/retailer/auth.go` defines `AuthTier` (`AuthBasic`/`AuthElevated`) plus the
      `Operation` enum (`OpResolve`/`OpSearch`/`OpCreateList`/`OpSyncList`/`OpToCart`/`OpBarcode`/
      `OpBonus`/`OpOffers`). Generic over `RetailerKind` — no ICA-specific type, no new `internal/auth`
      package.
- [x] 2.2 Mark ICA's operations: `Resolve` = basic, `CreateShoppingList` = elevated (per
      `expose-shopping-price-and-notes-bridge` D3's finding that anonymous ecom search is never
      stale but the OAuth2 wishlist-push session is).
      **Done:** `Client.TierFor(op)` returns `AuthBasic` for ICA's anonymous ecom surface
      (`OpResolve`/`OpSearch`/`OpBarcode`/`OpOffers`) and `AuthElevated` for the account-bound writes
      (`OpCreateList`/`OpSyncList`/`OpToCart`/`OpBonus`). Willys and Hemköp are single-tier (every op
      `AuthBasic`). `Client` now carries its `kind` (set by `New`/`NewICA`/`NewHemkop`).
- [x] 2.3 Add the elevated-credential file-path field to `Config` (task 1.1); wire it into
      `internal/icaretailer`'s construction instead of any independent env read.
      **Done:** `Config.ICAAuthFile` (env `ICA_AUTH_FILE`) + `HasICAAuth()` predicate. Wired into the
      client's construction via `Client.WithAuthFile(path)` / `Client.AuthFile()` — the client never
      reads the env itself. **Note:** the task named `internal/icaretailer`, but that package is
      imported by no code (the real ICA path is `internal/retailer.Client` via `NewFromKind`), so the
      wiring lives on `retailer.Client` and is applied at every construction site
      (`cmd/food-brain/{plan,sync_offers,sync_prices,push_shopping_list}.go` via `envOr("ICA_AUTH_FILE","")`,
      `cmd/mcp-server/adapters.go` via `a.cfg.ICAAuthFile`). The manual Playwright login stays on the
      TS/ica-adapter side; Go only knows where the credential file is.
- [x] 2.4 Surface elevated-auth staleness as a distinct, typed condition (reusing the 401-detection
      approach from `expose-shopping-price-and-notes-bridge` D3) rather than a generic error.
      **Done:** `httpclient.StatusError` now carries `StatusCode` (message format preserved) so a
      non-2xx is inspectable. `retailer.ErrElevatedStale` sentinel + `IsStaleCredential(code)` (401/403)
      + `IsElevatedStale(err)` (matches `StatusError` status or a wrapped sentinel). Detection keys off
      the status code, not "did it throw" — a 502 (catchable parse-error path) is not stale.
- [x] 2.5 Unit tests: a basic-tier call proceeds without the elevated credential present; an
      elevated-tier call with a missing/stale credential reports the typed staleness condition.
      **Done:** `internal/retailer/auth_test.go` — tier declaration (ICA + single-tier), `IsStaleCredential`,
      `IsElevatedStale` (StatusError / sentinel / end-to-end 401-vs-502), `WithAuthFile`/`AuthFile`, and
      `TestBasicTierProceedsWithoutCredential` (resolve succeeds with no credential wired).
      `internal/config/config_test.go` — `ICA_AUTH_FILE` parse + `HasICAAuth` (set/unset). Verified:
      `go build ./... && go vet ./... && go test ./...` green (536 tests, 28 packages);
      `go test ./internal/architecturetest/` still passes (no new forbidden edge).
- [x] 2.6 Document the manual elevated-login handoff (who runs the Playwright login, where the
      resulting credential file goes) in `docs/infrastructure/` — today this has no documented
      location at all.
      **Done:** `docs/infrastructure/ica-elevated-login.md` documents the handoff end-to-end: what the
      elevated credential is (ICA mobile OAuth2/PKCE Bearer session; anonymous ecom is never stale), who
      runs the login (a human, Playwright-driven / human-in-the-loop, on the TS ica-adapter side), where
      the credential file goes (`ICA_AUTH_FILE` → `Config.ICAAuthFile`, `0600`, never committed), how
      staleness is detected (401/403 keyed off status, not "did it throw" — with the silent-false-success
      caveat and the required TS-side `res.ok` guard), and a quick-reference symptom→fix table.

## 3. SSE progress streaming

- [x] 3.1 Add an SSE endpoint to `internal/httpapi` streaming progress for a running plan/resolve
      operation (event per item as `cmd/food-brain/plan.go`'s resolve loop progresses).
      → `POST /plans/run/stream` (`internal/httpapi/progress.go`) streams `text/event-stream`
      progress events. `RunPlanInput.Progress` callback in `cmd/food-brain/plan.go` emits
      phase events (started/planning/resolving/wishlist/done). Per-item events are not
      possible today: `ResolveRequirements` is a single blocking adapter HTTP call, so the
      Go side only observes phase boundaries.
- [x] 3.2 Confirm the existing synchronous plan endpoint is unaffected — SSE is additive.
      → `POST /plans/run` unchanged; `TestRunPlanStream_SyncEndpointUnaffected` + existing
      `TestRunPlan_*` all pass.
- [x] 3.3 Integration test: drive a plan/resolve against a fake slow adapter and assert progress
      events arrive incrementally, not all at once at the end.
      → `TestRunPlanStream_ProgressiveEvents` (`internal/httpapi/progress_test.go`) drives a
      real `httptest.Server` against `fakePlansSvc` with a 60ms `progressDelay`, reads the SSE
      stream incrementally, and asserts the first→last event gap ≥ 120ms (not a burst).
- [ ] 3.4 Sequence this after task 4 lands a real consumer (design.md's migration plan) — don't
      finalize the event payload shape until the frontend's first slice needs it.
      → Payload kept minimal (`PlanProgress{phase,message,at}`) per this constraint; no frontend
      SSE consumer added yet. OpenAPI + generated-client `PlanProgress` documented, not finalized.

## 4. Frontend first slice

- [x] 4.1 Scaffold `web/`: Vite + React + TypeScript. **Deviation from the original "match
      tengil/web-ui (React 18)" note:** the user mandated the *latest* stack, so this uses React
      19.2.8, Vite 8.2.2, TanStack Query 5.102.8, openapi-fetch 0.17.0, ESLint 9 (flat config), and
      **TypeScript 7.0.2** (the native/Go compiler) as the project compiler. Files:
      `web/package.json`, `web/vite.config.ts` (React plugin + `/api` → `:8080` proxy),
      `web/tsconfig.json` / `web/tsconfig.app.json` / `web/tsconfig.node.json`,
      `web/eslint.config.js`, `web/index.html`, `web/.gitignore`.
- [x] 4.2 Typed client from `api/openapi.yaml`. **Deviation — hand-written, not codegen'd:**
      `openapi-typescript` generates types by calling the TypeScript compiler API
      (`ts.factory.*`), which TS 7.0 does not expose as a JS module (upstream
      `openapi-ts/openapi-typescript#2841`; TS 7 support deferred to TS 7.1). Per the user's
      "TS 7 only, no older TS" directive, the client types are hand-transcribed from the spec into
      `web/src/generated/spisordning.ts` (schemas + `paths`, mirroring the wire shape in
      `internal/openapi/types.gen.go`). Path-level OpenAPI parameters are inlined into each
      operation's `parameters` so openapi-fetch resolves `path` params correctly. The file header
      documents how to re-sync it when the spec changes.
- [x] 4.3 Full multi-page frontend (expanded from the original read-only week view). `web/src/App.tsx`
      is a `react-router-dom` (HashRouter) shell with a sidebar nav and nine pages under
      `web/src/pages/`, each a TanStack Query + openapi-fetch consumer of the real REST API at
      `VITE_API_URL` (default `http://localhost:8080`) — no mock data:
      - `PlannerPage` — list/create plans, run the planner (`POST /plans/run`), week view, pick
        candidates per day, approve (`PATCH /plans/{planId}`), commit decisions
        (`POST /plans/{planId}/decisions`), and show shopping requirements.
      - `ShoppingPage` — lists, add items (manually or via `GET /ingredients/search`), check off,
        delete, push to a retailer (`POST /shopping-lists/{listId}/push`).
      - `ComparePage` — cheapest grocery bag across Willys/ICA via `POST /compare` (see 4.7);
        per-item prices, cheapest retailer, and bag totals.
      - `RecipesPage` — browse/filter the recipe library (variants shown as "not available yet").
      - `PreferencesPage` — read per-person preferences (editing shown as "not available yet").
      - `PantryPage` — locations, stock lots, record purchases, consume.
      - `OrdersPage` — browse orders + items. `TonightPage` — tonight's meal + one-tap reactions.
      - `SyncPage` — status of external sync sources (Mealie/offer triggers shown as "not available
        yet"; the Apple Notes checklist bridge is noted as live).
      Supporting files: `web/src/main.tsx` (React 19 `createRoot` + `QueryClientProvider` +
      `HashRouter`), `web/src/api/client.ts` (openapi-fetch client), `web/src/components/ui.tsx`
      (shared primitives), `web/src/lib/format.ts` (price/date/quantity formatting),
      `web/src/index.css` (design system), `web/src/vite-env.d.ts`.
- [x] 4.4 Write actions wired where the API supports them (Planner, Shopping, Compare, Pantry,
      Tonight). Features with no backend endpoint — recipe variants, preference editing, and
      Mealie/offer sync triggers — are rendered as explicit "not available yet" states, not faked.
- [x] 4.5 `web/README.md` documents the full feature set and running locally against `food-brain serve`.
- [x] 4.7 **New backend endpoint — `POST /compare` (cross-retailer price comparison).** The
      comparison logic already existed in `internal/retailer/compare.go` (`Compare`) and was exposed
      only to MCP (`compare_shopping_prices`); no HTTP route surfaced it. Added, following the
      layered architecture: `internal/httpapi/compare.go` defines the `PriceComparisonService`
      interface + response DTOs (`CompareInput`/`CompareRequirement`/`PriceComparison`/
      `ItemComparison`/`RetailerPriceResult`) and the `compareHandler`, registered as `POST /compare`
      in `RegisterHandlers` (nil-guarded like the other optional routes). The composition root
      (`cmd/food-brain/adapters.go`) supplies `priceComparisonAdapter`, which maps
      `httpapi.CompareRequirement` → `domain.ShoppingRequirement`, calls `retailer.Compare`, and maps
      the result to the HTTP wire shape — the sole edge that knows both the retailer client and the
      httpapi DTOs (enforced by `internal/architecturetest`). Wired only when `ADAPTER_URL` is set,
      matching the optional-client convention.
- [x] 4.6 **TS 7 side-by-side toolchain (new, required by 4.1's TS 7 choice):** TS 7.0 ships no
      compiler API, so tooling that needs it (typescript-eslint) can't run against it. Per the
      official TS 7.0 announcement ("Running Side-by-Side with TypeScript 6.0"), `web/package.json`
      uses npm aliases: `typescript` → `@typescript/typescript6` (TS 6.0 API, for typescript-eslint)
      and `typescript7` → `typescript@7.0.2` (the native `tsc` used for the build). `npm run build`
       invokes `node node_modules/typescript7/bin/tsc -b` so the **build is type-checked by TS 7**,
       while `npm run lint` uses the TS 6 API. No older TS is the project compiler.
- [x] 4.8 **Iteration 2 — four more backend capabilities + frontend pages (all layered, all tested).**
       The persistence layer already had the schema + domain for these; the gap was the HTTP surface.
       Each was added following the repo's layered architecture (persistence → service → dto →
       httpapi → composition root), with the architecture test still passing:
       - **Recipe families (git-like recipe hierarchy)** — the schema
         (`migrations/000003_recipe_family.sql`) and in-memory domain
         (`internal/recipefamily`) existed but had **no persistence and no HTTP**. Added
         `internal/persistence/recipe_family.go` (Create/Get/List for family/variant/revision,
         `SetRecipeFamilyDefaultVariant`, `AddRecipeRevisionParent`, `ListRecipeRevisionParents`;
         JSONB encode/decode for `ingredients`/`steps`; `pgx.ErrNoRows` on miss, per convention).
         `internal/service/recipe_family.go` implements `dto.RecipeFamilyService` (maps
         `pgx.ErrNoRows` → `dto.ErrNotFound`, slugifies ids, enforces the "default variant resolves
         within its own family" invariant). `internal/dto/recipe_family.go` holds the wire DTOs +
         interface. `internal/httpapi/recipe_family.go` + registration expose
         `GET/POST /recipe-families`, `GET/POST /recipe-families/{id}/variants`,
         `GET/POST …/variants/{variantId}/revisions`, `GET …/revisions/{revisionId}`, and
         `POST …/variants/{variantId}/default`. Wired in `cmd/food-brain/adapters.go`.
       - **Favorites + ratings** — persistence already existed (`UpsertFavorite`/`DeleteFavorite`/
         `ListFavoritesForRecipe`/`GetRecipeRating` in `internal/persistence/meals.go`); no HTTP.
         Added `internal/service/favorites.go` (implements `dto.FavoritesService`),
         `internal/dto/favorites.go`, and `internal/httpapi/favorites.go` exposing
         `GET/POST/DELETE /recipes/{id}/favorites` and `GET /recipes/{id}/rating`.
       - **Price intelligence reads** — `ListCurrentPrices` (the
         `current_store_product_price` view) existed; no HTTP. Added
         `internal/service/prices.go` (implements `dto.PriceIntelligenceService`, joins
         store/retailer names, groups by retailer product, computes the cheapest store per product),
         `internal/dto/prices.go`, and `internal/httpapi/prices.go` exposing `GET /prices`.
       - **Best-before notifications** — added `Store.ListExpiringLots` in
         `internal/persistence/pantry.go` (non-empty lots with `best_before <= now+within`, most
         urgent first), a `ListExpiring` method on the Pantry service, and `GET /pantry/expiring`
         (`?withinHours`, default 168) in `internal/httpapi/pantry.go`.
       - **Store locator (closest store)** — the `store` table had no position, so added
         `migrations/000015_store_geo.sql` (nullable `latitude`/`longitude` WGS84 columns; no
         PostGIS — the household's store set is tiny, so a plain scan + in-process haversine is
         right). Extended `domain.Store` with `*float64` geo fields, and the persistence store
         queries (`CreateStore`/`GetStore`/`ListStores`/`UpsertStore`/`ListAllStores`) to read/write
         them. Rather than a separate service, the existing `Stores` service
         (`internal/service/stores.go`) gained a `LocateStores(ctx, LocateStoresInput)` method
         (haversine great-circle distance, nearest-first with geo-less stores last; ordered by
         retailer+name when no origin) and `ListStores` now delegates to it, enriching every store
         with `retailer_name`, `latitude`, `longitude`, and `distance_km`. The `GET /stores` handler
         (`internal/httpapi/stores.go`) parses optional `?latitude`/`?longitude` (400 on a
         non-number). `dto.Store` and `dto.StoresService` extended accordingly.
       - **Dashboard / widgets** — added `GET /widgets/dashboard?householdId=` which aggregates
         tonight's meal, a pantry summary (locations/lots/expiring counts), and the top-5 expiring
         lots in one round-trip (the "widgets" use case). `internal/service/dashboard.go`
         (`Dashboard`, implements `dto.DashboardService`) depends on a `TonightProvider` +
         `PantryProvider` (narrow interfaces) + the `Store` (for lot counts). To keep the service
         layer from importing httpapi (architecture rule), **moved `TonightView` and
         `ErrNoMealTonight` from `internal/httpapi` to `internal/dto`** (they are wire types
         composed of dto types); httpapi now references `dto.TonightView`/`dto.ErrNoMealTonight`,
         and the composition root's `storeAdapter.GetTonight` returns `dto.TonightView`. The
         composition root adds a `dashboardTonightProvider` adapter that maps the adapter's
         `dto.ErrNoMealTonight` → the same sentinel so the dashboard treats "no meal" as empty, not
         an error. `dto/dashboard.go` holds the wire DTOs.
       - **Barcode scanner** — the backend `GET /products/by-gtin` (Matpriskollen) already existed;
         added `BarcodePage` (manual GTIN entry + optional camera scan via the
         `BarcodeDetector` API when available) wired to it.
       - **Ingredient nicknames (configurable nickname matching)** — added
         `migrations/000016_ingredient_alias.sql` (`ingredient_alias`: unique
         `(household_id, alias)` → `ingredient_id`; NULL household = global alias).
         Persistence in `internal/persistence/catalog.go` (`UpsertIngredientAlias`,
         `GetIngredientAlias`, `ListIngredientAliases` (incl. globals),
         `DeleteIngredientAlias`, `ResolveIngredientAlias` — household alias wins over global).
         `internal/service/ingredient_alias.go` (`IngredientAlias`, implements
         `dto.IngredientAliasService`) normalizes aliases (lowercase+trim) and returns
         `dto.ErrInvalidAlias` (→ 400) on missing fields. `internal/httpapi/ingredient_alias.go`
          exposes `GET/POST /ingredient-aliases`, `DELETE /ingredient-aliases/{alias}`, and
          `GET /ingredient-aliases/resolve/{alias}`. `dto/ingredient_alias.go` holds the wire DTOs +
          the `ErrInvalidAlias` sentinel.
        - **Preferences editing (personal preferences)** — the persistence layer already had
          `UpsertPreference`; exposed it. `dto/preferences.go` gained `SetPreferenceInput` +
          `ErrInvalidPreference` and the `PreferencesService` interface gained `SetPreference`.
          `service.Preferences.SetPreference` validates (person_id+tag required, sentiment in
          [-2,2], confidence in [0,1]) then upserts and re-reads the authoritative row.
          `httpapi/preferences.go` adds `POST /preferences` (400 on `ErrInvalidPreference`).
        - **Person/profile editing (user profiles)** — `persistence.UpdatePerson` (updates name;
          weight only when > 0; returns `pgx.ErrNoRows` on miss). `dto/people.go` gained
          `PersonUpdate` + `PersonService.UpdatePerson`. `service.People.UpdatePerson` maps
          `pgx.ErrNoRows` → `dto.ErrNotFound` (→ 404) and requires a non-empty name.
          `httpapi/people.go` adds `PATCH /people/{id}` (400 on empty name, 404 on miss).
        - **Inspiration ("what can I make from my pantry")** — `persistence.ListPantryIngredientIDs`
          (distinct ingredient ids with quantity > 0) and `persistence.ListAllRecipeIngredients`
          (all recipe_ingredient rows). `dto/inspiration.go` holds `InspirationSuggestion` +
          `InspirationService`. `service/inspiration.go` (`Inspiration`) joins the pantry's
          ingredient ids with each recipe's ingredient lines, scores by match ratio, omits recipes
          with nothing in common, and ranks most-matched-first (title tiebreak).
          `httpapi/inspiration.go` exposes `GET /inspiration`.
        - **OpenAPI + typed client** — `api/openapi.yaml` documents all new routes + schemas
          (RecipeFamily/Variant/Revision/Ingredient, Favorite/RecipeRating, ProductPriceGroup/
          StorePrice, PersonUpdate, PersonPreferenceNew, InspirationSuggestion);
          `web/src/generated/spisordning.ts` mirrors them (hand-transcribed, per 4.2).
       - **Frontend pages** — `DashboardPage` (the new root `/`: tonight's meal, pantry stat
         cards, expiring-soon list), `RecipeFamilyPage` (family → variants → revisions, create
         family/variant/revision, set default variant, show revision parentage), `PricesPage`
         (cheapest store per product, per-store price rows), `StoreLocatorPage` (origin input +
         "use my location" geolocation, stores ranked nearest-first with distance labels),
         `BarcodePage` (GTIN entry + camera scan → Matpriskollen product lookup),
          `AliasesPage` (add/remove household ingredient nicknames, "potatis → potato"),
          `InspirationPage` (recipes ranked by pantry coverage, % on hand + what's still needed),
          favorites + rating on `RecipesPage` (toggle favorite, star rating), a "Best before"
          section on `PantryPage` (expired vs. expiring-soon lots), an "Edit profile" form on
          `PreferencesPage` (rename a person / change their weight via `PATCH /people/{id}`), and a
          working "Edit preferences" form (set a tag's sentiment + confidence via
          `POST /preferences`, replacing the old "not available yet" placeholder). New nav entries +
          routes in `App.tsx`; new CSS in `index.css`; `daysUntil`/`expiryLabel` helpers in
          `lib/format.ts`.
        - **Tests** — new service unit tests (recipe family not-found mapping, slugify via
          `CreateFamily`, favorites rating, price cheapest-store computation, pantry expiring,
          preference validation + upsert, person update not-found/required-name, inspiration
          ranking + no-match omission + empty pantry) and fakeStore/fakePantrySvc/dbAdapter/fake
          service updates to satisfy the widened interfaces.

- [x] 4.9 **Iteration 2 — Grocy integration (self-hosted inventory) + Hemköp retailer (price comparison).**
        Two more external integrations, both following the repo's layered architecture:
        - **Grocy (self-hosted inventory/stock/shopping-list)** — the repo had no Grocy code
          (pure research target per `PLAN.md`). Added, end-to-end:
          - `internal/grocy/client.go` — a Grocy REST client (products/stock/shopping-list reads +
            add-stock/consume-stock/add-shopping-item writes + `Ping` via `/api/system/info`),
            authenticating with the `GROCY-API-KEY` header via the shared `httpclient`.
          - `internal/dto/grocy.go` — wire DTOs (`GrocyProduct`, `GrocyStockEntry`,
            `GrocyShoppingItem`, `GrocyStatus`) + the `GrocyService` interface.
          - `internal/service/grocy.go` — the `Grocy` service + `ErrGrocyNotConfigured` sentinel.
            A nil client (no `GROCY_BASE_URL`) degrades to `ErrGrocyNotConfigured` (→ 503).
            `ListStock` enriches entries with product names and filters zero-amount lots;
            `AddStock` validates `best_before` as `YYYY-MM-DD`.
          - `internal/httpapi/grocy.go` — `grocyHandler` + `writeGrocyError`
            (`ErrGrocyNotConfigured` → 503, other → 502). Registered in `people.go`'s
            `Dependencies` + `RegisterHandlers`: `GET /grocy/status`, `GET /grocy/products`,
            `GET /grocy/stock`, `GET /grocy/shopping-list`, `POST /grocy/stock/add`,
            `POST /grocy/stock/consume`, `POST /grocy/shopping-list/items`.
          - `cmd/food-brain/adapters.go` — composition root wires `service.NewGrocy(client, baseURL)`
            when `GROCY_BASE_URL` is set, else a nil client (not configured).
          - `internal/architecturetest/checker.go` — `internal/grocy` classified into the `client`
            layer (it is an external API client, like `mealie`/`retailer`/`icaretailer`).
          - Frontend: `GrocyPage` (status badge, stock list with best-before, shopping list,
            add-free-text-item form) + nav entry + route in `App.tsx`.
          - OpenAPI: 7 `/grocy/*` paths + 4 Grocy schemas in `api/openapi.yaml`;
            `web/src/generated/spisordning.ts` mirrors them.
          - Tests: `internal/service/grocy_test.go` (fake Grocy httptest server: status
            configured/not-configured, list products, list stock filters-zero + names, list
            shopping list, add stock, add stock bad date, add stock not-configured, consume stock,
            add shopping item) + `internal/httpapi/handlers_test.go` (`fakeGrocySvc` +
            `TestGrocyStatus_NotConfigured` 503, `TestGrocyListProducts_HappyPath`,
            `TestGrocyAddStock_HappyPath`, `TestGrocyAddStock_NotConfigured` 503).
        - **Hemköp (third retailer for cross-retailer price comparison)** — Hemköp and Willys share
          one SAP Commerce (Axfood) backend, so the hemkop-adapter (sibling store-clients repo, not
          yet built) mirrors the willys-adapter's HTTP shape. The spisordning side:
          - `internal/retailer/client.go` — added `RetailerHemkop` kind + `NewHemkop` constructor
            (error prefix "hemkop-adapter"); `NewFromKind` now takes a third `hemkopURL` param.
          - `internal/retailer/compare.go` — `RetailerOrder` extended to
            `[willys, ica, hemkop]`; `Compare` takes a third `hemkopURL` param and resolves each
            requirement against all three retailers in parallel (graceful degradation: an
            unreachable hemkop-adapter is marked unavailable, not an error).
          - Call sites updated to pass `HEMKOP_ADAPTER_URL` (default `http://localhost:8404`):
            `cmd/food-brain/adapters.go` (`priceComparisonAdapter`), `cmd/mcp-server/adapters.go`
            (`mcpStoreAdapter`), `cmd/mcp-server/main.go`, `cmd/food-brain/plan.go`,
            `cmd/food-brain/push_shopping_list.go`, `cmd/food-brain/sync_offers.go`,
            `cmd/food-brain/sync_prices.go`.
          - `internal/retailer/compare_test.go` — updated for the third retailer (3 results,
            hemkop unavailable when no adapter URL).
          - Note: the hemkop-adapter itself is a store-clients repo task (it must be built before
            Hemköp resolves live); this client is the spisordning side, gated on the adapter landing
            — the same convention as the ICA client.

### Verification (task 4)

- `cd web && npm run build` — green (TS 7 `tsc -b` type-check + Vite 8 production build, 90 modules).
- `cd web && npm run lint` — green (ESLint 9 flat config, typescript-eslint on the TS 6 API; 0
  errors, 4 pre-existing `react-hooks/exhaustive-deps` warnings on derived-from-query data; the new
  pages add none).
- `go build ./... && go vet ./...` — green (includes the new recipe-family, favorites, prices,
  best-before, store-locator, dashboard, ingredient-alias, preference-edit, person-edit, inspiration,
  grocy, and hemkop endpoints + the `000015_store_geo` and `000016_ingredient_alias` migrations +
  the `TonightView`/`ErrNoMealTonight` move from httpapi to dto + the `internal/grocy` client-layer
  classification).
- `go test ./...` — green (507 tests, 27 packages), including the architecture-test job (the grocy
  client is classified as `client`; the grocy service depends only on dto + the grocy client, never
  httpapi) and the new grocy service + handler tests + the hemkop retailer comparison tests.
- Manual run against a live `food-brain serve` is task 5.2's acceptance (needs the backend up).

## 5. Verification & docs

- [x] 5.1 `go build ./... && go test ./... && go vet ./...` green, including the architecture-test
      job, after all Go-side tasks (1–3) land.
      → All green: `go build` Success, `go vet` no issues, `go test` 538 passed in 28 packages,
      architecture-test 13 passed. No new forbidden edges.
- [ ] 5.2 `web/` builds and runs against a local backend (manual check, task 4.3's acceptance).
      → Build verified (`npm run build` + `npm run lint` green, TS 7 type-check passes). The
      "runs against a local backend" half is a manual browser check with `food-brain serve` up —
      not automatable here; left as the acceptance gate.
- [x] 5.3 Update `docs/research/current-state.md` to reflect the new `internal/config`,
      `AuthTier` concept, SSE endpoint, and `web/` frontend's existence.
      → Added a "Presentation layer" section covering `internal/config`, `AuthTier`/`TierFor`,
      the SSE endpoint, and the `web/` frontend; updated the layout tree, the Grocy entry (now
      real), the date, and the AGENTS.md/docs note.
