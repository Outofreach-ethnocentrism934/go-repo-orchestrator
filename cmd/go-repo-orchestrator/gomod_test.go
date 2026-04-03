package main_test

import (
	"bufio"
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// moduleRoot returns the absolute path to the repository root (where go.mod lives).
func moduleRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// This file lives at cmd/go-repo-orchestrator/gomod_test.go,
	// so two parent directories up is the module root.
	return filepath.Join(filepath.Dir(filename), "..", "..")
}

// readGoMod returns the parsed lines of go.mod from the module root.
func readGoMod(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(moduleRoot(t), "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan go.mod: %v", err)
	}
	return lines
}

// readGoSum returns the lines of go.sum from the module root.
func readGoSum(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(moduleRoot(t), "go.sum")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read go.sum: %v", err)
	}
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan go.sum: %v", err)
	}
	return lines
}

// TestRemovedDirectDependenciesAbsentFromGoMod verifies that the two direct
// dependencies removed in this PR no longer appear in go.mod.
func TestRemovedDirectDependenciesAbsentFromGoMod(t *testing.T) {
	t.Parallel()

	removed := []string{
		"github.com/go-sql-driver/mysql",
		"gopkg.in/andygrunwald/go-jira.v1",
	}

	lines := readGoMod(t)
	for _, dep := range removed {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, dep) {
				t.Errorf("go.mod still references removed direct dependency %q (line: %q)", dep, trimmed)
			}
		}
	}
}

// TestRemovedIndirectDependenciesAbsentFromGoMod verifies that the indirect
// dependencies removed in this PR are no longer listed in go.mod.
func TestRemovedIndirectDependenciesAbsentFromGoMod(t *testing.T) {
	t.Parallel()

	removed := []string{
		"filippo.io/edwards25519",
		"github.com/fatih/structs",
		"github.com/google/go-querystring",
		"github.com/pkg/errors",
		"github.com/trivago/tgo",
	}

	lines := readGoMod(t)
	for _, dep := range removed {
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, dep) {
				t.Errorf("go.mod still references removed indirect dependency %q (line: %q)", dep, trimmed)
			}
		}
	}
}

// TestGoJoseVersionBumpedToV305 verifies that go-jose/go-jose/v3 was upgraded
// to v3.0.5 and the old v3.0.4 is no longer present.
func TestGoJoseVersionBumpedToV305(t *testing.T) {
	t.Parallel()

	lines := readGoMod(t)

	foundV305 := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "github.com/go-jose/go-jose/v3") {
			if strings.Contains(trimmed, "v3.0.4") {
				t.Errorf("go.mod still references old go-jose v3.0.4: %q", trimmed)
			}
			if strings.Contains(trimmed, "v3.0.5") {
				foundV305 = true
			}
		}
	}

	if !foundV305 {
		t.Error("go.mod does not contain expected go-jose/go-jose/v3 v3.0.5")
	}
}

// TestRemovedDependenciesAbsentFromGoSum verifies that go.sum no longer contains
// entries for the packages whose go.sum lines were explicitly removed in this PR.
// Note: github.com/pkg/errors was removed from go.mod but its go.sum entries were
// intentionally retained (still transitively referenced), so it is not checked here.
func TestRemovedDependenciesAbsentFromGoSum(t *testing.T) {
	t.Parallel()

	removedPrefixes := []string{
		"filippo.io/edwards25519",
		"github.com/fatih/structs",
		"github.com/go-sql-driver/mysql",
		"github.com/google/go-querystring",
		"github.com/trivago/tgo",
		"gopkg.in/andygrunwald/go-jira.v1",
	}

	lines := readGoSum(t)
	for _, prefix := range removedPrefixes {
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), prefix) {
				t.Errorf("go.sum still contains entry for removed package %q: %q", prefix, strings.TrimSpace(line))
			}
		}
	}
}

// TestGoJoseOldVersionAbsentFromGoSum verifies that go.sum no longer contains
// the old go-jose v3.0.4 hash entry.
func TestGoJoseOldVersionAbsentFromGoSum(t *testing.T) {
	t.Parallel()

	lines := readGoSum(t)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "github.com/go-jose/go-jose/v3") &&
			strings.Contains(trimmed, "v3.0.4") {
			t.Errorf("go.sum still contains old go-jose v3.0.4 entry: %q", trimmed)
		}
	}
}

// TestGoJoseNewVersionPresentInGoSum verifies that go.sum contains the new
// go-jose v3.0.5 hash entry.
func TestGoJoseNewVersionPresentInGoSum(t *testing.T) {
	t.Parallel()

	lines := readGoSum(t)
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "github.com/go-jose/go-jose/v3") &&
			strings.Contains(trimmed, "v3.0.5") {
			found = true
			break
		}
	}
	if !found {
		t.Error("go.sum does not contain expected go-jose/go-jose/v3 v3.0.5 entry")
	}
}

// TestGoModVerify runs "go mod verify" to confirm that all modules on disk
// match their expected hashes in go.sum.
func TestGoModVerify(t *testing.T) {
	root := moduleRoot(t)

	cmd := exec.Command("go", "mod", "verify")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod verify failed: %v\n%s", err, out)
	}
}

// TestGoModTidy verifies the module graph is tidy — that is, running "go mod tidy"
// would produce no diff. It relies on "go mod tidy" exiting non-zero when the
// module is not tidy in recent Go toolchains.
func TestGoModTidy(t *testing.T) {
	root := moduleRoot(t)

	// Capture current go.mod and go.sum content before attempting tidy.
	goModPath := filepath.Join(root, "go.mod")
	goSumPath := filepath.Join(root, "go.sum")

	originalGoMod, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod before tidy: %v", err)
	}
	originalGoSum, err := os.ReadFile(goSumPath)
	if err != nil {
		t.Fatalf("read go.sum before tidy: %v", err)
	}

	// Run go mod tidy.
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go mod tidy failed: %v\n%s", err, out)
	}

	// Read updated files.
	newGoMod, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatalf("read go.mod after tidy: %v", err)
	}
	newGoSum, err := os.ReadFile(goSumPath)
	if err != nil {
		t.Fatalf("read go.sum after tidy: %v", err)
	}

	// Restore originals regardless of outcome.
	t.Cleanup(func() {
		_ = os.WriteFile(goModPath, originalGoMod, 0o644)
		_ = os.WriteFile(goSumPath, originalGoSum, 0o644)
	})

	if !bytes.Equal(originalGoMod, newGoMod) {
		t.Error("go.mod changed after go mod tidy — module is not tidy")
	}
	if !bytes.Equal(originalGoSum, newGoSum) {
		t.Error("go.sum changed after go mod tidy — module is not tidy")
	}
}