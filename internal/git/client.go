package git

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

// Client выполняет git-операции: через go-git по умолчанию и git CLI для bundle/clone/fetch-сценариев.
type Client struct {
	timeout      time.Duration
	workspaceDir string

	lockMu sync.Mutex
	locks  map[string]*sync.Mutex
}

// ResolveRepoPath возвращает рабочий путь репозитория по настройкам источника.
func (c *Client) ResolveRepoPath(repoName, repoURL, localPath string) (string, error) {
	localPath = strings.TrimSpace(localPath)
	repoURL = strings.TrimSpace(repoURL)

	if localPath != "" {
		absPath, err := filepath.Abs(localPath)
		if err != nil {
			return "", fmt.Errorf("определение локального пути: %w", err)
		}
		if !isGitRepo(absPath) {
			return "", fmt.Errorf("локальный путь не является git-репозиторием: %s", absPath)
		}
		return absPath, nil
	}

	if repoURL == "" {
		return "", errors.New("для управляемого клона требуется адрес репозитория")
	}

	return c.EnsureManagedClone(repoName, repoURL)
}

// NewClient создает Git-клиент с таймаутом на каждую команду.
func NewClient(timeout time.Duration, workspaceDir string) *Client {
	return &Client{
		timeout:      timeout,
		workspaceDir: workspaceDir,
		locks:        make(map[string]*sync.Mutex),
	}
}

// EnsureManagedClone гарантирует наличие managed clone и актуализирует remote refs.
func (c *Client) EnsureManagedClone(repoName, repoURL string) (string, error) {
	if c.workspaceDir == "" {
		return "", errors.New("требуется рабочая директория")
	}

	managedPath := filepath.Join(c.workspaceDir, safeManagedRepoDir(repoName, repoURL))
	unlock := c.lockForPath(managedPath)
	defer unlock()

	if err := ensureDir(c.workspaceDir); err != nil {
		return "", err
	}

	if !isGitRepo(managedPath) {
		if err := c.cloneRepo(repoURL, managedPath); err != nil {
			return "", err
		}
	}

	if err := c.ensureOriginURL(managedPath, repoURL); err != nil {
		return "", err
	}

	if err := c.fetchPrune(managedPath); err != nil {
		return "", err
	}

	if err := c.seedLocalBranchesFromOrigin(managedPath); err != nil {
		return "", err
	}

	return managedPath, nil
}

// UpdateOpensourceRepo реализует логику обновления opensource-репозитория (аналог скрипта update-opensource.sh).
func (c *Client) UpdateOpensourceRepo(url, targetPath, branch string) error {
	targetPath = strings.TrimSpace(targetPath)
	branch = strings.TrimSpace(branch)
	if targetPath == "" {
		return errors.New("требуется целевой путь")
	}

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("определение целевого пути: %w", err)
	}

	unlock := c.lockForPath(absPath)
	defer unlock()

	if !isGitRepo(absPath) {
		if err := ensureDir(filepath.Dir(absPath)); err != nil {
			return err
		}
		if err := c.cloneRepo(url, absPath); err != nil {
			return err
		}
	} else {
		if err := c.ensureOriginURL(absPath, url); err != nil {
			return err
		}
	}

	if err := c.fetchPrune(absPath); err != nil {
		return err
	}

	if branch == "" {
		return nil
	}

	if _, err := c.runGit(absPath, "reset", "--hard", "HEAD", "--quiet"); err != nil {
		return fmt.Errorf("ошибка git reset: %w", err)
	}
	if _, err := c.runGit(absPath, "clean", "-fd", "--quiet"); err != nil {
		return fmt.Errorf("ошибка git clean: %w", err)
	}

	existsLocal, err := c.BranchExists(absPath, branch)
	if err != nil {
		return err
	}
	if existsLocal {
		if _, err := c.runGit(absPath, "checkout", "--quiet", branch); err != nil {
			return fmt.Errorf("ошибка checkout %q: %w", branch, err)
		}
	} else {
		if err := c.runGitStatusOnly(absPath, "show-ref", "--verify", "--quiet", "refs/tags/"+branch); err == nil {
			if _, err := c.runGit(absPath, "checkout", "--quiet", branch); err != nil {
				return fmt.Errorf("ошибка checkout тега %q: %w", branch, err)
			}
		} else {
			if _, err := c.runGit(absPath, "checkout", "-B", branch, "origin/"+branch, "--quiet"); err != nil {
				return fmt.Errorf("ошибка checkout -B %q origin/%q: %w", branch, branch, err)
			}
		}
	}

	if err := c.runGitStatusOnly(absPath, "show-ref", "--verify", "--quiet", "refs/remotes/origin/"+branch); err == nil {
		if _, resetErr := c.runGit(absPath, "reset", "--hard", "origin/"+branch, "--quiet"); resetErr != nil {
			return fmt.Errorf("ошибка reset к origin/%q: %w", branch, resetErr)
		}
	}

	return nil
}

