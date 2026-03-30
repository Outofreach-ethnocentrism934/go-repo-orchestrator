package usecase

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"
	"github.com/agelxnash/go-repo-orchestrator/internal/jira"
	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

type fakeGitClient struct {
	resolveRepoPathFn     func(repoName, repoURL, localPath string) (string, error)
	managedRepoPathFn     func(repoName, repoURL string) string
	ensureManagedCloneFn  func(repoName, repoURL string) (string, error)
	fetchAndPullFn        func(repoPath, repoURL string) error
	detectDefaultBranchFn func(repoPath, currentBranch string) (string, error)
	listBranchesFn        func(repoPath string) ([]model.BranchInfo, error)
	currentBranchFn       func(repoPath string) (string, error)
	branchMetadataFn      func(repoPath, branch, defaultBranch string) (model.MergeStatus, string, error)
	getDirtyStatsFn       func(repoPath string) (model.DirtyStats, error)
	getRepoStatFn         func(repoPath string) (model.RepoStat, error)
	syncRemoteFn          func(repoPath, repoURL string) error
	updateOpensourceFn    func(url, targetPath, branch string) error
	createTrackingFn      func(repoPath, localBranch, remoteBranch string) error
}

type fakeStatusResolver struct {
	resolveFn  func(group, ticketURL, jiraBaseURL, key string) jira.StatusResult
	prefetchFn func(requests []jira.StatusBatchRequest)
}

func (f fakeStatusResolver) ResolveStatus(group, ticketURL, jiraBaseURL, key string) jira.StatusResult {
	if f.resolveFn != nil {
		return f.resolveFn(group, ticketURL, jiraBaseURL, key)
	}

	return jira.StatusResult{Status: "-", State: jira.StatusStateUnmapped, Reason: jira.StatusReasonNoMapping}
}

func (f fakeStatusResolver) PrefetchStatuses(requests []jira.StatusBatchRequest) {
	if f.prefetchFn != nil {
		f.prefetchFn(requests)
	}
}

func (f *fakeGitClient) ResolveRepoPath(repoName, repoURL, localPath string) (string, error) {
	if f.resolveRepoPathFn != nil {
		return f.resolveRepoPathFn(repoName, repoURL, localPath)
	}
	return "", nil
}

func (f *fakeGitClient) ManagedRepoPath(repoName, repoURL string) string {
	if f.managedRepoPathFn != nil {
		return f.managedRepoPathFn(repoName, repoURL)
	}
	return ""
}

func (f *fakeGitClient) EnsureManagedClone(repoName, repoURL string) (string, error) {
	if f.ensureManagedCloneFn != nil {
		return f.ensureManagedCloneFn(repoName, repoURL)
	}
	return "", nil
}

func (f *fakeGitClient) FetchAndPull(repoPath, repoURL string) error {
	if f.fetchAndPullFn != nil {
		return f.fetchAndPullFn(repoPath, repoURL)
	}
	return nil
}

func (f *fakeGitClient) DetectDefaultBranch(repoPath, currentBranch string) (string, error) {
	if f.detectDefaultBranchFn != nil {
		return f.detectDefaultBranchFn(repoPath, currentBranch)
	}
	return "", nil
}

func (f *fakeGitClient) ListBranches(repoPath string) ([]model.BranchInfo, error) {
	if f.listBranchesFn != nil {
		return f.listBranchesFn(repoPath)
	}
	return nil, nil
}

func (f *fakeGitClient) CurrentBranch(repoPath string) (string, error) {
	if f.currentBranchFn != nil {
		return f.currentBranchFn(repoPath)
	}
	return "", nil
}

func (f *fakeGitClient) DeleteLocalBranch(repoPath, branch string) error {
	return nil
}

func (f *fakeGitClient) BranchMetadata(repoPath, branch, defaultBranch string) (model.MergeStatus, string, error) {
	if f.branchMetadataFn != nil {
		return f.branchMetadataFn(repoPath, branch, defaultBranch)
	}
	return model.MergeStatusUnknown, "-", nil
}

func (f *fakeGitClient) GetDirtyStats(repoPath string) (model.DirtyStats, error) {
	if f.getDirtyStatsFn != nil {
		return f.getDirtyStatsFn(repoPath)
	}
	return model.DirtyStats{}, nil
}

