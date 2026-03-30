package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

func TestResolveRepoPathAcceptsGitWorktree(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary is required")
	}

	dir := t.TempDir()
	mainRepoPath := filepath.Join(dir, "repo")
	worktreePath := filepath.Join(dir, "repo-worktree")

	runCmd(t, dir, "git", "init", mainRepoPath)
	runCmd(t, mainRepoPath, "git", "config", "user.email", "test@example.com")
	runCmd(t, mainRepoPath, "git", "config", "user.name", "tester")
	writeFile(t, filepath.Join(mainRepoPath, "README.md"), "init\n")
	runCmd(t, mainRepoPath, "git", "add", "README.md")
	runCmd(t, mainRepoPath, "git", "commit", "-m", "init")
	runCmd(t, mainRepoPath, "git", "checkout", "-b", "feature/worktree")
	runCmd(t, mainRepoPath, "git", "checkout", "master")
	runCmd(t, mainRepoPath, "git", "worktree", "add", worktreePath, "feature/worktree")

	client := NewClient(5*time.Second, filepath.Join(dir, "workspace"))
	resolved, err := client.ResolveRepoPath("local", "", worktreePath)
	if err != nil {
		t.Fatalf("resolve worktree repo path: %v", err)
	}

	absWorktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		t.Fatalf("resolve absolute worktree path: %v", err)
	}
	if resolved != absWorktreePath {
		t.Fatalf("expected resolved path %s, got %s", absWorktreePath, resolved)
	}
}

func TestListBranchesIncludesRemoteBranchesWithUniqueKeys(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary is required")
	}

	dir := t.TempDir()
	remotePath := filepath.Join(dir, "remote.git")
	seedPath := filepath.Join(dir, "seed")
	clonePath := filepath.Join(dir, "clone")

	runCmd(t, dir, "git", "init", "--bare", remotePath)
	runCmd(t, dir, "git", "clone", remotePath, seedPath)
	runCmd(t, seedPath, "git", "config", "user.email", "test@example.com")
	runCmd(t, seedPath, "git", "config", "user.name", "tester")

	runCmd(t, seedPath, "git", "checkout", "-b", "main")
	writeFile(t, filepath.Join(seedPath, "README.md"), "init\n")
	runCmd(t, seedPath, "git", "add", "README.md")
	runCmd(t, seedPath, "git", "commit", "-m", "init")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "main")
	runCmd(t, dir, "git", "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")

	runCmd(t, seedPath, "git", "checkout", "-b", "feature/only-remote")
	writeFile(t, filepath.Join(seedPath, "feature.txt"), "feature\n")
	runCmd(t, seedPath, "git", "add", "feature.txt")
	runCmd(t, seedPath, "git", "commit", "-m", "feature")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "feature/only-remote")

	runCmd(t, seedPath, "git", "checkout", "main")
	runCmd(t, seedPath, "git", "branch", "-D", "feature/only-remote")

	runCmd(t, dir, "git", "clone", remotePath, clonePath)
	runCmd(t, clonePath, "git", "fetch", "--prune", "origin")

	client := NewClient(5*time.Second, filepath.Join(dir, "workspace"))
	branches, err := client.ListBranches(clonePath)
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}

	var hasLocalMain bool
	var hasRemoteFeature bool
	keys := make(map[string]struct{}, len(branches))
	for _, br := range branches {
		if _, exists := keys[br.Key]; exists {
			t.Fatalf("duplicate branch key detected: %s", br.Key)
		}
		keys[br.Key] = struct{}{}

		if br.Scope == model.BranchScopeLocal && br.Name == "main" {
			hasLocalMain = true
		}
		if br.Scope == model.BranchScopeRemote && br.Name == "feature/only-remote" {
			hasRemoteFeature = true
			if br.RemoteName != "origin" {
				t.Fatalf("expected remote name origin, got %q", br.RemoteName)
			}
			if !strings.HasPrefix(br.FullRef, "refs/remotes/") {
				t.Fatalf("expected full remote ref, got %q", br.FullRef)
			}
		}
	}

	if !hasLocalMain {
		t.Fatal("expected local main branch in list")
	}
	if !hasRemoteFeature {
		t.Fatal("expected remote feature branch in list")
	}
}

