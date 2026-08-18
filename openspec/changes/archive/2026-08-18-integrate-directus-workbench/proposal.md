# Integrate Directus workbench

## Why

`PLAN.md` positions Directus as "an OPTIONAL admin/development workbench" that "must remain
removable" — never a permanent runtime dependency. `docs/research/current-state.md` confirms
Directus is currently a pure research target: "no code or config anywhere... the 'Directus
Research Spike' in `PLAN.md` has not been performed." `PLAN.md`'s own instruction is explicit:
"Do not just install Directus and start using it," followed by ten numbered questions that must
be answered first, plus a classification scheme (`SAFE_DIRECT_CRUD` / `READ_ONLY` /
`DOMAIN_CONTROLLED` / `HIDDEN`) to be applied to every collection Directus would expose.

`PLAN.md` separately defines a "Directus Exit Gate": after Recipe + Catalog + Inventory exist
(Epics B/C/D), write a keep-directus/remove-directus ADR based on observed development
experience. That ADR is explicitly **out of scope for this change** — it cannot be written yet
because Recipe/Catalog/Inventory don't exist yet, and `PLAN.md` says not to retain Directus "by
inertia" (i.e. not to skip the exit-gate exercise later just because the spike went smoothly).
This change is the spike only; the exit-gate ADR is a distinct future follow-on, noted here so it
isn't silently folded into this change's scope or forgotten.

## What Changes

- Answer, in order, `PLAN.md`'s ten Directus Research Spike questions (transcribed verbatim in
  `tasks.md`, not paraphrased):
  1. Can Spisordning remain sole migration owner?
  2. What Directus metadata does it add?
  3. Can Directus safely expose read-only PostgreSQL views?
  4. Can database permissions limit Directus writes?
  5. Which tables should be `SAFE_DIRECT_CRUD`?
  6. Which must be `DOMAIN_CONTROLLED`?
  7. How does media handling affect portability?
  8. What are current licensing implications?
  9. How painful are upgrades?
  10. Would custom Go admin endpoints actually be simpler?
- Classify every collection Directus would expose against the `SAFE_DIRECT_CRUD` / `READ_ONLY` /
  `DOMAIN_CONTROLLED` / `HIDDEN` scheme, against whatever schema exists in `migrations/` at the
  time this change is executed.
- Stand up a real Directus instance against spisordning's actual Postgres database (isolated,
  disposable, matching `PLAN.md`'s "Selected Initial Strategy" pattern already used for the
  Mealie/Grocy reference lab) to test the above questions empirically rather than from
  documentation alone.
- Produce `docs/research/directus-evaluation.md`, per `PLAN.md`'s "Research Documents" list.
- Verify the deployment invariant `PLAN.md` states directly: "stopping Directus does not affect
  Spisordning" — this is part of the "Initial Definition of Done" and must be demonstrated, not
  assumed.

## Non-Goals

- No keep-directus/remove-directus ADR — deferred to the future Directus Exit Gate, gated on
  Recipe + Catalog + Inventory (Epics B/C/D) existing first.
- No permanent wiring of Directus into `docker-compose.yml` as a required service — if added at
  all in this change's spike environment, it is explicitly optional and removable.
- No Directus-authored migrations — Spisordning's own `migrations/` stays the only migration
  owner unless spike question 1 concludes otherwise (unlikely, but not to be assumed away).
- No production admin UI decision — this change informs that decision; it does not make it.

## Capabilities

### New Capabilities

- `directus-workbench`: the invariants governing Directus as an optional, removable admin/dev
  tool — migration ownership stays with Spisordning, every exposed collection is explicitly
  classified, and Directus's presence or absence never affects Spisordning's own runtime
  correctness.

### Modified Capabilities

<!-- none -->

## Impact

- Requires a disposable Directus instance (local docker-compose or Tengil-provisioned, isolated
  persistence) pointed at a copy of spisordning's schema — not the production database, per
  `PLAN.md`'s reference-lab discipline of isolated persistence for evaluation tooling.
- Findings feed the future Directus Exit Gate ADR (out of scope here) once Epics B/C/D land.
- Cross-references `establish-enforced-go-architecture`'s CI/architecture-enforcement work: the
  spike's answer to question 4 (DB permission limits) should be checked against whatever DB role
  model that change establishes, not designed independently of it.
- Part of Epic G: AI & Admin Tooling (tracking issue #7).
