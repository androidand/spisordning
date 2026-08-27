## 1. Config layer

- [x] 1.1 Create `internal/config` with a `Config` struct covering every env var currently read in
      `cmd/food-brain/adapters.go` and `cmd/mcp-server/adapters.go` (`DATABASE_URL`, `MEALIE_BASE_URL`,
      `MEALIE_API_TOKEN`, `SKOLMATEN_*`, `ADAPTER_URL`, `ICA_ADAPTER_URL`, `DABAS_ENABLED`,
      `SLV_BASE_URL`, `MPK_ENABLED`, `OLLA_*`, `SPISORNING_ADDR`, `SPISORNING_MCP_ADDR`).
      (Done: `internal/config/config.go`. Scope grew beyond the two `adapters.go` files listed —
      a full grep found `os.Getenv` calls scattered across 7 files total
      (`cmd/food-brain/{adapters,main,plan,sync_recipes,sync_nutrition,sync_offers,sync_prices}.go`
      and `cmd/mcp-server/{adapters,main}.go`); all covered. `DATABASE_URL` deliberately excluded —
      see 1.2's note, `internal/persistence.FromEnv` already owns DB config cleanly and duplicating
      it would be two copies of the same parsing logic.)
- [x] 1.2 `config.Load()` validates required-for-this-command combinations (e.g. `DATABASE_URL`
      required for `serve`/`migrate`) and fails fast with a clear message naming the missing
      variable, before any client constructor runs.
      (Refined during implementation — see design.md D1: `Load()` never fails; it gathers values
      with defaults, and each call site validates what it needs via helpers like
      `Config.MealieEnabled()`/`Config.OllaEnabled()`, exactly mirroring the `!= ""` checks each
      call site already had before this change. Centralizing validation in `Load()` would require
      it to know every command's requirements — rejected as the wrong coupling. `DATABASE_URL`
      validation stays entirely in `persistence.FromEnv`, unchanged.)
- [x] 1.3 Unit tests: `Load()` with a full valid env, with required vars missing (asserts the
      specific error), and with optional integrations unset (asserts they report disabled, not an
      error).
      (Done: `internal/config/config_test.go` — `TestLoad_Defaults`, `TestLoad_
      OverridesAndOptionalIntegrations`, `TestLoad_PartialMealieIsNotEnabled`,
      `TestEnvBool_ExplicitFalseStaysDisabled`. Uses `t.Setenv` for isolation.)
- [x] 1.4 Migrate `cmd/food-brain/adapters.go` to build one `Config` and pass its fields into
      existing client constructors — no constructor signatures change, only where their arguments
      come from. `go build ./... && go test ./...` green.
      (Done — and extended to every `os.Getenv` call site in the `food-brain` binary, not just
      `adapters.go`: `main.go` (`SPISORNING_ADDR`), `plan.go` (Mealie/adapter-URLs/Skolmaten/Olla/
      school-flag-default), `sync_recipes.go`, `sync_nutrition.go`, `sync_offers.go`,
      `sync_prices.go`. Two now-dead helper functions removed (`envDefault` in `main.go`, `envOr`
      in `plan.go`) once nothing referenced them.)
- [x] 1.5 Migrate `cmd/mcp-server/adapters.go` the same way.
      (Done — `main.go` (`SPISORNING_MCP_ADDR`, adapter URLs) and `adapters.go` (Skolmaten) both
      migrated; dead `envDefault` helper removed from `main.go`; unused `"os"` import removed from
      `adapters.go` once its only use was gone.)
- [x] 1.6 Confirm `internal/architecturetest` passes with `internal/config` added — it should be
      importable by both composition roots and by any layer that needs config values, without
      creating a new forbidden edge.
      (Done: added a `Config` layer to `internal/architecturetest/checker.go` with two new rules —
      "config must import only domain and external" and "only cmd may import config" (clients/
      services/httpapi take plain constructor args instead, per design.md's actual pattern:
      `internal/config` has zero non-stdlib imports and is consumed only by `cmd/food-brain` and
      `cmd/mcp-server`). `go build ./... && go vet ./... && go test ./...` — 455 passed, 26
      packages, architecture test green.)

