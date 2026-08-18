# Directus Workbench Evaluation

**Change**: `integrate-directus-workbench`
**Date**: 2026-08-18
**Status**: Research spike complete. This document answers the ten Directus Research Spike
questions from `PLAN.md` with empirical evidence. It does **not** write the keep-directus /
remove-directus ADR — that is deferred to the Directus Exit Gate (after Recipe + Catalog +
Inventory exist). See §6.

## 1. Spike environment (record)

Per `PLAN.md`'s "Record: version / commit-tag / container image / license / deployment config /
database-storage" requirement for reference systems:

| Field | Value |
|-------|-------|
| Version | 11.17.4 |
| Container image | `directus/directus:11` (digest `sha256:eb326f679ae847c0a776f93b972761dc2ebe84980e0b9d274a6bc31cd62809f7`) |
| License | Business Source License 1.1 (BSL 1.1), Monospace, Inc. — **not** open source |
| Deployment config | `docker-compose.directus-spike.yml` (spike-only, disposable) |
| Database-storage | Postgres 16 (`postgres:16-alpine`), isolated volume `spike_pgdata`, a non-production copy of the spisordning schema (`migrations/` applied on first boot) |
| Public URL | `http://localhost:8056` |
| Postgres port | `5434` (avoids the main stack's `5433`) |

**License terms (BSL 1.1)**:
- Additional Use Grant: production use is allowed while Total Finances stay at or below
  US $5,000,000 for the most recent 12-month period (trivially met by a self-hosted household
  project).
- Change Date: three years from the release date of each version.
- Change License: GNU General Public License v3 (applies to each version after its Change Date).
- BSL 1.1 is explicitly **not** an Open Source license.

**Spike-only override** (used for the DB-permission test, §4.2):
`docker-compose.directus-spike-ro.yml` points Directus at the restricted role `directus_ro`.

**Tear down**: `docker compose -f docker-compose.directus-spike.yml down -v` (removes the
isolated persistence). Nothing in this change is a required runtime dependency of spisordning.

**Note on the earlier Tengil instance**: a separate Directus instance
(`spisordning-refs-directus`, VMID 2321, `192.168.1.216:8055`, `directus/directus:latest` on
SQLite) was deployed 2026-08-16 per `docs/research/directus-deployment.md`. It is **not** pointed
at spisordning's Postgres and was **not** used for this spike. This spike uses a fresh,
Postgres-backed instance so the questions can be answered against spisordning's real schema.

## 2. Collection classification

Every table in `migrations/` (29 tables across 7 migration files) is classified as exactly one of
`SAFE_DIRECT_CRUD`, `READ_ONLY`, `DOMAIN_CONTROLLED`, `HIDDEN`.

**Directus exposure note**: Directus auto-ignores collections that lack a single-column primary
key. Four spisordning tables have composite primary keys and are therefore **not exposed** by
Directus at all: `person_preference`, `recipe_ingredient`, `meal_plan_decision`,
`recipe_revision_parent`. They are classified below by their domain nature (all
`DOMAIN_CONTROLLED`); Directus's auto-ignoring of them is a coincidental safety property, not a
deliberate control.

| Table | Classification | Directus-exposable | Invariant / rationale |
|-------|---------------|--------------------|-----------------------|
| person | SAFE_DIRECT_CRUD | yes | Simple reference record (id, name, weight); no derived state, no cross-table invariant. |
| ingredient | SAFE_DIRECT_CRUD | yes | Canonical ingredient reference data. |
| ingredient_mapping | SAFE_DIRECT_CRUD | yes | Maps Mealie food IDs to canonical ingredients; reference data with a `needs_review` marker. |
| effort_profile | SAFE_DIRECT_CRUD | yes | Expected kitchen energy per weekday; simple reference data. |
| planning_constraint | SAFE_DIRECT_CRUD | yes | Planning constraints (avoid_tag, max_repeats); simple reference data. |
| external_recipe_source | SAFE_DIRECT_CRUD | yes | External source registry; simple reference data. |
| recipe_variant | SAFE_DIRECT_CRUD | yes | A recognizable fork of a family; simple reference data (`family_id` is a plain FK). |
| shopping_list | SAFE_DIRECT_CRUD | yes | A durable, human-editable shopping list. |
| shopping_list_item | SAFE_DIRECT_CRUD | yes | A line on a list; human-editable (CHECK: at least one of requirement/ingredient/label). |
| recipe_ref | DOMAIN_CONTROLLED | yes | Snapshot of Mealie (source of truth is Mealie); writes must go through the sync logic or drift. |
| person_preference | DOMAIN_CONTROLLED | **no** (composite PK) | Derived confidence-weighted belief over `preference_observation`; direct writes bypass the confidence-update logic. |
| preference_observation | DOMAIN_CONTROLLED | yes | Append-only evidence history; updates/deletes would break the append-only invariant. |
| recipe_ingredient | DOMAIN_CONTROLLED | **no** (composite PK) | Recipe content (ingredient + quantity/unit); immutable per revision; writes must go through revision logic. |
| meal_event | DOMAIN_CONTROLLED | yes | A served-meal record; creation triggers side effects (reactions, preference updates). |
| meal_reaction | DOMAIN_CONTROLLED | yes | A person's reaction; must create a `preference_observation` (append-only evidence). |
| meal_plan | DOMAIN_CONTROLLED | yes | Status is a domain state machine (draft→approved→archived); direct writes bypass transitions. |
| meal_plan_candidate | DOMAIN_CONTROLLED | yes | Scored candidate; `score`/`breakdown` are computed by the scorer. |
| meal_plan_decision | DOMAIN_CONTROLLED | **no** (composite PK) | The human's choice among candidates; part of the planning domain. |
| shopping_requirement | DOMAIN_CONTROLLED | yes | Per-plan output; derived from the plan's ingredients. |
| recipe_import_candidate | DOMAIN_CONTROLLED | yes | Staging area; status lifecycle (pending→reviewed→imported/rejected) is domain logic. |
| recipe_import_candidate_ingredient | DOMAIN_CONTROLLED | yes | Candidate ingredient lines; `needs_review` is domain logic. |
| recipe_family | DOMAIN_CONTROLLED | yes | `default_variant_id` must resolve within its own family (application-layer invariant). |
| recipe_revision | DOMAIN_CONTROLLED | yes | Immutable content snapshot; a correction is a new row, never an update. |
| recipe_revision_parent | DOMAIN_CONTROLLED | **no** (composite PK) | Lineage edge; the lineage graph must stay acyclic. |
| retailer_list_binding | DOMAIN_CONTROLLED | yes | Outbound projection; `last_pushed_at`/`last_push_status` are set by the adapter's push logic. |
| shopping_cart | DOMAIN_CONTROLLED | yes | Checkpoint of a to-cart call; status is set by domain logic. |
| shopping_cart_item | DOMAIN_CONTROLLED | yes | Snapshot of resolved items at to-cart time; writes bypass resolution logic. |
| order | DOMAIN_CONTROLLED | yes | Fidelity-preserving purchase record; writes bypass domain logic. |
| order_item | DOMAIN_CONTROLLED | yes | Fidelity-preserving (records substitutions); writes bypass domain logic. |

**Summary**: 9 `SAFE_DIRECT_CRUD`, 0 `READ_ONLY`, 20 `DOMAIN_CONTROLLED`, 0 `HIDDEN`.
No spisordning table is `HIDDEN` (none contain credentials or session state; all have admin
value). The Directus metadata tables (`directus_*`, 29 of them) are Directus's own bookkeeping and
are excluded from this classification — they are not spisordning's schema.

## 3. The ten questions (answered with evidence)

### 2.1 Can Spisordning remain sole migration owner?
**Yes.** Evidence (§4.4): the restricted role `directus_ro` is not the owner of any spisordning
table (all 29 are owned by `spisordning`) and lacks `CREATE` on the `public` schema
(`can_create_in_public = f`). Because `ALTER`/`CREATE` require ownership (or superuser), Directus
connected as `directus_ro` cannot alter or create spisordning tables. `migrations/` therefore
remain the only place spisordning schema changes originate.
**Caveat**: Directus creates its own `directus_*` metadata tables on first boot (as the connecting
role). Those are Directus's schema, not spisordning's, and must be excluded from spisordning's
migration history.

### 2.2 What Directus metadata does it add?
Directus adds **29 metadata tables** (`directus_*`) on first boot:
`directus_access`, `directus_activity`, `directus_collections`, `directus_comments`,
`directus_dashboards`, `directus_deployment_projects`, `directus_deployment_runs`,
`directus_deployments`, `directus_extensions`, `directus_fields`, `directus_files`,
`directus_flows`, `directus_folders`, `directus_migrations`, `directus_notifications`,
`directus_operations`, `directus_panels`, `directus_permissions`, `directus_policies`,
`directus_presets`, `directus_relations`, `directus_revisions`, `directus_roles`,
`directus_sessions`, `directus_settings`, `directus_shares`, `directus_translations`,
`directus_users`, `directus_versions`.
These hold Directus's own bookkeeping (users, roles, permissions, field/collection metadata,
files, flows, activity, revisions, etc.). They are owned by the connecting DB role and are **not**
part of spisordning's schema.