// SyncRemote обновляет remote refs для локального репозитория без изменения рабочего дерева.
func (c *Client) SyncRemote(repoPath, repoURL string) error {
	unlock := c.lockForPath(repoPath)
	defer unlock()

	if err := c.ensureOriginURL(repoPath, repoURL); err != nil {
		return err
	}

	if err := c.fetchPrune(repoPath); err != nil {
		return err
	}

	return nil
}

// FetchAndPull выполняет безопасный fetch + pull для текущей ветки.
func (c *Client) FetchAndPull(repoPath, repoURL string) error {
	unlock := c.lockForPath(repoPath)
	defer unlock()

	if strings.TrimSpace(repoURL) != "" {
		if err := c.ensureOriginURL(repoPath, repoURL); err != nil {
			return err
		}
	}

	if err := c.fetchPrune(repoPath); err != nil {
		return err
	}

	statusOut, err := c.runGit(repoPath, "status", "--porcelain")
	if err != nil {
		return fmt.Errorf("проверка рабочего дерева: %w", err)
	}
	if strings.TrimSpace(statusOut) != "" {
		return errors.New("рабочее дерево содержит незакоммиченные изменения; pull отменен")
	}

	currentBranchOut, err := c.runGit(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("определение текущей ветки: %w", err)
	}
	currentBranch := strings.TrimSpace(currentBranchOut)
	if currentBranch == "" || currentBranch == "HEAD" {
		return errors.New("pull недоступен: репозиторий в состоянии detached HEAD")
	}

	if err := c.runGitStatusOnly(repoPath, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err != nil {
		return fmt.Errorf("pull недоступен: для ветки %q не настроен upstream", currentBranch)
	}

	if _, err := c.runGit(repoPath, "pull", "--ff-only"); err != nil {
		return fmt.Errorf("git pull --ff-only: %w", err)
	}

	return nil
}

// ForceCheckout принудительно переключает репозиторий на указанную ветку, сбрасывая все изменения.
func (c *Client) ForceCheckout(repoPath, branch string) error {
	unlock := c.lockForPath(repoPath)
	defer unlock()

	_, err := c.runGit(repoPath, "checkout", "-f", branch)
	if err != nil {
		return fmt.Errorf("ошибка git checkout -f %s: %w", branch, err)
	}
	return nil
}

// CreateTrackingBranchAndCheckout создает локальную tracking-ветку из remote и переключается на нее.
func (c *Client) CreateTrackingBranchAndCheckout(repoPath, localBranch, remoteBranch string) error {
	unlock := c.lockForPath(repoPath)
	defer unlock()

	exists, err := c.BranchExists(repoPath, localBranch)
	if err != nil {
		return fmt.Errorf("проверка существования локальной ветки %q: %w", localBranch, err)
	}

	if exists {
		if _, err := c.runGit(repoPath, "checkout", localBranch); err != nil {
			return fmt.Errorf("ошибка checkout существующей ветки %q: %w", localBranch, err)
		}
		return nil
	}

	if _, err := c.runGit(repoPath, "checkout", "--track", "-b", localBranch, remoteBranch); err != nil {
		return fmt.Errorf("ошибка создания ветки отслеживания %q из %q: %w", localBranch, remoteBranch, err)
	}

	return nil
}

// DetectDefaultBranch определяет default branch:
// origin/HEAD -> main -> master -> current branch.
func (c *Client) DetectDefaultBranch(repoPath, currentBranch string) (string, error) {
	stdout, err := c.runGit(repoPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		ref := strings.TrimSpace(stdout)
		if strings.HasPrefix(ref, "origin/") && len(ref) > len("origin/") {
			return strings.TrimPrefix(ref, "origin/"), nil
		}
	}

	for _, fallback := range []string{"main", "master"} {
		exists, checkErr := c.BranchExists(repoPath, fallback)
		if checkErr != nil {
			return "", checkErr
		}
		if exists {
			return fallback, nil
		}
	}

	if currentBranch != "" {
		return currentBranch, nil
	}

	return "", nil
}

// BranchMetadata возвращает merged-статус и базовую ветку для branch.
func (c *Client) BranchMetadata(repoPath, branch, defaultBranch string) (model.MergeStatus, string, error) {
	mergeStatus, err := c.mergeStatus(repoPath, branch, defaultBranch)
	if err != nil {
		return model.MergeStatusUnknown, "-", err
	}

	baseBranch := c.resolveBaseBranch(repoPath, branch)
	if baseBranch == "" {
		baseBranch = "-"
	}

	return mergeStatus, baseBranch, nil
}

// ManagedRepoPath вычисляет путь managed clone для репозитория.
func (c *Client) ManagedRepoPath(repoName, repoURL string) string {
	return filepath.Join(c.workspaceDir, safeManagedRepoDir(repoName, repoURL))
}

// ListBranches возвращает локальные и удаленные ветки, отсортированные по дате последнего коммита.
func (c *Client) ListBranches(repoPath string) ([]model.BranchInfo, error) {
	repo, err := c.open(repoPath)
	if err != nil {
		return nil, err
	}

	localBranches, err := c.listLocalBranches(repo)
	if err != nil {
		return nil, err
	}

	remoteBranches, err := c.listRemoteBranches(repo)
	if err != nil {
		return nil, err
	}

	result := append(localBranches, remoteBranches...)

	sort.Slice(result, func(i, j int) bool {
		return result[i].LastCommitAt.After(result[j].LastCommitAt)
	})

	return result, nil
}

func (c *Client) listLocalBranches(repo *git.Repository) ([]model.BranchInfo, error) {
	branches, err := repo.Branches()
	if err != nil {
		return nil, fmt.Errorf("получение списка локальных веток: %w", err)
	}

	var result []model.BranchInfo
	err = branches.ForEach(func(ref *plumbing.Reference) error {
		commit, err := commitForRef(repo, ref)
		if err != nil {
			return fmt.Errorf("определение коммита для ветки %q: %w", ref.Name().Short(), err)
		}

		name := ref.Name().Short()
		result = append(result, model.BranchInfo{
			Key:           ref.Name().String(),
			Name:          name,
			QualifiedName: name,
			FullRef:       ref.Name().String(),
			Scope:         model.BranchScopeLocal,
			LastCommitAt:  commit.Committer.When,
			LastCommitSHA: ref.Hash().String(),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *Client) listRemoteBranches(repo *git.Repository) ([]model.BranchInfo, error) {
	refs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("получение списка ссылок: %w", err)
	}

	var result []model.BranchInfo
	err = refs.ForEach(func(ref *plumbing.Reference) error {
		if !ref.Name().IsRemote() {
			return nil
		}

		if ref.Name().Short() == "origin/HEAD" || strings.HasSuffix(ref.Name().String(), "/HEAD") {
			return nil
		}

		short := ref.Name().Short()
		parts := strings.SplitN(short, "/", 2)
		if len(parts) != 2 {
			return nil
		}

		commit, err := commitForRef(repo, ref)
		if err != nil {
			return fmt.Errorf("определение коммита для удаленной ветки %q: %w", short, err)
		}

		remoteName := parts[0]
		name := parts[1]
		result = append(result, model.BranchInfo{
			Key:           ref.Name().String(),
			Name:          name,
			QualifiedName: short,
			FullRef:       ref.Name().String(),
			Scope:         model.BranchScopeRemote,
			RemoteName:    remoteName,
			LastCommitAt:  commit.Committer.When,
			LastCommitSHA: ref.Hash().String(),
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

// CurrentBranch возвращает имя текущей ветки в репозитории.
func (c *Client) CurrentBranch(repoPath string) (string, error) {
	repo, err := c.open(repoPath)
	if err != nil {
		return "", err
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("определение HEAD: %w", err)
	}

	return head.Name().Short(), nil
}

// DeleteLocalBranch принудительно удаляет локальную ветку.
func (c *Client) DeleteLocalBranch(repoPath, branch string) error {
	exists, err := c.BranchExists(repoPath, branch)
	if err != nil {
		return fmt.Errorf("проверка существования ветки %q: %w", branch, err)
	}
	if !exists {
		return fmt.Errorf("ветка %q не найдена", branch)
	}

	if _, err := c.runGit(repoPath, "branch", "-D", branch); err != nil {
		return fmt.Errorf("ошибка удаления ветки %q: %w", branch, err)
	}

	return nil
}

// BranchExists проверяет существование локальной ветки по имени.
func (c *Client) BranchExists(repoPath, branch string) (bool, error) {
	repo, err := c.open(repoPath)
	if err != nil {
		return false, err
	}

	_, err = repo.Reference(plumbing.NewBranchReferenceName(branch), true)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, plumbing.ErrReferenceNotFound) {
		return false, nil
	}

	return false, fmt.Errorf("определение ветки %q: %w", branch, err)
}

// GetDirtyStats проверяет наличие измененных или неотслеживаемых файлов в рабочей директории и подсчитывает их.
func (c *Client) GetDirtyStats(repoPath string) (model.DirtyStats, error) {
	stdout, err := c.runGit(repoPath, "status", "--porcelain")
	if err != nil {
		return model.DirtyStats{}, fmt.Errorf("git status --porcelain: %w", err)
	}

	var stats model.DirtyStats
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}
		x := line[0]
		y := line[1]
		file := strings.TrimSpace(line[2:])

		if x == '?' && y == '?' {
			stats.Untracked = append(stats.Untracked, file)
			continue
		}

		if x == 'M' || y == 'M' {
			stats.Modified = append(stats.Modified, file)
		}
		if x == 'A' || y == 'A' {
			stats.Added = append(stats.Added, file)
		}
		if x == 'D' || y == 'D' {
			stats.Deleted = append(stats.Deleted, file)
		}
	}

	return stats, nil
}

// GetRepoStat собирает CurrentBranch и IsDirty для репозитория.
func (c *Client) GetRepoStat(repoPath string) (model.RepoStat, error) {
	branch, err := c.CurrentBranch(repoPath)
	if err != nil {
		return model.RepoStat{}, err
	}

	dirty, err := c.GetDirtyStats(repoPath)
	if err != nil {
		return model.RepoStat{}, err
	}

	return model.RepoStat{
		CurrentBranch: branch,
		DirtyStats:    dirty,
		Loaded:        true,
	}, nil
}

func (c *Client) cloneRepo(repoURL, managedPath string) error {
	if err := ensureDir(filepath.Dir(managedPath)); err != nil {
		return err
	}

	_, err := c.runGit("", "clone", repoURL, managedPath)
	if err != nil {
		return fmt.Errorf("ошибка клонирования репозитория %q в %q: %w", repoURL, managedPath, err)
	}

	return nil
}

func (c *Client) ensureOriginURL(repoPath, repoURL string) error {
	stdout, err := c.runGit(repoPath, "remote", "get-url", "origin")
	if err != nil {
		return fmt.Errorf("чтение адреса origin: %w", err)
	}

	currentURL := strings.TrimSpace(stdout)
	if currentURL == repoURL {
		return nil
	}

	if _, err := c.runGit(repoPath, "remote", "set-url", "origin", repoURL); err != nil {
		return fmt.Errorf("установка адреса origin: %w", err)
	}

	return nil
}

func (c *Client) fetchPrune(repoPath string) error {
	if _, err := c.runGit(repoPath, "fetch", "--prune", "origin"); err != nil {
		return fmt.Errorf("ошибка fetch --prune origin: %w", err)
	}

	return nil
}

func (c *Client) seedLocalBranchesFromOrigin(repoPath string) error {
	stdout, err := c.runGit(repoPath, "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin")
	if err != nil {
		return fmt.Errorf("получение списка удаленных refs: %w", err)
	}

	for _, remoteBranch := range strings.Split(strings.TrimSpace(stdout), "\n") {
		remoteBranch = strings.TrimSpace(remoteBranch)
		if remoteBranch == "" || remoteBranch == "origin/HEAD" {
			continue
		}
		if !strings.HasPrefix(remoteBranch, "origin/") {
			continue
		}

		branchName := strings.TrimPrefix(remoteBranch, "origin/")
		exists, checkErr := c.BranchExists(repoPath, branchName)
		if checkErr != nil {
			return checkErr
		}
		if exists {
			continue
		}

		if _, err := c.runGit(repoPath, "branch", "--track", branchName, remoteBranch); err != nil {
			return fmt.Errorf("ошибка создания локальной ветки %q из %q: %w", branchName, remoteBranch, err)
		}
	}

	return nil
}

func (c *Client) open(repoPath string) (*git.Repository, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("открытие репозитория %s: %w", repoPath, err)
	}

	return repo, nil
}

func commitForRef(repo *git.Repository, ref *plumbing.Reference) (*object.Commit, error) {
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	return commit, nil
}

func (c *Client) runGit(repoPath string, args ...string) (string, error) {
	ctx := context.Background()
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	if repoPath != "" {
		cmd.Dir = repoPath
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("таймаут git %s после %s: %w", args[0], c.timeout, err)
		}
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr == "" {
			stderrStr = err.Error()
		}
		return "", fmt.Errorf("ошибка git %s: %s (%w)", args[0], stderrStr, err)
	}

	return stdout.String(), nil
}

func (c *Client) mergeStatus(repoPath, branch, defaultBranch string) (model.MergeStatus, error) {
	if strings.TrimSpace(defaultBranch) == "" || branch == defaultBranch {
		return model.MergeStatusUnknown, nil
	}

	err := c.runGitStatusOnly(repoPath, "merge-base", "--is-ancestor", branch, defaultBranch)
	if err == nil {
		return model.MergeStatusMerged, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.ExitStatus() == 1 {
			return model.MergeStatusUnmerged, nil
		}
	}

	return model.MergeStatusUnknown, nil
}

func (c *Client) resolveBaseBranch(repoPath, branch string) string {
	stdout, err := c.runGit(repoPath, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil {
		return ""
	}

	upstream := strings.TrimSpace(stdout)
	if upstream == "" {
		return ""
	}

	return upstream
}

func (c *Client) runGitStatusOnly(repoPath string, args ...string) error {
	ctx := context.Background()
	if c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	if repoPath != "" {
		cmd.Dir = repoPath
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("таймаут git %s после %s: %w", args[0], c.timeout, err)
		}
		stderrStr := strings.TrimSpace(stderr.String())
		if stderrStr == "" {
			stderrStr = err.Error()
		}
		return fmt.Errorf("ошибка git %s: %s (%w)", args[0], stderrStr, err)
	}

	return nil
}

func (c *Client) lockForPath(path string) func() {
	c.lockMu.Lock()
	mu, ok := c.locks[path]
	if !ok {
		mu = &sync.Mutex{}
		c.locks[path] = mu
	}
	c.lockMu.Unlock()

	mu.Lock()
	return mu.Unlock
}

func safeManagedRepoDir(repoName, repoURL string) string {
	hash := sha256.Sum256([]byte(repoURL))
	hashSuffix := hex.EncodeToString(hash[:8])
	safeName := sanitizeDirPart(repoName)
	if safeName == "" {
		safeName = "repo"
	}

	return safeName + "__" + hashSuffix
}

func sanitizeDirPart(s string) string {
	b := strings.Builder{}
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '.' || ch == '-' || ch == '_' {
			b.WriteByte(ch)
			continue
		}
		b.WriteByte('_')
	}

	return strings.Trim(b.String(), "_")
}

func isGitRepo(path string) bool {
	if path == "" {
		return false
	}

	cmd := exec.Command("git", "-C", path, "rev-parse", "--is-inside-work-tree")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false
	}

	insideWorkTree := strings.TrimSpace(stdout.String())
	if insideWorkTree != "true" {
		return false
	}

	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
