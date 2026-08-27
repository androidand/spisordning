package httpapi

import (
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

// storesHandler handles /stores and /products routes.
type storesHandler struct {
	svc dto.StoresService
}

func (h *storesHandler) listStores(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListStores(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list stores: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *storesHandler) listStoreOffers(w http.ResponseWriter, r *http.Request) {
	storeID := r.PathValue("id")
	out, err := h.svc.ListStoreOffers(r.Context(), storeID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list store offers: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
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
