package httpapi

import (
	"encoding/json"
	"errors"
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

func (h *prefsHandler) setPreference(w http.ResponseWriter, r *http.Request) {
	var in dto.SetPreferenceInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.SetPreference(r.Context(), in)
	if err != nil {
		if errors.Is(err, dto.ErrInvalidPreference) {
			writeJSON(w, http.StatusBadRequest, errorBody{Message: err.Error()})
			return
		}
		writeError(w, http.StatusInternalServerError, "set preference: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
