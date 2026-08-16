# Integrate AI provider abstraction

## Why

`internal/llm/olla.go` already implements a real, tested client for the local **Olla** proxy — an
OpenAI-compatible endpoint — used additively by the meal-planning suggestion engine (`Explain` for
human-readable Swedish explanations, `ProposeOrder` for reordering within an already-feasible
candidate set; per its own doc comment, "the LLM can reorder within the feasible set, nothing
more"). `PLAN.md`'s "AI Provider" section asks for exactly this to be generalized: "Implement
provider abstraction later. Primary initial target: OpenAI-compatible API, for local llama-skein.
AI SHALL call application-layer tools. Never expose unrestricted SQL." The client today is
concrete and Olla-specific (`baseURL`/`model` constructor, hand-rolled `chatRequest`/
`chatResponse` structs, two purpose-built methods). This change formalizes what already exists —
it does not start from zero, and it does not replace Olla as the primary target; Olla stays the
initial, already-deployed backend behind a new interface.

This change owns the **provider-abstraction layer only** — an interface any OpenAI-compatible
endpoint could satisfy, with Olla as the first and primary implementation. It explicitly does
**not** own the MCP tool surface itself (the actual set of application-layer tools an AI may
call); that is Epic A's `implement-mcp-server` change. The two changes share one invariant —
tools only, never raw SQL — phrased consistently so neither change re-derives it independently.

## What Changes

- Define a `Provider` interface (name TBD during implementation, e.g. `llm.Provider`) generalizing
  `internal/llm/olla.go`'s current concrete shape: a chat-completion call over an OpenAI-compatible
  endpoint, parameterized by base URL, model, and timeout, rather than hard-coded to one Olla
  instance.
- Reimplement `Client` (Olla) as the interface's first, primary implementation — behavior-
  preserving: `Explain` and `ProposeOrder`'s existing contracts (additive-only, feasible-set
  validation, unparseable-output fallback) SHALL NOT regress. Existing tests in
  `internal/llm` continue to pass against the refactored shape.
- Leave room for additional OpenAI-compatible backends later (e.g. a cloud fallback) without
  committing to implementing one now — this change generalizes the *shape*, not the roster of
  backends.
- Encode, as a spec requirement, the hard invariant `PLAN.md` states for AI Providers: AI SHALL
  call application-layer tools; AI SHALL NOT be given unrestricted SQL access. Phrase this
  consistently with the equivalent invariant `implement-mcp-server` (Epic A) owns for the MCP tool
  surface itself, so the two do not drift into inconsistent wording.
- Explicitly preserve the existing, already-tested non-load-bearing behavior: the LLM never
  determines plan feasibility (`internal/scoring`'s deterministic scorer remains authoritative);
  this change does not touch that boundary, only the transport/provider shape beneath it.

## Non-Goals

- No new MCP tool surface or tool implementations — Epic A's `implement-mcp-server` owns which
  application-layer tools exist and what they do; this change owns the layer that would call them,
  once such tools exist to call.
- No new AI-driven feature (this change is a refactor/generalization of transport, not new
  capability) — `Explain`/`ProposeOrder` behavior is preserved, not extended.
- No cloud LLM backend implementation — the interface permits one later; none is added now.
- No change to the scoring engine's authority over feasibility.

## Capabilities

### New Capabilities

- `ai-provider`: the OpenAI-compatible provider abstraction, Olla as its primary implementation,
  and the non-negotiable invariant that AI interaction is additive and tool-mediated — never a
  path to unrestricted SQL or a path to override deterministic feasibility.

### Modified Capabilities

<!-- none directly — meal-planning's existing "LLM is additive, never load-bearing" requirement
     (food-brain-first-slice/specs/meal-planning/spec.md) is unchanged in substance; this change
     generalizes the transport underneath it, not the meal-planning contract itself -->

## Impact

- `internal/llm/olla.go`: refactored into an interface + Olla implementation; existing callers in
  `cmd/food-brain` update to the interface type, not the concrete `*Client`, but observable
  behavior is unchanged.
- Existing `internal/llm` tests (4 tests, per `docs/research/current-state.md`) continue to pass;
  new tests cover the interface boundary itself (e.g. a fake second provider implementation used
  only in tests, to prove the abstraction isn't Olla-shaped in disguise).
- Consumed by Epic A's `implement-mcp-server`, which is the intended caller of tool-mediated AI
  interaction through this abstraction — cross-referenced here, not implemented here.
- Part of Epic G: AI & Admin Tooling (tracking issue #7).
