package httpapi

import (
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// inspirationHandler handles /inspiration routes.
type inspirationHandler struct {
	svc dto.InspirationService
}

func (h *inspirationHandler) suggest(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Suggest(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "suggest: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
