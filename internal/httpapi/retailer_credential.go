package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

// ── Retailer elevated-credential upload ─────────────────────────────────────
//
// Receiving end of the flow documented in docs/infrastructure/ica-elevated-
// auth.md: ICA's elevated (ecom) session can only be refreshed via a real
// browser login, which can't run on the headless Proxmox deployment target.
// The login runs wherever a human and a display exist (today: a Mac script),
// and its result is uploaded here instead of assumed to share a filesystem
// with whichever adapter actually needs it.

// RetailerCredentialService is the application surface for uploading and
// fetching a retailer's manually-refreshed elevated-auth credential.
type RetailerCredentialService interface {
	UploadRetailerCredential(ctx context.Context, retailer string, payload json.RawMessage) (RetailerCredentialResponse, error)
	GetRetailerCredential(ctx context.Context, retailer string) (RetailerCredentialResponse, error)
}

// RetailerCredentialResponse is the JSON view for both the upload response
// and the fetch response. Payload is returned verbatim — spisordning never
// interprets its contents (e.g. ICA's ImportedCookie[] shape).
type RetailerCredentialResponse struct {
	Retailer   string          `json:"retailer"`
	Payload    json.RawMessage `json:"payload"`
	UploadedAt time.Time       `json:"uploaded_at"`
}

type retailerCredentialHandler struct{ svc RetailerCredentialService }

// upload handles POST /retailers/{retailer}/elevated-credential. The body is
// the credential payload itself (e.g. ICA's cookie array) — no envelope, so
// the uploading script doesn't need to know spisordning's response shape,
// only its own data.
func (h *retailerCredentialHandler) upload(w http.ResponseWriter, r *http.Request) {
	retailer := strings.TrimSpace(r.PathValue("retailer"))
	if retailer == "" {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "retailer is required"})
		return
	}
	var payload json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, errorBody{Message: "invalid JSON body: " + err.Error()})
		return
	}
	out, err := h.svc.UploadRetailerCredential(r.Context(), retailer, payload)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload retailer credential: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

// get handles GET /retailers/{retailer}/elevated-credential — the fetch side
// an adapter (e.g. ica-adapter, deployed separately from wherever the login
// ran) polls to pick up the latest uploaded credential.
func (h *retailerCredentialHandler) get(w http.ResponseWriter, r *http.Request) {
	retailer := strings.TrimSpace(r.PathValue("retailer"))
	out, err := h.svc.GetRetailerCredential(r.Context(), retailer)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeJSON(w, http.StatusNotFound, errorBody{Message: "no credential uploaded yet for " + retailer})
			return
		}
		writeError(w, http.StatusInternalServerError, "get retailer credential: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, out)
}
