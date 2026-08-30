package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/androidand/spisordning/internal/dto"
)

// pantryHandler handles /pantry routes.
type pantryHandler struct {
	svc dto.PantryService
}

func (h *pantryHandler) listLocations(w http.ResponseWriter, r *http.Request) {
	householdID := r.URL.Query().Get("householdId")
	out, err := h.svc.ListLocations(r.Context(), householdID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list locations: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *pantryHandler) createLocation(w http.ResponseWriter, r *http.Request) {
	var in dto.PantryLocationNew
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.CreateLocation(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create location: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *pantryHandler) listLots(w http.ResponseWriter, r *http.Request) {
	locationID := r.PathValue("id")
	out, err := h.svc.ListLots(r.Context(), locationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list lots: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *pantryHandler) purchase(w http.ResponseWriter, r *http.Request) {
	var in dto.PantryPurchaseInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'quantity' must be > 0"})
		return
	}
	out, err := h.svc.Purchase(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "purchase: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *pantryHandler) consume(w http.ResponseWriter, r *http.Request) {
	lotIDStr := r.PathValue("id")
	var in dto.PantryConsumeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.Quantity <= 0 {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'quantity' must be > 0"})
		return
	}
	if err := h.svc.Consume(r.Context(), lotIDStr, in); err != nil {
		writeError(w, http.StatusInternalServerError, "consume: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// listExpiring returns non-empty lots whose best_before is within ?within
// (default 7 days, in hours). Already-expired lots (best_before in the past)
// are always included.
func (h *pantryHandler) listExpiring(w http.ResponseWriter, r *http.Request) {
	within := 7 * 24 * time.Hour
	if v := r.URL.Query().Get("withinHours"); v != "" {
		hours, err := strconv.Atoi(v)
		if err != nil || hours <= 0 {
			writeJSON(w, http.StatusBadRequest, errorBody{Message: "query param 'withinHours' must be a positive integer"})
			return
		}
		within = time.Duration(hours) * time.Hour
	}
	out, err := h.svc.ListExpiring(r.Context(), within)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list expiring lots: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
