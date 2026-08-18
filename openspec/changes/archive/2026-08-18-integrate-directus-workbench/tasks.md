# Tasks: integrate-directus-workbench

## 1. Spike environment

- [x] 1.1 Stand up a disposable Directus instance (isolated persistence, matching `PLAN.md`'s
      reference-lab discipline already applied to Mealie/Grocy) pointed at a non-production copy
      of spisordning's Postgres schema.
  - [x] `docker-compose.directus-spike.yml` (Directus 11.17.4 + Postgres 16, isolated `spike_pgdata`
        volume, ports 8056/5434); all 7 migrations applied; `admin` + `spisordning` + `directus_ro`
        roles created. Torn down after the spike.
- [x] 1.2 Record version, container image, license, and deployment config for the Directus
      instance used, mirroring `PLAN.md`'s "Record: version / commit-tag / container image /
      license / deployment config / database-storage" requirement for reference systems.
  - [x] Captured in `docs/research/directus-evaluation.md` §2 (version 11.17.4, image
        `directus/directus:11`, digest, BSL 1.1 license, ports, DB storage).

## 2. The ten Directus Research Spike questions (verbatim from `PLAN.md`)

- [x] 2.1 Can Spisordning remain sole migration owner?
  - [x] Yes — Directus never owns spisordning tables; `directus_ro` cannot `CREATE` in `public`. See doc §2.1 / §4.4.
- [x] 2.2 What Directus metadata does it add?
  - [x] 29 `directus_*` metadata tables on first boot (Directus schema, not spisordning). See doc §2.2.
- [x] 2.3 Can Directus safely expose read-only PostgreSQL views?
  - [x] Not auto — a view (`person_readonly`) existed in Postgres but Directus did not expose it, even after restart. Manual collection config would be required. See doc §2.3 / §4.1.
- [x] 2.4 Can database permissions limit Directus writes?
  - [x] Yes — restricted role `directus_ro` blocked writes to `DOMAIN_CONTROLLED` `preference_observation` while allowing `person`. See doc §2.4 / §4.2.
- [x] 2.5 Which tables should be `SAFE_DIRECT_CRUD`?
  - [x] 9 tables: `person`, `ingredient`, `ingredient_mapping`, `effort_profile`, `planning_constraint`, `external_recipe_source`, `recipe_variant`, `shopping_list`, `shopping_list_item`. See doc §2.5.
- [x] 2.6 Which must be `DOMAIN_CONTROLLED`?
  - [x] 20 tables (recipes, preferences, meals, plans, shopping, commerce, revisions, imports). See doc §2.6.
- [x] 2.7 How does media handling affect portability?
  - [x] Directus media/files live in its own storage (S3/disk), separate from spisordning; would add a second media store. See doc §2.7.
- [x] 2.8 What are current licensing implications?
  - [x] BSL 1.1 (not open source); production use allowed ≤ $5M/12mo; each version → GPL v3 after 3 years. See doc §2.8.
- [x] 2.9 How painful are upgrades?
  - [x] Moderate — Directus manages its own `directus_*` schema migrations on upgrade; spisordning tables untouched, but Directus upgrades are a separate moving part. See doc §2.9.
- [x] 2.10 Would custom Go admin endpoints actually be simpler?
  - [x] Yes, for the current slice — no UI needed yet, and Go endpoints reuse existing domain invariants. See doc §2.10.

## 3. Collection classification

- [x] 3.1 Enumerate every table in `migrations/` at the time this change is executed (expected to
      include, by then, tables from `establish-household-and-catalog`,
      `implement-recipe-family-and-revisions`, `implement-pantry-inventory`, this Epic's own
      `implement-shopping-and-commerce` and `implement-price-intelligence`, in addition to the
      existing `food-brain-first-slice` tables).
  - [x] 29 tables across 7 migration files enumerated. See doc §3.
