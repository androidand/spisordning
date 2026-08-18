package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/androidand/spisordning/internal/domain"
	"github.com/androidand/spisordning/internal/scoring"
)

func fakeOlla(t *testing.T, reply string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": reply}},
			},
		})
	}))
}

func sc(id string, feasible bool, score float64) scoring.ScoredCandidate {
	return scoring.ScoredCandidate{
		Candidate: domain.Candidate{MealieRecipeID: id, Title: id},
		Score:     score,
		Feasible:  feasible,
		Reason:    "ok",
	}
}

func TestProposeOrder_AcceptsValidReordering(t *testing.T) {
	srv := fakeOlla(t, `[{"mealieRecipeId":"b","reason":"variation"},{"mealieRecipeId":"a","reason":"klassiker"}]`)
	defer srv.Close()

	ranked := []scoring.ScoredCandidate{sc("a", true, 2), sc("b", true, 1)}
	got, err := ProposeOrder(New(srv.URL, "m"), context.Background(), ranked)
	if err != nil {
		t.Fatalf("ProposeOrder: %v", err)
	}
	if got[0].Candidate.MealieRecipeID != "b" || got[1].Candidate.MealieRecipeID != "a" {
		t.Errorf("expected LLM order [b a], got %v", ids(got))
	}
	if got[0].Reason != "variation" {
		t.Errorf("expected LLM reason to carry through, got %q", got[0].Reason)
	}
}

func TestProposeOrder_RejectsInventedAndInfeasibleIDs(t *testing.T) {
	// The LLM invents "z" and tries to sneak in the infeasible "c".
	srv := fakeOlla(t, `[{"mealieRecipeId":"z","reason":"påhittad"},{"mealieRecipeId":"c","reason":"orkar inte"},{"mealieRecipeId":"b","reason":"ok"}]`)
	defer srv.Close()

	ranked := []scoring.ScoredCandidate{sc("a", true, 2), sc("b", true, 1), sc("c", false, 3)}
	got, err := ProposeOrder(New(srv.URL, "m"), context.Background(), ranked)
	if err != nil {
		t.Fatalf("ProposeOrder: %v", err)
	}
	// c was never offered (infeasible), z doesn't exist: both rejected.
	// b accepted, a appended from scorer order.
	if len(got) != 2 || got[0].Candidate.MealieRecipeID != "b" || got[1].Candidate.MealieRecipeID != "a" {
		t.Errorf("expected [b a], got %v", ids(got))
	}
	for _, s := range got {
		if !s.Feasible {
			t.Errorf("infeasible candidate leaked through the LLM layer: %v", s.Candidate.MealieRecipeID)
		}
	}
}

func TestProposeOrder_UnparseableFallsBackToScorerOrder(t *testing.T) {
	srv := fakeOlla(t, "Jag tycker ni ska äta pannkakor hela veckan!")
	defer srv.Close()

	ranked := []scoring.ScoredCandidate{sc("a", true, 2), sc("b", true, 1)}
	got, err := ProposeOrder(New(srv.URL, "m"), context.Background(), ranked)
	if err == nil {
		t.Fatal("expected an error for unparseable proposal")
	}
	// Even on error, the scorer order is returned so the plan proceeds.
	if len(got) != 2 || got[0].Candidate.MealieRecipeID != "a" {
		t.Errorf("expected scorer-order fallback [a b], got %v", ids(got))
	}
}

func TestProposeOrder_ToleratesCodeFences(t *testing.T) {
	srv := fakeOlla(t, "Här är min plan:\n```json\n[{\"mealieRecipeId\":\"a\",\"reason\":\"bra\"}]\n```")
	defer srv.Close()

	ranked := []scoring.ScoredCandidate{sc("a", true, 2), sc("b", true, 1)}
	got, err := ProposeOrder(New(srv.URL, "m"), context.Background(), ranked)
	if err != nil {
		t.Fatalf("ProposeOrder: %v", err)
	}
	if len(got) != 2 || got[0].Candidate.MealieRecipeID != "a" || got[1].Candidate.MealieRecipeID != "b" {
		t.Errorf("expected [a b] with b appended, got %v", ids(got))
	}
}

func ids(scs []scoring.ScoredCandidate) []string {
	var out []string
	for _, s := range scs {
		out = append(out, s.Candidate.MealieRecipeID)
	}
	return out
}

// fakeProvider is a test-only second Provider implementation: a fixed-response
// stub that needs no real Olla instance and no HTTP. It proves the interface is
// not secretly Olla-shaped — Explain/ProposeOrder work against it exactly as
// they do against the real Client.
type fakeProvider struct {
	reply string
	err   error
}

func (f fakeProvider) Chat(ctx context.Context, system, user string) (string, error) {
	return f.reply, f.err
}

