// Package tools pins tooling dependencies for spisordning so they are reproducible
// and recorded in go.mod without being compiled into any shipped binary.
//
// Add a tool's command path here, then `go mod tidy` to record it; invoke via
// `go run <importpath>` (e.g. in the Makefile) or `go generate`.
//
// The //go:build tools tag keeps this file out of every normal build and,
// importantly, out of `go list -deps ./...` — so internal/architecturetest never
// sees these tool imports and the layer rules stay clean.
//
//go:build tools
// +build tools

package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
