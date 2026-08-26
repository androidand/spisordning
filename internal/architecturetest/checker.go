// Package architecturetest mechanically enforces the layering documented in
// openspec/changes/establish-enforced-go-architecture/design.md: the import
// graph of every package in this module is walked with `go list -deps` and
// checked against a small set of boundary rules. A violation fails
// `go test ./...` (and therefore CI) with the exact offending edge.
//
// Layers:
//
//	domain       internal/domain, internal/recipefamily, internal/scoring,
//	             internal/openapi, internal/ambient, internal/availability
//	application  internal/planning                                — use-case logic
//	service      internal/service, internal/mcp                   — app services
//	contract     internal/dto                                     — shared DTOs + service interfaces
//	client       internal/mealie, internal/skolmaten, internal/retailer,
//	             internal/icaretailer, internal/llm, internal/httpclient,
//	             internal/recipeimport, internal/matpriskollen, internal/ingredients
//	persistence  internal/persistence                      — Postgres repos
//	httpapi      internal/httpapi                          — HTTP handlers
//	mcptools     internal/mcptools                         — MCP tool adapters (no persistence)
//	cmd          cmd/...                                   — composition root
//
// Every future internal/ package MUST be classified here; an unclassified
// internal/ package is a violation by design.
package architecturetest

import (
	"fmt"
	"sort"
	"strings"
)

// Layer is one of the documented layers, or External/Test for non-module and
// self-test packages.
type Layer string

const (
	Domain      Layer = "domain"
	Application Layer = "application"
	Client      Layer = "client"
	Persistence Layer = "persistence"
	HTTPAPI     Layer = "httpapi"
	MCPTools    Layer = "mcptools"
	Cmd         Layer = "cmd"
	Test        Layer = "test"
	External    Layer = "external"
	Service     Layer = "service"
	Contract    Layer = "contract"
	Unknown     Layer = "unknown"
)

type prefix struct {
	layer    Layer
	prefixes []string
}

var layerPrefixes = []prefix{
	{Cmd, []string{"cmd", "cmd/mcp-server"}},
	{Test, []string{"internal/architecturetest"}},
	{Domain, []string{"internal/domain", "internal/recipefamily", "internal/scoring", "internal/openapi", "internal/ambient", "internal/availability"}},
	{Application, []string{"internal/planning"}},
	{Service, []string{"internal/service", "internal/mcp"}},
	{Contract, []string{"internal/dto"}},
	{Client, []string{
		"internal/mealie",
		"internal/skolmaten",
		"internal/retailer",
		"internal/icaretailer",
		"internal/llm",
		"internal/httpclient",
		"internal/recipeimport",
		"internal/matpriskollen",
		"internal/ingredients",
	}},
	{Persistence, []string{"internal/persistence"}},
	{HTTPAPI, []string{"internal/httpapi"}},
	{MCPTools, []string{"internal/mcptools"}},
}

// layerOf classifies a full import path relative to the module.
func layerOf(module, pkg string) Layer {
	if pkg == module {
		return Cmd
	}
	if !strings.HasPrefix(pkg, module+"/") {
		return External
	}
	rel := strings.TrimPrefix(pkg, module+"/")
	for _, p := range layerPrefixes {
		for _, pref := range p.prefixes {
			if rel == pref || strings.HasPrefix(rel, pref+"/") {
				return p.layer
			}
		}
	}
	if strings.HasPrefix(rel, "internal/") {
		return Unknown
	}
	return External
}

// rule is one boundary constraint: it reports a violation when bad(from, to)
// is true.
type rule struct {
	name string
	bad  func(from, to Layer) bool
}

var rules = []rule{
	{"domain must not import any non-domain internal package", func(f, t Layer) bool {
		return f == Domain && t != Domain && t != External && t != Test
	}},
	{"application must not import clients, service, persistence, httpapi, or cmd", func(f, t Layer) bool {
		return f == Application && (t == Client || t == Service || t == Persistence || t == HTTPAPI || t == Cmd || t == Unknown)
	}},
	// service may import domain, client, contract, and persistence: the
	// proposal's Store interface (internal/service) is defined over
	// persistence row types, so services depend on persistence by design.
	{"service must not import httpapi or cmd", func(f, t Layer) bool {
		return f == Service && (t == HTTPAPI || t == Cmd || t == Unknown)
	}},
	{"contract must not import application, service, client, persistence, httpapi, or cmd", func(f, t Layer) bool {
		return f == Contract && (t == Application || t == Service || t == Client || t == Persistence || t == HTTPAPI || t == Cmd || t == Unknown)
	}},
	{"clients must not import application, service, persistence, httpapi, or cmd", func(f, t Layer) bool {
		return f == Client && (t == Application || t == Service || t == Persistence || t == HTTPAPI || t == Cmd || t == Unknown)
	}},
	{"persistence must import only domain and external packages", func(f, t Layer) bool {
		return f == Persistence && t != Domain && t != External && t != Test
	}},
	{"httpapi must not import persistence or clients", func(f, t Layer) bool {
		return f == HTTPAPI && (t == Persistence || t == Client)
	}},
	{"mcptools must not import clients, persistence, httpapi, or cmd", func(f, t Layer) bool {
		return f == MCPTools && (t == Client || t == Persistence || t == HTTPAPI || t == Cmd || t == Unknown)
	}},
	{"no internal package may import the cmd composition root", func(f, t Layer) bool {
		return t == Cmd
	}},
	{"every internal package must be classified into a layer", func(f, t Layer) bool {
		return f == Unknown || t == Unknown
	}},
}

// Check verifies the import graph (from package -> its direct imports) against
// the layer rules and returns a sorted list of human-readable violations.
func Check(module string, deps map[string][]string) []string {
	var violations []string
	froms := make([]string, 0, len(deps))
	for from := range deps {
		froms = append(froms, from)
	}
	sort.Strings(froms)

	for _, from := range froms {
		fl := layerOf(module, from)
		if fl == External {
			continue // only module packages are policed
		}
		for _, to := range deps[from] {
			tl := layerOf(module, to)
			for _, r := range rules {
				if r.bad(fl, tl) {
					violations = append(violations, fmt.Sprintf("%s: %s -> %s", r.name, from, to))
				}
			}
		}
	}
	return violations
}
