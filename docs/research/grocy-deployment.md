# Grocy reference-lab deployment

Deployed 2026-08-16 via Tengil (`androidand/tengil`, new catalog app `grocy`,
`packages/grocy-oci.yml`, authored as part of this restructuring — Grocy had no prior Tengil
manifest) as an isolated study instance, per `PLAN.md`'s First Principle.

- **Image**: `lscr.io/linuxserver/grocy:latest` (the LinuxServer.io image, Grocy's own
  community-recommended container)
- **Instance**: hostname `spisordning-refs-grocy`, VMID 2320, node `proxmox`,
  `192.168.1.183:80`
- **Health**: `GET /` → `302` (redirect to login — expected, healthy)
- **Config**: `PUID=1000`, `PGID=1000`, `TZ=Europe/Stockholm`, `GROCY_CULTURE=en`; 1 core /
  512MB / 8GB disk, managed volume at `/config` (SQLite DB + uploads)

## Next steps (tracked in `establish-reference-lab`)

Investigate products, barcodes, locations, stock, stock journal, lots, expiry, purchase,
consume, discard, transfer, adjust, mark empty, units, unit conversion, product-specific
conversion, shopping, recipes, meal planning, cost tracking, API, database, migrations, tests
— see that change's `tasks.md` for the full PLAN.md-derived checklist. This directly informs
`implement-pantry-inventory`'s inventory-event vocabulary
(PURCHASE/CONSUME/DISCARD/ADJUST/TRANSFER/MARK_EMPTY/OPEN), which explicitly names Grocy's
behavior as its primary reference.

## See also

That investigation is complete — see `grocy-inventory-and-stock.md` (products, barcodes,
locations, stock, stock journal, lots, expiry, purchase/consume/discard/transfer/adjust/
mark-empty), `grocy-units-and-planning.md` (units, unit conversion, product-specific conversion,
shopping, recipes, meal planning, cost tracking), and `grocy-api-and-database.md` (API, database,
migrations, tests, full database archaeology, and a Mermaid ER diagram).
