---
name: spisordning-infra
description: Spisordning-specific facts layered on top of the global specsync/tengil/proxmox skills — access details, reference-lab app locations, and the Epic/Milestone convention for this repo. Use whenever deploying, checking, or syncing spisordning-related infrastructure.
---

# Spisordning infra

This skill exists because the global `proxmox`/`tengil`/`specsync` skills are generic and, in
places, out of date relative to the real app config. It does not replace them — use them for
the mechanics (SSH/API call shapes, CLI flags). Use this skill for the project-specific facts.

## Access — full details

See [`docs/infrastructure/deployment-and-access.md`](../../../docs/infrastructure/deployment-and-access.md)
in this repo for the corrected Proxmox/Tengil env var names, the working `ssh proxmox` alias,
Tengil API auth schemes, and the current LAN-only safety caveat. Read it before following the
global skills literally — their documented env var names do not match Tengil's real config.

## Reference-lab apps (Epic H)

Mealie, Grocy, and Directus are deployed via Tengil as isolated, disposable reference/study
instances — never permanent runtime dependencies of spisordning itself (per `PLAN.md`'s First
Principle). Package manifests:

- Mealie: `~/dev/tengil/packages/mealie-oci.yml` (existing).
- Grocy: `~/dev/tengil/packages/grocy-oci.yml` (added as part of Epic H).
- Directus: `~/dev/tengil/packages/directus-oci.yml` (added as part of Epic H).

Deployed instance details (VMIDs, hostnames) belong in
`docs/research/mealie-deployment.md` / `docs/research/grocy-deployment.md` once live — check
those files rather than assuming a VMID.

## Spisordning's own deployment target

Deploy via `katla stack deploy` (Tengil's CLI, `~/go/bin/katla`, source in
`~/dev/tengil/cmd/katla`) using **`docker-compose.proxmox.yml`** in this repo — not
`docker-compose.yml`, which is local-dev-only:

```
katla stack deploy -node proxmox -compose docker-compose.proxmox.yml -env-file .env.proxmox
```

**spisordning already has a real, deployed database — do not create a new one.**
`main-postgres` (VMID 2327, `192.168.1.93:5432`) holds real household data; a prior deploy
attempt already created it (and apparently others like it before) without documenting it,
which is exactly the confusion this note exists to stop. Full details — password handling,
migration status, why it's on Postgres 16, how it was confirmed — are in
`docs/infrastructure/deployment-and-access.md`'s "spisordning's real database" section. Read
that before touching spisordning's data or deploying anything that creates a postgres
container.

`willys-adapter` is likewise already running standalone (`stack:willys-adapter`, VMID 2335) —
don't redeploy it either; see `docker-compose.yml`'s header comment and the
`embed-retailer-clients-in-food-brain` OpenSpec change for the longer-term plan to fold it into
food-brain directly.

Tengil's compose parser resolves `${VAR}` with no `.env`/process-env context of its own — an
unset var with no `:-` default silently becomes empty. `katla stack deploy`'s `-env-file` flag
supplies real values (like `POSTGRES_PASSWORD`) as a top-level override, but only for
standalone env keys a service leaves unset — never for a `${VAR}` embedded inside a larger
string like a `DATABASE_URL`. Keep such compose-file variables standalone if they need to come
from `-env-file`; see `katla stack deploy -h` for the mechanics.

## Epic ↔ Milestone convention

PLAN.md's workstreams are grouped into 8 Epics (see `AGENTS.md`), each a GitHub Milestone in
`androidand/spisordning` plus one `epic`-labeled tracking issue. When creating a new OpenSpec
change:

1. Confirm which epic it belongs to (check the epic tracking issues first — don't duplicate
   scope already covered by an existing change).
2. Write `proposal.md`/`tasks.md` (+ `design.md` if the domain modeling is non-trivial).
3. `specsync -dry-run -change <slug>`, then `specsync -change <slug>` to create the issue.
4. `gh issue edit <n> --milestone "<epic name>"` to file it under its epic.
5. Add `- [ ] #<n>` to the epic's tracking issue body (or use `specsync link` for
   change-to-change cross-references — milestones + the tracking issue's checklist are the
   epic-grouping mechanism, since specsync itself has no epic primitive).