func (f *fakeGitClient) GetRepoStat(repoPath string) (model.RepoStat, error) {
	if f.getRepoStatFn != nil {
		return f.getRepoStatFn(repoPath)
	}
	return model.RepoStat{}, nil
}

func (f *fakeGitClient) SyncRemote(repoPath, repoURL string) error {
	if f.syncRemoteFn != nil {
		return f.syncRemoteFn(repoPath, repoURL)
	}
	return nil
}

func (f *fakeGitClient) UpdateOpensourceRepo(url, targetPath, branch string) error {
	if f.updateOpensourceFn != nil {
		return f.updateOpensourceFn(url, targetPath, branch)
	}
	return nil
}

func (f *fakeGitClient) ForceCheckout(repoPath, branch string) error {
	return nil
}

func (f *fakeGitClient) CreateTrackingBranchAndCheckout(repoPath, localBranch, remoteBranch string) error {
	if f.createTrackingFn != nil {
		return f.createTrackingFn(repoPath, localBranch, remoteBranch)
	}
	return nil
}

func TestLoadRepoBranchesOpensourceClonesMissingPathViaUpdateFlow(t *testing.T) {
	t.Parallel()

	updateCalled := false
	resolveCalled := false

	git := &fakeGitClient{
		updateOpensourceFn: func(url, targetPath, branch string) error {
			updateCalled = true
			if url != "ssh://git.example/repo.git" {
				t.Fatalf("unexpected opensource url: %q", url)
			}
			if targetPath != "/tmp/bff" {
				t.Fatalf("unexpected opensource path: %q", targetPath)
			}
			if branch != "develop" {
				t.Fatalf("unexpected autoswitch branch: %q", branch)
			}
			return nil
		},
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			resolveCalled = true
			if repoURL != "" {
				t.Fatalf("unexpected repoURL in ResolveRepoPath: %q", repoURL)
			}
			return localPath, nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "feature/local-bff",
				QualifiedName: "feature/local-bff",
				Scope:         model.BranchScopeLocal,
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	cleaner := NewCleaner(git)
	repo := config.RepoConfig{
		Name: "bff",
		URL:  "ssh://git.example/repo.git",
		Path: "/tmp/bff",
		Branch: config.Branch{
			Autoswitch: "develop",
		},
	}

	rb, err := cleaner.LoadRepoBranches(repo)
	if err != nil {
		t.Fatalf("expected opensource update success, got error: %v", err)
	}
	if !updateCalled {
		t.Fatal("expected opensource update to be called")
	}
	if !resolveCalled {
		t.Fatal("expected path resolution after opensource update")
	}
	if rb.RepoPath != repo.Path {
		t.Fatalf("expected local repo path %q, got %q", repo.Path, rb.RepoPath)
	}
	if len(rb.Branches) != 1 {
		t.Fatalf("expected local branches to be loaded, got %d", len(rb.Branches))
	}
	if rb.Branches[0].Name != "feature/local-bff" {
		t.Fatalf("unexpected branch loaded: %q", rb.Branches[0].Name)
	}
	if rb.SyncWarning != "" {
		t.Fatalf("expected no sync warning after successful opensource update, got %q", rb.SyncWarning)
	}
	if rb.DefaultBranch != "main" {
		t.Fatalf("expected default branch main, got %q", rb.DefaultBranch)
	}
}

func TestLoadRepoBranchesOpensourceKeepsLocalDataAndReturnsSyncWarning(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		updateOpensourceFn: func(url, targetPath, branch string) error {
			return errors.New("connection refused")
		},
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			if repoURL != "" {
				t.Fatalf("unexpected repoURL in ResolveRepoPath: %q", repoURL)
			}
			return localPath, nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "feature/local-bff",
				QualifiedName: "feature/local-bff",
				Scope:         model.BranchScopeLocal,
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	cleaner := NewCleaner(git)
	repo := config.RepoConfig{Name: "bff", URL: "ssh://git.example/repo.git", Path: "/tmp/bff"}

	rb, err := cleaner.LoadRepoBranches(repo)
	if err != nil {
		t.Fatalf("expected local fallback with warning, got error: %v", err)
	}
	if rb.RepoPath != repo.Path {
		t.Fatalf("expected local repo path %q, got %q", repo.Path, rb.RepoPath)
	}
	if len(rb.Branches) != 1 {
		t.Fatalf("expected local branches to be loaded, got %d", len(rb.Branches))
	}
	if rb.Branches[0].Name != "feature/local-bff" {
		t.Fatalf("unexpected branch loaded: %q", rb.Branches[0].Name)
	}
	if rb.SyncWarning == "" {
		t.Fatal("expected sync warning for opensource repo")
	}
	if !strings.Contains(rb.SyncWarning, "синхронизация remote не выполнена") {
		t.Fatalf("unexpected warning: %q", rb.SyncWarning)
	}
}

func TestLoadRepoBranchesOpensourceReturnsErrorWhenUpdateFailsAndNoLocalRepo(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		updateOpensourceFn: func(url, targetPath, branch string) error {
			return errors.New("connection refused")
		},
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "", errors.New("local path is not a git repository")
		},
	}

	cleaner := NewCleaner(git)
	repo := config.RepoConfig{Name: "bff", URL: "ssh://git.example/repo.git", Path: "/tmp/missing"}

	_, err := cleaner.LoadRepoBranches(repo)
	if err == nil {
		t.Fatal("expected hard error when opensource update fails and fallback repo is unavailable")
	}
	if !strings.Contains(err.Error(), "prepare repository") {
		t.Fatalf("expected prepare repository error wrapper, got: %v", err)
	}
}

