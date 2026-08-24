package httpapi

import (
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// storesHandler handles /products routes.
type storesHandler struct {
	svc dto.StoresService
}

func (h *storesHandler) searchProducts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	out, err := h.svc.SearchProducts(r.Context(), query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search products: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *storesHandler) searchByGTIN(w http.ResponseWriter, r *http.Request) {
	gtin := r.URL.Query().Get("gtin")
	if gtin == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "query param 'gtin' is required"})
		return
	}
	out, err := h.svc.SearchProductsByGTIN(r.Context(), gtin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search by gtin: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
