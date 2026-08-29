## Context

Every application-layer piece (persistence, HTTP API, MCP server) already exists and is tested; the
only thing missing is a real, running deployment. `docker-compose.yml` and CI both already assume
GHCR images that have never been published (`push: false` in `.github/workflows/ci.yml`'s `docker`
job). `docs/infrastructure/deployment-and-access.md` documents the real Proxmox/Tengil access story
(as opposed to the global `proxmox`/`tengil` skills, which use different env var names than the
actual app config) and names Tengil (an LXC, VMID 300) as the target. The tengil repo already has a
reference `compose.yaml` for spisordning, explicitly marked aspirational pending this change.

## Goals / Non-Goals

**Goals:**
- `food-brain` and `mcp-server` images are actually published to GHCR on a real trigger (at minimum:
  on push to `main`), not just build-verified in CI.
- spisordning's full stack (Postgres, migrate, willys-adapter, food-brain, mcp-server) runs on
  Tengil, reachable on the household LAN.
- The deployment is documented in `docs/infrastructure/deployment-and-access.md` (which already has
  a placeholder section acknowledging it's "aspirational until food-brain has a published image") so
  the doc stops being aspirational.

**Non-Goals:**
- No public/internet-facing exposure — LAN-only, matching `docs/infrastructure/deployment-and-
  access.md`'s existing caution about Tengil's own API auth model.
- No HA/redundancy design — this is a single-household homelab deployment, not a production SaaS.
- No changes to what spisordning's HTTP API or MCP server actually do — pure deployment plumbing.

## Decisions

**D1 — Publish `food-brain`'s image from the existing CI workflow by flipping `push: false` to a
real push, gated on branch (`main`) rather than every PR.** Alternative considered: a separate
release workflow triggered by tags — rejected for now as unnecessary ceremony for a homelab project
with one deployer; revisit if/when release versioning matters.

**D2 — Resolve whether `mcp-server` needs its own image or keeps using `food-brain`'s image with an
entrypoint override, as `docker-compose.yml` already does locally.** Check whether `Dockerfile.mcp`
(exists at repo root, currently unreferenced by `docker-compose.yml`) is a leftover from before the
entrypoint-override approach was adopted, or is used somewhere else (e.g. a different deploy path).
Default assumption pending verification: it's dead and should be removed, since one multi-stage
`Dockerfile` producing both binaries and switching via entrypoint is simpler to keep in sync — but
confirm before deleting anything.

**D3 — Deploy via Tengil's `type=compose` app path (`POST /api/apps`), using the tengil repo's
existing reference `compose.yaml` as the base**, per `docs/infrastructure/deployment-and-access.md`
and `~/dev/tengil/docs/compose-packages.md`. Confirm during implementation whether Tengil's compose
path requires pre-built pullable images for every service (in which case `willys-adapter`'s own
publish story, sibling-repo concern, needs resolving too) or can build on the host.

## Risks / Trade-offs

- [`willys-adapter`'s image was unpublished — a sibling-repo (`store-clients`) concern this change
  doesn't directly control] → **Resolved 2026-08-27:** the `store-clients` repo now has a
  `willys-adapter-image` CI workflow that builds `willys-client/Dockerfile.adapter` and publishes
  `ghcr.io/androidand/store-clients/willys-adapter` on `master`; this repo's `docker-compose.yml` and
  the tengil reference compose were repointed to that name. Remaining: confirm pullability after a
  real `store-clients` master push (task 2.4) and that the service starts (task 3.2).
- [Tengil's own auth model has a known open issue (`harden-api-auth-model`, per
  `docs/infrastructure/deployment-and-access.md`)] → Mitigation: deploy only from the trusted LAN, as
  already documented; this change doesn't need to wait on that Tengil-side fix, just needs to respect
  the existing caution.
- [Migrations run as a one-shot `migrate` compose service against a fresh Postgres — first real
  deploy is the first time this path runs against non-throwaway data] → Mitigation: the `migrate`
  service already exists and CI already validates `food-brain migrate up` locally; still worth a
  manual dry run against a disposable Postgres before pointing it at the real deployed volume.

## Migration Plan

1. Verify/resolve `Dockerfile.mcp`'s status (D2).
2. Update `.github/workflows/ci.yml`'s `docker` job to push on `main` with real tags.
3. Verify `willys-adapter`'s publish status in the sibling repo; resolve or scope separately if
   unpublished.
4. Deploy via Tengil's compose app path using the tengil repo's reference `compose.yaml`, adapted
   for whatever D3 confirms about image-vs-build requirements.
5. Update `docker-compose.yml`'s stale comment (references a nonexistent
   `openspec/changes/deploy-via-docker-compose/`) and `docs/infrastructure/deployment-and-access.md`
   once live.
6. Rollback: Tengil app removal via its own API/UI; no spisordning-side rollback complexity since no
   application code changes.

## Open Questions

- Does Tengil's `type=compose` deploy path build on-host or require pre-pulled images for every
  service? Confirms/refutes the willys-adapter blocker above.
- Is `Dockerfile.mcp` live or dead? Confirm before deleting.
