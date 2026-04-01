package usecase

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"
	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

func TestGenerateDeleteScriptSkipsProtectedBranches(t *testing.T) {
	t.Parallel()

	repoPath := t.TempDir()
	cleaner := NewCleaner(nil)

	branches := []model.BranchInfo{
		{Name: "main", Protected: true, MergeStatus: model.MergeStatusMerged},
		{Name: "feature/ok", Protected: false, MergeStatus: model.MergeStatusMerged},
	}

	result, err := cleaner.GenerateDeleteScript(config.RepoConfig{Name: "demo"}, repoPath, branches, model.ScriptFormatSH)
	if err != nil {
		t.Fatalf("generate script with protected branch in input: %v", err)
	}
	if result.BranchesCount != 1 {
		t.Fatalf("expected 1 eligible branch, got %d", result.BranchesCount)
	}

	bodyBytes, err := os.ReadFile(result.ScriptPath)
	if err != nil {
		t.Fatalf("read generated script: %v", err)
	}
	body := string(bodyBytes)
	if strings.Contains(body, "'main'") {
		t.Fatalf("protected branch command must not be generated: %s", body)
	}
	if !strings.Contains(body, "git branch -d 'feature/ok'") {
		t.Fatalf("expected command for eligible branch, got: %s", body)
	}

	cwd, _ := os.Getwd()
	if filepath.Dir(result.ScriptPath) != cwd {
		t.Fatalf("script must be generated in cwd path, expected %s, got %s", cwd, filepath.Dir(result.ScriptPath))
	}
	defer func() {
		_ = os.Remove(result.ScriptPath)
	}()
}

func TestGenerateDeleteScriptFailsWhenAllBranchesProtected(t *testing.T) {
	t.Parallel()

	cleaner := NewCleaner(nil)
	_, err := cleaner.GenerateDeleteScript(
		config.RepoConfig{Name: "demo"},
		t.TempDir(),
		[]model.BranchInfo{{Name: "main", Protected: true}},
		model.ScriptFormatSH,
	)
	if err == nil {
		t.Fatalf("expected error when all selected branches are protected")
	}
	if !strings.Contains(err.Error(), "подходящие ветки не выбраны") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateDeleteScriptIncludesRemoteDeleteCommands(t *testing.T) {
	t.Parallel()

	repoPath := t.TempDir()
	cleaner := NewCleaner(nil)

	branches := []model.BranchInfo{
		{Name: "feature/local", Scope: model.BranchScopeLocal, MergeStatus: model.MergeStatusMerged},
		{Name: "feature/remote", Scope: model.BranchScopeRemote, RemoteName: "origin", QualifiedName: "origin/feature/remote", MergeStatus: model.MergeStatusMerged},
	}

	result, err := cleaner.GenerateDeleteScript(config.RepoConfig{Name: "demo"}, repoPath, branches, model.ScriptFormatSH)
	if err != nil {
		t.Fatalf("generate script with remote branch in input: %v", err)
	}
	if result.BranchesCount != 2 {
		t.Fatalf("expected 2 eligible branches, got %d", result.BranchesCount)
	}

	bodyBytes, err := os.ReadFile(result.ScriptPath)
	if err != nil {
		t.Fatalf("read generated script: %v", err)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "git branch -d 'feature/local'") {
		t.Fatalf("expected command for local branch, got: %s", body)
	}
	if !strings.Contains(body, "git push 'origin' --delete 'feature/remote'") {
		t.Fatalf("expected remote delete command, got: %s", body)
	}

	defer func() {
		_ = os.Remove(result.ScriptPath)
	}()
}

func TestGenerateDeleteScriptSkipsAmbiguousRemoteBranches(t *testing.T) {
	t.Parallel()

	repoPath := t.TempDir()
	cleaner := NewCleaner(nil)

	branches := []model.BranchInfo{{
		Name:          "feature/remote",
		Scope:         model.BranchScopeRemote,
		RemoteName:    "origin",
		QualifiedName: "upstream/feature/remote",
	}}

	_, err := cleaner.GenerateDeleteScript(config.RepoConfig{Name: "demo"}, repoPath, branches, model.ScriptFormatSH)
	if err == nil {
		t.Fatal("expected error when only ambiguous remote branches are selected")
	}
	if !strings.Contains(err.Error(), "подходящие ветки не выбраны") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateDeleteScriptUsesUniqueNames(t *testing.T) {
	t.Parallel()

	const runs = 24
	repoPath := t.TempDir()
	cleaner := NewCleaner(nil)
	branches := []model.BranchInfo{{Name: "feature/ok", MergeStatus: model.MergeStatusMerged}}

	paths := make(chan string, runs)
	errCh := make(chan error, runs)
	var wg sync.WaitGroup

	for i := 0; i < runs; i++ {
		wg.Go(func() {
			result, err := cleaner.GenerateDeleteScript(config.RepoConfig{Name: "demo"}, repoPath, branches, model.ScriptFormatSH)
			if err != nil {
				errCh <- err
				return
			}

			paths <- result.ScriptPath
		})
	}

	wg.Wait()
	close(errCh)
	close(paths)

	for err := range errCh {
		if err != nil {
			t.Fatalf("generate script concurrently: %v", err)
		}
	}

	seen := make(map[string]struct{}, runs)
	for path := range paths {
		name := filepath.Base(path)
		if _, exists := seen[name]; exists {
			t.Fatalf("duplicate script filename: %s", name)
		}
		seen[name] = struct{}{}
		_ = os.Remove(path)
	}

	if len(seen) != runs {
		t.Fatalf("expected %d unique scripts, got %d", runs, len(seen))
	}
}
