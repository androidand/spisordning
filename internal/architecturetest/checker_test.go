package architecturetest

import (
	"strings"
	"testing"
)

const testModule = "example.com/spisordning"

// graph builds a deps map from "from: to1 to2" lines for readable tests.
func graph(lines ...string) map[string][]string {
	m := map[string][]string{}
	for _, l := range lines {
		parts := strings.Fields(l)
		if len(parts) == 0 {
			continue
		}
		from := strings.TrimSuffix(parts[0], ":")
		m[from] = parts[1:]
	}
	return m
}

func TestCheck_CleanGraphPasses(t *testing.T) {
	deps := graph(
		"example.com/spisordning/internal/domain: fmt strings",
		"example.com/spisordning/internal/recipefamily: fmt example.com/spisordning/internal/domain",
		"example.com/spisordning/internal/scoring: example.com/spisordning/internal/domain",
		"example.com/spisordning/internal/planning: example.com/spisordning/internal/domain example.com/spisordning/internal/scoring",
		"example.com/spisordning/internal/httpclient: net/http",
		"example.com/spisordning/internal/mealie: example.com/spisordning/internal/httpclient",
		"example.com/spisordning/internal/recipeimport: example.com/spisordning/internal/domain",
		"example.com/spisordning/internal/persistence: example.com/spisordning/internal/domain github.com/jackc/pgx/v5",
		"example.com/spisordning/internal/httpapi: example.com/spisordning/internal/planning",
		"example.com/spisordning/cmd/food-brain: example.com/spisordning/internal/persistence example.com/spisordning/internal/httpapi",
		"fmt: ",
	)
	if v := Check(testModule, deps); len(v) != 0 {
		t.Fatalf("expected clean graph, got violations:\n%s", strings.Join(v, "\n"))
	}
}

func TestCheck_DomainImportsPersistence(t *testing.T) {
	deps := graph(
		"example.com/spisordning/internal/domain: example.com/spisordning/internal/persistence",
		"example.com/spisordning/internal/persistence: fmt",
	)
	v := Check(testModule, deps)
	if len(v) == 0 {
		t.Fatal("expected a violation for domain importing persistence")
	}
	if !strings.Contains(v[0], "internal/domain") || !strings.Contains(v[0], "internal/persistence") {
		t.Fatalf("violation should name both packages, got: %q", v[0])
	}
}

func TestCheck_ClientImportsApplication(t *testing.T) {
	deps := graph(
		"example.com/spisordning/internal/mealie: example.com/spisordning/internal/planning",
	)
	v := Check(testModule, deps)
	if len(v) == 0 {
		t.Fatal("expected a violation for a client importing application")
	}
}

func TestCheck_HttpapiImportsPersistence(t *testing.T) {
	deps := graph(
		"example.com/spisordning/internal/httpapi: example.com/spisordning/internal/persistence",
		"example.com/spisordning/internal/persistence: fmt",
	)
	v := Check(testModule, deps)
	if len(v) == 0 {
		t.Fatal("expected a violation for httpapi importing persistence")
	}
}

func TestCheck_InternalImportsCmd(t *testing.T) {
	deps := graph(
		"example.com/spisordning/internal/planning: example.com/spisordning/cmd/food-brain",
	)
	v := Check(testModule, deps)
	if len(v) == 0 {
		t.Fatal("expected a violation for internal importing the composition root")
	}
}

func TestCheck_UnclassifiedInternalPackage(t *testing.T) {
	deps := graph(
		"example.com/spisordning/internal/future-layer: example.com/spisordning/internal/domain",
		"example.com/spisordning/internal/domain: fmt",
	)
	v := Check(testModule, deps)
	if len(v) == 0 {
		t.Fatal("expected a violation for an unclassified internal package")
	}
}
