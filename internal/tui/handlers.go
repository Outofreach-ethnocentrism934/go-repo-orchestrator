package tui

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		if m.confirmType == confirmReleaseSelect {
			m.confirmType = confirmNone
			m.statusLine = "Release-driven автопометка отменена"
			return m, nil
		}
		m.confirmType = confirmNone
		m.statusLine = "Генерация скрипта отменена"
		return m, nil
	case "left", "right", "t":
		if m.confirmType != confirmGenerate {
			return m, nil
		}
		if m.scriptFormat == model.ScriptFormatSH {
			m.scriptFormat = model.ScriptFormatBAT
		} else {
			m.scriptFormat = model.ScriptFormatSH
		}
		return m, nil
	case "enter", "y", "f8":
		if m.confirmType == confirmReleaseSelect {
			if len(m.releaseOptions) == 0 {
				m.confirmType = confirmNone
				m.statusLine = "Released версии Jira не найдены"
				return m, nil
			}
			idx := m.releaseOptionIdx
			if idx < 0 || idx >= len(m.releaseOptions) {
				idx = 0
			}
			return m, m.startApplyReleaseAutocheck(m.releaseOptions[idx])
		}

		repo, ok := m.cfg.RepoByName(m.activeRepo.RepoName)
		if !ok {
			m.err = errors.New("репозиторий не найден в конфигурации")
			return m, nil
		}

		switch m.confirmType {
		case confirmGenerate:
			branches := m.selectedBranches(m.activeRepo.RepoName)
			if len(branches) == 0 {
				m.confirmType = confirmNone
				m.statusLine = "Нет выбранных веток"
				return m, nil
			}
			return m, generateScriptCmd(m.clean, repo, m.activeRepo.RepoPath, branches, m.scriptFormat)
		case confirmCheckout:
			if msg.String() == "f8" {
				return m, nil
			}
			m.confirmType = confirmNone
			repoName := repo.Name
			m.repoLoading[repoName] = true
			m.statusLine = fmt.Sprintf("Переключение на ветку %s...", m.checkoutTarget)
			actionKey := actionKeyCheckout(repoName)
			ctx, actionID := m.beginAction(actionKey)
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				err := m.clean.ForceCheckoutLocalBranch(ctx, repo, m.checkoutTarget)
				return checkoutCompletedMsg{actionKey: actionKey, actionID: actionID, repoName: repo.Name, err: err}
			})
		}
	case "up", "k":
		if m.confirmType == confirmReleaseSelect && len(m.releaseOptions) > 0 {
			if m.releaseOptionIdx > 0 {
				m.releaseOptionIdx--
			}
			return m, nil
		}
	case "down", "j":
		if m.confirmType == confirmReleaseSelect && len(m.releaseOptions) > 0 {
			if m.releaseOptionIdx < len(m.releaseOptions)-1 {
				m.releaseOptionIdx++
			}
			return m, nil
		}
	}

	return m, nil
}

func (m Model) updateReposPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	indices := m.visibleRepoIndices()
	if len(indices) == 0 {
		m.repoOffset = 0
		return m, nil
	}

	m.ensureRepoCursorVisible()
	pos := m.repoVisiblePosition(indices)
	if pos < 0 {
		m.repoIdx = indices[0]
		pos = 0
	}
	rows := max(1, m.repoViewportRows())
	key := strings.ToLower(msg.String())
	if isHomeNavigationKey(msg) {
		key = "home"
	} else if isEndNavigationKey(msg) {
		key = "end"
	}

	prevIdx := m.repoIdx
	switch key {
	case "j", "down":
		if pos < len(indices)-1 {
			m.repoIdx = indices[pos+1]
		}
	case "k", "up":
		if pos > 0 {
			m.repoIdx = indices[pos-1]
		}
	case "pgdown":
		nextPos := min(len(indices)-1, pos+rows)
		m.repoIdx = indices[nextPos]
	case "pgup":
		prevPos := max(0, pos-rows)
		m.repoIdx = indices[prevPos]
	case "home":
		m.repoIdx = indices[0]
	case "end":
		m.repoIdx = indices[len(indices)-1]
	case "enter":
		if ok, reason := m.canActivateBranches(); !ok {
			m.statusLine = reason
			return m, nil
		}
		m.searchMode = false
		m.searchInput.Reset()
		m.focus = focusBranches
		m.ensureBranchCursorVisible(m.activeRepo.RepoName)
		m.statusLine = "Активен таб: Ветки"
		return m, nil
	case "f5", "r":
		return m, m.startRescanAllRepos()
	}

	if m.repoIdx != prevIdx {
		m.ensureRepoCursorVisible()
		m.activateSelectedRepoFromCache()
		m.statusLine = ""
		return m, nil
	}
	m.ensureRepoCursorVisible()

	return m, nil
}