- [x] 3.2 Classify every table Directus would be capable of exposing as exactly one of:
      `SAFE_DIRECT_CRUD`, `READ_ONLY`, `DOMAIN_CONTROLLED`, `HIDDEN`.
  - [x] 9 `SAFE_DIRECT_CRUD`, 0 `READ_ONLY`, 20 `DOMAIN_CONTROLLED`, 0 `HIDDEN`. See doc §3.
- [x] 3.3 For each `DOMAIN_CONTROLLED` table, name the specific invariant Directus's generic CRUD
      would violate if allowed direct writes (e.g. a table with derived/computed columns, a table
      whose writes must trigger domain-layer side effects, a table underpinning an append-only
      history invariant like `price_observation` from `implement-price-intelligence`).
  - [x] Per-table invariants named (derived columns, append-only history, domain side effects). See doc §3.
- [x] 3.4 For each `HIDDEN` table, name why it must not be exposed at all (e.g. contains
      credentials, session state, or internal bookkeeping with no admin value).
  - [x] No `HIDDEN` tables in the current slice (0). Noted in doc §3.
- [x] 3.5 Record the full classification table in `docs/research/directus-evaluation.md`.
  - [x] Full classification table recorded in doc §3.

## 4. Empirical verification

- [x] 4.1 Test question 2.3 empirically: create a read-only PostgreSQL view and confirm Directus
      can browse it without being able to write through it.
  - [x] View `person_readonly` created; Directus did NOT auto-expose it (even after restart) → manual config required. See doc §4.1.
- [x] 4.2 Test question 2.4 empirically: create a restricted DB role and confirm Directus,
      connected as that role, cannot write to tables classified `READ_ONLY`/`HIDDEN`/
      `DOMAIN_CONTROLLED`.
  - [x] Restricted role `directus_ro` created; Directus as that role was blocked writing to `DOMAIN_CONTROLLED` `preference_observation` while allowed to write `person`. See doc §4.2.
- [x] 4.3 Verify `PLAN.md`'s stated Definition-of-Done invariant: stopping the Directus instance
      does not affect Spisordning's own service availability or data integrity. Demonstrate this
      (stop Directus, confirm Spisordning's own API/tests still pass), do not merely assert it.
  - [x] Directus stopped; `go build`, `go vet`, `go test ./...` all passed (103 tests, 10 packages). See doc §4.3.
- [x] 4.4 Confirm Spisordning's own `migrations/` remain the only place schema changes originate
      during the spike (i.e. no schema drift introduced via Directus's UI) — direct evidence for
      question 2.1.
  - [x] All spisordning tables owned by `spisordning`; `directus_ro` cannot `CREATE` in `public` → no Directus-driven schema drift. See doc §4.4.

## 5. Findings and recommendation

- [x] 5.1 Write `docs/research/directus-evaluation.md` answering all ten questions from §2 with
      evidence from §3/§4, not speculation.
  - [x] `docs/research/directus-evaluation.md` written, all ten questions answered with evidence.
- [x] 5.2 Recommend whether to keep evaluating Directus going forward (informing, not writing,
      the future Directus Exit Gate ADR) or to deprioritize it in favor of custom Go admin
      endpoints (question 2.10's answer should drive this directly).
  - [x] Recommendation: deprioritize Directus for now in favor of custom Go admin endpoints; keep the option open. See doc §5.
- [x] 5.3 Explicitly note in the findings doc that the keep-directus/remove-directus ADR itself is
      deferred until Recipe + Catalog + Inventory (Epics B/C/D) exist, per `PLAN.md`'s "Directus
      Exit Gate" section — this change does not write that ADR.
  - [x] ADR deferral explicitly noted in doc §5 (per `PLAN.md` Directus Exit Gate).

## 6. Verification

- [x] 6.1 `docs/research/directus-evaluation.md` exists and answers all ten numbered questions.
  - [x] Doc exists and answers all ten numbered questions (§2.1–2.10).
- [x] 6.2 The spike's Directus instance and any spike-only config are torn down or clearly marked
      disposable/non-production; nothing from this change becomes a required runtime dependency.
  - [x] Spike torn down (`docker compose -f docker-compose.directus-spike.yml down -v`); compose files marked disposable/non-production; no Go code references Directus.
