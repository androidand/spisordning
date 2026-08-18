# Design: integrate-ai

## 1.1 Current concrete surface of `internal/llm/olla.go`

Read in full (184 lines). The package today is a single concrete `Client` bound to the local
Olla proxy (an OpenAI-compatible endpoint). This enumeration is the baseline for the interface
design in 1.2–1.4 and the behavior-preservation contract in section 2.

### Public surface

| Symbol | Signature | Notes |
|---|---|---|
| `Client` | `struct { baseURL, model string; http *http.Client }` | Concrete, Olla-shaped; all fields unexported. |
| `New` | `func New(baseURL, model string) *Client` | Trims trailing `/` from `baseURL`; sets `http.Client{Timeout: 120 * time.Second}`. Doc comment names Olla explicitly. |
| `Explain` | `func (c *Client) Explain(ctx context.Context, sc scoring.ScoredCandidate) (string, error)` | One-sentence Swedish explanation grounded in `sc.Breakdown` (Preference, Effort, Repetition, SchoolDedup, Campaign). Non-fatal: caller falls back to the scorer's machine reason. |
| `ProposeOrder` | `func (c *Client) ProposeOrder(ctx context.Context, ranked []scoring.ScoredCandidate) ([]scoring.ScoredCandidate, error)` | Strict feasible-set validation: invented ids dropped, infeasible never offered, omissions appended in scorer order. Unparseable output → returns scorer order + error. LLM never load-bearing. |

### Private surface

| Symbol | Signature | Notes |
|---|---|---|
| `chatRequest` | `struct { Model string; Messages []chatMessage }` | OpenAI chat-completions request body. |
| `chatMessage` | `struct { Role, Content string }` | |
| `chatResponse` | `struct { Choices []struct{ Message chatMessage } }` | Only `choices[0].message.content` is read. |
| `proposal` | `struct { MealieRecipeID, Reason string }` | JSON shape the LLM must answer with in `ProposeOrder`. |
| `chat` | `func (c *Client) chat(ctx context.Context, system, user string) (string, error)` | The transport: marshals `chatRequest`, POSTs to `baseURL + "/chat/completions"`, decodes `chatResponse`. Every error is wrapped with an `"olla: %w"` prefix; non-200 → `"olla: HTTP %d"`; empty `choices` → `"olla: empty response"`. |
| `extractJSONArray` | `func extractJSONArray(s string) string` | Tolerates models that wrap JSON in prose/code fences: returns `s[start:end+1]` where `start` = first `[`, `end` = last `]`; falls back to `s` when no pair is found. |

### Olla-specific vs. generic (what the interface must lift out)

- **Generic (OpenAI-compatible transport):** the `chat` method's request/response shapes
  (`chatRequest`/`chatMessage`/`chatResponse`) and the `POST {baseURL}/chat/completions` call are
  standard OpenAI chat-completions — any compatible endpoint satisfies them.
- **Olla-specific (must not leak into the interface):**
  - the `"olla: %w"` error prefix (backend-named error wrapping);
  - the `New` doc comment and `Client` naming that assume Olla;
  - the fixed `120 * time.Second` timeout (a reasonable default, but a per-instance knob, not an
    interface constant).
- **Domain coupling (must stay above the interface, task 1.3):** `Explain` and `ProposeOrder`
  reference `scoring.ScoredCandidate` and build Swedish prompts. A non-Olla provider must not need
  to know about `scoring.ScoredCandidate`.

### Behavioral invariants to preserve (feed section 2)

- **Transport seam:** one `chat(ctx, system, user) (string, error)` call — the natural interface
  method.
- **Per-instance config:** base URL, model, and timeout are set at construction (`New`), not per
  call; no per-call timeout override exists today.
- **Feasibility authority:** `ProposeOrder` output is always a permutation of the feasible input
  set — the LLM reorders within it, nothing more. `internal/scoring` stays authoritative.
