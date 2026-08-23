package httpapi

import (
	"context"
	"net/http"
)

// StoresService is the surface the /stores and /products handlers need.
type StoresService interface {
	SearchProducts(ctx context.Context, query string) ([]IngredientProduct, error)
	SearchProductsByGTIN(ctx context.Context, gtin string) ([]IngredientProduct, error)
}

// storesHandler handles /stores and /products routes.
type storesHandler struct {
	svc StoresService
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
