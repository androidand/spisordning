package httpapi

import (
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// dashboardHandler handles /widgets routes (the aggregate dashboard / widgets).
type dashboardHandler struct {
	svc dto.DashboardService
}

// getDashboard serves GET /widgets/dashboard?householdId=...
func (h *dashboardHandler) getDashboard(w http.ResponseWriter, r *http.Request) {
	householdID := r.URL.Query().Get("householdId")
	out, err := h.svc.Get(r.Context(), householdID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get dashboard: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
