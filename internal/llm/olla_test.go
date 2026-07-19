package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	got, err := New(srv.URL, "m").ProposeOrder(context.Background(), ranked)
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
	got, err := New(srv.URL, "m").ProposeOrder(context.Background(), ranked)
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
	got, err := New(srv.URL, "m").ProposeOrder(context.Background(), ranked)
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
	got, err := New(srv.URL, "m").ProposeOrder(context.Background(), ranked)
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
