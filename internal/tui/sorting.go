package tui

import (
	"fmt"
	"strings"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"
	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

func (m Model) branchScopeLabel() string {
	switch m.branchScope {
	case branchScopeRemote:
		return "удаленные"
	case branchScopeAll:
		return "все"
	default:
		return "локальные"
	}
}

func (m Model) branchScopeCodeLabel() string {
	switch m.branchScope {
	case branchScopeRemote:
		return "remote"
	case branchScopeAll:
		return "all"
	default:
		return "local"
	}
}

func (m Model) isBranchInScope(branch model.BranchInfo) bool {
	switch m.branchScope {
	case branchScopeLocal:
		return branch.IsLocal()
	case branchScopeRemote:
		return branch.IsRemote()
	case branchScopeAll:
		return true
	default:
		return branch.IsLocal()
	}
}

func (m *Model) toggleBranchScope() {
	switch m.branchScope {
	case branchScopeLocal:
		m.branchScope = branchScopeRemote
	case branchScopeRemote:
		m.branchScope = branchScopeAll
	default:
		m.branchScope = branchScopeLocal
	}
	m.clampBranchCursor(m.activeRepo.RepoName)
	m.ensureBranchCursorVisible(m.activeRepo.RepoName)
	m.statusLine = fmt.Sprintf("Scope веток: %s", m.branchScopeLabel())
}

func (m *Model) clearRepoSelection(repoName string) {
	delete(m.selected, repoName)
}

func (m *Model) toggleBranchSortMode() {
	switch m.branchSort {
	case branchSortByName:
		m.branchSort = branchSortByCommitDate
	case branchSortByCommitDate:
		m.branchSort = branchSortByMergeStatus
	case branchSortByMergeStatus:
		m.branchSort = branchSortByJiraStatus
	default:
		m.branchSort = branchSortByName
	}
	m.clampBranchCursor(m.activeRepo.RepoName)
	m.ensureBranchCursorVisible(m.activeRepo.RepoName)
	m.statusLine = fmt.Sprintf("Сортировка веток: %s", m.branchSortLabel())
}

func (m *Model) toggleRepoSortMode() {
	if m.repoSort == repoSortByName {
		m.repoSort = repoSortByActiveBranch
	} else {
		m.repoSort = repoSortByName
	}
	m.selectFirstVisibleRepo()
	m.ensureRepoCursorVisible()
	m.activateSelectedRepoFromCache()
	m.statusLine = fmt.Sprintf("Сортировка репозиториев: %s", m.repoSortLabel())
}

func (m *Model) selectFirstVisibleRepo() {
	indices := m.visibleRepoIndices()
	if len(indices) == 0 {
		m.repoIdx = 0
		m.repoOffset = 0
		return
	}
	m.repoIdx = indices[0]
	m.repoOffset = 0
}

func (m Model) branchSortLabel() string {
	switch m.branchSort {
	case branchSortByCommitDate:
		return "по дате последнего коммита"
	case branchSortByMergeStatus:
		return "по статусу слияния"
	case branchSortByJiraStatus:
		return "по статусу задачи"
	default:
		return "по имени ветки"
	}
}

func (m Model) branchSortCodeLabel() string {
	switch m.branchSort {
	case branchSortByCommitDate:
		return "дата"
	case branchSortByMergeStatus:
		return "слияние"
	case branchSortByJiraStatus:
		return "jira"
	default:
		return "имя"
	}
}

func (m Model) repoSortLabel() string {
	if m.repoSort == repoSortByActiveBranch {
		return "по имени активной ветки"
	}
	return "по имени репозитория"
}

func (m Model) repoSortCodeLabel() string {
	if m.repoSort == repoSortByActiveBranch {
		return "ветка"
	}
	return "имя"
}

func (m Model) lessBranch(left, right model.BranchInfo) bool {
	if m.branchSort == branchSortByCommitDate {
		if !left.LastCommitAt.Equal(right.LastCommitAt) {
			return left.LastCommitAt.After(right.LastCommitAt)
		}
		leftName := strings.ToLower(m.branchDisplayName(left))
		rightName := strings.ToLower(m.branchDisplayName(right))
		return leftName < rightName
	}

	if m.branchSort == branchSortByMergeStatus {
		leftRank := mergeStatusSortRank(left.MergeStatus)
		rightRank := mergeStatusSortRank(right.MergeStatus)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		leftName := strings.ToLower(m.branchDisplayName(left))
		rightName := strings.ToLower(m.branchDisplayName(right))
		return leftName < rightName
	}

	if m.branchSort == branchSortByJiraStatus {
		leftBucket := jiraStatusSortBucket(left.JiraStatus)
		rightBucket := jiraStatusSortBucket(right.JiraStatus)
		if leftBucket != rightBucket {
			return leftBucket < rightBucket
		}

		leftStatus := strings.ToLower(strings.TrimSpace(left.JiraStatus))
		rightStatus := strings.ToLower(strings.TrimSpace(right.JiraStatus))
		if leftStatus != rightStatus {
			return leftStatus < rightStatus
		}

		leftName := strings.ToLower(m.branchDisplayName(left))
		rightName := strings.ToLower(m.branchDisplayName(right))
		if leftName != rightName {
			return leftName < rightName
		}

		return left.LastCommitAt.After(right.LastCommitAt)
	}

	leftName := strings.ToLower(m.branchDisplayName(left))
	rightName := strings.ToLower(m.branchDisplayName(right))
	if leftName != rightName {
		return leftName < rightName
	}

	return left.LastCommitAt.After(right.LastCommitAt)
}

func mergeStatusSortRank(status model.MergeStatus) int {
	switch status {
	case model.MergeStatusMerged:
		return 0
	case model.MergeStatusUnmerged:
		return 1
	default:
		return 2
	}
}

func jiraStatusSortBucket(status string) int {
	normalized := strings.ToLower(strings.TrimSpace(status))
	if normalized == "" || normalized == "-" || normalized == "unknown" || normalized == "none" || normalized == "n/a" {
		return 4
	}

	if strings.Contains(normalized, "done") || strings.Contains(normalized, "closed") || strings.Contains(normalized, "resolved") {
		return 0
	}

	if strings.Contains(normalized, "testing") || strings.Contains(normalized, "qa") || strings.Contains(normalized, "uat") {
		return 1
	}

	if strings.Contains(normalized, "in progress") || strings.Contains(normalized, "development") || strings.Contains(normalized, "develop") {
		return 2
	}

	if strings.Contains(normalized, "open") || strings.Contains(normalized, "todo") || strings.Contains(normalized, "backlog") {
		return 3
	}

	return 4
}

func (m Model) lessRepo(left, right config.RepoConfig) bool {
	if m.repoSort == repoSortByActiveBranch {
		leftBranch := strings.ToLower(m.repoActiveBranchSortKey(left.Name))
		rightBranch := strings.ToLower(m.repoActiveBranchSortKey(right.Name))
		if leftBranch != rightBranch {
			return leftBranch < rightBranch
		}
	}

	return strings.ToLower(left.Name) < strings.ToLower(right.Name)
}

func (m Model) repoActiveBranchSortKey(repoName string) string {
	branch, _ := m.repoListState(repoName)
	if branch == "" || branch == "-" {
		return "~"
	}
	return branch
}
