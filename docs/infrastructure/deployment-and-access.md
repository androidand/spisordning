# Deployment and infrastructure access

This is the repo-local source of truth for reaching Proxmox, Tengil, and GitHub from this
project. It exists because the global `proxmox`/`tengil` skills (`~/.config/opencode/skills/`,
symlinked into `~/.claude/skills/`) document environment variable names that **do not match**
the real application config, and neither skill says where credentials should actually live.
Read this before following the skills literally.

## Proxmox

The skills document `PROXMOX_HOST` / `PROXMOX_TOKEN_NAME` / `PROXMOX_TOKEN`. The real app
config (`~/dev/tengil/.env.example`, and Tengil's `internal/config/config.go`) uses different
names, with the skill names accepted only as secondary fallbacks:

| Skill env var (docs) | Tengil's real/primary env var | Notes |
|---|---|---|
| `PROXMOX_HOST` | `PROXMOX_URL` | Tengil wants a full URL (`https://192.168.1.42:8006`); it also accepts `PROXMOX_HOST` + `PROXMOX_PORT` as a fallback pair. |
| `PROXMOX_TOKEN_NAME` | `PROXMOX_TOKEN_ID` | Full token id, e.g. `root@pam!tengil`. Tengil accepts `PROXMOX_TOKEN_NAME` as a fallback. |
| `PROXMOX_TOKEN` | `PROXMOX_TOKEN_SECRET` | The token secret (UUID). Tengil accepts `PROXMOX_TOKEN` as a fallback. |
| `PROXMOX_VERIFY_SSL` | `PROXMOX_INSECURE_TLS` | Inverted polarity — check the sense before setting. |

**When calling the Proxmox REST API directly** (per the `proxmox` skill), either naming works
since Tengil accepts both — but prefer the `PROXMOX_URL`/`PROXMOX_TOKEN_ID`/`PROXMOX_TOKEN_SECRET`
names in anything written down here, since that's what the actual app and its `.env.example`
use.

**SSH to the Proxmox host**: a working alias already exists in `~/.ssh/config`:

```
Host proxmox
    User root
    HostName 192.168.1.42
    IdentityFile ~/.ssh/id_ed25519
```

Prefer `ssh proxmox` over `ssh root@${PROXMOX_HOST}` — it's equivalent but doesn't depend on
the env var being set correctly in your shell. Container-level commands: `ssh proxmox pct exec
<VMID> -- <command>`. There's also a `Host z4` alias (`192.168.1.106`) for a second Proxmox
node, unrelated to the primary `proxmox` host.

## Tengil

Tengil is the deployment target for the reference-lab apps (Mealie/Grocy/Directus) and,
eventually, spisordning itself. It runs as an LXC on the Proxmox host above (VMID `300` by
default).

- `TENGIL_HOST` — same host as Proxmox unless otherwise noted.
- Three auth schemes to Tengil's own API (`http://${TENGIL_HOST}:8080`): session cookie
  (`tengil_session=<token>`, from `/auth/login`), bearer token (`Authorization: Bearer <token>`,
  set via `TENGIL_API_TOKEN` on the Tengil side), or Proxmox-token passthrough
  (`Authorization: PVEAPIToken=${PROXMOX_TOKEN_ID}=${PROXMOX_TOKEN_SECRET}`).
- **Current safety caveat**: Tengil's `harden-api-auth-model` OpenSpec change is still open —
  there is a known auth-bypass issue being fixed. Only reach the Tengil API from the trusted
  LAN; do not expose it further.
- Deploy scripts live in the tengil repo (`scripts/quick-deploy.sh`, `scripts/deploy-tengil-files.sh`)
  and only apply to redeploying Tengil itself. To deploy a *sibling app* (like the reference-lab
  apps) through Tengil's own API, use `scripts/deploy-local-profile.sh --app <name> [options]`
  or the `/api/apps` endpoint directly (`type: oci`, `type: compose`, or `type: package` —
  see `~/dev/tengil/docs/compose-packages.md`).
- Reference stack definitions for this project live in the **tengil repo**, not here:
  `~/dev/tengil/openspec/changes/full-stack-compose-deploys/specs/spisordning/compose.yaml`
  (Postgres + food-brain + willys-adapter) and the reference-lab package manifests under
  `~/dev/tengil/packages/` (`mealie-oci.yml` exists; `grocy-oci.yml`/`directus-oci.yml` are
  added as part of Epic H).

## GitHub / specsync

- `gh` CLI is already authenticated on this machine (`gh auth status` → account `androidand`,
  broad token scopes including `repo`, `admin:org`, `workflow`). specsync shells out to `gh`
  for everything — it has no credential handling of its own, and there is nothing to configure.
- Repo target is auto-detected from `git remote` (or pass `-repo owner/name`). Once
  `androidand/spisordning` exists as a remote, no further config is needed.

## What NOT to put in this repo's `.env`

`spisordning/.env` holds only this project's own app config (`POSTGRES_PASSWORD`,
`WILLYS_USERNAME`/`WILLYS_PASSWORD`, `MEALIE_BASE_URL`/`MEALIE_API_TOKEN`, etc — see
`.env.example`). Proxmox/Tengil credentials are cross-project infrastructure secrets that
belong to the tengil repo's own `.env` (`/opt/tengil/.env` on the deployed container itself is
a *separate* copy again — killing the tengil process respawns it from that file, not from your
shell) and to `~/.ssh/config`. Do not duplicate them here.
