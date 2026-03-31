package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"
	"github.com/agelxnash/go-repo-orchestrator/internal/model"
)

func (m *Model) ensureRepoSelection(repoName string) map[string]bool {
	if _, ok := m.selected[repoName]; !ok {
		m.selected[repoName] = make(map[string]bool)
	}
	return m.selected[repoName]
}

func (m *Model) ensureRepoState(repoName string) {
	_ = m.ensureRepoSelection(repoName)
	if _, ok := m.branchCursor[repoName]; !ok {
		m.branchCursor[repoName] = 0
	}
	if _, ok := m.branchOffset[repoName]; !ok {
		m.branchOffset[repoName] = 0
	}
}

func (m Model) isSelected(repoName, key string) bool {
	repoSelected, ok := m.selected[repoName]
	if !ok {
		return false
	}

	return repoSelected[key]
}

func (m Model) selectedBranches(repoName string) []model.BranchInfo {
	repoSelected, ok := m.selected[repoName]
	if !ok {
		return nil
	}

	result := make([]model.BranchInfo, 0, len(repoSelected))
	for _, branch := range m.activeRepo.Branches {
		if !repoSelected[m.branchSelectionKey(branch)] {
			continue
		}
		if branch.Protected {
			continue
		}
		result = append(result, branch)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].LastCommitAt.After(result[j].LastCommitAt)
	})

	return result
}

func (m Model) visibleBranches() []model.BranchInfo {
	query := ""
	if m.searchMode || m.searchInput.Value() != "" {
		if m.focus == focusBranches {
			query = strings.ToLower(m.searchInput.Value())
		}
	}

	visible := make([]model.BranchInfo, 0, len(m.activeRepo.Branches))
	for _, branch := range m.activeRepo.Branches {
		if !m.isBranchInScope(branch) {
			continue
		}
		if m.hideProtected && branch.Protected {
			continue
		}
		displayName := strings.ToLower(m.branchDisplayName(branch))
		if query != "" && !strings.Contains(displayName, query) {
			continue
		}
		visible = append(visible, branch)
	}

	sort.SliceStable(visible, func(i, j int) bool {
		return m.lessBranch(visible[i], visible[j])
	})

	return visible
}

func (m Model) visibleRepoIndices() []int {
	query := ""
	if m.searchMode || m.searchInput.Value() != "" {
		if m.focus == focusRepos {
			query = strings.ToLower(m.searchInput.Value())
		}
	}

	var indices []int
	for i, repo := range m.cfg.Repos {
		if query != "" && !strings.Contains(strings.ToLower(repo.Name), query) {
			continue
		}
		indices = append(indices, i)
	}

	sort.SliceStable(indices, func(i, j int) bool {
		left := m.cfg.Repos[indices[i]]
		right := m.cfg.Repos[indices[j]]
		return m.lessRepo(left, right)
	})

	return indices
}

func (m Model) currentCursor(repoName string) int {
	cursor, ok := m.branchCursor[repoName]
	if !ok {
		return 0
	}
	return cursor
}

func (m *Model) clampBranchCursor(repoName string) {
	branches := m.visibleBranches()
	if len(branches) == 0 {
		m.branchCursor[repoName] = 0
		m.branchOffset[repoName] = 0
		return
	}

	cur := m.currentCursor(repoName)
	if cur >= len(branches) {
		cur = len(branches) - 1
	}
	if cur < 0 {
		cur = 0
	}
	m.branchCursor[repoName] = cur
}

func (m *Model) ensureRepoCursorVisible() {
	indices := m.visibleRepoIndices()
	if len(indices) == 0 {
		m.repoOffset = 0
		return
	}

	pos := m.repoVisiblePosition(indices)
	if pos < 0 {
		m.repoIdx = indices[0]
		pos = 0
	}

	rows := m.repoViewportRows()
	m.repoOffset = adjustedViewportOffset(m.repoOffset, pos, len(indices), rows)
}

func (m Model) repoVisiblePosition(indices []int) int {
	for i, idx := range indices {
		if idx == m.repoIdx {
			return i
		}
	}
	return -1
}

func (m *Model) ensureBranchCursorVisible(repoName string) {
	if repoName == "" {
		return
	}

	m.ensureRepoState(repoName)
	branches := m.visibleBranches()
	if len(branches) == 0 {
		m.branchCursor[repoName] = 0
		m.branchOffset[repoName] = 0
		return
	}

	cursor := m.currentCursor(repoName)
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(branches) {
		cursor = len(branches) - 1
	}
	m.branchCursor[repoName] = cursor

	rows := m.branchViewportRows()
	m.branchOffset[repoName] = adjustedViewportOffset(m.branchOffset[repoName], cursor, len(branches), rows)
}

