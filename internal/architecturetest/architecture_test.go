package architecturetest

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// goListDeps returns the module path and the direct-import graph of every
// package, produced by `go list -deps` run from the module root (the test's
// working directory is the package dir, so ./... must be resolved there).
//
// The module path is derived from this test package's own import path rather
// than `go list -m` (which lists every module in the enclosing go.work
// workspace, e.g. /Users/andreas/dev/go.work includes sibling projects).
func goListDeps(t *testing.T) (module string, deps map[string][]string) {
	t.Helper()

	gomodOut, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD: %v", err)
	}
	root := filepath.Dir(strings.TrimSpace(string(gomodOut)))

	selfOut, err := exec.Command("go", "list", "-f", "{{.ImportPath}}", ".").Output()
	if err != nil {
		t.Fatalf("go list -f ImportPath: %v", err)
	}
	self := strings.TrimSpace(string(selfOut))
	const pkgSuffix = "/internal/architecturetest"
	if !strings.HasSuffix(self, pkgSuffix) {
		t.Fatalf("unexpected package path %q (expected %q suffix)", self, pkgSuffix)
	}
	module = strings.TrimSuffix(self, pkgSuffix)

	cmd := exec.Command("go", "list", "-deps", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	deps = map[string][]string{}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		deps[fields[0]] = fields[1:]
	}
	return module, deps
}

// TestLayeredArchitecture enforces the layer-boundary rules from
// design.md section 1 against the real import graph. It is the CI guard:
// any package importing across a forbidden boundary fails the build.
func TestLayeredArchitecture(t *testing.T) {
	module, deps := goListDeps(t)
	violations := Check(module, deps)
	if len(violations) > 0 {
		t.Fatalf("layer-boundary violations:\n%s", strings.Join(violations, "\n"))
	}
}
