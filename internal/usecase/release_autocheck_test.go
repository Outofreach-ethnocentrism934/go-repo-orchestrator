package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/spf13/viper"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"
	"github.com/agelxnash/go-repo-orchestrator/internal/jira"
	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

type fakeReleaseService struct {
	listVersionsFn  func(ctx context.Context, group string) ([]jira.ReleaseVersion, error)
	listIssueKeysFn func(ctx context.Context, group, releaseID string) ([]string, error)
}

func (f fakeReleaseService) ListReleasedFixVersions(ctx context.Context, group string) ([]jira.ReleaseVersion, error) {
	if f.listVersionsFn != nil {
		return f.listVersionsFn(ctx, group)
	}
	return nil, nil
}

func (f fakeReleaseService) ListDoneIssueKeysByRelease(ctx context.Context, group, releaseID string) ([]string, error) {
	if f.listIssueKeysFn != nil {
		return f.listIssueKeysFn(ctx, group, releaseID)
	}
	return nil, nil
}

func TestListRepoReleasedFixVersionsCollectsOnlyMappedGroups(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("jira", []map[string]any{{"group": "TASKS", "url": "https://tasks.example.org"}})
	v.Set("repos", []map[string]any{{
		"name": "demo",
		"path": "/tmp/demo",
		"branch": map[string]any{
			"jira": []string{`^(?P<TASKS>OPS-\d+)$`},
		},
	}})

	cfg, err := config.LoadFromViper(v)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	repo := cfg.Repos[0]

	called := 0
	cleaner := NewCleaner(nil, WithJiraReleaseService(fakeReleaseService{
		listVersionsFn: func(_ context.Context, group string) ([]jira.ReleaseVersion, error) {
			called++
			if group != "TASKS" {
				t.Fatalf("unexpected group: %s", group)
			}
			return []jira.ReleaseVersion{{ID: "11", Name: "R11", ReleaseDate: "2026-04-20"}}, nil
		},
	}))

	branches := []model.BranchInfo{
		{Name: "OPS-123", Scope: model.BranchScopeLocal},
		{Name: "misc/no-jira", Scope: model.BranchScopeLocal},
	}

	releases, err := cleaner.ListRepoReleasedFixVersions(t.Context(), repo, branches)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called != 1 {
		t.Fatalf("expected single group call, got %d", called)
	}
	if len(releases) != 1 {
		t.Fatalf("expected 1 release option, got %d", len(releases))
	}
	if releases[0].Group != "TASKS" || releases[0].Version.ID != "11" {
		t.Fatalf("unexpected release option: %#v", releases[0])
	}
}

func TestBuildReleaseAutocheckCandidatesMapsAndFiltersProtected(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("jira", []map[string]any{{"group": "TASKS", "url": "https://tasks.example.org"}})
	v.Set("repos", []map[string]any{{
		"name": "demo",
		"path": "/tmp/demo",
		"branch": map[string]any{
			"jira": []string{`^(?P<TASKS>OPS-\d+)$`},
		},
	}})

	cfg, err := config.LoadFromViper(v)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	repo := cfg.Repos[0]

	cleaner := NewCleaner(nil, WithJiraReleaseService(fakeReleaseService{
		listIssueKeysFn: func(_ context.Context, group, releaseID string) ([]string, error) {
			if group != "TASKS" || releaseID != "42" {
				t.Fatalf("unexpected release request: group=%s releaseID=%s", group, releaseID)
			}
			return []string{"OPS-1", "OPS-2"}, nil
		},
	}))

	branches := []model.BranchInfo{
		{Name: "OPS-1", Key: "refs/heads/OPS-1", Scope: model.BranchScopeLocal, Protected: false},
		{Name: "OPS-2", Key: "refs/heads/OPS-2", Scope: model.BranchScopeLocal, Protected: true},
		{Name: "OPS-3", Key: "refs/heads/OPS-3", Scope: model.BranchScopeLocal, Protected: false},
		{Name: "misc/no-jira", Key: "refs/heads/misc/no-jira", Scope: model.BranchScopeLocal, Protected: false},
	}

	summary, candidates, err := cleaner.BuildReleaseAutocheckCandidates(t.Context(), repo, branches, "TASKS", "42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.IssueKeysTotal != 2 {
		t.Fatalf("expected 2 issues, got %d", summary.IssueKeysTotal)
	}
	if summary.BranchMatches != 2 {
		t.Fatalf("expected 2 branch matches, got %d", summary.BranchMatches)
	}
	if summary.BranchSkippedProtect != 1 {
		t.Fatalf("expected 1 protected skip, got %d", summary.BranchSkippedProtect)
	}
	if len(candidates) != 1 || candidates[0].Name != "OPS-1" {
		t.Fatalf("expected single candidate OPS-1, got %#v", candidates)
	}
}

func TestBuildReleaseAutocheckCandidatesPropagatesReleaseError(t *testing.T) {
	t.Parallel()

	v := viper.New()
	v.Set("jira", []map[string]any{{"group": "TASKS", "url": "https://tasks.example.org"}})
	v.Set("repos", []map[string]any{{
		"name": "demo",
		"path": "/tmp/demo",
		"branch": map[string]any{
			"jira": []string{`^(?P<TASKS>OPS-\d+)$`},
		},
	}})

	cfg, err := config.LoadFromViper(v)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	repo := cfg.Repos[0]

	cleaner := NewCleaner(nil, WithJiraReleaseService(fakeReleaseService{
		listIssueKeysFn: func(_ context.Context, _, _ string) ([]string, error) {
			return nil, errors.New("jira unavailable")
		},
	}))

	_, _, err = cleaner.BuildReleaseAutocheckCandidates(t.Context(), repo, []model.BranchInfo{{Name: "OPS-1", Scope: model.BranchScopeLocal}}, "TASKS", "42")
	if err == nil {
		t.Fatal("expected error")
	}
}
