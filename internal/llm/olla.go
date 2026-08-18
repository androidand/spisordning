// Package llm provides the AI provider abstraction for the suggestion engine's
// ADDITIVE layer. A Provider is the transport-only seam for any OpenAI-compatible
// chat-completion endpoint; the local Olla proxy (Client) is the primary
// implementation. Above the provider, Explain and ProposeOrder build Swedish
// prompts and validate LLM output against the deterministic scorer's feasible
// set — the LLM can never gate feasibility, and the planner works fully without
// this package (Available=false).
package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/httpclient"
	"github.com/androidand/spisordning/internal/scoring"
)

// Provider is the transport-only seam for an OpenAI-compatible chat-completion
// endpoint. It is deliberately backend-agnostic: a local Olla proxy, a
// llama-skein endpoint, or a future cloud provider all satisfy it by
// implementing Chat. Everything domain-specific (Swedish prompt-building,
// feasible-set validation) lives above it — see Explain and ProposeOrder — so a
// non-Olla provider never needs to know about scoring.ScoredCandidate.
type Provider interface {
	// Chat sends a system + user message to the endpoint and returns the
	// assistant's content.
	Chat(ctx context.Context, system, user string) (string, error)
}

// Client is the Olla implementation of Provider: it talks to an
// OpenAI-compatible chat completions endpoint (Olla).
type Client struct {
	model string
	http  *httpclient.Client
}

// New returns a Client for the endpoint at baseURL (e.g.
// "http://192.168.1.240:40114/olla/openai/v1") using the given model name.
func New(baseURL, model string) *Client {
	return &Client{
		model: model,
		http:  httpclient.New(baseURL, "olla", 120*time.Second),
	}
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// Chat implements Provider. It marshals an OpenAI chat-completions request and
// POSTs it to baseURL + "/chat/completions". Every error is wrapped with an
// "olla:" prefix (backend-specific, not part of the Provider contract);
// non-2xx → "olla: HTTP %d"; empty choices → "olla: empty response".
func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	var out chatResponse
	if err := c.http.PostJSON(ctx, "/chat/completions", chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	}, &out, nil); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("olla: empty response")
	}
	return out.Choices[0].Message.Content, nil
}

// Explain asks the LLM for a one-sentence Swedish explanation of why a meal
// was chosen, grounded in the scorer's breakdown. It is an application-layer
// helper above the transport-only Provider: it builds the Swedish prompt and
// calls p.Chat. Failure is non-fatal: the caller falls back to the scorer's
// machine reason.
func Explain(p Provider, ctx context.Context, sc scoring.ScoredCandidate) (string, error) {
	user := fmt.Sprintf(
		"Rätt: %s. Poäng: preferens %+.2f, energi %+.2f, upprepning %+.2f, skollunch-krock %+.2f, kampanj %+.2f, familiaritet %+.2f. "+
			"Förklara på en mening, vardaglig svenska, varför denna rätt passar ikväll.",
		sc.Candidate.Title, sc.Breakdown.Preference, sc.Breakdown.Effort,
		sc.Breakdown.Repetition, sc.Breakdown.SchoolDedup, sc.Breakdown.Campaign,
		sc.Breakdown.Familiarity,
	)
	return p.Chat(ctx, "Du är en hjälpsam matplanerare för en familj.", user)
}

// proposal is the JSON shape the LLM must answer with in ProposeOrder.
type proposal struct {
	MealieRecipeID string `json:"mealieRecipeId"`
	Reason         string `json:"reason"`
}

// ProposeOrder shows the LLM the feasible candidates and lets it suggest an
// order (for variety/narrative). The result is strictly validated:
//   - ids not in the feasible input set are dropped (the LLM cannot invent meals)
//   - infeasible candidates cannot appear (they were never offered)
//   - any candidate the LLM omitted is appended in scorer order
//
// So the output is always a permutation of the feasible input — the LLM can
// reorder within the feasible set, nothing more.
func ProposeOrder(
	p Provider,
	ctx context.Context,
	ranked []scoring.ScoredCandidate,
) ([]scoring.ScoredCandidate, error) {
	feasible := make([]scoring.ScoredCandidate, 0, len(ranked))
	byID := map[string]scoring.ScoredCandidate{}
	for _, sc := range ranked {
		if sc.Feasible {
			feasible = append(feasible, sc)
			byID[sc.Candidate.MealieRecipeID] = sc
		}
	}
	if len(feasible) <= 1 {
		return feasible, nil
	}

	var menu strings.Builder
	for _, sc := range feasible {
		fmt.Fprintf(&menu, "- id=%s: %s (poäng %.2f)\n", sc.Candidate.MealieRecipeID, sc.Candidate.Title, sc.Score)
	}
	user := fmt.Sprintf(
		"Möjliga middagar:\n%s\nFöreslå en ordning med variation i åtanke. "+
			"Svara ENDAST med en JSON-array: [{\"mealieRecipeId\":\"...\",\"reason\":\"...\"}]. "+
			"Använd endast id:n från listan.",
		menu.String(),
	)

	answer, err := p.Chat(ctx, "Du är en matplanerare. Svara alltid med giltig JSON.", user)
	if err != nil {
		return feasible, err // caller keeps scorer order
	}

	var proposals []proposal
	if err := json.Unmarshal([]byte(extractJSONArray(answer)), &proposals); err != nil {
		return feasible, fmt.Errorf("olla: unparseable proposal: %w", err)
	}

	// Validate: only known feasible ids, no duplicates, omissions appended.
	var out []scoring.ScoredCandidate
	used := map[string]bool{}
	for _, prop := range proposals {
		sc, ok := byID[prop.MealieRecipeID]
		if !ok || used[prop.MealieRecipeID] {
			continue // invented or duplicated id: rejected
		}
		if prop.Reason != "" {
			sc.Reason = prop.Reason
		}
		used[prop.MealieRecipeID] = true
		out = append(out, sc)
	}
	for _, sc := range feasible {
		if !used[sc.Candidate.MealieRecipeID] {
			out = append(out, sc)
		}
	}
	return out, nil
}

// extractJSONArray tolerates models that wrap JSON in prose or code fences.
func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