func TestCreateTrackingBranchAndCheckoutFromRemote(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary is required")
	}

	dir := t.TempDir()
	remotePath := filepath.Join(dir, "remote.git")
	seedPath := filepath.Join(dir, "seed")
	clonePath := filepath.Join(dir, "clone")

	runCmd(t, dir, "git", "init", "--bare", remotePath)
	runCmd(t, dir, "git", "clone", remotePath, seedPath)
	runCmd(t, seedPath, "git", "config", "user.email", "test@example.com")
	runCmd(t, seedPath, "git", "config", "user.name", "tester")

	runCmd(t, seedPath, "git", "checkout", "-b", "main")
	writeFile(t, filepath.Join(seedPath, "README.md"), "init\n")
	runCmd(t, seedPath, "git", "add", "README.md")
	runCmd(t, seedPath, "git", "commit", "-m", "init")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "main")
	runCmd(t, dir, "git", "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")

	runCmd(t, seedPath, "git", "checkout", "-b", "feature/tracking")
	writeFile(t, filepath.Join(seedPath, "tracking.txt"), "tracking\n")
	runCmd(t, seedPath, "git", "add", "tracking.txt")
	runCmd(t, seedPath, "git", "commit", "-m", "tracking")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "feature/tracking")

	runCmd(t, dir, "git", "clone", remotePath, clonePath)
	runCmd(t, clonePath, "git", "fetch", "--prune", "origin")

	client := NewClient(5*time.Second, filepath.Join(dir, "workspace"))
	err := client.CreateTrackingBranchAndCheckout(clonePath, "feature/tracking", "origin/feature/tracking")
	if err != nil {
		t.Fatalf("create tracking branch: %v", err)
	}

	out, err := exec.Command("git", "-C", clonePath, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("resolve HEAD after tracking branch create: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "feature/tracking" {
		t.Fatalf("expected HEAD to be feature/tracking, got %q", strings.TrimSpace(string(out)))
	}
}

func TestUpdateOpensourceRepoClonesWhenPathMissing(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary is required")
	}

	dir := t.TempDir()
	remotePath := filepath.Join(dir, "remote.git")
	seedPath := filepath.Join(dir, "seed")
	targetPath := filepath.Join(dir, "opensource")

	runCmd(t, dir, "git", "init", "--bare", remotePath)
	runCmd(t, dir, "git", "clone", remotePath, seedPath)
	runCmd(t, seedPath, "git", "config", "user.email", "test@example.com")
	runCmd(t, seedPath, "git", "config", "user.name", "tester")
	runCmd(t, seedPath, "git", "checkout", "-b", "main")
	writeFile(t, filepath.Join(seedPath, "README.md"), "init\n")
	runCmd(t, seedPath, "git", "add", "README.md")
	runCmd(t, seedPath, "git", "commit", "-m", "init")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "main")
	runCmd(t, dir, "git", "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")

	client := NewClient(5*time.Second, filepath.Join(dir, "workspace"))
	if err := client.UpdateOpensourceRepo(remotePath, targetPath, ""); err != nil {
		t.Fatalf("update opensource repo: %v", err)
	}

	if !isGitRepo(targetPath) {
		t.Fatalf("expected git repository at %s after update", targetPath)
	}
}