func TestLoadRepoStatURLFallsBackToManagedCacheOnSyncFailure(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		ensureManagedCloneFn: func(repoName, repoURL string) (string, error) {
			return "", errors.New("no route to host")
		},
		managedRepoPathFn: func(repoName, repoURL string) string {
			return "/tmp/state/workspace/repo"
		},
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			if localPath == "" {
				t.Fatal("expected fallback local path")
			}
			return localPath, nil
		},
		getRepoStatFn: func(repoPath string) (model.RepoStat, error) {
			return model.RepoStat{CurrentBranch: "main", Loaded: true}, nil
		},
	}

	cleaner := NewCleaner(git)
	repo := config.RepoConfig{Name: "simplewine", URL: "ssh://git.example/simplewine.git"}

	stat, err := cleaner.LoadRepoStat(repo)
	if err != nil {
		t.Fatalf("expected local managed fallback, got error: %v", err)
	}
	if stat.SyncWarning == "" {
		t.Fatal("expected sync warning when managed sync fails but cache is available")
	}
	if !strings.Contains(stat.SyncWarning, "синхронизация remote не выполнена") {
		t.Fatalf("unexpected warning: %q", stat.SyncWarning)
	}
}

func TestLoadRepoStatURLReturnsHardErrorWhenNoLocalCache(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		ensureManagedCloneFn: func(repoName, repoURL string) (string, error) {
			return "", errors.New("connection refused")
		},
		managedRepoPathFn: func(repoName, repoURL string) string {
			return "/tmp/state/workspace/missing"
		},
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "", errors.New("local path is not a git repository")
		},
	}

	cleaner := NewCleaner(git)
	repo := config.RepoConfig{Name: "simplewine", URL: "ssh://git.example/simplewine.git"}

	_, err := cleaner.LoadRepoStat(repo)
	if err == nil {
		t.Fatal("expected hard error when both sync and local cache are unavailable")
	}
	if !strings.Contains(err.Error(), "prepare repository") {
		t.Fatalf("expected prepare repository error wrapper, got: %v", err)
	}
}