// TestProviderSubstitutability proves Explain and ProposeOrder depend only on
// the Provider interface, not on Olla: the same feasible set and the same LLM
// reply produce the same outcome whether the reply comes from the real Olla
// Client (OpenAI-compatible endpoint over httptest) or from the test-only
// fakeProvider (no Olla instance, no HTTP).
func TestProviderSubstitutability(t *testing.T) {
	reply := `[{"mealieRecipeId":"b","reason":"variation"},{"mealieRecipeId":"a","reason":"klassiker"}]`
	ranked := []scoring.ScoredCandidate{sc("a", true, 2), sc("b", true, 1)}
	want := []string{"b", "a"}

	// Real Olla client (OpenAI-compatible endpoint over httptest).
	srv := fakeOlla(t, reply)
	defer srv.Close()
	gotOlla, err := ProposeOrder(New(srv.URL, "m"), context.Background(), ranked)
	if err != nil {
		t.Fatalf("ProposeOrder via Olla: %v", err)
	}
	if got := ids(gotOlla); !sameIDs(got, want) {
		t.Errorf("Olla ProposeOrder: expected %v, got %v", want, got)
	}

	// Test-only fake provider (no Olla instance, no HTTP) — identical outcome.
	gotFake, err := ProposeOrder(fakeProvider{reply: reply}, context.Background(), ranked)
	if err != nil {
		t.Fatalf("ProposeOrder via fake: %v", err)
	}
	if got := ids(gotFake); !sameIDs(got, want) {
		t.Errorf("fake ProposeOrder: expected %v, got %v", want, got)
	}

	// Explain is substitutable too: each provider's reply is returned verbatim.
	// The Olla side gets its own server answering prose, so the assertion is
	// about the explanation text itself, not a JSON reorder reply.
	const expl = "En bra middag."
	explSrv := fakeOlla(t, expl)
	defer explSrv.Close()
	candidate := sc("a", true, 2)
	candidate.Breakdown = scoring.Breakdown{Preference: 1, Effort: -0.5}
	explOlla, err := Explain(New(explSrv.URL, "m"), context.Background(), candidate)
	if err != nil {
		t.Fatalf("Explain via Olla: %v", err)
	}
	explFake, err := Explain(fakeProvider{reply: expl}, context.Background(), candidate)
	if err != nil {
		t.Fatalf("Explain via fake: %v", err)
	}
	if explOlla != expl {
		t.Errorf("Olla Explain: expected the provider reply %q, got %q", expl, explOlla)
	}
	if explFake != expl {
		t.Errorf("fake Explain: expected %q, got %q", expl, explFake)
	}
}

// TestProviderError_Propagates covers the other half of the Provider contract:
// a failing backend must surface its error (never a silent empty success), and
// ProposeOrder must still return the feasible scorer order so the plan can
// proceed without the LLM.
func TestProviderError_Propagates(t *testing.T) {
	boom := errors.New("backend down")
	// Two feasible candidates: enough to warrant a reorder call, so Chat runs
	// and its error surfaces.
	ranked := []scoring.ScoredCandidate{sc("a", true, 2), sc("b", true, 1), sc("c", false, 3)}

	got, err := ProposeOrder(fakeProvider{err: boom}, context.Background(), ranked)
	if err == nil {
		t.Fatal("ProposeOrder should propagate the provider error")
	}
	if !errors.Is(err, boom) {
		t.Errorf("ProposeOrder error should wrap the provider error, got %v", err)
	}
	// Feasible candidates come back in scorer order so the caller can fall back.
	if len(got) != 2 || got[0].Candidate.MealieRecipeID != "a" || got[1].Candidate.MealieRecipeID != "b" {
		t.Errorf("ProposeOrder should return the feasible set on error, got %v", ids(got))
	}

	if expl, err := Explain(fakeProvider{err: boom}, context.Background(), sc("a", true, 2)); err == nil || expl != "" {
		t.Errorf("Explain should propagate the provider error (%v) and return no text, got %q", err, expl)
	}
}

func sameIDs(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// recordingProvider is a test-only Provider that captures the prompt instead of
// answering, so tests can assert on the exact Swedish prompt built above the
// transport.
type recordingProvider struct {
	system, user string
}

func (r *recordingProvider) Chat(_ context.Context, system, user string) (string, error) {
	r.system, r.user = system, user
	return "ok", nil
}

// TestExplain_PromptCoversEveryBreakdownSignal guards against drift between the
// scorer's Breakdown fields and the Swedish prompt the LLM is shown: every
// signal must appear, including the Familiarity dimension added after the
// prompt was first written.
func TestExplain_PromptCoversEveryBreakdownSignal(t *testing.T) {
	candidate := sc("a", true, 1)
	candidate.Breakdown = scoring.Breakdown{
		Preference: 1, Effort: -0.5, Repetition: -0.2,
		SchoolDedup: -0.1, Campaign: 0.3, Familiarity: 0.4,
	}
	rec := &recordingProvider{}
	if _, err := Explain(rec, context.Background(), candidate); err != nil {
		t.Fatalf("Explain: %v", err)
	}
	for _, want := range []string{"preferens", "energi", "upprepning", "skollunch-krock", "kampanj", "familiaritet", "+0.40"} {
		if !strings.Contains(rec.user, want) {
			t.Errorf("Explain prompt missing %q: %q", want, rec.user)
		}
	}
}
