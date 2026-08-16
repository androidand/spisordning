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

`~/dev/tengil/openspec/changes/full-stack-compose-deploys/specs/spisordning/compose.yaml` is
the reference stack (Postgres + food-brain + willys-adapter). It is **aspirational** until
`establish-enforced-go-architecture` ships food-brain's HTTP server, Dockerfile, and a
published image — do not attempt to deploy it before then; check that change's status first.

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
