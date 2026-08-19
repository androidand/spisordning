package httpapi

import (
	"encoding/json"
	"net/http"
)

// errorBody is the shared error envelope (openapi: components/schemas/Error).
type errorBody struct {
	Message string `json:"message"`
}

// writeJSON encodes v as JSON with the given status. It panics-safe on encode
// failure only in tests; in production a marshaling failure is a server bug,
// so we fall back to a plain-text 500.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Last-resort: encode already wrote a header, so just emit text.
		http.Error(w, "internal marshal error", http.StatusInternalServerError)
	}
}

// writeError is a convenience for unexpected/internal errors where we only have
// a reason string.
func writeError(w http.ResponseWriter, status int, reason string) {
	writeJSON(w, status, errorBody{Message: reason})
}
