# Tasks: integrate-ai

## 1. Provider interface design

- [x] 1.1 Read `internal/llm/olla.go` in full and enumerate its current concrete surface:
      `New(baseURL, model string) *Client`, `Explain(ctx, scoring.ScoredCandidate) (string,
      error)`, `ProposeOrder(ctx, []scoring.ScoredCandidate) ([]scoring.ScoredCandidate, error)`,
      plus the private `chat` helper and `extractJSONArray` tolerance parsing.
      (read all 184 lines; enumerated in design.md §1.1 — public: Client{baseURL,model,http},
      New (trims `/`, 120s timeout), Explain, ProposeOrder; private: chatRequest/chatMessage/
      chatResponse/proposal structs, chat (POST {base}/chat/completions, "olla:" error prefix,
      HTTP %d + empty-choices errors), extractJSONArray (first-[..last-] tolerance). Confirmed the
      actual surface matches the task's enumeration exactly; noted Olla-specific vs. generic split
      and the scoring.ScoredCandidate domain coupling that 1.2/1.3 must lift out. Baseline green:
      go build/vet OK, 4 llm tests pass.)
- [x] 1.2 Design a `Provider` interface generalizing the OpenAI-compatible chat-completion
      transport (`chat(ctx, system, user string) (string, error)`-shaped, or equivalent) without
      baking in Olla-specific assumptions (base URL, model name) at the interface level.
      (design.md §1.2 — `Provider` has exactly one method `Chat(ctx, system, user string)
      (string, error)`; no URL/model/timeout at the interface level; the "olla:" error prefix and
      the 120s default stay on the Olla `Client`.)
- [x] 1.3 Decide where domain-specific methods (`Explain`, `ProposeOrder`) live: on the interface
      itself, or as free functions/an application-layer service that takes a `Provider` and
      builds the Swedish prompts — prefer keeping the interface transport-only and the
      domain-specific prompt-building above it, so a future non-Olla provider doesn't need to
      know about `scoring.ScoredCandidate`.
      (design.md §1.3 — decided: free functions above the interface. `Explain(p Provider, ...)`
      and `ProposeOrder(p Provider, ...)` build the Swedish prompts and call `p.Chat`; the
      interface stays transport-only, so a non-Olla provider never references
      scoring.ScoredCandidate.)
- [x] 1.4 Confirm the interface supports configurable base URL, model, and timeout per instance,
      matching Olla's existing `New(baseURL, model string)` + `120 * time.Second` timeout.
      (design.md §1.4 — confirmed: `New(baseURL, model string) *Client` preserved exactly (trims
      trailing `/`, sets 120s timeout); base URL/model/timeout are per-instance config on the
      implementation, not interface constants, so any future provider carries its own the same way.)

## 2. Olla as the primary implementation

- [x] 2.1 Reimplement `internal/llm`'s `Client` to satisfy the new `Provider` interface,
      preserving its existing constructor shape and behavior exactly.
      (added `Provider` interface; promoted private `chat` to exported `Chat` on `*Client` (same
      body, same "olla:" error wrapping); `New(baseURL, model string) *Client` unchanged. Compile
      check: `var _ Provider = (*Client)(nil)` satisfied by the single `Chat` method. go build OK.)