func (m Model) repoViewportRows() int {
	contentHeight := max(8, m.height-4)
	repoHeight := contentHeight
	if m.showInfo {
		infoHeight := min(14, max(8, contentHeight/3))
		repoHeight = max(7, contentHeight-infoHeight-1)
	}
	return m.repoRowsForPanelHeight(repoHeight)
}

func (m Model) branchViewportRows() int {
	contentHeight := max(8, m.height-4)
	branchesHeight := contentHeight
	if m.showInfo {
		infoHeight := min(14, max(8, contentHeight/3))
		branchesHeight = max(7, contentHeight-infoHeight-1)
	}
	return m.branchRowsForPanelHeight(branchesHeight)
}

func (m Model) repoRowsForPanelHeight(panelHeight int) int {
	return max(0, panelHeight-4)
}

func (m Model) branchRowsForPanelHeight(panelHeight int) int {
	return max(0, panelHeight-4)
}

func adjustedViewportOffset(offset, cursor, total, rows int) int {
	if total <= 0 || rows <= 0 {
		return 0
	}

	if total <= rows {
		return 0
	}

	maxOffset := total - rows
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}

	if cursor < 0 {
		cursor = 0
	}
	if cursor >= total {
		cursor = total - 1
	}

	if cursor < offset {
		offset = cursor
	}
	if cursor >= offset+rows {
		offset = cursor - rows + 1
	}

	if offset < 0 {
		return 0
	}
	if offset > maxOffset {
		return maxOffset
	}

	return offset
}

func (m Model) currentBranch() *model.BranchInfo {
	branches := m.visibleBranches()
	if len(branches) == 0 {
		return nil
	}
	cur := m.currentCursor(m.activeRepo.RepoName)
	if cur < 0 || cur >= len(branches) {
		return nil
	}
	branch := branches[cur]
	return &branch
}

func (m Model) selectedRepoName() string {
	if len(m.cfg.Repos) == 0 || m.repoIdx < 0 || m.repoIdx >= len(m.cfg.Repos) {
		return ""
	}
	return m.cfg.Repos[m.repoIdx].Name
}

func (m Model) selectedRepoStat() (model.RepoStat, bool) {
	repoName := m.selectedRepoName()
	if repoName == "" {
		return model.RepoStat{}, false
	}

	if stat, ok := m.repoStats[repoName]; ok && stat.Loaded {
		return stat, true
	}

	if m.activeRepo.RepoName == repoName {
		if !m.hasLoadedBranches(repoName) {
			return model.RepoStat{}, false
		}
		return model.RepoStat{
			CurrentBranch: m.activeRepo.CurrentBranch,
			DirtyStats:    m.activeRepo.DirtyStats,
			Loaded:        true,
		}, true
	}

	return model.RepoStat{}, false
}

func (m Model) repoListState(repoName string) (string, string) {
	branch := "-"
	status := "Не загружен"

	if stat, ok := m.repoStats[repoName]; ok && stat.Loaded {
		if stat.CurrentBranch != "" {
			branch = stat.CurrentBranch
		}
		if stat.HasError() {
			return branch, "Ошибка"
		}
		if stat.HasSyncWarning() {
			return branch, "Предупреждение"
		}
		if stat.DirtyStats.HasChanges() {
			return branch, "Изменения"
		}
		return branch, "Чисто"
	}

	if m.repoLoading[repoName] {
		return branch, "Загрузка"
	}

	return branch, status
}

func (m Model) renderRepoStatus(status string) string {
	switch status {
	case "Ошибка":
		return errorStyle.Render("[E]")
	case "Предупреждение":
		return warnStyle.Render("[W]")
	case "Изменения":
		return dirtyStyle.Render("[D]")
	case "Чисто":
		return cleanStyle.Render("[C]")
	case "Загрузка":
		return warnStyle.Render("[~]")
	default:
		return mutedStyle.Render("[ ]")
	}
}

func (m Model) repoStatusCode(status string) string {
	switch status {
	case "Ошибка":
		return "E"
	case "Предупреждение":
		return "W"
	case "Изменения":
		return "D"
	case "Чисто":
		return "C"
	case "Загрузка":
		return "~"
	default:
		return "-"
	}
}

func (m Model) branchSelectionKey(branch model.BranchInfo) string {
	if strings.TrimSpace(branch.Key) != "" {
		return branch.Key
	}
	if strings.TrimSpace(branch.FullRef) != "" {
		return branch.FullRef
	}
	if strings.TrimSpace(branch.QualifiedName) != "" {
		return branch.QualifiedName
	}
	return branch.Name
}

