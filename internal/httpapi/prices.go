package httpapi

import (
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// pricesHandler handles /prices routes (price intelligence reads).
type pricesHandler struct {
	svc dto.PriceIntelligenceService
}

func (h *pricesHandler) listProductPrices(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListProductPrices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list product prices: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
