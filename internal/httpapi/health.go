// Package httpapi is the outermost layer of food-brain: HTTP handlers and
// wiring, sourced from api/openapi.yaml. It depends only on the application
// and domain layers (never persistence directly — enforced by the architecture
// test) and registers its handlers via stdlib net/http with no external router
// until api/openapi.yaml is code-gen'd (task 3.2).
package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/androidand/spisordning/internal/dto"
)

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(dto.Health{Status: "ok"})
}
