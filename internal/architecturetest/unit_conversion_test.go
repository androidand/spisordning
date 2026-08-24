package architecturetest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoSilentUnitConversion enforces design.md invariant 11:
// no code path (trigger, default-fill, or application code reacting to
// "purchase unit differs from stock unit") is allowed to insert a placeholder
// or default conversion into unit_conversion or ingredient_unit_conversion.
// Every conversion row must be written only via an explicit
// DefineUnitConversion / DefineIngredientUnitConversion command.
//
// This test scans all Go source files for SQL INSERT/UPDATE targeting those
// tables and asserts they only appear in:
//   - the migration file (0011_household_and_catalog.sql, which seeds reference
//     data, not application code)
//   - method bodies whose name contains "DefineUnitConversion" or
//     "DefineIngredientUnitConversion"
func TestNoSilentUnitConversion(t *testing.T) {
	forbiddenTables := []string{"unit_conversion", "ingredient_unit_conversion"}

	// Locate the module root by walking up from the current working directory
	// until we find go.mod. grep is then run from the module root so that
	// ./internal/... and ./cmd/... resolve correctly regardless of which
	// package's test is executing.
	moduleRoot := moduleRootDir(t)

	// Run grep from the module root.
	// Note: grep does not understand shell glob patterns like "..." — we pass
	// the actual directories and rely on grep -r to recurse. Exit code 2
	// means no matches (grep treated the directory as empty), which is fine.
	cmd := exec.Command("grep", "-rn",
		"--include=*.go",
		"INSERT INTO \\|UPDATE .* SET",
		filepath.Join(moduleRoot, "internal"),
		filepath.Join(moduleRoot, "cmd"),
	)
	cmd.Dir = moduleRoot
	out, err := cmd.Output()
	if err != nil && !isNoMatch(err) {
		// grep exit code 2 = no matches found (empty search result), which is
		// the desired "nothing to check" outcome.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			return
		}
		t.Fatalf("grep: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return // no SQL writes found — nothing to check
	}

	for _, line := range lines {
		for _, tbl := range forbiddenTables {
			if !strings.Contains(line, tbl) {
				continue
			}
			// Extract the file path (first field before the colon).
			parts := strings.SplitN(line, ":", 3)
			if len(parts) < 2 {
				continue
			}
			filePath := parts[0]

			// Allow the migration file (seed data, not application code).
			if strings.Contains(filePath, "0011_household_and_catalog.sql") {
				continue
			}

			// Allow test files that explicitly test the invariant.
			if strings.Contains(filePath, "unit_conversion_test") ||
				strings.Contains(filePath, "unit_test") {
				continue
			}

			// Check if the method name in the surrounding context is an
			// explicit DefineUnitConversion / DefineIngredientUnitConversion.
			// Read the file and look for the method declaration near the match.
			if isAllowedMethod(filePath, line) {
				continue
			}

			t.Errorf("forbidden SQL write to %s outside explicit conversion method: %s",
				tbl, strings.TrimSpace(line))
		}
	}
}

// moduleRootDir walks up from the current working directory until it finds a
// go.mod file and returns that directory. Fails the test if go.mod is not
// found within a reasonable depth.
func moduleRootDir(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find module root (go.mod not found)")
		}
		dir = parent
	}
}

// isAllowedMethod checks whether the SQL write on the given line is inside a
// method whose name contains "DefineUnitConversion" or
// "DefineIngredientUnitConversion".
func isAllowedMethod(filePath, line string) bool {
	// Read the file and scan backwards from the matched line for a func
	// declaration.
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	// Simpler approach: search the file content for the line and then
	// look backwards for a func declaration.
	content := string(data)
	searchTarget := strings.TrimSpace(strings.SplitN(line, ":", 3)[2])
	idx := strings.Index(content, searchTarget)
	if idx < 0 {
		return false
	}

	// Scan backwards from idx for a func declaration.
	prefix := content[:idx]
	// Find the last "\nfunc " before idx.
	for {
		funcIdx := strings.LastIndex(prefix, "\nfunc ")
		if funcIdx < 0 {
			break
		}
		// Extract the method name, skipping the receiver ("(s *Store) ") when
		// present so "func (s *Store) DefineUnitConversion(...)" isn't
		// mistaken for a bare "func (...)" with an empty name.
		afterFunc := prefix[funcIdx+6:] // skip "\nfunc "
		if strings.HasPrefix(afterFunc, "(") {
			closeIdx := strings.Index(afterFunc, ")")
			if closeIdx < 0 {
				break
			}
			afterFunc = strings.TrimLeft(afterFunc[closeIdx+1:], " ")
		}
		nameEnd := strings.Index(afterFunc, "(")
		if nameEnd < 0 {
			nameEnd = strings.Index(afterFunc, "{")
		}
		if nameEnd < 0 {
			break
		}
		methodName := afterFunc[:nameEnd]
		if strings.Contains(methodName, "DefineUnitConversion") ||
			strings.Contains(methodName, "DefineIngredientUnitConversion") {
			return true
		}
		prefix = prefix[:funcIdx]
	}
	return false
}

func isNoMatch(err error) bool {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode() == 1 || exitErr.ExitCode() == 2
	}
	return false
}
