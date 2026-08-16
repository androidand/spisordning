# Mealie reference-lab deployment

Deployed 2026-08-16 via Tengil (`androidand/tengil`, catalog app `mealie`,
`packages/mealie-oci.yml`) as an isolated study instance, per `PLAN.md`'s First Principle
(observe, don't permanently depend on).

- **Image**: `ghcr.io/mealie-recipes/mealie:latest` (OCI, official)
- **Instance**: hostname `spisordning-refs-mealie`, VMID 2319, node `proxmox`,
  `192.168.1.22:9000`
- **Health**: `GET /` → `200 OK`
- **Config**: default catalog manifest env (`ALLOW_SIGNUP=false`), 2 cores / 1024MB / 16GB disk,
  managed volume at `/app/data`

Note: an older, separate `hlab-mealie` instance also exists in the homelab's Tengil state
(currently stopped) — that one is unrelated to this reference-lab instance and is what
`spisordning/.env.example`'s `MEALIE_BASE_URL` default (`http://hlab-mealie:9000`) refers to.
The two should not be confused: this document's instance is disposable and for investigation
only (`establish-reference-lab`); `hlab-mealie` (if brought back up) would be the actual data
source for `internal/mealie` were spisordning to keep using Mealie as a live dependency, which
per `PLAN.md` it should not.

## Next steps (tracked in `establish-reference-lab`)

Investigate recipe model, editing, import, parsing, structured ingredients, foods, units,
servings, scaling, images, tags, categories, cookbooks, search, meal plans, shopping,
households, users, API, database, migrations, tests, provenance — see that change's `tasks.md`
for the full PLAN.md-derived checklist.

## See also

The investigation above is written up in full in three companion documents, each covering a
slice of `establish-reference-lab` task 2 (items 2.1–2.24) against this same live instance,
plus PLAN.md's Phase 2 database archaeology and a Mermaid ER diagram:

- `mealie-recipe-model.md` — recipe model, editing, import, parsing, structured ingredients,
  foods, units, servings, scaling, images, tags, categories, cookbooks (2.1–2.13).
- `mealie-planning-and-search.md` — search, meal plans, shopping, households, users, ratings
  (2.14–2.19).
- `mealie-api-and-database.md` — API, database, migrations, tests, provenance, full database
  archaeology, and the ER diagram (2.20–2.24).