func TestUpdateOpensourceRepoFetchesExistingRepoWithoutResetWhenAutoswitchEmpty(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary is required")
	}

	dir := t.TempDir()
	remotePath := filepath.Join(dir, "remote.git")
	seedPath := filepath.Join(dir, "seed")
	targetPath := filepath.Join(dir, "opensource")

	runCmd(t, dir, "git", "init", "--bare", remotePath)
	runCmd(t, dir, "git", "clone", remotePath, seedPath)
	runCmd(t, seedPath, "git", "config", "user.email", "test@example.com")
	runCmd(t, seedPath, "git", "config", "user.name", "tester")
	runCmd(t, seedPath, "git", "checkout", "-b", "main")
	writeFile(t, filepath.Join(seedPath, "README.md"), "init\n")
	runCmd(t, seedPath, "git", "add", "README.md")
	runCmd(t, seedPath, "git", "commit", "-m", "init")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "main")
	runCmd(t, dir, "git", "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")

	runCmd(t, dir, "git", "clone", remotePath, targetPath)
	writeFile(t, filepath.Join(targetPath, "README.md"), "dirty\n")

	runCmd(t, seedPath, "git", "checkout", "-b", "feature/new-remote")
	writeFile(t, filepath.Join(seedPath, "feature.txt"), "feature\n")
	runCmd(t, seedPath, "git", "add", "feature.txt")
	runCmd(t, seedPath, "git", "commit", "-m", "feature")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "feature/new-remote")

	client := NewClient(5*time.Second, filepath.Join(dir, "workspace"))
	if err := client.UpdateOpensourceRepo(remotePath, targetPath, ""); err != nil {
		t.Fatalf("update opensource repo: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(targetPath, "README.md"))
	if err != nil {
		t.Fatalf("read working tree file: %v", err)
	}
	if strings.TrimSpace(string(content)) != "dirty" {
		t.Fatalf("expected working tree changes to be preserved, got %q", strings.TrimSpace(string(content)))
	}

	if err := exec.Command("git", "-C", targetPath, "show-ref", "--verify", "--quiet", "refs/remotes/origin/feature/new-remote").Run(); err != nil {
		t.Fatalf("expected new remote branch to be fetched: %v", err)
	}
}

func TestUpdateOpensourceRepoAutoswitchResetsAndChecksOutBranch(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary is required")
	}

	dir := t.TempDir()
	remotePath := filepath.Join(dir, "remote.git")
	seedPath := filepath.Join(dir, "seed")
	targetPath := filepath.Join(dir, "opensource")

	runCmd(t, dir, "git", "init", "--bare", remotePath)
	runCmd(t, dir, "git", "clone", remotePath, seedPath)
	runCmd(t, seedPath, "git", "config", "user.email", "test@example.com")
	runCmd(t, seedPath, "git", "config", "user.name", "tester")
	runCmd(t, seedPath, "git", "checkout", "-b", "main")
	writeFile(t, filepath.Join(seedPath, "README.md"), "init\n")
	runCmd(t, seedPath, "git", "add", "README.md")
	runCmd(t, seedPath, "git", "commit", "-m", "init")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "main")
	runCmd(t, dir, "git", "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")

	runCmd(t, seedPath, "git", "checkout", "-b", "develop")
	writeFile(t, filepath.Join(seedPath, "develop.txt"), "develop\n")
	runCmd(t, seedPath, "git", "add", "develop.txt")
	runCmd(t, seedPath, "git", "commit", "-m", "develop")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "develop")

	runCmd(t, dir, "git", "clone", remotePath, targetPath)
	writeFile(t, filepath.Join(targetPath, "README.md"), "dirty\n")

	client := NewClient(5*time.Second, filepath.Join(dir, "workspace"))
	if err := client.UpdateOpensourceRepo(remotePath, targetPath, "develop"); err != nil {
		t.Fatalf("update opensource repo with autoswitch: %v", err)
	}

	branchOut, err := exec.Command("git", "-C", targetPath, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("resolve current branch: %v (%s)", err, string(branchOut))
	}
	if strings.TrimSpace(string(branchOut)) != "develop" {
		t.Fatalf("expected current branch develop, got %q", strings.TrimSpace(string(branchOut)))
	}

	statusOut, err := exec.Command("git", "-C", targetPath, "status", "--porcelain").CombinedOutput()
	if err != nil {
		t.Fatalf("read git status: %v (%s)", err, string(statusOut))
	}
	if strings.TrimSpace(string(statusOut)) != "" {
		t.Fatalf("expected clean working tree after autoswitch, got %q", strings.TrimSpace(string(statusOut)))
	}
}

