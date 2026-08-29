## 1. Resolve open questions before touching CI/compose

- [x] 1.1 Check `willys-client`'s own CI/publish setup for `Dockerfile.adapter` — confirm whether
      `willys-adapter:latest` is actually published to GHCR anywhere today.

      ✅ Verified 2026-08-27: **not published — no publish path exists.** `willys-client` actually
      lives at `~/dev/store-clients/willys-client` (the `~/dev/willys/` path in docs is stale) and has
      **no CI of its own** (no `.github/`, no `.gitlab-ci.yml`/`Jenkinsfile`/`.circleci`). The parent
      `store-clients` repo's only workflow (`.github/workflows/ci.yml`) does OpenAPI lint + Go client
      codegen/build and never builds or pushes a Docker image. `Dockerfile.adapter` is only a local
      build recipe (referenced by its own comment + spisordning docs); spisordning's own `docker` CI
      job only build-verifies `food-brain` with `push: false` and never touches `willys-adapter`. No
      Makefile, no `ghcr` refs in any openspec, no publish scripts. So `docker-compose.yml`'s
      `image: ghcr.io/androidand/spisordning/willys-adapter:latest` assumes a published image that
       nothing produces — a real blocker for the `willys-adapter` service (→ scope a CI fix in the
       sibling repo, task 2.3). **Resolved 2026-08-27:** the `store-clients` repo now publishes
       `ghcr.io/androidand/store-clients/willys-adapter` via CI and this repo's compose was repointed
       to that name (task 2.3).
- [x] 1.2 Confirm whether `Dockerfile.mcp` is referenced by anything (CI, docs, a different deploy
      path) or is dead now that `docker-compose.yml`'s `mcp-server` service builds from `Dockerfile`
      with an entrypoint override.

      ✅ Verified 2026-08-27: **dead.** The main `Dockerfile` already builds BOTH `food-brain` and
      `mcp-server` (Dockerfile:15-16) and copies both into the image; CI's `docker` job and
      `docker-compose.yml`'s `mcp-server` service both use that `Dockerfile` (compose overrides
      `entrypoint: ["mcp-server"]`, docker-compose.yml:72-87). `Dockerfile.mcp` is referenced only by
      its own build comment, a stale line in `docs/research/current-state.md:181` (which also lists the
      wrong port, 8401 vs the real 8081), and this change's own docs. → remove it (task 2.2).
