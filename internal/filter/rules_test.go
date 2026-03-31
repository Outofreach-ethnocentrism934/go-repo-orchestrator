package filter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"
	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

func TestEvaluate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	raw := "repos:\n  - name: test\n    url: git@gitlab.com:anHome/git-branch-cleaner-test.git\n    branch:\n      keep: ['^release/.*$']\n"
	if err := os.WriteFile(configPath, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	repo := cfg.Repos[0]

	if _, reason := Evaluate(repo, model.BranchInfo{Name: "main", LastCommitAt: time.Now()}, "feature/a", "main"); reason != "ветка по умолчанию" {
		t.Fatalf("unexpected reason for default branch: %s", reason)
	}

	allowed, reason := Evaluate(repo, model.BranchInfo{Name: "feature/a", LastCommitAt: time.Now()}, "feature/current", "main")
	if !allowed || reason == "" {
		t.Fatalf("feature branch should be allowed")
	}

	allowed, reason = Evaluate(repo, model.BranchInfo{Name: "release/1.0.0", LastCommitAt: time.Now()}, "feature/current", "main")
	if allowed || reason == "" {
		t.Fatalf("release branch should be blocked by branch.keep")
	}
}