## 2. Retailer auth tiers

- [x] 2.1 Add an `AuthTier` type (`AuthBasic`/`AuthElevated`) to `internal/retailer`.
      (Done: `internal/retailer/auth.go`. **Scope correction**: task 2.3's original wording targeted
      `internal/icaretailer` — checked first and found it's dead code, zero consumers outside itself
      anywhere in `cmd/`/`internal/` per a full grep; `internal/retailer.Client` (via `NewICA`/
      `NewFromKind`) is what's actually used for both retailers. `AuthTier` and everything in this
      task group targets `internal/retailer` instead. `internal/icaretailer`'s dead-code status is
      worth a cleanup decision but is out of scope for this change — flagging, not deleting.)
- [x] 2.2 Mark ICA's operations: `Resolve` = basic, `CreateShoppingList` = elevated (per
      `expose-shopping-price-and-notes-bridge` D3's finding that anonymous ecom search is never
      stale but the OAuth2 wishlist-push session is).
      (Done: doc comments on `ResolveRequirements` (AuthBasic), `CreateShoppingList` and
      `SyncShoppingList` (AuthElevated) in `internal/retailer/client.go`, each citing the specific
      research finding they're grounded in.)
- [x] 2.3 Add the elevated-credential file-path field to `Config` (task 1.1); wire it into
      `internal/icaretailer`'s construction instead of any independent env read.
      (Done, redirected per 2.1's note: `Config.ICAElevatedCredentialPath`
      (`ICA_ELEVATED_CREDENTIAL_PATH`) added to `internal/config/config.go`. Honest limitation
      documented in the field's own doc comment and in `docs/infrastructure/ica-elevated-auth.md`
      (task 2.6): this path is not yet wired into `ica-adapter` itself — `ica-client`'s `IcaClient`
      constructor in that sibling repo's `server.ts` doesn't currently accept a `cookieCachePath`
      override, so this Config field is a documented, discoverable slot for a future health check,
      not something Go reads today. The 401/403 staleness detection (2.4) doesn't need it — it
      reads the adapter's live HTTP response, not this file.)
- [x] 2.4 Surface elevated-auth staleness as a distinct, typed condition (reusing the 401-detection
      approach from `expose-shopping-price-and-notes-bridge` D3) rather than a generic error.
      (Done: added `httpclient.StatusError` (new type, carries the real HTTP status code, recoverable
      via `errors.As` — previously the status was only embedded in an unstructured error string) to
      `internal/httpclient`, and `internal/retailer.ErrElevatedAuthStale` +
      `wrapElevatedAuthError()` in `internal/retailer/auth.go`, applied to `CreateShoppingList` and
      `SyncShoppingList`. Detects 401/403 specifically. **Documented limitation** (in `auth.go`'s
      doc comment and the D2 write-up): this only catches the "catchable" ICA failure shape from
      that research (opaque 401/502) — the "dangerous" shape (stale session, still 200/201, silently
      fabricated data) cannot be detected from this side of the HTTP boundary at all; closing that
      gap needs a fix inside `ica-adapter` itself, tracked in the other change, not here.)
- [x] 2.5 Unit tests: a basic-tier call proceeds without the elevated credential present; an
      elevated-tier call with a missing/stale credential reports the typed staleness condition.
      (Done: `internal/retailer/auth_test.go` — 401 and 403 both classify as `ErrElevatedAuthStale`
      via `errors.Is`; a 502 (the distinct "catchable" shape) does NOT get misclassified as
      elevated-auth-stale; `ResolveRequirements` (AuthBasic) never wraps even a hypothetical 401 as
      elevated-auth-stale, since it doesn't go through `wrapElevatedAuthError` at all; plus direct
      unit tests on `wrapElevatedAuthError` for nil and non-`StatusError` inputs. Also added
      `TestNon2xx_SurfacesBackendError`'s sibling assertions in `internal/httpclient/httpclient_test.go`
      confirming `errors.As` recovers the real status code and body. 7 new tests, all passing.)
- [x] 2.6 Document the manual elevated-login handoff (who runs the Playwright login, where the
      resulting credential file goes) in `docs/infrastructure/` — today this has no documented
      location at all.
      (Done: `docs/infrastructure/ica-elevated-auth.md`, grounded in reading `ica-client/src/auth/
      {oauth2,ecom-session}.ts` directly rather than assuming. Key finding worth a second look:
      `ensureLogin()` opens a real, visible browser window and waits up to 10 minutes for a human —
      there is no separate script, it's triggered reactively whenever the cached session fails live
      validation. **Flagged as a real deployment risk**: this cannot work unattended on a headless
      Proxmox LXC (`deploy-food-brain-to-proxmox`'s target) — ICA wishlist push realistically needs
      to stay a human-attended operation unless/until a remote-login-friendly path exists.)
- [x] 2.7 Build the actual upload path (added 2026-08-28, in response to the household recalling
      the existing login-capture mechanism and asking for it to upload to spisordning directly,
      rather than leaving the Mac→Proxmox sync as an undocumented manual copy per 2.6's original
      writeup).
      (Done, spisordning side: `db/migrations/000015_retailer_credential.sql` (one
      `retailer_credential(retailer, tier, payload jsonb, uploaded_at)` table, payload opaque to
      Go); `internal/persistence.{Upsert,Get}RetailerCredential` +
      `internal/persistence/retailer_credential_test.go` (3 tests: upload-then-get, overwrite
      semantics, per-retailer independence — written against the same `skipWithoutDB` pattern as
      the rest of the suite; **not run against a live Postgres this session — Docker wasn't
      running** — verify before merging); `internal/httpapi/retailer_credential.go`
      (`RetailerCredentialService`, `POST`/`GET /retailers/{retailer}/elevated-credential`) wired
      through `storeAdapter` (`cmd/food-brain/adapters.go`), matching the existing "newer
      capabilities added straight against persistence.Store" pattern; 4 httpapi unit tests, all
      passing against a fake service. Full `go build`/`go vet`/`go test ./...` — 465 passed.
      Store-clients side (uncommitted — see D2b): new
      `apps/ica-adapter/upload-elevated-credential.ts` runs `ecomAuth.ensureLogin()` then POSTs the
      resulting cookie JSON to spisordning's upload endpoint, `--apply`-gated dry-run-by-default
      matching `spisordning-bridge.ts`'s established convention. Verified the script loads/parses
      correctly (imported as a module so `main()` doesn't fire) — **did not run it for real**, since
      that would open a real browser window against the live ICA account, a genuine side effect
      requiring explicit sign-off, not something to trigger while just verifying code.)
- [ ] 2.8 Have `ica-adapter` itself consume the uploaded credential: on startup (and/or
      periodically), fetch `GET /retailers/ica/elevated-credential` from spisordning and write the
      result to its local `cookieCachePath` (the `ICA_ELEVATED_CREDENTIAL_PATH` it already reads
      per D2a) before `EcomAuth.ensureLogin()` runs, so a Proxmox-deployed adapter actually picks up
      what the Mac script uploaded. **Not yet built** — scoped separately since it's entirely
      `ica-adapter`-side (store-clients) work with no spisordning-side component; the upload half
      (2.7) is a complete, independently useful piece without it (a human could still fetch the
      credential from spisordning's GET endpoint and place it manually, in the interim).
- [x] 2.9 Verify 2.7's persistence tests against a real Postgres — Docker was unavailable locally,
      so deployed one on Proxmox via Tengil instead (the user's suggestion).
      (Done 2026-08-28. **Two `type=oci` attempts failed** before finding the real fix:
      `postgres:19beta3-alpine` and `postgres:19beta3` (Debian) both got created as an LXC then
      immediately stopped — Tengil's OCI→LXC conversion needs a real init process in the image's
      rootfs, and official Postgres images (Docker/containerd-style, PID-1-is-postgres) don't have
      one; confirmed via the install task's own preflight diagnostics
      (`HasInit:false`/`HasBinSh:false` on the second attempt) and by checking the container
      directly (`pct status` showed `stopped` even while Tengil's own task API still reported
      `running`). Both throwaway attempts torn down. **Found a better fix**: `main-postgres`
      (VMID 2327, `192.168.1.93:5432`) was already running on this Tengil instance — `postgres:16-
      alpine`, already provisioned with `POSTGRES_DB=spisordning`/`POSTGRES_USER=spisordning` from
      earlier background work — so used that instead of fighting the OCI conversion further (also
      reset its password to a known value via `pct exec`, since the deployed env had an unresolved
      `${POSTGRES_PASSWORD:?...}` template string; **this is a shared instance other sessions may
      use — flagged to the user, not silently changed**).
      Applied all 15 migrations cleanly (`000001` through `000015`, including this change's new
      one) against a database that had none applied yet. Ran the full suite:
      `go test ./... ` — **539 passed** (up from 465 with DB tests skipping), zero failures on the
      first real run except one **real bug the fake-DB-less unit tests couldn't have caught**:
      `TestRetailerCredential_UploadThenGet` compared the stored JSONB payload against the upload
      byte-for-byte, but Postgres's `JSONB` column re-serializes with its own canonical whitespace
      (`{"name": "sid", ...}` vs. the uploaded `{"name":"sid",...}`) — the data round-tripped
      correctly, the test's exact-string assertion was just too strict. Fixed with a `jsonEqual`
      helper (parse both sides, `reflect.DeepEqual`) in `retailer_credential_test.go`, applied to
      both assertions in that file that compare JSON payloads. Also independently verified the
      `POST`/`GET /retailers/ica/elevated-credential` HTTP round-trip against this same real
      database (`food-brain serve` + `curl`), not just the fake-service unit tests from 2.7.)

## 3. SSE progress streaming

- [ ] 3.1 Add an SSE endpoint to `internal/httpapi` streaming progress for a running plan/resolve
      operation (event per item as `cmd/food-brain/plan.go`'s resolve loop progresses).
- [ ] 3.2 Confirm the existing synchronous plan endpoint is unaffected — SSE is additive.
- [ ] 3.3 Integration test: drive a plan/resolve against a fake slow adapter and assert progress
      events arrive incrementally, not all at once at the end.
- [ ] 3.4 Sequence this after task 4 lands a real consumer (design.md's migration plan) — don't
      finalize the event payload shape until the frontend's first slice needs it.

## 4. Frontend first slice

- [ ] 4.1 Scaffold `web/`: Vite + React + TypeScript, matching `~/dev/tengil/web-ui`'s stack
      (React 18, TanStack Query) for toolchain consistency across the household's homelab projects.
- [ ] 4.2 Generate a TS client from `api/openapi.yaml` (mirroring how `internal/openapi` is
      generated for Go) rather than hand-writing request/response types.
- [ ] 4.3 Build the one read-only view: current week's dinner plan + its shopping list, calling the
      real REST API against a running `food-brain serve` instance — no mock data.
- [ ] 4.4 Confirm no write action exists in this first slice (per the spec's explicit scope limit).
- [ ] 4.5 Document how to run the frontend locally against a local `food-brain serve` (README in
      `web/`).

## 5. Verification & docs

- [ ] 5.1 `go build ./... && go test ./... && go vet ./...` green, including the architecture-test
      job, after all Go-side tasks (1–3) land.
- [ ] 5.2 `web/` builds and runs against a local backend (manual check, task 4.3's acceptance).
- [ ] 5.3 Update `docs/research/current-state.md` to reflect the new `internal/config`,
      `AuthTier` concept, SSE endpoint, and `web/` frontend's existence.
