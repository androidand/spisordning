## Why

spisordning is not deployed anywhere today. `docker-compose.yml` already defines the full local
stack (Postgres, `migrate`, `willys-adapter`, `food-brain`, `mcp-server`) and references
`ghcr.io/androidand/spisordning/food-brain:latest` and `.../willys-adapter:latest`, but CI's
`docker` job only verifies the `Dockerfile` builds (`push: false`, `.github/workflows/ci.yml`) — no
image has ever actually been published to GHCR. A stale comment in `docker-compose.yml` even
references `openspec/changes/deploy-via-docker-compose/` for the Tengil deploy path; that change was
never created. `docs/infrastructure/deployment-and-access.md` and the tengil repo's own
`openspec/changes/full-stack-compose-deploys/specs/spisordning/compose.yaml` both already describe
Tengil (an LXC on the user's Proxmox host, VMID 300) as the intended target and call it "aspirational
until food-brain has a published image" — this change is what makes it non-aspirational. Without
this, the shopping/price MCP tools and the Apple Notes bridge (`expose-shopping-price-and-notes-
bridge`) have no real server to talk to from the Mac.

## What Changes

- Add a CI workflow step (or extend the existing `docker` job) that actually pushes built images to
  GHCR — at minimum `food-brain` (also used for the one-shot `migrate` step and, via entrypoint
  override, `mcp-server`), tagged appropriately (`:latest` on `main`, plus a version/sha tag).
  Confirm whether `Dockerfile.mcp` is still needed given `mcp-server` currently builds from the same
  `Dockerfile` via entrypoint override in `docker-compose.yml` — remove it if it's dead, or wire it
  in if `mcp-server` should actually be a distinct image.
- Deploy the compose stack to Tengil (the Proxmox LXC), using the reference `compose.yaml` in the
  tengil repo as the starting point, adapted for real published images instead of local builds where
  Tengil's `type=compose` deploy path requires them (confirm this requirement against
  `~/dev/tengil/docs/compose-packages.md` before assuming).
- Confirm `willys-adapter`'s own image publish story (it's a sibling-repo concern —
  `willys-client/Dockerfile.adapter` — verify whether it already publishes or needs the same
  treatment as food-brain).
- Expose the deployed `mcp-server`'s HTTP endpoint and `food-brain`'s HTTP API somewhere the Mac-
  local Apple Notes bridge (`expose-shopping-price-and-notes-bridge`) and an MCP-capable chat client
  can actually reach on the household's LAN.

## Capabilities

### New Capabilities
- `deployment`: the observable, testable contract for "spisordning is actually deployed" — images
  are published and pullable, and the deployed HTTP API and MCP server are reachable from the
  household LAN.

### Modified Capabilities
- None.

## Impact

- `.github/workflows/ci.yml`: add image push (GHCR login already present; only `push: false` → real
  push + tagging changes).
- `docker-compose.yml`: verify/update image references once real tags exist; resolve the
  `Dockerfile.mcp` question.
- Tengil repo (`~/dev/tengil`): the actual deploy target — this change's implementation work happens
  partly there (per `docs/infrastructure/deployment-and-access.md`'s deploy mechanism), not only in
  spisordning.
- No application code changes expected in spisordning itself beyond what CI/compose config needs.
