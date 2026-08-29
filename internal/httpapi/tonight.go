package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// TonightService is the read surface the /tonight handler needs.
type TonightService interface {
	GetTonight(ctx context.Context) (dto.TonightView, error)
}

type tonightHandler struct {
	svc TonightService
}

func (h *tonightHandler) getTonight(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.GetTonight(r.Context())
	if err != nil {
		if errors.Is(err, dto.ErrNoMealTonight) {
			writeJSON(w, http.StatusNotFound, errorBody{Message: "no meal planned for today"})
			return
		}
		writeError(w, http.StatusInternalServerError, "get tonight: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
