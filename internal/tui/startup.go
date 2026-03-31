package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) startInitialLoads() tea.Cmd {
	return m.startPreloadPass(true, false)
}

func (m *Model) startRescanAllRepos() tea.Cmd {
	return m.startPreloadPass(false, true)
}

func (m *Model) startPreloadPass(startup bool, keepSelection bool) tea.Cmd {
	if len(m.cfg.Repos) == 0 {
		return nil
	}

	if !keepSelection {
		m.selectFirstVisibleRepo()
	}

	m.startupLoading = startup
	m.startupPending = 0
	m.startupURLTotal = 0
	m.startupURLDone = 0

	if !startup {
		m.refreshLocked = true
		m.refreshAll = true
		m.refreshRepo = ""
		m.refreshReqID = 0
		m.err = nil
		for repoName := range m.refreshPending {
			delete(m.refreshPending, repoName)
		}
	}

	var cmds []tea.Cmd

	if startup && m.startupPlaywrightStartFn != nil && !m.startupPlaywrightScheduled {
		cmds = append(cmds, tea.Batch(
			func() tea.Msg { return startupLogMsg{"[PLAYWRIGHT] bootstrap/start runtime..."} },
			func() tea.Msg {
				return playwrightStartupCompletedMsg{err: m.startupPlaywrightStartFn()}
			},
		))
		m.startupPending++
		m.startupPlaywrightScheduled = true
		m.startupPlaywrightState = startupPlaywrightPending
	}

	for idx, repo := range m.cfg.Repos {
		if !startup {
			m.refreshPending[repo.Name] = true
		}
		switch repo.SourceType() {
		case "url":
			m.startupURLTotal++
			cmds = append(cmds, m.startLoadRepo(repo, false, startup))
			m.startupPending++
		case "opensource":
			m.startupURLTotal++
			cmds = append(cmds, m.startLoadRepo(repo, false, startup))
			m.startupPending++
		case "path":
			if idx == m.repoIdx {
				cmds = append(cmds, m.startLoadRepo(repo, false, startup))
				m.startupPending++
			} else {
				actionKey := actionKeyRepoStat(repo.Name)
				ctx, actionID := m.beginAction(actionKey)
				cmds = append(cmds, loadRepoStatCmd(ctx, m.clean, repo, startup, actionKey, actionID))
				m.startupPending++
			}
		}
	}

	if !startup {
		m.statusLine = "Пересканирование всех репозиториев..."
	} else {
		m.setStartupProgressStatus()
	}

	m.activateSelectedRepoFromCache()
	if m.startupPending == 0 {
		m.startupLoading = false
		m.resetRefreshLock()
		return nil
	}

	if m.loadingSelectedRepo() || m.startupLoading || m.refreshLocked {
		cmds = append(cmds, m.spinner.Tick)
	}

	return tea.Batch(cmds...)
}

func (m *Model) finishStartupTaskIfNeeded(startup bool) {
	if !startup || !m.startupLoading {
		return
	}
	if m.startupPending > 0 {
		m.startupPending--
	}
	if m.startupPending == 0 {
		m.startupLoading = false
		if m.startupURLTotal > 0 {
			m.statusLine = fmt.Sprintf("Первичная синхронизация URL-репозиториев завершена: %d/%d", m.startupURLDone, m.startupURLTotal)
		}
	}
}

func (m *Model) finishStartupURLTaskIfNeeded(repoName string, startup bool) {
	if !startup {
		return
	}

	repo, ok := m.cfg.RepoByName(repoName)
	if !ok {
		return
	}

	source := repo.SourceType()
	if source != "url" && source != "opensource" {
		return
	}

	if m.startupURLDone < m.startupURLTotal {
		m.startupURLDone++
		m.pushLog(fmt.Sprintf("[%d/%d] %s — синхронизирован", m.startupURLDone, m.startupURLTotal, repoName))
	}
}

func (m *Model) setStartupProgressStatus() {
	if m.startupURLTotal <= 0 {
		m.statusLine = "Первичная загрузка репозиториев..."
		return
	}

	m.statusLine = fmt.Sprintf("Первичная синхронизация URL-репозиториев: %d/%d", m.startupURLDone, m.startupURLTotal)
}

func (m *Model) finishRefreshIfMatched(repoName string, requestID int) {
	if !m.refreshLocked {
		return
	}
	if m.refreshAll {
		return
	}
	if m.refreshRepo != repoName || m.refreshReqID != requestID {
		return
	}
	m.resetRefreshLock()
}

func (m *Model) releaseRefreshLock(repoName string) {
	if !m.refreshLocked {
		return
	}
	if m.refreshAll {
		return
	}
	if m.refreshRepo != repoName {
		return
	}
	m.resetRefreshLock()
}

