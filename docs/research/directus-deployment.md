# Directus reference-lab deployment

Deployed 2026-08-16 via Tengil (`androidand/tengil`, new catalog app `directus`,
`packages/directus-oci.yml`, authored as part of this restructuring) as an isolated
evaluation instance for `PLAN.md`'s Directus Research Spike — not yet pointed at
Spisordning's own Postgres database.

- **Image**: `directus/directus:latest` (official), SQLite quick-start (`DB_CLIENT=sqlite3`)
- **Instance**: hostname `spisordning-refs-directus`, VMID 2321, node `proxmox`,
  `192.168.1.216:8055`
- **Health**: `GET /server/ping` → `pong`; `GET /` → `302` (redirect to login, expected)

## Deploy issues found and fixed (worth recording — these are real Tengil gaps, not spisordning bugs)

Getting this instance running took four attempts, surfacing two genuine issues in Tengil's
catalog-install path (documented in `androidand/tengil`'s `add-reference-lab-packages` change,
task 3):

1. **`catalog.inputs` in the install request does not populate the container's env.** The
   first attempt passed `KEY`/`SECRET`/`ADMIN_PASSWORD` via `catalog: {id, inputs: {...}}`
   (mirroring the `EnvSpec.prompt` convention in `mealie-oci.yml`/`nextcloud-oci.yml`) — the
   resulting container had all three vars **empty**, and Directus (which requires `KEY`/`SECRET`
   to boot) crash-looped immediately. Fix: pass secrets via the top-level `env: {...}` field on
   the install request instead — that's what actually reaches the container.
2. **PM2-based Docker images need `HOME` set explicitly under Tengil's LXC (non-Docker) init.**
   Even with correct env, the container's `docker-entrypoint.sh ... pm2-runtime start` step
   crashed on `ENOENT /etc/.pm2/module_conf.json` / `EACCES /etc/.pm2` — pm2 was resolving its
   home directory to `/etc` because `HOME` isn't set the way a real Docker runtime would set it
   from the image's own `ENV HOME=...`. Fix: explicitly pass `HOME=/directus` (and
   `PM2_HOME=/directus/.pm2`) in the install request's `env`. This is likely to recur for any
   other PM2/Node-based OCI image deployed this way — worth a general note in Tengil's
   OCI-to-LXC conversion docs.

A third, separate issue (unrelated to the two above) was an **email-validation rejection of
`admin@spisordning.local`** during Directus's own bootstrap (`.local` isn't accepted by its
email validator) — switched to `admin@example.com` (the manifest's own default) instead.

Also found in passing: a VMID allocation race when two catalog installs are submitted back to
back (both got assigned VMID 2320) — install sequentially, not concurrently, until Tengil's
VMID allocator is confirmed atomic under concurrent requests.

## Next steps (tracked in `integrate-directus-workbench`)

Answer the 10 spike questions from `PLAN.md` against this live instance; classify collections
as SAFE_DIRECT_CRUD/READ_ONLY/DOMAIN_CONTROLLED/HIDDEN once Spisordning's own schema exists to
evaluate against.