func TestFetchAndPullRepoPathSourceUsesLocalPath(t *testing.T) {
	t.Parallel()

	called := false
	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			if repoName != "demo" {
				t.Fatalf("unexpected repo name: %q", repoName)
			}
			if repoURL != "" {
				t.Fatalf("unexpected repoURL for path source: %q", repoURL)
			}
			if localPath != "/tmp/demo" {
				t.Fatalf("unexpected local path: %q", localPath)
			}
			return localPath, nil
		},
		fetchAndPullFn: func(repoPath, repoURL string) error {
			called = true
			if repoPath != "/tmp/demo" {
				t.Fatalf("unexpected repo path: %q", repoPath)
			}
			if repoURL != "" {
				t.Fatalf("unexpected repo URL for path source: %q", repoURL)
			}
			return nil
		},
	}

	cleaner := NewCleaner(git)
	err := cleaner.FetchAndPullRepo(config.RepoConfig{Name: "demo", Path: "/tmp/demo"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !called {
		t.Fatal("expected fetch+pull call")
	}
}

func TestFetchAndPullRepoURLSourceUsesManagedClone(t *testing.T) {
	t.Parallel()

	called := false
	git := &fakeGitClient{
		ensureManagedCloneFn: func(repoName, repoURL string) (string, error) {
			if repoName != "demo" {
				t.Fatalf("unexpected repo name: %q", repoName)
			}
			if repoURL != "ssh://git.example/demo.git" {
				t.Fatalf("unexpected repo URL: %q", repoURL)
			}
			return "/tmp/state/workspace/demo", nil
		},
		fetchAndPullFn: func(repoPath, repoURL string) error {
			called = true
			if repoPath != "/tmp/state/workspace/demo" {
				t.Fatalf("unexpected repo path: %q", repoPath)
			}
			if repoURL != "ssh://git.example/demo.git" {
				t.Fatalf("unexpected repo URL: %q", repoURL)
			}
			return nil
		},
	}

	cleaner := NewCleaner(git)
	err := cleaner.FetchAndPullRepo(config.RepoConfig{Name: "demo", URL: "ssh://git.example/demo.git"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !called {
		t.Fatal("expected fetch+pull call")
	}
}

func TestCreateLocalTrackingBranchResolvesRepoAndCallsGit(t *testing.T) {
	t.Parallel()

	called := false
	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			if repoName != "demo" {
				t.Fatalf("unexpected repo name: %q", repoName)
			}
			return "/tmp/demo", nil
		},
		createTrackingFn: func(repoPath, localBranch, remoteBranch string) error {
			called = true
			if repoPath != "/tmp/demo" {
				t.Fatalf("unexpected repoPath: %q", repoPath)
			}
			if localBranch != "feature/new" {
				t.Fatalf("unexpected localBranch: %q", localBranch)
			}
			if remoteBranch != "origin/feature/new" {
				t.Fatalf("unexpected remoteBranch: %q", remoteBranch)
			}
			return nil
		},
	}

	cleaner := NewCleaner(git)
	err := cleaner.CreateLocalTrackingBranch(config.RepoConfig{Name: "demo", Path: "/tmp/demo"}, "feature/new", "origin/feature/new")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !called {
		t.Fatal("expected tracking branch creation call")
	}
}

func TestLoadRepoBranchesProtectsRemoteBranchWithUnknownRemoteName(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "/tmp/demo", nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "feature/remote",
				Scope:         model.BranchScopeRemote,
				QualifiedName: "feature/remote",
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	cleaner := NewCleaner(git)
	rb, err := cleaner.LoadRepoBranches(config.RepoConfig{Name: "demo", Path: "/tmp/demo"})
	if err != nil {
		t.Fatalf("load repo branches: %v", err)
	}
	if len(rb.Branches) != 1 {
		t.Fatalf("expected one branch, got %d", len(rb.Branches))
	}
	if !rb.Branches[0].Protected {
		t.Fatal("expected branch to be protected")
	}
	if rb.Branches[0].Reason != "remote name unknown" {
		t.Fatalf("unexpected protection reason: %q", rb.Branches[0].Reason)
	}
}

func TestLoadRepoBranchesSetsJiraLinkFieldsFromNamedGroup(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "/tmp/demo", nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "feature/OPS-101",
				QualifiedName: "feature/OPS-101",
				Scope:         model.BranchScopeLocal,
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	v := viper.New()
	v.Set("jira", []map[string]any{{
		"group":      "TASKS",
		"url":        "https://tasks.example.org",
		"playwright": false,
	}})
	v.Set("repos", []map[string]any{{
		"name": "demo",
		"path": "/tmp/demo",
		"branch": map[string]any{
			"jira": []string{"^feature/(?P<TASKS>[A-Z]+-\\d+)$"},
		},
	}})

	cfg, err := config.LoadFromViper(v)
	if err != nil {
		t.Fatalf("load config from viper: %v", err)
	}

	cleaner := NewCleaner(git)
	rb, err := cleaner.LoadRepoBranches(cfg.Repos[0])
	if err != nil {
		t.Fatalf("load repo branches: %v", err)
	}
	if len(rb.Branches) != 1 {
		t.Fatalf("expected one branch, got %d", len(rb.Branches))
	}

	branch := rb.Branches[0]
	if branch.JiraKey != "OPS-101" {
		t.Fatalf("expected jira key OPS-101, got %q", branch.JiraKey)
	}
	if branch.JiraGroup != "TASKS" {
		t.Fatalf("expected jira group TASKS, got %q", branch.JiraGroup)
	}
	if branch.JiraURL != "https://tasks.example.org" {
		t.Fatalf("expected jira url, got %q", branch.JiraURL)
	}
	if branch.JiraTicketURL != "https://tasks.example.org/browse/OPS-101" {
		t.Fatalf("unexpected jira ticket url: %q", branch.JiraTicketURL)
	}
}

func TestLoadRepoBranchesResolvesJiraStatusForMappedTicket(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "/tmp/demo", nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "feature/OPS-500",
				QualifiedName: "feature/OPS-500",
				Scope:         model.BranchScopeLocal,
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	v := viper.New()
	v.Set("jira", []map[string]any{{
		"group": "TASKS",
		"url":   "https://tasks.example.org",
	}})
	v.Set("repos", []map[string]any{{
		"name": "demo",
		"path": "/tmp/demo",
		"branch": map[string]any{
			"jira": []string{"^feature/(?P<TASKS>[A-Z]+-\\d+)$"},
		},
	}})

	cfg, err := config.LoadFromViper(v)
	if err != nil {
		t.Fatalf("load config from viper: %v", err)
	}

	cleaner := NewCleaner(git, WithJiraStatusResolver(fakeStatusResolver{resolveFn: func(group, ticketURL, jiraBaseURL, key string) jira.StatusResult {
		if group != "TASKS" {
			t.Fatalf("unexpected jira group: %q", group)
		}
		if ticketURL != "https://tasks.example.org/browse/OPS-500" {
			t.Fatalf("unexpected ticket url: %q", ticketURL)
		}
		if jiraBaseURL != "https://tasks.example.org" {
			t.Fatalf("unexpected jira base url: %q", jiraBaseURL)
		}
		if key != "OPS-500" {
			t.Fatalf("unexpected jira key: %q", key)
		}

		return jira.StatusResult{Status: "In Progress", State: jira.StatusStateReady, Reason: jira.StatusReasonNone}
	}}))

	rb, err := cleaner.LoadRepoBranches(cfg.Repos[0])
	if err != nil {
		t.Fatalf("load repo branches: %v", err)
	}
	if len(rb.Branches) != 1 {
		t.Fatalf("expected one branch, got %d", len(rb.Branches))
	}

	if rb.Branches[0].JiraStatus != "In Progress" {
		t.Fatalf("expected jira status to be resolved, got %q", rb.Branches[0].JiraStatus)
	}
	if rb.Branches[0].JiraState != model.JiraStatusStateReady {
		t.Fatalf("expected jira state ready, got %q", rb.Branches[0].JiraState)
	}
	if rb.Branches[0].JiraReason != model.JiraStatusReasonNone {
		t.Fatalf("expected jira reason none, got %q", rb.Branches[0].JiraReason)
	}
}

func TestLoadRepoBranchesMapsJiraAuthState(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "/tmp/demo", nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "feature/OPS-777",
				QualifiedName: "feature/OPS-777",
				Scope:         model.BranchScopeLocal,
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	v := viper.New()
	v.Set("jira", []map[string]any{{
		"group": "TASKS",
		"url":   "https://tasks.example.org",
	}})
	v.Set("repos", []map[string]any{{
		"name": "demo",
		"path": "/tmp/demo",
		"branch": map[string]any{
			"jira": []string{"^feature/(?P<TASKS>[A-Z]+-\\d+)$"},
		},
	}})

	cfg, err := config.LoadFromViper(v)
	if err != nil {
		t.Fatalf("load config from viper: %v", err)
	}

	cleaner := NewCleaner(git, WithJiraStatusResolver(fakeStatusResolver{resolveFn: func(group, ticketURL, jiraBaseURL, key string) jira.StatusResult {
		return jira.StatusResult{Status: "-", State: jira.StatusStateAuth, Reason: jira.StatusReasonAuthRequired}
	}}))

	rb, err := cleaner.LoadRepoBranches(cfg.Repos[0])
	if err != nil {
		t.Fatalf("load repo branches: %v", err)
	}
	if rb.Branches[0].JiraState != model.JiraStatusStateAuth {
		t.Fatalf("expected jira state auth, got %q", rb.Branches[0].JiraState)
	}
	if rb.Branches[0].JiraReason != model.JiraStatusReasonAuthRequired {
		t.Fatalf("expected jira reason auth_required, got %q", rb.Branches[0].JiraReason)
	}
}

func TestLoadRepoBranchesPrefetchesJiraStatusesBeforeResolve(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "/tmp/demo", nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{
				{Name: "OPS-101", QualifiedName: "OPS-101", Scope: model.BranchScopeLocal},
				{Name: "IDEA-7", QualifiedName: "IDEA-7", Scope: model.BranchScopeLocal},
				{Name: "misc/no-jira", QualifiedName: "misc/no-jira", Scope: model.BranchScopeLocal},
			}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	v := viper.New()
	v.Set("jira", []map[string]any{
		{"group": "TASKS", "url": "https://tasks.example.org"},
		{"group": "IDEA", "url": "https://idea.example.org"},
	})
	v.Set("repos", []map[string]any{{
		"name": "demo",
		"path": "/tmp/demo",
		"branch": map[string]any{
			"jira": []string{`^(?P<TASKS>OPS-\d+)$`, `^(?P<IDEA>IDEA-\d+)$`},
		},
	}})

	cfg, err := config.LoadFromViper(v)
	if err != nil {
		t.Fatalf("load config from viper: %v", err)
	}

	prefetchCalls := 0
	prefetched := make([]jira.StatusBatchRequest, 0)
	cleaner := NewCleaner(git, WithJiraStatusResolver(fakeStatusResolver{
		prefetchFn: func(requests []jira.StatusBatchRequest) {
			prefetchCalls++
			prefetched = append(prefetched, requests...)
		},
		resolveFn: func(group, ticketURL, jiraBaseURL, key string) jira.StatusResult {
			return jira.StatusResult{Status: "Ready", State: jira.StatusStateReady, Reason: jira.StatusReasonNone}
		},
	}))

	_, err = cleaner.LoadRepoBranches(cfg.Repos[0])
	if err != nil {
		t.Fatalf("load repo branches: %v", err)
	}

	if prefetchCalls != 1 {
		t.Fatalf("expected single prefetch call, got %d", prefetchCalls)
	}
	if len(prefetched) != 2 {
		t.Fatalf("expected two mapped jira requests in prefetch, got %d", len(prefetched))
	}
}

func TestLoadRepoBranchesSetsNoGroupConfigReasonForNamedGroupWithoutTopLevelConfig(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "/tmp/demo", nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "BFF-1004",
				QualifiedName: "BFF-1004",
				Scope:         model.BranchScopeLocal,
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	v := viper.New()
	v.Set("repos", []map[string]any{{
		"name": "demo",
		"path": "/tmp/demo",
		"branch": map[string]any{
			"jira": []string{`^(?P<SIMPLEWINE>(WEB|MOBI|BFF)-\d+)$`},
		},
	}})

	cfg, err := config.LoadFromViper(v)
	if err != nil {
		t.Fatalf("load config from viper: %v", err)
	}

	cleaner := NewCleaner(git)
	rb, err := cleaner.LoadRepoBranches(cfg.Repos[0])
	if err != nil {
		t.Fatalf("load repo branches: %v", err)
	}

	if rb.Branches[0].JiraKey != "BFF-1004" {
		t.Fatalf("expected jira key BFF-1004, got %q", rb.Branches[0].JiraKey)
	}
	if rb.Branches[0].JiraReason != model.JiraStatusReasonNoGroupConfig {
		t.Fatalf("expected jira reason no_group_config, got %q", rb.Branches[0].JiraReason)
	}
}

func TestLoadRepoBranchesSetsRegexKeyOnlyReasonForFallbackJIRA(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "/tmp/demo", nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "feature/OPS-321",
				QualifiedName: "feature/OPS-321",
				Scope:         model.BranchScopeLocal,
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	v := viper.New()
	v.Set("repos", []map[string]any{{
		"name": "demo",
		"path": "/tmp/demo",
		"branch": map[string]any{
			"jira": []string{`^feature/(?P<JIRA>[A-Z]+-\d+)$`},
		},
	}})

	cfg, err := config.LoadFromViper(v)
	if err != nil {
		t.Fatalf("load config from viper: %v", err)
	}

	cleaner := NewCleaner(git)
	rb, err := cleaner.LoadRepoBranches(cfg.Repos[0])
	if err != nil {
		t.Fatalf("load repo branches: %v", err)
	}

	if rb.Branches[0].JiraKey != "OPS-321" {
		t.Fatalf("expected jira key OPS-321, got %q", rb.Branches[0].JiraKey)
	}
	if rb.Branches[0].JiraReason != model.JiraStatusReasonRegexKeyOnly {
		t.Fatalf("expected jira reason regex_key_only, got %q", rb.Branches[0].JiraReason)
	}
}

func TestLoadRepoBranchesProtectsRemoteBranchByKeepPattern(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "/tmp/demo", nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "release/1.0",
				Scope:         model.BranchScopeRemote,
				RemoteName:    "origin",
				QualifiedName: "origin/release/1.0",
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	v := viper.New()
	v.Set("repos", []map[string]any{{
		"name": "demo",
		"path": "/tmp/demo",
		"branch": map[string]any{
			"keep": []string{"^release/.*$"},
		},
	}})

	cfg, err := config.LoadFromViper(v)
	if err != nil {
		t.Fatalf("load config from viper: %v", err)
	}

	cleaner := NewCleaner(git)
	rb, err := cleaner.LoadRepoBranches(cfg.Repos[0])
	if err != nil {
		t.Fatalf("load repo branches: %v", err)
	}
	if len(rb.Branches) != 1 {
		t.Fatalf("expected one branch, got %d", len(rb.Branches))
	}
	if !rb.Branches[0].Protected {
		t.Fatal("expected branch to be protected")
	}
	if !strings.Contains(rb.Branches[0].Reason, "matched keep pattern") {
		t.Fatalf("unexpected protection reason: %q", rb.Branches[0].Reason)
	}
}

func TestLoadRepoBranchesRemoteWithFullRefIsSelectable(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "/tmp/demo", nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "feature/remote-ok",
				Scope:         model.BranchScopeRemote,
				RemoteName:    "origin",
				QualifiedName: "refs/remotes/origin/feature/remote-ok",
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "main", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	cleaner := NewCleaner(git)
	rb, err := cleaner.LoadRepoBranches(config.RepoConfig{Name: "demo", Path: "/tmp/demo"})
	if err != nil {
		t.Fatalf("load repo branches: %v", err)
	}
	if len(rb.Branches) != 1 {
		t.Fatalf("expected one branch, got %d", len(rb.Branches))
	}
	if rb.Branches[0].Protected {
		t.Fatalf("expected remote branch to be selectable, got protected reason: %q", rb.Branches[0].Reason)
	}
}

func TestLoadRepoBranchesProtectsAndSkipsRemoteDefaultBranch(t *testing.T) {
	t.Parallel()

	git := &fakeGitClient{
		resolveRepoPathFn: func(repoName, repoURL, localPath string) (string, error) {
			return "/tmp/demo", nil
		},
		listBranchesFn: func(repoPath string) ([]model.BranchInfo, error) {
			return []model.BranchInfo{{
				Name:          "main",
				Scope:         model.BranchScopeRemote,
				RemoteName:    "origin",
				QualifiedName: "origin/main",
			}}, nil
		},
		currentBranchFn: func(repoPath string) (string, error) {
			return "feature/local", nil
		},
		detectDefaultBranchFn: func(repoPath, currentBranch string) (string, error) {
			return "main", nil
		},
	}

	cleaner := NewCleaner(git)
	repo := config.RepoConfig{Name: "demo", Path: "/tmp/demo"}

	rb, err := cleaner.LoadRepoBranches(repo)
	if err != nil {
		t.Fatalf("load repo branches: %v", err)
	}
	if len(rb.Branches) != 1 {
		t.Fatalf("expected one branch, got %d", len(rb.Branches))
	}
	if !rb.Branches[0].Protected {
		t.Fatal("expected remote default branch to be protected")
	}
	if rb.Branches[0].Reason != "default branch" {
		t.Fatalf("unexpected protection reason: %q", rb.Branches[0].Reason)
	}

	_, err = cleaner.GenerateDeleteScript(repo, rb.RepoPath, rb.Branches, model.ScriptFormatSH)
	if err == nil {
		t.Fatal("expected script generation to skip protected remote default branch")
	}
	if !strings.Contains(err.Error(), "no eligible branches selected") {
		t.Fatalf("unexpected generate script error: %v", err)
	}
}
