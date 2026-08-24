package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// ReactionNew is the request body for POST /reactions (api/openapi.yaml ReactionNew).
type ReactionNew struct {
	PersonID  string  `json:"person_id"`
	Sentiment int     `json:"sentiment"` // -2..2
	Note      *string `json:"note,omitempty"`
}

// ReactionService is the write surface the /reactions handler needs.
type ReactionService interface {
	CreateReaction(ctx context.Context, in ReactionNew) (dto.MealReactionResponse, error)
}

type reactionsHandler struct {
	svc ReactionService
}

func (h *reactionsHandler) createReaction(w http.ResponseWriter, r *http.Request) {
	var in ReactionNew
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	if in.PersonID == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "field 'person_id' is required"})
		return
	}
	if in.Sentiment < -2 || in.Sentiment > 2 {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "sentiment must be in [-2, 2]"})
		return
	}
	out, err := h.svc.CreateReaction(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create reaction: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
