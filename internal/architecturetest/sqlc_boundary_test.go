package architecturetest

import (
	"strings"
	"testing"
)

// forbiddenDriverImports are external packages that must only appear inside
// internal/persistence. Any other package naming a concrete database driver
// or sqlc-generated code violates the adapter boundary.
var forbiddenDriverImports = []string{
	"github.com/jackc/pgx",
	"github.com/jackc/pgx/v5",
}

// TestNoPgxInDomain fails when the domain layer imports pgx or sqlc-generated
// code. The domain must be persistence-agnostic.
func TestNoPgxInDomain(t *testing.T) {
	module, deps := goListDeps(t)
	assertNoDriverImports(t, module, deps, Domain, "domain")
}

// TestNoPgxInApplication fails when the application layer imports pgx or
// sqlc-generated code.
func TestNoPgxInApplication(t *testing.T) {
	module, deps := goListDeps(t)
	assertNoDriverImports(t, module, deps, Application, "application")
}

// TestNoPgxInService fails when the service layer imports pgx or
// sqlc-generated code. Services depend on the Store interface, not the driver.
func TestNoPgxInService(t *testing.T) {
	module, deps := goListDeps(t)
	assertNoDriverImports(t, module, deps, Service, "service")
}

// TestNoPgxInHTTPAPI fails when the HTTP layer imports pgx or sqlc-generated
// code.
func TestNoPgxInHTTPAPI(t *testing.T) {
	module, deps := goListDeps(t)
	assertNoDriverImports(t, module, deps, HTTPAPI, "httpapi")
}

// TestNoPgxInMCPTools fails when the MCP tools layer imports pgx or
// sqlc-generated code.
func TestNoPgxInMCPTools(t *testing.T) {
	module, deps := goListDeps(t)
	assertNoDriverImports(t, module, deps, MCPTools, "mcptools")
}

// TestNoPgxInContract fails when the DTO/contract layer imports pgx or
// sqlc-generated code.
func TestNoPgxInContract(t *testing.T) {
	module, deps := goListDeps(t)
	assertNoDriverImports(t, module, deps, Contract, "contract")
}

// TestNoPgxInClient fails when any client package imports pgx or
// sqlc-generated code.
func TestNoPgxInClient(t *testing.T) {
	module, deps := goListDeps(t)
	assertNoDriverImports(t, module, deps, Client, "client")
}

// TestNoPgxInConfig fails when the config layer imports pgx or
// sqlc-generated code.
func TestNoPgxInConfig(t *testing.T) {
	module, deps := goListDeps(t)
	assertNoDriverImports(t, module, deps, Config, "config")
}

// assertNoDriverImports walks the import graph and fails the test when any
// package classified as layer imports a forbidden driver or sqlc package.
func assertNoDriverImports(t *testing.T, module string, deps map[string][]string, layer Layer, label string) {
	t.Helper()
	sqlcPkg := module + "/internal/persistence/sqlc"

	for pkg, imports := range deps {
		if layerOf(module, pkg) != layer {
			continue
		}
		for _, imp := range imports {
			for _, f := range forbiddenDriverImports {
				if imp == f || strings.HasPrefix(imp, f+"/") {
					t.Errorf("%s violation: %s imports %s", label, pkg, imp)
				}
			}
			if imp == sqlcPkg || strings.HasPrefix(imp, sqlcPkg+"/") {
				t.Errorf("%s violation: %s imports sqlc-generated code %s", label, pkg, imp)
			}
		}
	}
}
