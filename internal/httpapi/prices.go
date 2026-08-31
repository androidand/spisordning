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

func (h *pricesHandler) listRetailers(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListRetailers(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list retailers: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *pricesHandler) listRetailerStores(w http.ResponseWriter, r *http.Request) {
	rid := r.PathValue("id")
	out, err := h.svc.ListRetailerStores(r.Context(), rid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list retailer stores: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *pricesHandler) listRetailerProducts(w http.ResponseWriter, r *http.Request) {
	rid := r.PathValue("id")
	out, err := h.svc.ListRetailerProducts(r.Context(), rid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list retailer products: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *pricesHandler) priceHistoryForProduct(w http.ResponseWriter, r *http.Request) {
	rpid := r.PathValue("id")
	out, err := h.svc.PriceHistoryForProduct(r.Context(), rpid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "price history for product: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *pricesHandler) priceHistoryForStore(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("id")
	out, err := h.svc.PriceHistoryForStore(r.Context(), sid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "price history for store: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
