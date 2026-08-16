# Tasks: integrate-ai

## 1. Provider interface design

- [ ] 1.1 Read `internal/llm/olla.go` in full and enumerate its current concrete surface:
      `New(baseURL, model string) *Client`, `Explain(ctx, scoring.ScoredCandidate) (string,
      error)`, `ProposeOrder(ctx, []scoring.ScoredCandidate) ([]scoring.ScoredCandidate, error)`,
      plus the private `chat` helper and `extractJSONArray` tolerance parsing.
- [ ] 1.2 Design a `Provider` interface generalizing the OpenAI-compatible chat-completion
      transport (`chat(ctx, system, user string) (string, error)`-shaped, or equivalent) without
      baking in Olla-specific assumptions (base URL, model name) at the interface level.
- [ ] 1.3 Decide where domain-specific methods (`Explain`, `ProposeOrder`) live: on the interface
      itself, or as free functions/an application-layer service that takes a `Provider` and
      builds the Swedish prompts — prefer keeping the interface transport-only and the
      domain-specific prompt-building above it, so a future non-Olla provider doesn't need to
      know about `scoring.ScoredCandidate`.
- [ ] 1.4 Confirm the interface supports configurable base URL, model, and timeout per instance,
      matching Olla's existing `New(baseURL, model string)` + `120 * time.Second` timeout.

## 2. Olla as the primary implementation

- [ ] 2.1 Reimplement `internal/llm`'s `Client` to satisfy the new `Provider` interface,
      preserving its existing constructor shape and behavior exactly.
- [ ] 2.2 Preserve `Explain`'s existing contract: one-sentence Swedish explanation grounded in the
      scorer's breakdown, non-fatal on failure (caller falls back to the scorer's machine reason).
- [ ] 2.3 Preserve `ProposeOrder`'s existing contract: strict validation against the feasible
      input set (invented ids dropped, infeasible candidates never offered, omitted candidates
      appended in scorer order), unparseable output falls back to scorer order — the LLM is never
      load-bearing.
- [ ] 2.4 Preserve `extractJSONArray`'s tolerance for models that wrap JSON in prose or code
      fences.
- [ ] 2.5 Existing `internal/llm` tests (4 tests per `docs/research/current-state.md`) pass
      unmodified in behavior, updated only for the interface-shaped API where mechanically
      necessary.

## 3. Abstraction proof

- [ ] 3.1 Add a fake/test-only second `Provider` implementation used only in tests, to
      demonstrate the interface is not secretly Olla-shaped (e.g. a fixed-response fake usable
      without a real Olla instance).
- [ ] 3.2 Add a test asserting `Explain`/`ProposeOrder`-equivalent call sites work identically
      against both the real Olla client and the fake, proving substitutability.

## 4. Hard invariant: tools only, never raw SQL

- [ ] 4.1 Encode as a spec requirement (see `specs/ai-provider/spec.md`): AI interaction SHALL
      only occur through application-layer tools; the AI provider abstraction SHALL NOT be given
      a raw SQL execution path, a direct database connection, or any capability equivalent to
      unrestricted SQL.
- [ ] 4.2 Confirm the provider abstraction's interface signature makes this structurally true —
      it should be impossible to construct a call from `Provider` methods that reaches the
      database directly, by construction of the types involved (no `*sql.DB` or Postgres
      connection anywhere in the `internal/llm` package).
- [ ] 4.3 Cross-reference Epic A's `implement-mcp-server`: confirm (once that change's proposal
      exists) that its own tool-surface invariant is phrased consistently with this change's —
      both should state "tools only, never raw SQL" in compatible language, not two independently
      worded versions of the same rule.

## 5. Consumer wiring

- [ ] 5.1 Update `cmd/food-brain` call sites to depend on the `Provider` interface type, not the
      concrete `*Client`, so a future non-Olla provider can be substituted without touching
      caller code.
- [ ] 5.2 Confirm `internal/scoring`'s deterministic scorer remains fully independent of and
      authoritative over `internal/llm` — no import of `internal/llm` from `internal/scoring`.

## 6. Verification & docs

- [ ] 6.1 `go build ./... && go test ./... && go vet ./...` green.
- [ ] 6.2 Update `docs/research/current-state.md`'s description of `internal/llm/olla.go` once
      the interface lands, noting it as `internal/llm`'s provider abstraction with Olla as the
      primary implementation.
- [ ] 6.3 Confirm no behavior regression: run the existing end-to-end `food-brain plan` test path
      (`cmd/food-brain/plan_test.go`) against the refactored provider and confirm it still passes.