func (m Model) branchDisplayName(branch model.BranchInfo) string {
	if branch.IsRemote() && branch.RemoteName != "" {
		return branch.RemoteName + "/" + branch.Name
	}
	if strings.TrimSpace(branch.QualifiedName) != "" {
		return branch.QualifiedName
	}
	return branch.Name
}

func (m Model) branchListName(branch model.BranchInfo) string {
	name := m.branchDisplayName(branch)
	if branch.Protected {
		return name + "*"
	}
	return name
}

func (m Model) branchTypeLabel(branch model.BranchInfo) string {
	if branch.IsRemote() {
		return "remote"
	}
	return "local"
}

func (m Model) isBranchSelectable(branch model.BranchInfo) bool {
	return !branch.Protected
}

func (m *Model) invertVisibleBranchSelection(repoName string) int {
	selected := m.ensureRepoSelection(repoName)
	toggled := 0
	for _, branch := range m.visibleBranches() {
		if !m.isBranchSelectable(branch) {
			continue
		}
		key := m.branchSelectionKey(branch)
		selected[key] = !selected[key]
		toggled++
	}
	return toggled
}

func onOff(v bool) string {
	if v {
		return "вкл"
	}
	return "выкл"
}

func valueOrDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func repoSourceLabel(repo config.RepoConfig, ok bool) string {
	if !ok {
		return "-"
	}

	switch repo.SourceType() {
	case "path":
		return "локальный путь"
	case "url":
		return "удаленный URL"
	case "opensource":
		return "профиль \"опенсорс\""
	default:
		return repo.SourceType()
	}
}

func repoSourceCode(repo config.RepoConfig) string {
	switch repo.SourceType() {
	case "path":
		return "путь"
	case "url":
		return "URL"
	case "opensource":
		return "опенсорс"
	default:
		return repo.SourceType()
	}
}

func dirtySummary(st model.DirtyStats) string {
	return fmt.Sprintf(
		"изменено:%d добавлено:%d удалено:%d неотслеж:%d",
		len(st.Modified),
		len(st.Added),
		len(st.Deleted),
		len(st.Untracked),
	)
}

func userFacingError(err error) error {
	if err == nil {
		return nil
	}

	msg := strings.TrimSpace(err.Error())
	msg = strings.ReplaceAll(msg, "\n", " ")

	replacements := []struct {
		from string
		to   string
	}{
		{"prepare repository:", "подготовка репозитория:"},
		{"get repo stat:", "чтение состояния репозитория:"},
		{"read config:", "чтение конфигурации:"},
		{"resolve config path:", "проверка пути к конфигурации:"},
		{"create config directory:", "создание директории конфигурации:"},
		{"scan directory:", "сканирование директории:"},
		{"marshal config:", "формирование конфигурации:"},
		{"write config:", "запись конфигурации:"},
		{"repo not found", "репозиторий не найден"},
	}

	for _, r := range replacements {
		msg = strings.ReplaceAll(msg, r.from, r.to)
	}

	lowerMsg := strings.ToLower(msg)
	switch {
	case strings.Contains(lowerMsg, "context deadline exceeded") || strings.Contains(lowerMsg, "operation timed out") || strings.Contains(lowerMsg, "i/o timeout"):
		msg = "таймаут обращения к удаленному репозиторию: сервер недоступен или отвечает слишком долго"
	case strings.Contains(lowerMsg, "connection refused"):
		msg = "удаленный репозиторий недоступен: соединение отклонено"
	case strings.Contains(lowerMsg, "no such host") || strings.Contains(lowerMsg, "could not resolve host"):
		msg = "удаленный репозиторий недоступен: не удалось разрешить хост"
	case strings.Contains(lowerMsg, "no route to host"):
		msg = "удаленный репозиторий недоступен: нет маршрута до хоста"
	case strings.Contains(lowerMsg, "permission denied"):
		msg = "доступ к удаленному репозиторию запрещен (проверьте SSH-ключи/права)"
	}

	if strings.HasPrefix(msg, "git ") {
		msg = "ошибка git: " + msg
	}

	msg = strings.TrimSpace(msg)
	if msg == "" {
		msg = "неизвестная ошибка"
	}

	return errors.New(msg)
}

func truncate(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit == 1 {
		return "…"
	}
	return string(runes[:limit-1]) + "…"
}

func fitCell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	trimmed := truncate(s, width)
	return fmt.Sprintf("%-*s", width, trimmed)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
