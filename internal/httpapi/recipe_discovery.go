package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// recipeDiscoveryHandler handles the /recipes/discover and /recipes/discovery
// routes: fetch an external recipe URL, stage it as a review candidate, and
// promote/reject candidates into the native recipe_family hierarchy.
type recipeDiscoveryHandler struct {
	svc dto.DiscoveryService
}

func (h *recipeDiscoveryHandler) discover(w http.ResponseWriter, r *http.Request) {
	var in dto.DiscoverRecipeInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.URL == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'url' is required"})
		return
	}
	out, err := h.svc.DiscoverFromURL(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "discover recipe: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *recipeDiscoveryHandler) listCandidates(w http.ResponseWriter, r *http.Request) {
	var status *string
	if s := r.URL.Query().Get("status"); s != "" {
		status = &s
	}
	out, err := h.svc.ListCandidates(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list candidates: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *recipeDiscoveryHandler) getCandidate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	out, err := h.svc.GetCandidate(r.Context(), id)
	if errors.Is(err, dto.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "candidate " + id + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get candidate: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *recipeDiscoveryHandler) rejectCandidate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.svc.RejectCandidate(r.Context(), id); err != nil {
		if errors.Is(err, dto.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorBody{Message: "candidate " + id + " not found"})
			return
		}
		writeError(w, http.StatusInternalServerError, "reject candidate: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (h *recipeDiscoveryHandler) promoteCandidate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var in dto.PromoteCandidateInput
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil && !errors.Is(err, io.EOF) {
			writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
			return
		}
	}
	out, err := h.svc.PromoteCandidate(r.Context(), id, in)
	if errors.Is(err, dto.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "candidate " + id + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "promote candidate: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