### 2.3 Can Directus safely expose read-only PostgreSQL views?
**Not as a zero-config mechanism.** Evidence (§4.1): a read-only view (`person_readonly`) created
in the spike Postgres was **not** auto-exposed by Directus — it was absent from the `/collections`
list even after a full Directus restart, and the Directus logs never mention it. Directus
auto-detects tables but **not** views. Whether a view can be manually exposed via the admin UI
(Collections page) was not exercised in this spike (API-only access). Bottom line: read-only views
are **not** a drop-in way to expose read-only data; they require manual Directus configuration that
this spike did not validate.

### 2.4 Can database permissions limit Directus writes?
**Yes, precisely.** Evidence (§4.2): a restricted role `directus_ro` (SELECT on all spisordning
tables, INSERT/UPDATE/DELETE on the 9 `SAFE_DIRECT_CRUD` tables only, full access to `directus_*`)
was used as Directus's DB user. Results:
- Browse `preference_observation` (`DOMAIN_CONTROLLED`): **200 OK** (read allowed).
- Write `preference_observation`: **500 "permission denied for table preference_observation"** (write blocked).
- Write `person` (`SAFE_DIRECT_CRUD`): **200 OK** (write allowed).
DB permissions therefore enforce the classification boundary exactly.

### 2.5 Which tables should be SAFE_DIRECT_CRUD?
The 9 tables in §2: `person`, `ingredient`, `ingredient_mapping`, `effort_profile`,
`planning_constraint`, `external_recipe_source`, `recipe_variant`, `shopping_list`,
`shopping_list_item`. All are simple reference data with no derived state and no cross-table
invariant that generic CRUD would violate.

