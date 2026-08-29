package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/androidand/spisordning/internal/dto"
)

// recipeFamilyHandler handles /recipe-families routes (the git-like recipe
// hierarchy: family -> variant -> revision).
type recipeFamilyHandler struct {
	svc dto.RecipeFamilyService
}

func (h *recipeFamilyHandler) listFamilies(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.ListFamilies(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list recipe families: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *recipeFamilyHandler) getFamily(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	out, err := h.svc.GetFamily(r.Context(), id)
	if errors.Is(err, dto.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "recipe family " + id + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get recipe family: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *recipeFamilyHandler) createFamily(w http.ResponseWriter, r *http.Request) {
	var in dto.CreateRecipeFamilyInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.CreateFamily(r.Context(), in)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create recipe family: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *recipeFamilyHandler) listVariants(w http.ResponseWriter, r *http.Request) {
	familyID := r.PathValue("id")
	out, err := h.svc.ListVariants(r.Context(), familyID)
	if errors.Is(err, dto.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "recipe family " + familyID + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list recipe variants: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *recipeFamilyHandler) createVariant(w http.ResponseWriter, r *http.Request) {
	familyID := r.PathValue("id")
	var in dto.CreateRecipeVariantInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.CreateVariant(r.Context(), familyID, in)
	if errors.Is(err, dto.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "recipe family " + familyID + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create recipe variant: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *recipeFamilyHandler) listRevisions(w http.ResponseWriter, r *http.Request) {
	variantID := r.PathValue("variantId")
	out, err := h.svc.ListRevisions(r.Context(), variantID)
	if errors.Is(err, dto.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "recipe variant " + variantID + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list recipe revisions: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *recipeFamilyHandler) getRevision(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("revisionId"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "revision id must be an integer"})
		return
	}
	out, serr := h.svc.GetRevision(r.Context(), id)
	if errors.Is(serr, dto.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "recipe revision " + r.PathValue("revisionId") + " not found"})
		return
	}
	if serr != nil {
		writeError(w, http.StatusInternalServerError, "get recipe revision: "+serr.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *recipeFamilyHandler) createRevision(w http.ResponseWriter, r *http.Request) {
	variantID := r.PathValue("variantId")
	var in dto.CreateRecipeRevisionInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.CreateRevision(r.Context(), variantID, in)
	if errors.Is(err, dto.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, errorBody{Message: "recipe variant " + variantID + " not found"})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create recipe revision: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (h *recipeFamilyHandler) setDefaultVariant(w http.ResponseWriter, r *http.Request) {
	familyID := r.PathValue("id")
	variantID := r.PathValue("variantId")
	if err := h.svc.SetDefaultVariant(r.Context(), familyID, variantID); err != nil {
		if errors.Is(err, dto.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorBody{Message: "recipe family or variant not found"})
			return
		}
		writeError(w, http.StatusInternalServerError, "set default variant: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