- **Graceful degradation:** both `Explain` and `ProposeOrder` failures are non-fatal to the
  caller, which falls back to the scorer's reason/order.

### Callers & tests

- **Caller:** `cmd/food-brain/plan.go` is the only caller — `var olla *llm.Client` (line 93),
  `llm.New(base, model)` (line 95), `ProposeOrder` for reordering (line 121), `Explain` for
  presentation (line 145). Both call sites are non-fatal (scorer order/reason kept on error).
- **Tests:** `internal/llm/olla_test.go` — 4 tests, all against `ProposeOrder` via an
  `httptest.Server` fake (`fakeOlla`): valid reordering, invented/infeasible rejection,
  unparseable fallback, code-fence tolerance. No test exercises `Explain` directly.
- **Dependency direction:** `internal/llm` imports `internal/scoring` (for
  `scoring.ScoredCandidate`); `internal/scoring` does NOT import `internal/llm` (confirmed —
  relevant to task 5.2).

## 1.2 The `Provider` interface (transport-only)

The interface generalizes the OpenAI-compatible chat-completion transport and bakes in **no**
Olla-specific assumptions — no base URL, model name, or timeout at the interface level (those are
per-instance config on the implementation):

```go
// Provider is the transport-only seam for an OpenAI-compatible chat-completion
// endpoint. It is deliberately backend-agnostic: a local Olla proxy, a llama-skein
// endpoint, or a future cloud provider all satisfy it by implementing Chat.
type Provider interface {
    // Chat sends a system + user message to the endpoint and returns the
    // assistant's content. It is the ONLY method on the interface: everything
    // domain-specific (Swedish prompt-building, feasible-set validation) lives
    // ABOVE it, so a non-Olla provider never needs to know about
    // scoring.ScoredCandidate.
    Chat(ctx context.Context, system, user string) (string, error)
}
```

- **Method shape:** `Chat(ctx, system, user string) (string, error)` — exactly the current
  private `chat` helper's shape, promoted to an exported interface method.
- **No Olla assumptions at the interface level:** the interface names no backend, no URL, no
  model, no timeout. The `"olla: %w"` error prefix and the `120 * time.Second` default stay on the
  Olla implementation (`Client`) — they are backend-specific, not transport-generic.
- **One method:** a single chat-completion call is the whole transport surface; the current
  `Client` uses nothing else from the endpoint.

## 1.3 Where `Explain` / `ProposeOrder` live

Decision: **free functions above the interface**, not interface methods.

```go
func Explain(p Provider, ctx context.Context, sc scoring.ScoredCandidate) (string, error)
func ProposeOrder(p Provider, ctx context.Context, ranked []scoring.ScoredCandidate) ([]scoring.ScoredCandidate, error)
```

- The interface stays transport-only (`Chat`). `Explain`/`ProposeOrder` are the application-layer
  prompt-building + feasible-set validation that sit above it; they take a `Provider` as their
  first argument.
- A future non-Olla provider implements only `Chat`; it never references
  `scoring.ScoredCandidate`. The Swedish prompts and the feasible-set validation remain in this
  package, callable against any `Provider`.
- Rejected: putting `Explain`/`ProposeOrder` on the interface would force every provider to depend
  on `scoring.ScoredCandidate` — exactly the domain-shaped coupling this change removes.

## 1.4 Per-instance configuration

The interface is config-agnostic; each implementation carries its own base URL, model, and
timeout, matching Olla's existing `New(baseURL, model string)` + `120 * time.Second`:

- `New(baseURL, model string) *Client` is preserved exactly (trims trailing `/`, sets the 120s
  timeout). `*Client` satisfies `Provider` via its exported `Chat` method.
- Base URL, model, and timeout are per-instance (set at construction), not per-call and not
  interface constants — a future cloud provider would carry its own base URL/model/timeout the
  same way, so the interface already supports configurable base URL, model, and timeout per
  instance (task 1.4).
