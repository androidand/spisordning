## Context

Four largely-independent gaps, bundled into one change because the household asked for a single
architecture pass rather than sequencing them: config/DI (foundation for the other three), retailer
auth-tier modeling (grounded in a real ICA constraint), SSE (grounded in today's observed UX gap),
and a frontend first slice (the household's stated longer-term need). `establish-enforced-go-
architecture`'s layer rules and `internal/architecturetest` enforcement are untouched — this change
adds new packages within that existing structure, it doesn't renegotiate it.

## Goals / Non-Goals

**Goals:**
- One `internal/config.Config` per binary, built once, replacing duplicated `os.Getenv` reads in
  both `cmd/food-brain` and `cmd/mcp-server`.
- A generic, reusable basic/elevated auth-tier concept for retailer clients, with ICA as the first
  real instance — not an ICA-specific hack.
- SSE for the one long-running operation that's actually painful today (plan/resolve progress).
- A real, running frontend first slice (read-only week view), proving the API contract works for a
  browser client, not just MCP/curl.

**Non-Goals:**
- No WebSockets — no concrete use case exists; revisit only when one does (see D3).
- No config *file* support (YAML/TOML) — env-only for now (see D1); this is a deliberate deviation
  from tengil's pattern, not an oversight.
- No feature-complete frontend — the first slice is read-only (this week's plan + shopping list);
  write actions (record a reaction, push a wishlist) are explicitly follow-on work.
- No changes to `establish-enforced-go-architecture`'s layer rules themselves.

## Decisions

**D1 — Config is env-only, not env+YAML like tengil.** Tengil manages many independently-deployed
apps across a cluster and genuinely needs a persisted, editable config file per instance. Spisordning
is a single household application with one deployment target (`deploy-food-brain-to-proxmox`) and
already has an established `.env` convention (`docker-compose.yml`, `.env.example`). Adding a YAML
config layer on top would be two config mechanisms for one deployment — needless duplication for
this project's actual shape. Alternative considered: mirror tengil exactly for consistency —
rejected; consistency with a differently-shaped project isn't worth the extra mechanism.
`internal/config.Load()` reads env vars (with the same names already used today, so no `.env`
migration needed) into one struct. **Refined during implementation (task 1.2):** required-value
validation stays at each call site (e.g. `MealieEnabled()`), not centralized in `Load()` — `Load()`
never fails; it gathers values, and each command decides what it needs, exactly matching the
behavior each call site already had before this change (a config load failure inside a client
constructor was never actually the failure mode — a missing/empty value was). Centralizing that
validation would have meant `Load()` knowing about every command's requirements, coupling it to
callers it shouldn't need to know about.

**D2 — Auth tiers live as a field on `domain.RetailerKind`'s config, not a new domain type.**
`internal/retailer.RetailerKind` (`RetailerWillys`/`RetailerICA`) already exists as the retailer
identity concept `internal/retailer.NewFromKind` dispatches on. Add an `AuthTier` enum
(`AuthBasic`/`AuthElevated`) and let each retailer declare which of its operations need which tier —
for ICA: `Resolve`/search = basic (matches `expose-shopping-price-and-notes-bridge`'s finding that
anonymous ecom search is never stale); `CreateShoppingList`/wishlist push = elevated (the OAuth2
session, per that same change's D3 finding). `Config` gains a slot for where an elevated credential
is read from (env var pointing at a file path, mirroring tengil's `SecretsFile` pattern at D1's
smaller scale — one file, not a whole secrets subsystem) — the *manual* web-login step itself stays
entirely on the TS/`ica-adapter` side (Playwright-driven, human-in-the-loop by nature); Go only needs
to know where to find the resulting credential and how to detect it's stale (reusing
`expose-shopping-price-and-notes-bridge` D3's 401-detection finding). Alternative considered: a full
`internal/auth` package with pluggable strategies — rejected as premature; one real case (ICA) and
one hypothetical (a future Hemköp elevated tier, if it ever needs one) doesn't justify a strategy
framework yet. Revisit if a third tiered retailer shows up. **Done (task group 2)** — see
`internal/retailer/auth.go`; scope corrected from `internal/icaretailer` (confirmed dead code, zero
consumers) to `internal/retailer` (what's actually used).

**D2a — Where the manual elevated login actually runs, once `ica-adapter` deploys to Proxmox
(decided 2026-08-28).** The Playwright-driven login opens a real, visible browser window — verified
by reading `ecom-session.ts` directly — which cannot run on a headless Proxmox LXC. Decision: the
login stays Mac-side (where a display exists); the resulting cookie file gets manually synced to
wherever the Proxmox-deployed `ica-adapter` runs. This needed one small cross-repo change, now
done: `IcaClientOptions.cookieCachePath` (new, `store-clients/ica-client/src/index.ts`) plumbed
through to `EcomAuth`, and `ica-adapter/server.ts` reading `ICA_ELEVATED_CREDENTIAL_PATH` — the
same env var name `Config.ICAElevatedCredentialPath` (D2/task 2.3) already documented on the Go
side, now actually meaningful on the TS side too. The sync step itself (copying the file from Mac
to Proxmox) stays a manual, undocumented-as-automation step — see
`docs/infrastructure/ica-elevated-auth.md` for the full writeup and the two alternatives considered
(keeping `ica-adapter` Mac-resident like `willys-adapter`; a remote-viewable headless browser via
Xvfb+VNC) and why they were rejected. **Cross-repo note**: the `store-clients` half of this (three
files: `src/index.ts`, `apps/ica-adapter/server.ts`) is implemented and verified (runtime
construction check + full existing test suite, no regressions) but is sitting **uncommitted** in
that sibling repo — this session's git-worktree isolation blocks committing to any repo other than
this one. Needs a manual commit there before this decision is fully landed.

**D3 — SSE ships for plan/resolve progress; WebSockets are explicitly deferred, not built
speculatively.** `internal/httpapi` gains `GET /plans/{id}/progress` (or similar) streaming
`text/event-stream` events as `cmd/food-brain/plan.go`'s resolve loop progresses — the exact gap
observed today (resolving ~20 items through a rate-limited adapter with no feedback for minutes).
WebSockets would answer a *different* question (bidirectional, low-latency, multi-client) that
nothing in this household's actual usage needs yet — no live multi-user editing, no server-push
outside of "a specific long operation just happened to run." Building it now would be an
abstraction ahead of a real requirement, which cuts against how this codebase already operates
(no speculative generality elsewhere in the layer rules). Alternative considered: build both since
"presentation layer: REST, SSE, MCP, perhaps WebSockets" was the stated wish — rejected; "perhaps"
was already a hedge, and a concrete SSE win now is better than two half-motivated additions.

**D4 — Frontend: Vite + React + TypeScript + TanStack Query, mirroring `~/dev/tengil/web-ui`'s
stack exactly.** Same organization, same homelab, same person maintaining both — matching tengil's
proven choice (not a from-scratch stack evaluation) minimizes the number of frontend toolchains one
person has to context-switch between. Lives at a new top-level `web/` directory in this repo (not a
separate repo — the household's other multi-service repos, e.g. `store-clients`, keep sibling
packages in one repo; spisordning's own `api/openapi.yaml` is the natural contract boundary, and
codegen'ing a TS client from it, the same way `internal/openapi` is generated for Go, keeps the
frontend from hand-maintaining request/response types). First slice: one read-only view — the
current week's dinner plan (`GET` via the existing plan/candidates surface) and its shopping list
(`GET /shopping-lists`) — proving the REST contract serves a real browser client end-to-end.
Explicitly not in scope for the first slice: auth/login UI, write actions, the SSE view (D3), or
mobile responsiveness polish — those are natural follow-on changes once the shell exists.

## Risks / Trade-offs

- [Two composition roots (`food-brain`, `mcp-server`) currently duplicate config parsing; migrating
  both to `internal/config` touches both binaries' startup paths] → Mitigation: land `internal/
  config` and migrate one binary at a time behind the existing architecture test, which will catch
  any accidental new cross-layer import; `go build ./... && go test ./...` gates each step.
  **Confirmed in practice (task 1.4/1.5):** both binaries migrated, 455 tests pass, architecture
  test green.
- [ICA's elevated-auth credential file is a new, manually-maintained artifact (someone has to run
  the Playwright login and drop the result where `Config` expects it)] → Mitigation: this is not a
  regression — today that manual step already exists inside `ica-adapter`/`ica-client` with no
  documented handoff point at all; giving it one explicit, documented location is strictly better,
  not new complexity.
- [A frontend first slice is still real, non-trivial work (build tooling, generated TS client,
  hosting) for a read-only view] → Mitigation: scope it as genuinely minimal (one page, no auth, no
  writes) specifically so it proves the contract without becoming its own multi-week effort;
  resist scope creep into a second meal-planning UI mid-change.
- [SSE and a future SPA both want "live plan status" — risk of building the SSE contract without the
  frontend consumer to validate it against] → Mitigation: sequence tasks so the frontend's first
  slice (read-only, D4) lands before or alongside SSE wiring, so there's a real consumer, not a
  contract designed in isolation.

## Migration Plan

1. `internal/config`: new package, no consumers yet — safe to land standalone, tested in isolation.
   **Done.**
2. Migrate `cmd/food-brain/adapters.go` to `config.Load()`, then `cmd/mcp-server/adapters.go` —
   one PR each, `go build`/`go test`/architecture-test gate both. **Done — both binaries migrated
   in one pass (task 1.4/1.5), all `os.Getenv` call sites across both packages covered, not just
   `adapters.go`.**
3. Auth-tier: add the `AuthTier` field/enum to `internal/retailer`, wire ICA's elevated-credential
   path through `Config`; no behavior change for Willys (single-tier, unaffected).
4. Frontend first slice: scaffold `web/`, codegen the TS client from `api/openapi.yaml`, build the
   one read-only view against a running `food-brain serve`.
5. SSE: add the progress endpoint once the frontend has something to consume it with.
6. Rollback: every step is additive (new package, new field, new endpoint, new directory) — nothing
   removes or renames existing behavior, so any step can be reverted independently.

## Open Questions

- Does the elevated-auth credential file need encryption at rest (mirroring tengil's
  `SecretsKey`), or is file permissions (0600) on a single-user homelab box sufficient? Leaning
  toward file permissions only, given D1's "don't copy tengil's full mechanism, only what this
  project's shape needs" — revisit if this deployment target ever becomes multi-tenant.
- Should the frontend live in this repo's `web/` or as a sibling repo like `store-clients`? D4
  picked this repo; revisit if the frontend's own dependency churn (npm ecosystem) starts fighting
  the Go module's release cadence in practice.