- [x] 2.2 Preserve `Explain`'s existing contract: one-sentence Swedish explanation grounded in the
      scorer's breakdown, non-fatal on failure (caller falls back to the scorer's machine reason).
      (Explain is now a free function taking a `Provider`; identical Swedish prompt and
      `p.Chat` call; caller in plan.go still treats the result as non-fatal. Covered by the
      substitutability test's Explain assertions.)
- [x] 2.3 Preserve `ProposeOrder`'s existing contract: strict validation against the feasible
      input set (invented ids dropped, infeasible candidates never offered, omitted candidates
      appended in scorer order), unparseable output falls back to scorer order — the LLM is never
      load-bearing.
      (ProposeOrder is now a free function taking a `Provider`; validation logic unchanged
      (renamed the inner loop var `p`→`prop` to avoid shadowing the `Provider` param). All 4
      original ProposeOrder tests pass unmodified in behavior.)
- [x] 2.4 Preserve `extractJSONArray`'s tolerance for models that wrap JSON in prose or code
      fences.
      (extractJSONArray unchanged; TestProposeOrder_ToleratesCodeFences still passes.)
- [x] 2.5 Existing `internal/llm` tests (4 tests per `docs/research/current-state.md`) pass
      unmodified in behavior, updated only for the interface-shaped API where mechanically
      necessary.
      (the 4 tests' only mechanical change was `New(...).ProposeOrder(ctx, ranked)` →
      `ProposeOrder(New(...), ctx, ranked)`; assertions untouched. go test ./internal/llm/...
      green.)

## 3. Abstraction proof

- [x] 3.1 Add a fake/test-only second `Provider` implementation used only in tests, to
      demonstrate the interface is not secretly Olla-shaped (e.g. a fixed-response fake usable
      without a real Olla instance).
      (added `fakeProvider{reply, err}` in olla_test.go — a fixed-response `Chat` stub with no
      Olla instance and no HTTP. It compiles as a `Provider`, proving the interface is not
      Olla-shaped.)
- [x] 3.2 Add a test asserting `Explain`/`ProposeOrder`-equivalent call sites work identically
      against both the real Olla client and the fake, proving substitutability.
      (added TestProviderSubstitutability: same feasible set + same reply → same `[b a]` ordering
      via the real Olla `Client` (httptest) AND via `fakeProvider`; Explain returns each
      provider's reply verbatim in both cases. Passes.)

## 4. Hard invariant: tools only, never raw SQL

- [x] 4.1 Encode as a spec requirement (see `specs/ai-provider/spec.md`): AI interaction SHALL
      only occur through application-layer tools; the AI provider abstraction SHALL NOT be given
      a raw SQL execution path, a direct database connection, or any capability equivalent to
      unrestricted SQL.
      (spec.md requirement "AI SHALL only call application-layer tools, never raw SQL" with the
      "No SQL execution path exists in the provider package" scenario. openspec validate
      integrate-ai → valid.)
- [x] 4.2 Confirm the provider abstraction's interface signature makes this structurally true —
      it should be impossible to construct a call from `Provider` methods that reaches the
      database directly, by construction of the types involved (no `*sql.DB` or Postgres
      connection anywhere in the `internal/llm` package).
      (grep of internal/llm for `sql|database|postgres|pgx|lib/pq` → no matches; the only
      `Provider` method is `Chat(ctx, system, user string) (string, error)` which returns a
      string — no type in the package can carry a DB handle.)
- [x] 4.3 Cross-reference Epic A's `implement-mcp-server`: confirm (once that change's proposal
      exists) that its own tool-surface invariant is phrased consistently with this change's —
      both should state "tools only, never raw SQL" in compatible language, not two independently
      worded versions of the same rule.
      (implement-mcp-server proposal + spec exist; its "MCP tools never execute raw SQL"
      requirement states the same rule in compatible language. Added an explicit lockstep
      cross-reference sentence to this change's spec requirement naming that mcp-server
      requirement.)

## 5. Consumer wiring

- [x] 5.1 Update `cmd/food-brain` call sites to depend on the `Provider` interface type, not the
      concrete `*Client`, so a future non-Olla provider can be substituted without touching
      caller code.
      (plan.go: `var olla *llm.Client` → `var olla llm.Provider`; `olla.ProposeOrder(ctx, ranked)`
      → `llm.ProposeOrder(olla, ctx, ranked)`; `olla.Explain(ctx, s.winner)` →
      `llm.Explain(olla, ctx, s.winner)`. Call sites now depend only on the interface.)
- [x] 5.2 Confirm `internal/scoring`'s deterministic scorer remains fully independent of and
      authoritative over `internal/llm` — no import of `internal/llm` from `internal/scoring`.
      (grep of internal/scoring for `internal/llm` → no matches; dependency direction unchanged:
      llm imports scoring, never the reverse.)

## 6. Verification & docs

- [x] 6.1 `go build ./... && go test ./... && go vet ./...` green.
      (go build ./... success; go vet ./... no issues; go test ./... → 103 passed in 10 packages
      (was 102; +1 for TestProviderSubstitutability).)
- [x] 6.2 Update `docs/research/current-state.md`'s description of `internal/llm/olla.go` once
      the interface lands, noting it as `internal/llm`'s provider abstraction with Olla as the
      primary implementation.
      (current-state.md layout line updated: `llm/` now described as the AI provider abstraction
      (Provider interface) with Olla (Client) as the primary OpenAI-compatible implementation.)
- [x] 6.3 Confirm no behavior regression: run the existing end-to-end `food-brain plan` test path
      (`cmd/food-brain/plan_test.go`) against the refactored provider and confirm it still passes.
      (go test ./cmd/food-brain/... → 1 passed in 1 package; plan_test.go green against the
      refactored provider.)