func TestFetchAndPullFastForwardUpdatesCurrentBranch(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary is required")
	}

	dir := t.TempDir()
	remotePath := filepath.Join(dir, "remote.git")
	seedPath := filepath.Join(dir, "seed")
	clonePath := filepath.Join(dir, "clone")

	runCmd(t, dir, "git", "init", "--bare", remotePath)
	runCmd(t, dir, "git", "clone", remotePath, seedPath)
	runCmd(t, seedPath, "git", "config", "user.email", "test@example.com")
	runCmd(t, seedPath, "git", "config", "user.name", "tester")

	runCmd(t, seedPath, "git", "checkout", "-b", "main")
	writeFile(t, filepath.Join(seedPath, "README.md"), "v1\n")
	runCmd(t, seedPath, "git", "add", "README.md")
	runCmd(t, seedPath, "git", "commit", "-m", "init")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "main")
	runCmd(t, dir, "git", "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")

	runCmd(t, dir, "git", "clone", remotePath, clonePath)

	writeFile(t, filepath.Join(seedPath, "README.md"), "v2\n")
	runCmd(t, seedPath, "git", "add", "README.md")
	runCmd(t, seedPath, "git", "commit", "-m", "update")
	runCmd(t, seedPath, "git", "push", "origin", "main")

	client := NewClient(5*time.Second, filepath.Join(dir, "workspace"))
	if err := client.FetchAndPull(clonePath, remotePath); err != nil {
		t.Fatalf("fetch and pull: %v", err)
	}

	out, err := exec.Command("git", "-C", clonePath, "show", "-s", "--format=%s", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("read head commit message: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "update" {
		t.Fatalf("expected pulled commit message 'update', got %q", strings.TrimSpace(string(out)))
	}
}

func TestFetchAndPullFailsWithoutUpstream(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary is required")
	}

	dir := t.TempDir()
	remotePath := filepath.Join(dir, "remote.git")
	seedPath := filepath.Join(dir, "seed")
	repoPath := filepath.Join(dir, "repo")

	runCmd(t, dir, "git", "init", "--bare", remotePath)
	runCmd(t, dir, "git", "clone", remotePath, seedPath)
	runCmd(t, seedPath, "git", "config", "user.email", "test@example.com")
	runCmd(t, seedPath, "git", "config", "user.name", "tester")
	runCmd(t, seedPath, "git", "checkout", "-b", "main")
	writeFile(t, filepath.Join(seedPath, "README.md"), "init\n")
	runCmd(t, seedPath, "git", "add", "README.md")
	runCmd(t, seedPath, "git", "commit", "-m", "init")
	runCmd(t, seedPath, "git", "push", "-u", "origin", "main")
	runCmd(t, dir, "git", "--git-dir", remotePath, "symbolic-ref", "HEAD", "refs/heads/main")

	runCmd(t, dir, "git", "clone", remotePath, repoPath)
	runCmd(t, repoPath, "git", "checkout", "-b", "feature/no-upstream")

	client := NewClient(5*time.Second, filepath.Join(dir, "workspace"))
	err := client.FetchAndPull(repoPath, "")
	if err == nil {
		t.Fatal("expected error for branch without upstream")
	}
	if !strings.Contains(err.Error(), "не настроен upstream") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func runCmd(t *testing.T, workdir string, command string, args ...string) {
	t.Helper()
	cmd := exec.Command(command, args...)
	cmd.Dir = workdir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command failed: %s %v\n%s\n%v", command, args, string(out), err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}