func (m Model) updateBranchesPanel(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if ok, reason := m.canActivateBranches(); !ok {
		m.statusLine = reason
		m.focus = focusRepos
		return m, nil
	}

	repoName := m.activeRepo.RepoName
	m.clampBranchCursor(repoName)
	m.ensureBranchCursorVisible(repoName)
	branches := m.visibleBranches()
	rows := max(1, m.branchViewportRows())
	key := strings.ToLower(msg.String())
	if isHomeNavigationKey(msg) {
		key = "home"
	} else if isEndNavigationKey(msg) {
		key = "end"
	}

	if isInvertSelectionKey(msg) {
		toggled := m.invertVisibleBranchSelection(repoName)
		if toggled == 0 {
			m.statusLine = "Нет доступных веток для инверсии"
		} else {
			m.statusLine = fmt.Sprintf("Инверсия выбора: %d", toggled)
		}
		return m, nil
	}

	switch key {
	case "esc":
		m.searchMode = false
		m.searchInput.Reset()
		m.clearRepoSelection(repoName)
		m.focus = focusRepos
		m.statusLine = "Активен таб: Репозитории"
		return m, nil
	case "j", "down":
		cursor := m.currentCursor(repoName)
		if cursor < len(branches)-1 {
			m.branchCursor[repoName] = cursor + 1
		}
	case "k", "up":
		cursor := m.currentCursor(repoName)
		if cursor > 0 {
			m.branchCursor[repoName] = cursor - 1
		}
	case "pgdown":
		if len(branches) > 0 {
			cursor := m.currentCursor(repoName)
			m.branchCursor[repoName] = min(len(branches)-1, cursor+rows)
		}
	case "pgup":
		if len(branches) > 0 {
			cursor := m.currentCursor(repoName)
			m.branchCursor[repoName] = max(0, cursor-rows)
		}
	case "home":
		if len(branches) > 0 {
			m.branchCursor[repoName] = 0
		}
	case "end":
		if len(branches) > 0 {
			m.branchCursor[repoName] = len(branches) - 1
		}
	case " ", "space", "insert":
		if len(branches) == 0 {
			return m, nil
		}
		cursor := m.currentCursor(repoName)
		if cursor >= len(branches) {
			return m, nil
		}
		branch := branches[cursor]
		if branch.Protected {
			m.statusLine = "Ветка защищена правилами фильтра"
			return m, nil
		}
		selected := m.ensureRepoSelection(repoName)
		selectionKey := m.branchSelectionKey(branch)
		selected[selectionKey] = !selected[selectionKey]
		if cursor < len(branches)-1 {
			m.branchCursor[repoName] = cursor + 1
		}
	case "f5", "r":
		return m, m.startLoadActiveRepo()
	case "g", "f8":
		if !m.openConfirmIfPossible() {
			m.statusLine = "Нет выбранных веток для генерации скрипта"
			return m, nil
		}
	case "enter", "f3":
		if len(branches) > 0 {
			cursor := m.currentCursor(repoName)
			if cursor >= 0 && cursor < len(branches) {
				branch := branches[cursor]
				if branch.IsRemote() {
					if m.canUseF7ForActiveRepo() {
						m.statusLine = "Для удаленной ветки сначала создайте локальную копию (F7)"
					} else {
						m.statusLine = "Для URL-репозитория локальная копия удаленной ветки недоступна"
					}
					return m, nil
				}
				m.checkoutTarget = branch.Name
				m.confirmType = confirmCheckout
			}
		}
	}

	m.ensureBranchCursorVisible(repoName)

	return m, nil
}

func (m Model) canGenerateScript() bool {
	if m.focus != focusBranches {
		return false
	}
	if ok, _ := m.canActivateBranches(); !ok {
		return false
	}
	branches := m.selectedBranches(m.activeRepo.RepoName)
	if len(branches) == 0 {
		return false
	}
	for _, b := range branches {
		if !b.Protected {
			return true
		}
	}
	return false
}

func (m Model) canCheckoutBranch() bool {
	if m.focus != focusBranches || m.loadingSelectedRepo() {
		return false
	}
	if ok, _ := m.canActivateBranches(); !ok {
		return false
	}
	branch := m.currentBranch()
	if branch == nil {
		return false
	}
	return branch.IsLocal()
}

func (m Model) canCreateLocalFromRemote() bool {
	if m.focus != focusBranches || m.loadingSelectedRepo() {
		return false
	}
	if !m.canUseF7ForActiveRepo() {
		return false
	}
	if ok, _ := m.canActivateBranches(); !ok {
		return false
	}
	branch := m.currentBranch()
	if branch == nil {
		return false
	}
	return branch.IsRemote()
}

func (m Model) canActivateBranches() (bool, string) {
	if len(m.cfg.Repos) == 0 {
		return false, "Нет репозиториев"
	}

	repoName := m.selectedRepoName()
	if m.loadingSelectedRepo() {
		return false, "Подождите: ветки еще загружаются"
	}

	if stat, ok := m.repoStats[repoName]; ok && stat.Loaded && stat.HasError() {
		return false, "Ошибка репозитория: ветки недоступны (см. ИНФО)"
	}

	if m.activeRepo.RepoName != repoName {
		return false, "Ветки для выбранного репозитория еще не загружены"
	}

	return true, ""
}

func (m *Model) toggleProtectedFilter() {
	m.hideProtected = !m.hideProtected
	m.clampBranchCursor(m.activeRepo.RepoName)
	m.ensureBranchCursorVisible(m.activeRepo.RepoName)
	m.statusLine = fmt.Sprintf("Скрытое: %s", onOff(m.hideProtected))
}
