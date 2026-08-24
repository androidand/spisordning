package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// TonightView mirrors api/openapi.yaml components/schemas/TonightView.
type TonightView struct {
	ServedOn  string                     `json:"served_on"`
	Recipe    dto.RecipeRefResponse      `json:"recipe"`
	Reactions []dto.MealReactionResponse `json:"reactions"`
}

// TonightService is the read surface the /tonight handler needs.
type TonightService interface {
	GetTonight(ctx context.Context) (TonightView, error)
}

type tonightHandler struct {
	svc TonightService
}

func (h *tonightHandler) getTonight(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetTonight(r.Context())
	if err != nil {
		if errors.Is(err, ErrNoMealTonight) {
			writeJSON(w, http.StatusNotFound, errorBody{Message: "no meal planned for today"})
			return
		}
		writeError(w, http.StatusInternalServerError, "get tonight: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// ErrNoMealTonight is returned when there is no approved plan with a decision for today.
// Handlers map this to HTTP 404.
var ErrNoMealTonight = httpapiError("no meal tonight")

// httpapiError is a sentinel error type that handlers map to 404.
// It lives here (not in persistence) so httpapi stays decoupled from pgx.
type httpapiError string

func (e httpapiError) Error() string { return string(e) }