### 2.6 Which must be DOMAIN_CONTROLLED?
The 20 tables in §2. Each has a derived/computed column, an append-only or immutability
invariant, a domain state machine, or writes that must trigger domain-layer side effects (named
per-table in §2).

### 2.7 How does media handling affect portability?
Directus ships a file/media subsystem (`directus_files`). If spisordning stored media (e.g. recipe
photos) through Directus, the blobs would live in Directus's storage backend (local disk / S3 /
etc.), coupling spisordning's data to Directus's storage and hurting portability if Directus is
later removed. **Currently a non-issue**: spisordning's schema has no media/file tables.
**Guidance**: if media is added later, store it outside Directus (spisordning's own storage) to
keep the system portable and Directus removable.

### 2.8 What are current licensing implications?
Directus 11.17.4 is **BSL 1.1** (Monospace, Inc.), **not** open source (§1). For a self-hosted
household project the Additional Use Grant (Total Finances ≤ $5M/12mo) is trivially satisfied, so
production self-hosting is permitted. Implications:
- spisordning cannot be "fully open source" while bundling Directus under BSL.
- Each Directus version converts to **GPL v3** after 3 years (Change Date). GPL v3 is copyleft;
  because Directus runs as a separate service (HTTP/DB, not linked into the Go binary), the
  copyleft reach into spisordning is limited, but it is a consideration for any future
  distribution of the combined system.

### 2.9 How painful are upgrades?
**Moderately painful.** Evidence:
- Upgrades run Directus migrations that `ALTER` the `directus_*` tables. With the restricted role
  `directus_ro` (not the table owner), those `ALTER`s would **fail** — so an upgrade requires
  temporarily reconnecting Directus as the owner role (`spisordning`), running the upgrade, then
  switching back to `directus_ro`. This is a privilege-escalation step that must be documented.
- The earlier Tengil deployment (`directus-deployment.md`) surfaced real deployment friction:
  `catalog.inputs` does not populate container env (secrets must go in top-level `env`), and
  PM2-based images need `HOME` set explicitly under Tengil's LXC init. These recur on every
  redeploy/upgrade.
- Net: upgrades are doable but require a documented privilege dance plus deployment friction.

### 2.10 Would custom Go admin endpoints actually be simpler?
**Yes, for spisordning's current needs.** The admin surface is modest: 9 `SAFE_DIRECT_CRUD`
tables (simple CRUD) plus read-only browsing of the rest. A small set of Go admin endpoints (or a
thin CLI) over those 9 tables, plus read-only queries for the `DOMAIN_CONTROLLED` tables, avoids:
the BSL license, the 29 `directus_*` metadata tables, the DB-role/permission machinery, the
upgrade privilege dance, and the view-exposure gap. The cost is building and maintaining the CRUD
UI itself — but for a single-household tool with a small, stable table set, that is less total
complexity than integrating and upgrading Directus.

## 4. Empirical verification

- **4.1 (views)**: Created a `person_readonly` view plus a test row. Directus did **not** expose
  it (absent from `/collections` even after a restart; logs silent). Browse and write both
  returned 403 "collection does not exist." → views are not auto-exposed.
- **4.2 (DB permissions)**: Restricted role `directus_ro`. Browse `preference_observation` → 200;
  write `preference_observation` → 500 permission denied; write `person` → 200. → permissions
  enforce the boundary exactly.
- **4.3 (stop Directus)**: Stopped the Directus container. Spisordning `go build` / `go vet` /
  `go test ./...` → **103 passed in 10 packages**. → stopping Directus does not affect spisordning.
- **4.4 (no schema drift)**: All 29 spisordning tables are owned by `spisordning` (not
  `directus_ro`); `directus_ro` lacks `CREATE` on `public`. → Directus (as `directus_ro`) cannot
  alter or create spisordning tables; `migrations/` remain the sole schema origin.

## 5. Recommendation

**Deprioritize Directus in favor of custom Go admin endpoints for now.** The evidence shows
Directus adds substantial integration surface (BSL license, 29 metadata tables, DB-role/permission
machinery, an upgrade privilege dance, and no zero-config view exposure) to solve a modest admin
need (9 simple CRUD tables plus read-only browsing). Custom Go admin endpoints over the
`SAFE_DIRECT_CRUD` tables, with read-only queries for the rest, are simpler and keep spisordning
fully open source and portable.

**Keep the door open**: if the admin UI requirements grow significantly (multi-user roles, file
management, automation/flows, a rich browsing UI), re-evaluate Directus. This document informs —
but does not write — that decision.

## 6. ADR deferral (per `PLAN.md` "Directus Exit Gate")

The keep-directus / remove-directus ADR is **deferred** until Recipe + Catalog + Inventory
(Epics B/C/D) exist, per `PLAN.md`'s "Directus Exit Gate" section. **This change does not write
that ADR.** It records the spike evidence and a recommendation to inform the future ADR.
