// Package openapi contains the Go primitives generated from api/openapi.yaml.
//
// It is the single source of truth for the service's request/response shapes:
// the HTTP handlers in internal/httpapi and the persistence DTOs are mapped to
// these types. The file types.gen.go is committed (never hand-edited) and kept in
// sync with the spec by `go generate ./internal/openapi` (see doc.go) and the CI
// `codegen` job (`verify-codegen` Makefile target).
//
// Tool: github.com/oapi-codegen/oapi-codegen/v2 (v2.5.1), matching tengil's
// choice. Codegen uses `-generate types` only (stdlib net/http servers, no chi).
package openapi

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -generate types -package openapi -o types.gen.go ../../api/openapi.yaml