- [x] 1.3 Read `~/dev/tengil/docs/compose-packages.md` to confirm whether Tengil's `type=compose`
      deploy path builds on-host or requires pre-pulled images for every service.

      ✅ Verified 2026-08-27: **requires pre-pulled images — `build:` is not supported.**
      `compose-packages.md` lists `build` as "Not supported — use pre-built images" (Supported Compose
      Features table) and repeats it under Known Limitations ("No build context: Compose files with
      `build:` directives are not supported"). `image` is "Pulled from registry". So the Tengil compose
      file must reference published GHCR images for EVERY service — including `mcp-server`, which
      currently uses `build:` in the local `docker-compose.yml` and must instead use
      `image: .../food-brain:latest` + the `entrypoint: ["mcp-server"]` override. (→ task 3.1.)

## 2. Publish real images

- [x] 2.1 Update `.github/workflows/ci.yml`'s `docker` job: push `food-brain`'s image to GHCR on
      `main`, tagged `:latest` plus the commit sha.

      ✅ Edited 2026-08-27: `docker` job now builds on every push/PR but only publishes on a push to
      `main` (`push: ${{ github.event_name == 'push' && github.ref == 'refs/heads/main' }}`), tagged
      `:latest` + `${{ github.sha }}`. Also fixed an image-name mismatch: the old tag
      `ghcr.io/${{ github.repository_owner }}/food-brain:ci` (= `ghcr.io/androidand/food-brain`) did
      NOT match the `image:` refs in `docker-compose.yml`; it now tags
      `ghcr.io/${{ github.repository }}/food-brain` (= `ghcr.io/androidand/spisordning/food-brain`),
      which matches. YAML validated (parses; 5 jobs). The actual push is not run this run (forbidden)
      — verify pullability in task 2.4 after a real main merge.
- [x] 2.2 If task 1.2 finds `Dockerfile.mcp` is dead, remove it; if it's meant to be a distinct
      image, wire its own build+push step instead of relying on entrypoint override alone.

      ✅ Done 2026-08-27: task 1.2 found `Dockerfile.mcp` dead (the main `Dockerfile` already builds
      both `food-brain` and `mcp-server` and is what CI + compose use), so **removed** `Dockerfile.mcp`
      and fixed the one stale doc reference (`docs/research/current-state.md:181` now points at the
      main `Dockerfile` + the real port 8081). No distinct mcp image is needed — `mcp-server` reuses
      the `food-brain` image via an `entrypoint` override (which is also what Tengil requires, per
      task 1.3, since it can't `build:`).
- [x] 2.3 If task 1.1 finds `willys-adapter` unpublished, scope a matching CI fix in the sibling
      `willys-client` repo (tracked there, not duplicated here).

      ✅ Implemented 2026-08-27: the sibling `store-clients` repo now has the CI fix. Because
      `willys-client` is a subdir of the `store-clients` monorepo (no `.git` of its own), the workflow
      lives at the monorepo root: `.github/workflows/willys-adapter-image.yml`. It builds
      `willys-client/Dockerfile.adapter` (context `./willys-client`) and publishes
      `ghcr.io/androidand/store-clients/willys-adapter` on `master` (`:latest` + sha) using
      `GITHUB_TOKEN` — the namespace decision recorded in the sibling change
      (`willys-client/openspec/changes/publish-willys-adapter-image/`). This repo's `docker-compose.yml`
      and the tengil reference compose were repointed to the new image name. Remaining: confirm
      pullability after a real `store-clients` master push (task 2.4).
- [ ] 2.4 Verify the pushed images are pullable (`docker pull ghcr.io/androidand/spisordning/food-
      brain:latest` from a clean environment).

## 3. Deploy to Tengil

- [x] 3.1 Adapt the tengil repo's reference `compose.yaml`
      (`~/dev/tengil/openspec/changes/full-stack-compose-deploys/specs/spisordning/compose.yaml`)
      for the now-real image references, per task 1.3's findings.

      ✅ Done 2026-08-27: rewrote the reference compose (it actually lives under `openspec/archived/
      full-stack-compose-deploys/...`, not `changes/` — the path above is stale) to match the real
      spisordning stack: correct image names (`ghcr.io/androidand/spisordning/food-brain:latest`,
      `ghcr.io/androidand/store-clients/willys-adapter:latest`), `postgres:19beta3-alpine`, and — per
      task 1.3's no-`build:` finding — `mcp-server` now reuses the `food-brain` image via an
      `entrypoint` override (no separate mcp image / no `build:`), plus the one-shot `migrate` service
      (food-brain image, `migrate up --seed`, `service_completed_successfully` gate on food-brain). Kept
      the Tengil `target`/`published` port format and `x-tengil` package metadata (description corrected
      to the real system). YAML validated (5 services parse). Actual Tengil deploy is task 3.2 (not run —
      forbidden this run).
- [ ] 3.2 Deploy via Tengil's `POST /api/apps` (`type=compose`) or `scripts/deploy-local-profile.sh
      --app spisordning`, per `docs/infrastructure/deployment-and-access.md`.
- [ ] 3.3 Run the one-shot `migrate` service against the deployed Postgres; verify it completes
      successfully before `food-brain`/`mcp-server` start.
- [ ] 3.4 Verify `food-brain`'s HTTP API and `mcp-server`'s MCP endpoint are reachable on the
      household LAN (from the Mac, not just from the Proxmox host itself).

## 4. Docs & cleanup

- [x] 4.1 Update `docker-compose.yml`'s header comment — it references a nonexistent
      `openspec/changes/deploy-via-docker-compose/`; point it at this change instead.

      ✅ Done 2026-08-27: header now points at `openspec/changes/deploy-food-brain-to-proxmox/` and
      notes that Tengil doesn't support `build:`, so the Tengil compose file uses pre-pulled GHCR
      images for every service (mcp-server via entrypoint override) rather than the local `build:`.
- [ ] 4.2 Update `docs/infrastructure/deployment-and-access.md` to drop the "aspirational" caveat
      for spisordning once it's actually deployed, and record the deployed URLs/ports.
- [ ] 4.3 Update `docs/research/current-state.md`'s "no Dockerfile, no published image yet" line
      (`AGENTS.md` also repeats this — update both) now that it's no longer true.

## 5. Verification

- [ ] 5.1 Confirm `expose-shopping-price-and-notes-bridge`'s Mac-local notes bridge (task 6.3 in
      that change) can actually reach the deployed `food-brain` HTTP API from the Mac.
- [ ] 5.2 Confirm an MCP-capable chat client on the LAN can connect to the deployed `mcp-server`
      over Streamable HTTP and call `list_recipe_candidates` end-to-end.
