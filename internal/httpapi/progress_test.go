package httpapi

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestRunPlanStream_ProgressiveEvents drives the SSE endpoint against a fake
// slow adapter (progressDelay) and asserts progress events arrive
// incrementally over time, not all at once at the end (task 3.3).
func TestRunPlanStream_ProgressiveEvents(t *testing.T) {
	svc := &fakePlansSvc{
		result:        PlanRunResult{Status: "accepted", Message: "planned 7 dinners"},
		progressDelay: 60 * time.Millisecond,
	}
	mux := newMux(t, Dependencies{Plans: svc})
	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Post(server.URL+"/plans/run/stream", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("content-type = %q, want text/event-stream", ct)
	}

	type arrival struct {
		event string
		at    time.Time
	}
	var arrivals []arrival
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			arrivals = append(arrivals, arrival{event: strings.TrimPrefix(line, "event: "), at: time.Now()})
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan: %v", err)
	}

	// Expect 5 events: started, planning, resolving, wishlist (progress) + done.
	if len(arrivals) != 5 {
		t.Fatalf("got %d events, want 5: %+v", len(arrivals), arrivals)
	}
	wantEvents := []string{"progress", "progress", "progress", "progress", "done"}
	for i, want := range wantEvents {
		if arrivals[i].event != want {
			t.Fatalf("event[%d] = %q, want %q", i, arrivals[i].event, want)
		}
	}

	// Assert incremental arrival: the gap between the first and last event must
	// span the 3 progress delays (3 x 60ms = 180ms). A burst (all at once) would
	// produce a gap near zero.
	gap := arrivals[len(arrivals)-1].at.Sub(arrivals[0].at)
	if gap < 120*time.Millisecond {
		t.Fatalf("events arrived too fast (gap %v < 120ms); want incremental arrival", gap)
	}
}

// TestRunPlanStream_SyncEndpointUnaffected confirms the synchronous
// POST /plans/run still works alongside the SSE endpoint (task 3.2).
func TestRunPlanStream_SyncEndpointUnaffected(t *testing.T) {
	svc := &fakePlansSvc{result: PlanRunResult{Status: "accepted", Message: "ok"}}
	mux := newMux(t, Dependencies{Plans: svc})
	rec := doPost(t, mux, "/plans/run", `{}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rec.Code, rec.Body)
	}
	var got PlanRunResult
	mustJSON(t, rec.Body.Bytes(), &got)
	if got.Status != "accepted" {
		t.Fatalf("status = %q, want accepted", got.Status)
	}
}
