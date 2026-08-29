package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// planProgressHandler handles POST /plans/run/stream (SSE). It runs the plan
// and streams progress events (text/event-stream) as each phase completes.
// The synchronous POST /plans/run is unaffected — this endpoint is additive.
type planProgressHandler struct {
	svc PlanService
}

func (h *planProgressHandler) runPlanStream(w http.ResponseWriter, r *http.Request) {
	var in PlanRunInput
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			// Empty body is fine — use defaults.
			in = PlanRunInput{}
		}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, errorBody{Message: "streaming unsupported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	writeEvent := func(event string, data any) {
		buf, err := json.Marshal(data)
		if err != nil {
			return
		}
		if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, buf); err != nil {
			return
		}
		flusher.Flush()
	}

	writeEvent("progress", PlanProgress{Phase: "started", Message: "Plan run started", At: time.Now()})

	result, err := h.svc.RunPlanWithProgress(r.Context(), in, func(p PlanProgress) {
		writeEvent("progress", p)
	})
	if err != nil {
		writeEvent("error", errorBody{Message: "plan run: " + err.Error()})
		return
	}
	writeEvent("done", result)
}