func (m *Model) finishRefreshPendingIfNeeded(repoName string) {
	if !m.refreshLocked || !m.refreshAll {
		return
	}
	if !m.refreshPending[repoName] {
		return
	}
	delete(m.refreshPending, repoName)
	if len(m.refreshPending) > 0 {
		return
	}
	m.statusLine = "Пересканирование всех репозиториев завершено"
	m.resetRefreshLock()
}

func (m *Model) resetRefreshLock() {
	m.refreshLocked = false
	m.refreshAll = false
	m.refreshRepo = ""
	m.refreshReqID = 0
	for repoName := range m.refreshPending {
		delete(m.refreshPending, repoName)
	}
}

const maxEventLog = 50

func (m *Model) pushLog(msg string) {
	if msg == "" {
		return
	}
	m.eventLog = append(m.eventLog, msg)
	if len(m.eventLog) > maxEventLog {
		m.eventLog = m.eventLog[len(m.eventLog)-maxEventLog:]
	}
}

func (m Model) viewStartupScreen() string {
	usableW := max(40, m.width-4)
	usableH := max(12, m.height-2)

	progHeader := titleStyle.Width(usableW).Render("  go-repo-orchestrator  ")
	spinLine := fmt.Sprintf("  %s Инициализация репозиториев...  ", m.spinner.View())

	var progressLines []string
	if m.startupURLTotal > 0 {
		bar := startupProgressBar(m.startupURLDone, m.startupURLTotal, usableW-30)
		progressLines = append(progressLines, fmt.Sprintf("  Синхронизация URL-репо: %s %d/%d  ", bar, m.startupURLDone, m.startupURLTotal))
	}
	if m.startupPlaywrightStartFn != nil {
		playwrightLine := "  Playwright: ожидание...  "
		switch m.startupPlaywrightState {
		case startupPlaywrightReady:
			playwrightLine = "  Playwright: готов  "
		case startupPlaywrightFailed:
			playwrightLine = "  Playwright: недоступен (HTTP fallback)  "
		case startupPlaywrightPending:
			playwrightLine = "  Playwright: запуск...  "
		}
		progressLines = append(progressLines, playwrightLine)
	}
	if m.startupPending > 0 {
		progressLines = append(progressLines, fmt.Sprintf("  Осталось задач: %d  ", m.startupPending))
	}
	if m.startupWarn != "" {
		progressLines = append(progressLines, warnStyle.Render("  "+m.startupWarn+"  "))
	}

	headerBlock := lipgloss.JoinVertical(lipgloss.Left,
		progHeader,
		topMenuStyle.Width(usableW).Render(spinLine),
	)
	for _, l := range progressLines {
		headerBlock = lipgloss.JoinVertical(lipgloss.Left, headerBlock,
			statusStyle.Width(usableW).Render(l))
	}

	headerLines := lipgloss.Height(headerBlock)

	logH := max(4, usableH-headerLines-2)
	logBlock := m.viewStartupLogPanel(usableW, logH)

	full := lipgloss.JoinVertical(lipgloss.Left, headerBlock, logBlock)
	return appStyle.Width(m.width).Height(m.height).Render(
		lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Top, full),
	)
}

func startupProgressBar(done, total, width int) string {
	if width <= 0 || total <= 0 {
		return ""
	}
	width = min(40, max(10, width))
	filled := done * width / total
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return lipgloss.NewStyle().Foreground(mcBrightCyan).Render(bar)
}

func (m Model) viewStartupLogPanel(width, height int) string {
	logBg := lipgloss.Color("17")
	logFg := lipgloss.Color("252")
	dimFg := lipgloss.Color("240")
	borderFg := lipgloss.Color("27")

	st := lipgloss.NewStyle().
		Background(logBg).
		Foreground(logFg).
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderFg).
		Width(width).
		Height(height)

	headerSt := lipgloss.NewStyle().
		Background(lipgloss.Color("20")).
		Foreground(mcWhite).
		Bold(true).
		Width(width-2).
		Padding(0, 1)

	innerW := max(10, width-4)
	innerH := max(1, height-4)

	var lines []string
	lines = append(lines, headerSt.Render(" ЛОГА СОБЫТИЙ ЗАГРУЗКИ "))

	if len(m.eventLog) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(dimFg).Background(logBg).Render("  (ожидание событий...)"))
	} else {
		start := 0
		if len(m.eventLog) > innerH {
			start = len(m.eventLog) - innerH
		}
		for _, entry := range m.eventLog[start:] {
			var entryFg lipgloss.Color
			switch {
			case strings.HasPrefix(entry, "[WARN]") || strings.HasPrefix(entry, "[ERR]"):
				entryFg = mcYellow
			case strings.HasPrefix(entry, "[OK]"), strings.HasPrefix(entry, "[СКРИПТ]"):
				entryFg = lipgloss.Color("46")
			default:
				entryFg = logFg
			}
			entryLine := lipgloss.NewStyle().Foreground(entryFg).Background(logBg).Render(truncate("  "+entry, innerW))
			lines = append(lines, entryLine)
		}
	}

	return st.Render(strings.Join(lines, "\n"))
}
