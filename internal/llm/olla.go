// Package llm integrates the local Olla proxy (OpenAI-compatible) as the
// ADDITIVE layer of the suggestion engine: it writes human-readable
// explanations and proposes orderings among already-feasible candidates. Per
// spec, it can never gate feasibility — every LLM proposal is validated against
// the deterministic scorer's feasible set, and anything unknown or infeasible
// is rejected. The planner works fully without this package (Available=false).
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/androidand/spisordning/internal/scoring"
)

// Client talks to an OpenAI-compatible chat completions endpoint (Olla).
type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

// New returns a Client for the endpoint at baseURL (e.g.
// "http://192.168.1.240:40114/olla/openai/v1") using the given model name.
func New(baseURL, model string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{Timeout: 120 * time.Second},
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

func (c *Client) chat(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("content-type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("olla: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("olla: HTTP %d", res.StatusCode)
	}
	var out chatResponse
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("olla: decode: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("olla: empty response")
	}
	return out.Choices[0].Message.Content, nil
}

// Explain asks the LLM for a one-sentence Swedish explanation of why a meal
// was chosen, grounded in the scorer's breakdown. Failure is non-fatal: the
// caller falls back to the scorer's machine reason.
func (c *Client) Explain(ctx context.Context, sc scoring.ScoredCandidate) (string, error) {
	user := fmt.Sprintf(
		"Rätt: %s. Poäng: preferens %+.2f, energi %+.2f, upprepning %+.2f, skollunch-krock %+.2f, kampanj %+.2f. "+
			"Förklara på en mening, vardaglig svenska, varför denna rätt passar ikväll.",
		sc.Candidate.Title, sc.Breakdown.Preference, sc.Breakdown.Effort,
		sc.Breakdown.Repetition, sc.Breakdown.SchoolDedup, sc.Breakdown.Campaign,
	)
	return c.chat(ctx, "Du är en hjälpsam matplanerare för en familj.", user)
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
func (c *Client) ProposeOrder(
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

	answer, err := c.chat(ctx, "Du är en matplanerare. Svara alltid med giltig JSON.", user)
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
	for _, p := range proposals {
		sc, ok := byID[p.MealieRecipeID]
		if !ok || used[p.MealieRecipeID] {
			continue // invented or duplicated id: rejected
		}
		if p.Reason != "" {
			sc.Reason = p.Reason
		}
		used[p.MealieRecipeID] = true
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
