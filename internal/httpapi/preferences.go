package httpapi

import (
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// prefsHandler handles /preferences routes.
type prefsHandler struct {
	svc dto.PreferencesService
}

func (h *prefsHandler) listPreferences(w http.ResponseWriter, r *http.Request) {
	personID := r.URL.Query().Get("personId")
	out, err := h.svc.ListPreferences(r.Context(), personID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list preferences: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
