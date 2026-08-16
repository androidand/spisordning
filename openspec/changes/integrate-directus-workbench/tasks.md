# Tasks: integrate-directus-workbench

## 1. Spike environment

- [ ] 1.1 Stand up a disposable Directus instance (isolated persistence, matching `PLAN.md`'s
      reference-lab discipline already applied to Mealie/Grocy) pointed at a non-production copy
      of spisordning's Postgres schema.
- [ ] 1.2 Record version, container image, license, and deployment config for the Directus
      instance used, mirroring `PLAN.md`'s "Record: version / commit-tag / container image /
      license / deployment config / database-storage" requirement for reference systems.

## 2. The ten Directus Research Spike questions (verbatim from `PLAN.md`)

- [ ] 2.1 Can Spisordning remain sole migration owner?
- [ ] 2.2 What Directus metadata does it add?
- [ ] 2.3 Can Directus safely expose read-only PostgreSQL views?
- [ ] 2.4 Can database permissions limit Directus writes?
- [ ] 2.5 Which tables should be `SAFE_DIRECT_CRUD`?
- [ ] 2.6 Which must be `DOMAIN_CONTROLLED`?
- [ ] 2.7 How does media handling affect portability?
- [ ] 2.8 What are current licensing implications?
- [ ] 2.9 How painful are upgrades?
- [ ] 2.10 Would custom Go admin endpoints actually be simpler?

## 3. Collection classification

- [ ] 3.1 Enumerate every table in `migrations/` at the time this change is executed (expected to
      include, by then, tables from `establish-household-and-catalog`,
      `implement-recipe-family-and-revisions`, `implement-pantry-inventory`, this Epic's own
      `implement-shopping-and-commerce` and `implement-price-intelligence`, in addition to the
      existing `food-brain-first-slice` tables).
- [ ] 3.2 Classify every table Directus would be capable of exposing as exactly one of:
      `SAFE_DIRECT_CRUD`, `READ_ONLY`, `DOMAIN_CONTROLLED`, `HIDDEN`.
- [ ] 3.3 For each `DOMAIN_CONTROLLED` table, name the specific invariant Directus's generic CRUD
      would violate if allowed direct writes (e.g. a table with derived/computed columns, a table
      whose writes must trigger domain-layer side effects, a table underpinning an append-only
      history invariant like `price_observation` from `implement-price-intelligence`).
- [ ] 3.4 For each `HIDDEN` table, name why it must not be exposed at all (e.g. contains
      credentials, session state, or internal bookkeeping with no admin value).
- [ ] 3.5 Record the full classification table in `docs/research/directus-evaluation.md`.

## 4. Empirical verification

- [ ] 4.1 Test question 2.3 empirically: create a read-only PostgreSQL view and confirm Directus
      can browse it without being able to write through it.
- [ ] 4.2 Test question 2.4 empirically: create a restricted DB role and confirm Directus,
      connected as that role, cannot write to tables classified `READ_ONLY`/`HIDDEN`/
      `DOMAIN_CONTROLLED`.
- [ ] 4.3 Verify `PLAN.md`'s stated Definition-of-Done invariant: stopping the Directus instance
      does not affect Spisordning's own service availability or data integrity. Demonstrate this
      (stop Directus, confirm Spisordning's own API/tests still pass), do not merely assert it.
- [ ] 4.4 Confirm Spisordning's own `migrations/` remain the only place schema changes originate
      during the spike (i.e. no schema drift introduced via Directus's UI) — direct evidence for
      question 2.1.

## 5. Findings and recommendation

- [ ] 5.1 Write `docs/research/directus-evaluation.md` answering all ten questions from §2 with
      evidence from §3/§4, not speculation.
- [ ] 5.2 Recommend whether to keep evaluating Directus going forward (informing, not writing,
      the future Directus Exit Gate ADR) or to deprioritize it in favor of custom Go admin
      endpoints (question 2.10's answer should drive this directly).
- [ ] 5.3 Explicitly note in the findings doc that the keep-directus/remove-directus ADR itself is
      deferred until Recipe + Catalog + Inventory (Epics B/C/D) exist, per `PLAN.md`'s "Directus
      Exit Gate" section — this change does not write that ADR.

## 6. Verification

- [ ] 6.1 `docs/research/directus-evaluation.md` exists and answers all ten numbered questions.
- [ ] 6.2 The spike's Directus instance and any spike-only config are torn down or clearly marked
      disposable/non-production; nothing from this change becomes a required runtime dependency.
