# Spisordning — Agent Instructions

## Mission

Spisordning is a self-hosted household food knowledge system: recipe management with
git-like recipe evolution, household preferences, meal history, weekly planning,
recommendations, pantry/inventory, retailer integration, price intelligence, external recipe
discovery, and optional AI interaction.

The full mission brief, research phases, domain-model challenges, and expected documents live
in [`PLAN.md`](PLAN.md). Read it before starting any non-trivial change. `PLAN.md` describes a
large from-scratch relational rebuild — but a working first slice already exists (see below).
New work must acknowledge and build on that slice, not silently re-derive it.

## Current state (read this before assuming something doesn't exist)

- Go module (`go.mod`, stdlib-only, Go 1.26.1) with both CLI (`cmd/food-brain`) and HTTP
  server (`food-brain serve`). Dockerfile builds both `food-brain` and `mcp-server`; CI
  publishes `ghcr.io/androidand/spisordning/food-brain` to GHCR on `main`.
- Postgres schema exists (`db/migrations/`) and is applied by `docker-compose.yml` / the
  one-shot `migrate` service. The Go code reads and writes to it via `internal/persistence`.
- A read-only Mealie client (`internal/mealie`) is real and tested.
- Grocy: real, tested client/service/handler (`internal/grocy`, `internal/service/grocy.go`,
  `internal/httpapi/grocy.go`). Directus: still no code — pure research/reference target per
  `PLAN.md`.
- The retailer integration is further along than `PLAN.md` assumes: a sophisticated,
  **live-verified** resolution pipeline (household pins, a review-and-pick UI, promo-variant
  expansion, size-aware matching, split name/quantity confidence, an Apple Notes bridge) is
  implemented — but its code lives in the **sibling repo** `~/dev/willys/willys-client`
  (`apps/willys-adapter`), not here. That repo has its own `openspec/` (three changes) —
  check it before touching anything retailer-related. See
  `docs/research/willys-capabilities.md` for the full capability map and
  `docs/research/current-state.md` for the detailed as-of inventory.
- A second retailer client is taking shape alongside it, in the same `~/dev/willys/` directory:
  **`~/dev/willys/ica-client`** — a standalone TypeScript ICA client, structurally mirroring
  `willys-client`. Early-stage and not yet stable (see `docs/research/ica-current-api.md` §5 for
  the current snapshot — check it before assuming any capability listed there still holds, it
  drifts fast). Spisordning's own tracking for wrapping it once stable is
  `openspec/changes/integrate-ica/` — gated on the client's stabilization, not yet
  implementation-ready. Research on the two external seed repos (`ica-api`, `ha-ica-todo`) that
  informed it lives in `docs/research/ica-current-api.md`.
- Full infra/access details (Proxmox, Tengil, GitHub): `docs/infrastructure/deployment-and-access.md`.

## OpenSpec / specsync workflow

This repo uses [OpenSpec](openspec/) for planning (`openspec/changes/<slug>/`) and
[specsync](https://github.com/androidand/specsync) to project changes onto GitHub Issues.
OpenSpec files are the source of planning truth; GitHub Issues are the collaboration
projection.

- **One change → one GitHub Issue → one branch → one PR.** Branch `feat/<issue>-<slug>` off
  `main`. PR body includes `Closes #<issue>`.
- Dry-run before syncing: `specsync -dry-run -change <slug>`. Real sync: `specsync -change <slug>`.
- Related (not duplicate) changes are cross-linked with `specsync link <change1> <change2>`,
  not by hand-editing issue bodies.
- specsync has no credential handling of its own — it shells out to `gh`, which is already
  authenticated on this machine (`gh auth status` → account `androidand`). Nothing to
  configure.

### Epics

PLAN.md's workstreams are grouped into 8 Epics, each a GitHub Milestone plus one `epic`-labeled
tracking issue (GitHub Milestones/labels aren't a specsync primitive — this is a repo
convention layered on top). When creating a new OpenSpec change, assign its issue to the
matching milestone: `gh issue edit <n> --milestone "<epic name>"`. See the tracking issues
themselves (linked from the epic issues) for the current change list per epic — do not
duplicate an epic's scope in a new ad-hoc change without checking there first.

## Deployment

Local dev: `docker-compose.yml` (Postgres + `willys-adapter` + `food-brain` + `mcp-server`).
`food-brain` exposes the HTTP API on port 8080; `mcp-server` exposes the MCP endpoint on 8081.

Production/reference-lab target is **Tengil** (`~/dev/tengil`), not raw docker-compose — see
`docs/infrastructure/deployment-and-access.md` for exact access details, and the tengil repo's
`openspec/changes/full-stack-compose-deploys/specs/spisordning/compose.yaml` for the reference
stack definition.

## Skills

Use the `specsync`, `tengil`, and `proxmox` global skills for their respective tools. Where
those skills are silent or wrong for this project's actual setup (env var names, credential
locations, SSH access), `docs/infrastructure/deployment-and-access.md` and
`.claude/skills/spisordning-infra/SKILL.md` are the corrected, repo-local source of truth —
prefer them over guessing from the global skill text.
