# Research and integrate ICA

## Why

`PLAN.md`'s "External Research — ICA" section asks Spisordning to investigate current ICA access
before designing any ICA-side retailer interface — the same discipline `willys-capabilities.md`
already applied to Willys ("Do not design the retailer interface until this real capability map
exists"). No ICA research exists anywhere in this repo or its siblings today
(`docs/research/current-state.md` covers Mealie/Grocy/Directus/Willys only; there is no
`docs/research/ica-current-api.md` yet, though `PLAN.md`'s own "Research Documents" list expects
one). Two seed repositories are named as starting points — `ha-ica-todo` (newer, 2026-active) and
`ica-api` (older, flagged as inaccurate since ICA's April 2024 API changes) — and `ha-ica-todo` is
called out as containing an `ICA+Grocy.md` document describing an inventory lifecycle similar to
Spisordning's own, worth extracting ideas from without inheriting its Home-Assistant-specific
design.

This is a **pure research change**: its deliverable is investigation findings and a documented
recommendation, not ICA integration code. Per the task brief, this proposal does not require live
internet access to be written and does not fabricate findings — `tasks.md` below lists the
concrete investigation steps to be executed later, with explicit placeholders for what must be
verified rather than invented answers.

## What Changes

- Investigate `https://github.com/svendahlstrand/ica-api` (older client): confirm or refute
  `PLAN.md`'s stated initial observation that its documentation became inaccurate after ICA's
  April 2024 API changes; determine what, if anything, still works.
- Investigate `https://github.com/LazyTarget/ha-ica-todo` (newer client): confirm the presence
  and scope of 2026 work; catalog what it implements or investigates — shopping lists, offers,
  article grouping, auth refresh, synchronization.
- Locate and carefully inspect `ICA+Grocy.md` (or equivalent) within `ha-ica-todo`; extract
  useful ideas about ICA-side inventory lifecycle modeling **without** inheriting
  Home-Assistant-specific design choices (HA entity/service conventions, HA-specific
  auth/storage patterns) unless a design independently justifies adopting them.
- Determine current ICA authentication mechanism (login flow, token/session refresh, MFA if any)
  and its stability/reverse-engineered-ness, since this determines whether an ICA adapter is
  viable at all today.
- Produce `docs/research/ica-current-api.md` (per `PLAN.md`'s "Research Documents" list) as the
  durable record of findings, superseding nothing but filling a documented gap.
- Recommend, but do not implement, an ICA adapter design informed by the `retailer-adapter`
  capability's existing shape (so a future ICA adapter is structurally consistent with the
  Willys one) — actual ICA adapter implementation is explicitly out of scope for this change and
  would be a distinct future change once these findings exist.

## Non-Goals

- No ICA adapter implementation — this change is research only.
- No changes to the existing `retailer-adapter` capability (Willys) — ICA is additive, evaluated
  independently.
- No inheriting Home-Assistant-specific design from `ha-ica-todo` by default — each idea from
  `ICA+Grocy.md` must be evaluated on its own merits for Spisordning's domain model.
- No fabricated API behavior, pricing, or endpoint claims in this proposal — anything not already
  verifiable from `PLAN.md`'s stated initial observations is recorded as an open question in
  `tasks.md`, to be resolved when the investigation is actually executed.

## Capabilities

### New Capabilities

- `ica-integration`: currently a research-stage capability recording the non-negotiable
  invariants any future ICA adapter must satisfy (consistent with `retailer-adapter`'s existing
  invariants) and the documented state of ICA API access. It does not yet describe a working ICA
  adapter.

### Modified Capabilities

<!-- none — retailer-adapter (Willys) is untouched by this change -->

## Impact

- New `docs/research/ica-current-api.md`.
- No code changes to `internal/retailer` or the sibling `willys-client`/`willys-adapter` repos.
- Informs, but does not implement, a future `integrate-ica` change (not yet scheduled) once
  findings confirm current API access is viable.
- Cross-references `openspec/specs/retailer-adapter/spec.md` as the structural template a future
  ICA adapter should follow (pinning, review queue, confidence/quantity-uncertainty split, no
  automated checkout) rather than reinventing those invariants per retailer.
- Part of Epic F: Retailer, Pricing & Commerce (tracking issue #6).
- **Update (2026-08-18)**: a new sibling `~/dev/willys/ica-client` scaffold has independently
  started, mirroring `willys-client`'s architecture but targeting a different ICA surface (the
  web storefront, not the mobile API this research verified) with auth still unresolved. Tracked
  as an addendum in `docs/research/ica-current-api.md` rather than reopening this change's
  tasks, since it doesn't change this change's own completed scope — but a future `integrate-ica`
  change should read that addendum before assuming this document alone settles which ICA surface
  to build against.
