package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"
	"github.com/agelxnash/go-repo-orchestrator/internal/model"
	"github.com/agelxnash/go-repo-orchestrator/internal/usecase"
)

type panelFocus int

const (
	focusRepos panelFocus = iota
	focusBranches
)

type confirmMode int

const (
	confirmNone confirmMode = iota
	confirmGenerate
	confirmCheckout
)

type branchScopeFilter int

const (
	branchScopeLocal branchScopeFilter = iota
	branchScopeRemote
	branchScopeAll
)

type branchSortMode int

const (
	branchSortByName branchSortMode = iota
	branchSortByCommitDate
	branchSortByMergeStatus
	branchSortByJiraStatus
)

type repoSortMode int

const (
	repoSortByName repoSortMode = iota
	repoSortByActiveBranch
)

type startupPlaywrightState int

const (
	startupPlaywrightSkipped startupPlaywrightState = iota
	startupPlaywrightPending
	startupPlaywrightReady
	startupPlaywrightFailed
)

type branchesLoadedMsg struct {
	requestID    int
	repoName     string
	rb           model.RepoBranches
	err          error
	startup      bool
	jiraResolved int
	syncNote     string
}

// startupLogMsg позволяет фоновым задачам отправлять записи в лог загрузки TUI.
type startupLogMsg struct{ text string }

type playwrightStartupCompletedMsg struct{ err error }

type scriptGeneratedMsg struct {
	result model.ScriptResult
	err    error
}

type initialLoadMsg struct{}

type repoStatLoadedMsg struct {
	repoName string
	stat     model.RepoStat
	err      error
	startup  bool
}

type opensourceUpdatedMsg struct {
	repoName string
	err      error
}

type checkoutCompletedMsg struct {
	repoName string
	err      error
}

type localCopyCompletedMsg struct {
	repoName string
	branch   string
	err      error
}

type repoFetchPullCompletedMsg struct {
	repoName string
	err      error
}

// Model хранит состояние TUI в двухпанельном режиме.
type Model struct {
	cfg   *config.Config
	clean *usecase.Cleaner

	focus        panelFocus
	repoIdx      int
	repoOffset   int
	selected     map[string]map[string]bool
	branchCursor map[string]int
	branchOffset map[string]int

	repoStats   map[string]model.RepoStat
	repoData    map[string]model.RepoBranches
	repoLoading map[string]bool
	repoLoadReq map[string]int
	searchMode  bool
	searchInput textinput.Model

	activeRepo model.RepoBranches
	showInfo   bool

	hideProtected  bool
	branchScope    branchScopeFilter
	branchSort     branchSortMode
	repoSort       repoSortMode
	confirmType    confirmMode
	checkoutTarget string
	scriptFormat   model.ScriptFormat

	spinner spinner.Model

	startupLoading  bool
	startupPending  int
	startupURLTotal int
	startupURLDone  int

	startupPlaywrightStartFn   func() error
	startupPlaywrightState     startupPlaywrightState
	startupPlaywrightScheduled bool

	refreshLocked  bool
	refreshAll     bool
	refreshPending map[string]bool
	refreshRepo    string
	refreshReqID   int

	lastGenerated *model.ScriptResult
	err           error
	statusLine    string
	startupWarn   string

	eventLog []string

	width  int
	height int
}

// NewModel создает корневую модель интерфейса.
func NewModel(cfg *config.Config, cleaner *usecase.Cleaner, _ bool) Model {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	s.Style = warnStyle

	ti := textinput.New()
	ti.Placeholder = "Поиск..."
	ti.Prompt = "F3> "
	ti.CharLimit = 100

	return Model{
		cfg:                    cfg,
		clean:                  cleaner,
		focus:                  focusRepos,
		selected:               make(map[string]map[string]bool),
		branchCursor:           make(map[string]int),
		branchOffset:           make(map[string]int),
		repoStats:              make(map[string]model.RepoStat),
		repoData:               make(map[string]model.RepoBranches),
		repoLoading:            make(map[string]bool),
		repoLoadReq:            make(map[string]int),
		refreshPending:         make(map[string]bool),
		branchScope:            branchScopeAll,
		repoSort:               repoSortByName,
		branchSort:             branchSortByName,
		scriptFormat:           model.ScriptFormatSH,
		searchInput:            ti,
		spinner:                s,
		showInfo:               true,
		width:                  120,
		height:                 36,
		eventLog:               make([]string, 0, 50),
		startupPlaywrightState: startupPlaywrightSkipped,
	}
}

func (m *Model) SetStartupWarning(message string) {
	if m == nil {
		return
	}

	message = strings.TrimSpace(message)
	if message == "" {
		return
	}

	m.statusLine = message
	m.startupWarn = message
}

func (m *Model) SetPlaywrightStartupStartFn(startFn func() error) {
	if m == nil {
		return
	}

	m.startupPlaywrightStartFn = startFn
	if startFn == nil {
		m.startupPlaywrightState = startupPlaywrightSkipped
		m.startupPlaywrightScheduled = false
		return
	}

	m.startupPlaywrightState = startupPlaywrightPending
	m.startupPlaywrightScheduled = false
}

// Init запускает первичную загрузку данных репозиториев.
func (m Model) Init() tea.Cmd {
	if len(m.cfg.Repos) == 0 {
		return nil
	}

	return tea.Batch(
		textinput.Blink,
		func() tea.Msg {
			return initialLoadMsg{}
		},
	)
}

// Update обновляет состояние интерфейса.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ensureRepoCursorVisible()
		m.ensureBranchCursorVisible(m.activeRepo.RepoName)
		return m, nil

	case startupLogMsg:
		m.pushLog(msg.text)
		return m, nil

	case playwrightStartupCompletedMsg:
		m.finishStartupTaskIfNeeded(true)
		if msg.err != nil {
			warn := "Предупреждение: браузер Playwright не запущен: " + msg.err.Error()
			m.SetStartupWarning(warn)
			m.startupPlaywrightState = startupPlaywrightFailed
			m.pushLog("[WARN] Playwright: " + msg.err.Error())
			if m.startupLoading {
				m.setStartupProgressStatus()
			}
			return m, nil
		}

		m.startupPlaywrightState = startupPlaywrightReady
		m.pushLog("[OK] Playwright runtime готов")
		if m.startupLoading {
			m.setStartupProgressStatus()
		}
		return m, nil

	case spinner.TickMsg:
		if !m.loadingSelectedRepo() && !m.startupLoading && !m.refreshLocked {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case branchesLoadedMsg:
		if expectedReqID := m.repoLoadReq[msg.repoName]; expectedReqID != msg.requestID {
			return m, nil
		}
		startupInProgress := m.startupLoading
		m.repoLoading[msg.repoName] = false
		m.finishRefreshIfMatched(msg.repoName, msg.requestID)
		m.finishRefreshPendingIfNeeded(msg.repoName)
		m.finishStartupURLTaskIfNeeded(msg.repoName, msg.startup)
		m.finishStartupTaskIfNeeded(msg.startup)
		if msg.startup && startupInProgress && m.startupLoading {
			m.setStartupProgressStatus()
		}
		friendlyErr := userFacingError(msg.err)
		if msg.err != nil {
			stat := model.RepoStat{Loaded: true}
			if friendlyErr != nil {
				stat.LoadError = friendlyErr.Error()
			}
			m.repoStats[msg.repoName] = stat
			delete(m.repoData, msg.repoName)
			if msg.repoName == m.selectedRepoName() && !(msg.startup && startupInProgress) {
				m.statusLine = fmt.Sprintf("Не удалось загрузить %q: %s", msg.repoName, stat.LoadError)
			}
			m.activateSelectedRepoFromCache()
			return m, nil
		}
		m.repoData[msg.repoName] = msg.rb
		syncWarning := strings.TrimSpace(msg.rb.SyncWarning)
		if syncWarning != "" {
			if friendly := userFacingError(errors.New(syncWarning)); friendly != nil {
				syncWarning = friendly.Error()
			}
		}
		m.repoStats[msg.repoName] = model.RepoStat{
			CurrentBranch: msg.rb.CurrentBranch,
			DirtyStats:    msg.rb.DirtyStats,
			LoadError:     "",
			SyncWarning:   syncWarning,
			Loaded:        true,
		}
		m.ensureRepoState(msg.repoName)
		m.activateSelectedRepoFromCache()
		if msg.repoName == m.selectedRepoName() && !(msg.startup && startupInProgress) {
			if syncWarning != "" {
				m.statusLine = fmt.Sprintf("Репозиторий %q загружен из локальных данных: %s", msg.repoName, syncWarning)
			} else {
				m.statusLine = fmt.Sprintf("Репозиторий %q синхронизирован", msg.repoName)
			}
		}
		if msg.startup {
			if msg.err != nil {
				m.pushLog(fmt.Sprintf("[ERR] %s: %s", msg.repoName, msg.err.Error()))
			} else if syncWarning != "" {
				m.pushLog(fmt.Sprintf("[WARN] %s: %d веток [из кэша: %s]", msg.repoName, len(msg.rb.Branches), syncWarning))
			} else {
				jiraNote := ""
				if msg.jiraResolved > 0 {
					jiraNote = fmt.Sprintf(", Jira: %d", msg.jiraResolved)
				}
				m.pushLog(fmt.Sprintf("[OK] %s: %d веток%s%s",
					msg.repoName, len(msg.rb.Branches), jiraNote, msg.syncNote))
			}
		} else {
			if syncWarning != "" {
				m.pushLog(fmt.Sprintf("[WARN] %s: %s", msg.repoName, syncWarning))
			} else {
				m.pushLog(fmt.Sprintf("[OK] %s синхронизирован (%s)", msg.repoName, valueOrDash(msg.rb.CurrentBranch)))
			}
		}
		m.clampBranchCursor(msg.repoName)
		m.ensureRepoCursorVisible()
		m.ensureBranchCursorVisible(msg.repoName)
		return m, nil

	case initialLoadMsg:
		return m, m.startInitialLoads()

	case scriptGeneratedMsg:
		m.confirmType = confirmNone
		m.err = userFacingError(msg.err)
		if msg.err != nil {
			m.statusLine = "Ошибка генерации скрипта"
			return m, nil
		}
		m.lastGenerated = &msg.result
		m.statusLine = fmt.Sprintf("Скрипт создан: %s", filepath.Base(msg.result.ScriptPath))
		m.pushLog(fmt.Sprintf("[СКРИПТ] создан: %s", filepath.Base(msg.result.ScriptPath)))
		return m, nil

	case repoStatLoadedMsg:
		m.finishRefreshPendingIfNeeded(msg.repoName)
		m.finishStartupTaskIfNeeded(msg.startup)
		stat := msg.stat
		stat.Loaded = true
		if strings.TrimSpace(stat.SyncWarning) != "" {
			if friendly := userFacingError(errors.New(stat.SyncWarning)); friendly != nil {
				stat.SyncWarning = friendly.Error()
			}
		}
		if msg.err != nil {
			friendly := userFacingError(msg.err)
			if friendly != nil {
				stat.LoadError = friendly.Error()
			}
		}
		m.repoStats[msg.repoName] = stat
		return m, nil

	case opensourceUpdatedMsg:
		m.repoLoading[msg.repoName] = false
		m.err = userFacingError(msg.err)
		if msg.err != nil {
			m.statusLine = fmt.Sprintf("Ошибка обновления %q: %s", msg.repoName, m.err.Error())
			return m, nil
		}

		m.statusLine = fmt.Sprintf("Репозиторий %q успешно обновлен", msg.repoName)
		if m.activeRepo.RepoName == msg.repoName {
			return m, m.startLoadSelectedRepo()
		}
		repo, ok := m.cfg.RepoByName(msg.repoName)
		if ok {
			return m, loadRepoStatCmd(m.clean, repo, false)
		}
		return m, nil

	case checkoutCompletedMsg:
		m.repoLoading[msg.repoName] = false
		m.err = userFacingError(msg.err)
		if msg.err != nil {
			m.statusLine = fmt.Sprintf("Не удалось переключиться на ветку в %q: %s", msg.repoName, m.err.Error())
			return m, nil
		}

		m.statusLine = fmt.Sprintf("Ветка в %q переключена", msg.repoName)
		if m.activeRepo.RepoName == msg.repoName {
			return m, m.startLoadSelectedRepo()
		}

		repo, ok := m.cfg.RepoByName(msg.repoName)
		if ok {
			return m, loadRepoStatCmd(m.clean, repo, false)
		}
		return m, nil

	case localCopyCompletedMsg:
		m.repoLoading[msg.repoName] = false
		m.err = userFacingError(msg.err)
		if msg.err != nil {
			m.statusLine = fmt.Sprintf("Не удалось создать локальную копию в %q: %s", msg.repoName, m.err.Error())
			return m, nil
		}

		m.statusLine = fmt.Sprintf("Создана и активирована локальная ветка %q", msg.branch)
		if m.activeRepo.RepoName == msg.repoName {
			return m, m.startLoadSelectedRepo()
		}

		repo, ok := m.cfg.RepoByName(msg.repoName)
		if ok {
			return m, loadRepoStatCmd(m.clean, repo, false)
		}
		return m, nil

	case repoFetchPullCompletedMsg:
		m.repoLoading[msg.repoName] = false
		m.err = userFacingError(msg.err)
		if msg.err != nil {
			m.statusLine = fmt.Sprintf("Не удалось выполнить fetch + pull для %q: %s", msg.repoName, m.err.Error())
			m.releaseRefreshLock(msg.repoName)
			return m, nil
		}

		m.statusLine = fmt.Sprintf("Репозиторий %q обновлен (fetch + pull)", msg.repoName)
		if msg.repoName == m.selectedRepoName() {
			return m, m.startLoadSelectedRepo()
		}

		repo, ok := m.cfg.RepoByName(msg.repoName)
		if ok {
			m.releaseRefreshLock(msg.repoName)
			return m, loadRepoStatCmd(m.clean, repo, false)
		}
		m.releaseRefreshLock(msg.repoName)
		return m, nil

	case tea.KeyMsg:
		if m.isQuitKey(msg) {
			return m, tea.Quit
		}

		if m.startupLoading {
			return m, nil
		}

		if m.refreshLocked {
			return m, nil
		}

		if m.searchMode {
			switch msg.String() {
			case "enter", "esc":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			default:
				prevRepoIdx := m.repoIdx
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)

				if m.focus == focusRepos {
					indices := m.visibleRepoIndices()
					if len(indices) > 0 {
						found := false
						for _, idx := range indices {
							if idx == m.repoIdx {
								found = true
								break
							}
						}
						if !found {
							m.repoIdx = indices[0]
						}
					}
					if m.repoIdx != prevRepoIdx {
						m.ensureRepoCursorVisible()
						m.activateSelectedRepoFromCache()
						return m, cmd
					}
					m.ensureRepoCursorVisible()
				} else {
					m.clampBranchCursor(m.activeRepo.RepoName)
					m.ensureBranchCursorVisible(m.activeRepo.RepoName)
				}
				return m, cmd
			}
		}

		if m.confirmType != confirmNone {
			return m.updateConfirm(msg)
		}

		switch msg.String() {
		case "f2":
			m.showInfo = !m.showInfo
			m.statusLine = fmt.Sprintf("Инфо-панель: %s", onOff(m.showInfo))
			return m, nil
		case "f6":
			if m.focus == focusBranches {
				m.toggleBranchSortMode()
				return m, nil
			}
			m.toggleRepoSortMode()
			return m, nil
		case "tab":
			return m, nil
		case "f4":
			if m.focus != focusBranches {
				return m, nil
			}
			m.toggleBranchScope()
			return m, nil
		case "f9":
			if m.focus != focusBranches {
				return m, nil
			}
			m.toggleProtectedFilter()
			return m, nil
		case "f3":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case "f7":
			if m.focus == focusBranches {
				return m, m.startCreateLocalCopyFromCurrentRemoteBranch()
			}
			if m.focus == focusRepos {
				return m, m.startFetchAndPullActiveRepo()
			}
			return m, nil
		case "g", "f8":
			if !m.openConfirmIfPossible() {
				m.statusLine = "Нет выбранных веток для генерации скрипта"
				return m, nil
			}
			return m, nil
		}

		if m.focus == focusRepos {
			return m.updateReposPanel(msg)
		}
		return m.updateBranchesPanel(msg)
	}

	return m, nil
}

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.confirmType = confirmNone
		m.statusLine = "Генерация скрипта отменена"
		return m, nil
	case "left", "right", "t":
		if m.scriptFormat == model.ScriptFormatSH {
			m.scriptFormat = model.ScriptFormatBAT
		} else {
			m.scriptFormat = model.ScriptFormatSH
		}
		return m, nil
	case "enter", "y", "f8":
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
			return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
				err := m.clean.ForceCheckoutLocalBranch(repo, m.checkoutTarget)
				return checkoutCompletedMsg{repoName: repo.Name, err: err}
			})
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
		key := m.branchSelectionKey(branch)
		selected[key] = !selected[key]
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

// View формирует экран с табами Репозитории/Ветки.
func (m Model) View() string {
	if len(m.cfg.Repos) == 0 {
		return appStyle.Render("Нет репозиториев в конфиге")
	}

	if m.startupLoading {
		return appStyle.Render(m.viewStartupScreen())
	}

	usableWidth := max(64, m.width-2)
	contentHeight := max(8, m.height-4)

	body := m.viewReposTab(usableWidth, contentHeight)
	if m.focus == focusBranches {
		body = m.viewBranchesTab(usableWidth, contentHeight)
	}

	topMenu := m.viewTopMenu(m.width)
	header := titleStyle.Width(m.width).Render(m.viewTitleLine())
	statusLine := m.viewContextLine(m.width)
	hotkeys := m.viewHotkeyBar(m.width)

	view := lipgloss.JoinVertical(lipgloss.Left, topMenu, header, body, statusLine, hotkeys)
	if m.confirmType != confirmNone {
		view = m.viewConfirmModal(view)
	}

	return appStyle.Render(view)
}

func (m Model) viewReposTab(width, height int) string {
	if !m.showInfo {
		return m.viewReposPanel(width, height)
	}

	infoHeight := min(14, max(8, height/3))
	repoHeight := max(7, height-infoHeight-1)
	ruler := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(strings.Repeat("─", width))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewReposPanel(width, repoHeight),
		ruler,
		m.viewStatsPanel(width, infoHeight),
	)
}

func (m Model) viewBranchesTab(width, height int) string {
	if !m.showInfo {
		return m.viewBranchesPanel(width, height)
	}

	infoHeight := min(14, max(8, height/3))
	branchesHeight := max(7, height-infoHeight-1)
	ruler := lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(strings.Repeat("─", width))

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewBranchesPanel(width, branchesHeight),
		ruler,
		m.viewStatsPanel(width, infoHeight),
	)
}

func (m Model) viewReposPanel(width, height int) string {
	style := panelStyle.Width(width).Height(height)
	if m.focus == focusRepos {
		style = panelFocusedStyle.Width(width).Height(height)
	}

	compact := width < 40
	const branchWidth = 16
	const typeWidth = 10
	nameWidth := max(8, width-branchWidth-typeWidth-11)

	var lines []string
	lines = append(lines, panelTitleStyle.Width(width).Render(" РЕПОЗИТОРИИ "))
	if compact {
		compactNameWidth := max(8, width-14)
		lines = append(lines, panelHeaderStyle.Width(width).Render(fmt.Sprintf(" %-*s %-8s", compactNameWidth, "Репозиторий", "Ветка/ст")))
	} else {
		lines = append(lines, panelHeaderStyle.Width(width).Render(fmt.Sprintf(" %-*s %-*s %-*s %s", nameWidth, "Репозиторий", branchWidth, "Активная ветка", typeWidth, "Источник", "Ст")))
	}

	indices := m.visibleRepoIndices()
	rows := m.repoRowsForPanelHeight(height)
	pos := m.repoVisiblePosition(indices)
	offset := adjustedViewportOffset(m.repoOffset, pos, len(indices), rows)
	start := offset
	end := min(len(indices), start+rows)

	for _, idx := range indices[start:end] {
		repo := m.cfg.Repos[idx]
		cursor := " "
		if idx == m.repoIdx {
			cursor = cursorStyle.Render(">")
		}

		source := repoSourceCode(repo)
		branch, status := m.repoListState(repo.Name)
		statusStr := m.renderRepoStatus(status)

		displayBranch := branch
		if displayBranch != "" && displayBranch != "-" {
			displayBranch = "@" + displayBranch
		}

		var rowText string
		if compact {
			compactNameWidth := max(8, width-14)
			padName := fitCell(repo.Name, compactNameWidth)
			branchCell := fitCell(displayBranch+" "+m.repoStatusCode(status), 8)
			rowText = fmt.Sprintf("%s%s %-8s", cursor, padName, branchCell)
		} else {
			padName := fitCell(repo.Name, nameWidth)
			padBranch := fitCell(displayBranch, branchWidth)
			srcShort := fitCell(source, typeWidth)
			rowText = fmt.Sprintf("%s%s %s %-*s %s", cursor, padName, padBranch, typeWidth, srcShort, statusStr)
		}

		if idx == m.repoIdx {
			rowText = selectedStyle.Width(width).Render(rowText)
		}
		lines = append(lines, rowText)
	}

	if len(indices) == 0 {
		lines = append(lines, mutedStyle.Render("Ничего не найдено"))
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) viewBranchesPanel(width, height int) string {
	style := panelStyle.Width(width).Height(height)
	if m.focus == focusBranches {
		style = panelFocusedStyle.Width(width).Height(height)
	}

	compact := width < 58
	const dateWidth = 10
	const jiraWidth = 17
	const jiraStatusWidth = 20
	const mergeWidth = 9
	const typeWidth = 2
	const keepWidth = 2
	const fixedCols = 2 + 2 + 1 + dateWidth + 1 + jiraWidth + 1 + jiraStatusWidth + 1 + mergeWidth + 1 + typeWidth + 1 + keepWidth
	branchNameWidth := max(12, width-4-fixedCols)
	if compact {
		branchNameWidth = max(10, width-18)
	}

	var lines []string
	lines = append(lines, panelTitleStyle.Width(width).Render(" ВЕТКИ "))

	if m.loadingSelectedRepo() {
		lines = append(lines, fmt.Sprintf("%s Загрузка веток...", m.spinner.View()))
		lines = append(lines, mutedStyle.Render("Асинхронная загрузка веток (таймаут контролируется)"))
		return style.Render(strings.Join(lines, "\n"))
	}

	if stat, ok := m.selectedRepoStat(); ok && stat.HasError() {
		lines = append(lines, errorStyle.Render("Ошибка загрузки веток"))
		lines = append(lines, truncate(stat.LoadError, max(16, width-4)))
		return style.Render(strings.Join(lines, "\n"))
	}

	if m.activeRepo.RepoName == "" {
		lines = append(lines, mutedStyle.Render("Репозиторий не загружен"))
		return style.Render(strings.Join(lines, "\n"))
	}

	if m.activeRepo.RepoName != m.selectedRepoName() {
		lines = append(lines, mutedStyle.Render("Ветки для выбранного репозитория не загружены"))
		lines = append(lines, mutedStyle.Render("Нажмите F5 или r для загрузки"))
		return style.Render(strings.Join(lines, "\n"))
	}

	hdrBranch := fitCell("Ветка", branchNameWidth)
	if compact {
		lines = append(lines, panelHeaderStyle.Width(width).Render(fmt.Sprintf(" S %s %-9s", hdrBranch, "Слияние")))
	} else {
		lines = append(lines, panelHeaderStyle.Width(width).Render(fmt.Sprintf(" S %s %-10s %-17s %-20s %-9s %s %s", hdrBranch, "Дата", "JIRA", "Статус", "Слияние", "T", "З")))
	}

	visible := m.visibleBranches()
	repoName := m.activeRepo.RepoName
	cursor := m.currentCursor(repoName)
	rows := m.branchRowsForPanelHeight(height)
	offset := adjustedViewportOffset(m.branchOffset[repoName], cursor, len(visible), rows)
	start := offset
	end := min(len(visible), start+rows)

	for i, br := range visible[start:end] {
		absoluteIdx := start + i
		isActiveRow := absoluteIdx == cursor
		cursorStr := " "
		if isActiveRow {
			cursorStr = ">"
		}

		marker := " "
		if m.isSelected(repoName, m.branchSelectionKey(br)) {
			marker = branchMarkerStyle.Render("✓")
		} else if br.IsRemote() {
			marker = "·"
		}

		merge := m.renderMergeCell(br.MergeStatus, mergeWidth, isActiveRow)
		padName := fitCell(m.branchListName(br), branchNameWidth)
		line := ""
		if compact {
			line = fmt.Sprintf("%s %s %s %s", cursorStr, marker, padName, merge)
		} else {
			jira := valueOrDash(br.JiraKey)
			if jira == "-" {
				jira = "--"
			}
			jiraStatusCell := m.renderJiraStatusCell(br, jiraStatusWidth, isActiveRow)
			date := br.LastCommitAt.Format("2006-01-02")
			padJira := fitCell(jira, jiraWidth)
			keep := "-"
			if br.Protected {
				keep = "K"
			}
			branchType := "L"
			if br.IsRemote() {
				branchType = "R"
			}
			line = fmt.Sprintf("%s %s %s %-10s %s %s %s %-2s %-2s", cursorStr, marker, padName, date, padJira, jiraStatusCell, merge, branchType, keep)
		}

		if isActiveRow {
			line = selectedStyle.Width(width).Render(line)
		}

		lines = append(lines, line)
	}

	if len(visible) == 0 {
		lines = append(lines, mutedStyle.Render("Нет веток для отображения"))
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) viewStatsPanel(width, height int) string {
	style := infoStyle.Width(width).Height(height)
	contentWidth := max(12, width-6)
	selectedRepo := m.selectedRepoName()
	selectedRepoCfg, hasSelectedRepoCfg := m.cfg.RepoByName(selectedRepo)
	selectedCount := len(m.selectedBranches(selectedRepo))
	visibleCount := len(m.visibleBranches())

	labels := []struct{ key, val string }{
		{"Репозиториев", fmt.Sprintf("%d", len(m.cfg.Repos))},
		{"Веток", fmt.Sprintf("%d", len(m.activeRepo.Branches))},
		{"Видимых", fmt.Sprintf("%d", visibleCount)},
		{"Выбрано", fmt.Sprintf("%d", selectedCount)},
		{"Scope", m.branchScopeLabel()},
		{"Сорт репо", m.repoSortCodeLabel()},
		{"Сорт веток", m.branchSortCodeLabel()},
	}

	summaryParts := make([]string, 0, len(labels))
	for _, l := range labels {
		summaryParts = append(summaryParts, l.key+": "+l.val)
	}
	summary := strings.Join(summaryParts, "  |  ")

	lines := []string{
		panelTitleStyle.Width(width).Render(" ИНФО  " + truncate(summary, width-9)),
		truncate(fmt.Sprintf("Репозиторий: %s", valueOrDash(selectedRepo)), contentWidth),
		truncate(fmt.Sprintf("Источник: %s | Scope: %s | Скрытое: %s | Формат: .%s", valueOrDash(repoSourceLabel(selectedRepoCfg, hasSelectedRepoCfg)), m.branchScopeLabel(), onOff(m.hideProtected), m.scriptFormat), contentWidth),
	}

	if m.focus == focusBranches && m.activeRepo.RepoName == selectedRepo && m.hasLoadedBranches(selectedRepo) {
		current := m.currentBranch()
		if current != nil {
			jiraLine := truncate(fmt.Sprintf("Курсор: %s | Тип: %s | JIRA: %s | Статус Jira: %s | Слияние: %s", m.branchDisplayName(*current), m.branchTypeLabel(*current), valueOrDash(current.JiraKey), m.jiraStatusWithIndicator(*current), m.mergeStatusLabel(current.MergeStatus)), contentWidth)
			lines = append(lines, m.jiraInfoStyle(*current).Render(jiraLine))
			lines = append(lines, truncate(fmt.Sprintf("Jira group: %s | Ссылка: %s", valueOrDash(current.JiraGroup), valueOrDash(current.JiraTicketURL)), contentWidth))
			lines = append(lines, truncate(m.jiraStateText(*current), contentWidth))
		}
	}

	if stat, ok := m.selectedRepoStat(); ok {
		lines = append(lines, "", panelHeaderStyle.Width(width).Render(" Статус Git "))
		if stat.HasError() {
			lines = append(lines, errorStyle.Render("Ошибка доступа к репозиторию"))
			lines = append(lines, "  "+truncate(stat.LoadError, contentWidth))
		} else {
			lines = append(lines, truncate(fmt.Sprintf("Текущая ветка: %s", valueOrDash(stat.CurrentBranch)), contentWidth))
			if stat.HasSyncWarning() {
				lines = append(lines, warnStyle.Render(truncate("Синхронизация: предупреждение", contentWidth)))
				lines = append(lines, "  "+truncate(stat.SyncWarning, contentWidth))
			}
			st := stat.DirtyStats
			if !st.HasChanges() {
				lines = append(lines, cleanStyle.Render("Рабочее дерево: чисто"))
			} else {
				lines = append(lines, dirtyStyle.Render(truncate("Рабочее дерево: "+dirtySummary(st), contentWidth)))
				var dirtyFiles []string
				for _, f := range st.Modified {
					dirtyFiles = append(dirtyFiles, "  M "+f)
				}
				for _, f := range st.Added {
					dirtyFiles = append(dirtyFiles, "  A "+f)
				}
				for _, f := range st.Deleted {
					dirtyFiles = append(dirtyFiles, "  D "+f)
				}
				for _, f := range st.Untracked {
					dirtyFiles = append(dirtyFiles, "  ? "+f)
				}

				maxToShow := 10
				if len(dirtyFiles) > maxToShow {
					for i := 0; i < maxToShow; i++ {
						lines = append(lines, truncate(dirtyFiles[i], width-4))
					}
					lines = append(lines, mutedStyle.Render(fmt.Sprintf("  ... еще %d файлов", len(dirtyFiles)-maxToShow)))
				} else {
					for _, f := range dirtyFiles {
						lines = append(lines, truncate(f, width-4))
					}
				}
			}
		}
	} else {
		lines = append(lines, "", panelHeaderStyle.Width(width).Render(" Статус Git "), mutedStyle.Render("Нет данных (ветки не загружены)"))
	}

	if m.loadingSelectedRepo() {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("%s Синхронизация репозитория (при недоступности будет ошибка таймаута)", m.spinner.View()))
	}

	if m.lastGenerated != nil {
		lines = append(lines, "")
		lines = append(lines, panelHeaderStyle.Width(width).Render(" Последний скрипт "))
		lines = append(lines, truncate(filepath.Base(m.lastGenerated.ScriptPath), max(10, width-4)))
	}

	if m.err != nil {
		lines = append(lines, "")
		lines = append(lines, panelHeaderStyle.Width(width).Render(" Ошибка "))
		lines = append(lines, truncate(m.err.Error(), max(16, width-4)))
	}

	return style.Render(strings.Join(lines, "\n"))
}

func (m Model) viewTopMenu(width int) string {
	reposTab := tabInactiveStyle.Render("[ Репозитории ]")
	branchesTab := tabInactiveStyle.Render("[ Ветки ]")
	if m.focus == focusRepos {
		reposTab = tabActiveStyle.Render("[ Репозитории ]")
	} else {
		branchesTab = tabActiveStyle.Render("[ Ветки ]")
	}

	enterHint := "Enter - открыть репозиторий | F6 Сорт: " + m.repoSortCodeLabel()
	if m.focus == focusBranches {
		enterHint = fmt.Sprintf("Enter - checkout ветки | F4 Scope: %s | F6 Сорт: %s", m.branchScopeCodeLabel(), m.branchSortCodeLabel())
	}

	line := " " + reposTab + " " + branchesTab + "  " + enterHint + " "
	if lipgloss.Width(line) > max(16, width-2) {
		if m.focus == focusRepos {
			line = " [РЕПОЗИТОРИИ] [ветки] "
		} else {
			line = " [репозитории] [ВЕТКИ] "
		}
	}
	return topMenuStyle.Width(width).Render(line)
}

func (m Model) viewTitleLine() string {
	if m.searchMode || m.searchInput.Value() != "" {
		return truncate(fmt.Sprintf(" Поиск (F3) / Фильтр: %s", m.searchInput.View()), max(24, m.width-4))
	}
	if m.loadingSelectedRepo() {
		return truncate(fmt.Sprintf(" go-repo-orchestrator | %s загрузка: %s ", m.spinner.View(), m.cfg.Repos[m.repoIdx].Name), max(24, m.width-4))
	}
	return truncate("Выбор ветки перед формированием скрипта .sh/.bat для удаления", max(24, m.width-4))
}

func (m Model) viewContextLine(width int) string {
	if m.searchMode || m.searchInput.Value() != "" {
		return statusStyle.Width(width).Render(truncate(fmt.Sprintf(" ПОИСК: %s_ | Enter-применить, Esc-отмена", m.searchInput.Value()), width))
	}

	contextText := ""
	contextStyle := statusStyle
	if m.err != nil {
		contextText = "Ошибка: " + m.err.Error()
	} else if m.startupWarn != "" {
		contextText = m.startupWarn
	} else if m.statusLine != "" {
		contextText = m.statusLine
	}

	if m.focus == focusRepos {
		repo := m.cfg.Repos[m.repoIdx]
		branch, status := m.repoListState(repo.Name)
		ctx := fmt.Sprintf("Репозиторий: %s | Источник: %s | Ветка: %s | Статус: %s", repo.Name, repoSourceLabel(repo, true), valueOrDash(branch), status)
		if contextText == "" {
			contextText = ctx
		} else {
			contextText = contextText + " | " + ctx
		}
	} else {
		branch := m.currentBranch()
		if branch == nil {
			if contextText == "" {
				contextText = "Ветка не выбрана"
			}
		} else {
			contextStyle = m.jiraContextStyle(*branch)
			ctx := fmt.Sprintf("Ветка: %s | Тип: %s | JIRA: %s | Jira-статус: %s | Jira-состояние: %s | Ссылка: %s | Слияние: %s", m.branchDisplayName(*branch), m.branchTypeLabel(*branch), valueOrDash(branch.JiraKey), m.jiraStatusWithIndicator(*branch), m.jiraStateLabel(*branch), valueOrDash(branch.JiraTicketURL), m.mergeStatusLabel(branch.MergeStatus))
			if contextText == "" {
				contextText = ctx
			} else {
				contextText = contextText + " | " + ctx
			}
		}
	}

	return contextStyle.Width(width).Render(truncate(contextText, max(24, width-4)))
}

func (m Model) viewHotkeyBar(width int) string {
	repoSelected := len(m.cfg.Repos) > 0
	canCheckout := m.canCheckoutBranch()
	canCreateLocal := m.canCreateLocalFromRemote()

	var items []string
	if m.focus == focusRepos {
		items = []string{
			m.renderHotkeyItem("Enter", "Ветки", repoSelected),
			m.renderHotkeyItem("F2", "Контекст", true),
			m.renderHotkeyItem("F3", "Поиск", true),
			m.renderHotkeyItem("F5/r", "Рескан", repoSelected),
			m.renderHotkeyItem("F6", "Сорт: "+m.repoSortCodeLabel(), true),
			m.renderHotkeyItem("F7", "Fetch+Pull", repoSelected),
			m.renderHotkeyItem("F10/q", "Выход", true),
		}
	} else {
		items = []string{
			m.renderHotkeyItem("Esc", "Репо", true),
			m.renderHotkeyItem("Ins/Space", "Выбор", true),
			m.renderHotkeyItem("Enter", "Чекаут", canCheckout),
			m.renderHotkeyItem("F3", "Поиск", true),
			m.renderHotkeyItem("F4", "Scope: "+m.branchScopeCodeLabel(), true),
			m.renderHotkeyItem("F5/r", "Обновить", true),
			m.renderHotkeyItem("F6", "Сорт: "+m.branchSortCodeLabel(), true),
			m.renderHotkeyItem("F7", "Клонировать", canCreateLocal),
			m.renderHotkeyItem("F8/g", "Скрипт", m.canGenerateScript()),
			m.renderHotkeyItem("F9", "Скрытое", true),
			m.renderHotkeyItem("F10/q", "Выход", true),
		}
	}
	line := ""
	for _, item := range items {
		candidate := line + item
		if lipgloss.Width(candidate) > width {
			break
		}
		line = candidate
	}
	if line == "" {
		line = m.renderHotkeyItem("F10/q", "Выход", true)
	}
	return hotkeyStyle.Width(width).Render(line)
}

func (m Model) renderHotkeyItem(key, text string, active bool) string {
	if !active {
		return hotkeyInactiveNumStyle.Render(key) + hotkeyInactiveTextStyle.Render(text)
	}
	return hotkeyNumStyle.Render(key) + hotkeyTextStyle.Render(text)
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

func (m Model) viewConfirmModal(base string) string {
	var text []string

	switch m.confirmType {
	case confirmGenerate:
		branches := m.selectedBranches(m.activeRepo.RepoName)
		text = []string{
			"[ ПОДТВЕРЖДЕНИЕ ] Генерация скрипта",
			"",
			fmt.Sprintf("Репозиторий: %s", m.activeRepo.RepoName),
			fmt.Sprintf("Путь: %s", m.activeRepo.RepoPath),
			fmt.Sprintf("Выбрано веток: %d", len(branches)),
			fmt.Sprintf("Формат: .%s (t/left/right)", m.scriptFormat),
			"",
			"F8/Enter/y - создать скрипт",
			"Esc/n   - отмена",
		}
	case confirmCheckout:
		text = []string{
			"[ ВНИМАНИЕ ] Принудительное переключение",
			"",
			fmt.Sprintf("Репозиторий: %s", m.activeRepo.RepoName),
			fmt.Sprintf("Целевая ветка: %s", m.checkoutTarget),
			"",
			warnStyle.Render("ВНИМАНИЕ: Все незакоммиченные изменения"),
			warnStyle.Render("будут безвозвратно удалены!"),
			"",
			"Enter/y - выполнить checkout -f",
			"Esc/n   - отмена",
		}
	}

	modal := modalStyle.Width(min(72, m.width-6)).Render(strings.Join(text, "\n"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, lipgloss.JoinVertical(lipgloss.Left, base, modal))
}

func (m *Model) startLoadActiveRepo() tea.Cmd {
	repo := m.cfg.Repos[m.repoIdx]
	return m.startLoadRepo(repo, true, false)
}

func (m *Model) startLoadSelectedRepo() tea.Cmd {
	if len(m.cfg.Repos) == 0 || m.repoIdx < 0 || m.repoIdx >= len(m.cfg.Repos) {
		return nil
	}

	return m.startLoadRepo(m.cfg.Repos[m.repoIdx], true, false)
}

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
				cmds = append(cmds, loadRepoStatCmd(m.clean, repo, startup))
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

func (m *Model) startLoadRepo(repo config.RepoConfig, manual, startup bool) tea.Cmd {
	m.repoLoadReq[repo.Name]++
	requestID := m.repoLoadReq[repo.Name]
	m.repoLoading[repo.Name] = true
	m.err = nil

	if startup {
		m.setStartupProgressStatus()
	} else if manual {
		m.statusLine = fmt.Sprintf("Обновление %q...", repo.Name)
		m.refreshLocked = true
		m.refreshAll = false
		for repoName := range m.refreshPending {
			delete(m.refreshPending, repoName)
		}
		m.refreshRepo = repo.Name
		m.refreshReqID = requestID
	} else {
		m.statusLine = fmt.Sprintf("Загрузка %q...", repo.Name)
	}

	if repo.Name == m.selectedRepoName() {
		m.activeRepo = model.RepoBranches{RepoName: repo.Name, RepoSource: repo.SourceType()}
		return tea.Batch(m.spinner.Tick, loadRepoBranchesCmd(m.clean, repo, requestID, startup))
	}

	return loadRepoBranchesCmd(m.clean, repo, requestID, startup)
}

// loadRepoBranchesCmd запускает загрузку веток репозитория и отправляет поэтапные события в лог.
func loadRepoBranchesCmd(cleaner *usecase.Cleaner, repo config.RepoConfig, requestID int, startup bool) tea.Cmd {
	if !startup {
		// В режиме не startup — простой вызов без поэтапных событий
		return func() tea.Msg {
			rb, err := cleaner.LoadRepoBranches(repo)
			return branchesLoadedMsg{requestID: requestID, repoName: repo.Name, rb: rb, err: err, startup: false}
		}
	}

	// В режиме startup — шлем поэтапные события через tea.Batch
	stageGit := func() tea.Msg {
		return startupLogMsg{fmt.Sprintf("[GIT] %s: получение веток...", repo.Name)}
	}
	loadAndReport := func() tea.Msg {
		rb, err := cleaner.LoadRepoBranches(repo)
		if err != nil {
			return branchesLoadedMsg{requestID: requestID, repoName: repo.Name, rb: rb, err: err, startup: true}
		}
		jiraResolved := 0
		for _, b := range rb.Branches {
			if b.JiraKey != "-" && b.JiraKey != "" && b.JiraStatus != "-" && b.JiraStatus != "" {
				jiraResolved++
			}
		}
		syncNote := ""
		if rb.SyncWarning != "" {
			syncNote = " [из кэша]"
		}
		return branchesLoadedMsg{
			requestID:    requestID,
			repoName:     repo.Name,
			rb:           rb,
			err:          nil,
			startup:      true,
			jiraResolved: jiraResolved,
			syncNote:     syncNote,
		}
	}
	return tea.Batch(stageGit, loadAndReport)
}

func loadRepoStatCmd(cleaner *usecase.Cleaner, repo config.RepoConfig, startup bool) tea.Cmd {
	return func() tea.Msg {
		stat, err := cleaner.LoadRepoStat(repo)
		return repoStatLoadedMsg{repoName: repo.Name, stat: stat, err: err, startup: startup}
	}
}

func generateScriptCmd(cleaner *usecase.Cleaner, repo config.RepoConfig, repoPath string, branches []model.BranchInfo, format model.ScriptFormat) tea.Cmd {
	return func() tea.Msg {
		result, err := cleaner.GenerateDeleteScript(repo, repoPath, branches, format)
		return scriptGeneratedMsg{result: result, err: err}
	}
}

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

func (m Model) renderMergeCell(status model.MergeStatus, width int, highlighted bool) string {
	if width < 3 {
		width = 3
	}

	if highlighted {
		return lipgloss.NewStyle().Width(width).Render(m.mergeStatusShort(status))
	}

	switch status {
	case model.MergeStatusMerged:
		return lipgloss.NewStyle().Width(width).Render(mergedStyle.Render("слита"))
	case model.MergeStatusUnmerged:
		return lipgloss.NewStyle().Width(width).Render(unmergedStyle.Render("не слита"))
	default:
		return lipgloss.NewStyle().Width(width).Render(unknownStyle.Render("неизв."))
	}
}

func (m Model) mergeStatusShort(status model.MergeStatus) string {
	switch status {
	case model.MergeStatusMerged:
		return "слита"
	case model.MergeStatusUnmerged:
		return "не слита"
	default:
		return "неизв."
	}
}

func (m Model) mergeStatusLabel(status model.MergeStatus) string {
	switch status {
	case model.MergeStatusMerged:
		return "слита"
	case model.MergeStatusUnmerged:
		return "не слита"
	default:
		return "неизвестно"
	}
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

func (m Model) renderJiraStatusCell(branch model.BranchInfo, width int, highlighted bool) string {
	if width < 3 {
		width = 3
	}

	text := fitCell(m.jiraStatusDisplay(branch), width)
	if highlighted {
		return text
	}

	return m.jiraStatusStyle(branch).Render(text)
}

func (m Model) jiraStatusDisplay(branch model.BranchInfo) string {
	state := m.jiraState(branch)
	switch state {
	case model.JiraStatusStateTransient:
		return "недост."
	case model.JiraStatusStateLoading:
		return "недост."
	case model.JiraStatusStateAuth:
		return "auth"
	case model.JiraStatusStateError:
		return "ошибка"
	case model.JiraStatusStateUnmapped:
		return "-"
	default:
		return valueOrDash(branch.JiraStatus)
	}
}

func (m Model) jiraStatusStyle(branch model.BranchInfo) lipgloss.Style {
	state := m.jiraState(branch)
	switch state {
	case model.JiraStatusStateLoading:
		return jiraMutedStyle
	case model.JiraStatusStateTransient:
		return jiraWarningStyle
	case model.JiraStatusStateAuth:
		return jiraAuthStyle
	case model.JiraStatusStateError:
		return errorStyle
	case model.JiraStatusStateUnmapped:
		return jiraMutedStyle
	}

	switch jiraStatusSortBucket(branch.JiraStatus) {
	case 0:
		return jiraDoneStyle
	case 1:
		return jiraTestingStyle
	case 2:
		return jiraActiveStyle
	case 3:
		return jiraOpenStyle
	default:
		return jiraMutedStyle
	}
}

func (m Model) jiraStatusWithIndicator(branch model.BranchInfo) string {
	return fmt.Sprintf("%s [%s]", m.jiraStatusDisplay(branch), m.jiraStateIndicator(branch))
}

func (m Model) jiraStateLabel(branch model.BranchInfo) string {
	switch m.jiraState(branch) {
	case model.JiraStatusStateTransient:
		return "временно недоступно"
	case model.JiraStatusStateLoading:
		return "временно недоступно"
	case model.JiraStatusStateAuth:
		switch branch.JiraReason {
		case model.JiraStatusReasonForbidden:
			return "нет доступа"
		case model.JiraStatusReasonLoginRequired:
			return "требуется вход"
		default:
			return "нужна авторизация"
		}
	case model.JiraStatusStateError:
		switch branch.JiraReason {
		case model.JiraStatusReasonIssueNotFound:
			return "тикет не найден"
		case model.JiraStatusReasonClientError:
			return "ошибка запроса"
		default:
			return "ошибка Jira"
		}
	case model.JiraStatusStateUnmapped:
		return "нет mapping"
	default:
		return "ok"
	}
}

func (m Model) jiraStateIndicator(branch model.BranchInfo) string {
	switch m.jiraState(branch) {
	case model.JiraStatusStateTransient:
		return "!"
	case model.JiraStatusStateLoading:
		return "!"
	case model.JiraStatusStateAuth:
		return "A"
	case model.JiraStatusStateError:
		return "E"
	case model.JiraStatusStateUnmapped:
		return "-"
	default:
		return "OK"
	}
}

func (m Model) jiraStateText(branch model.BranchInfo) string {
	return fmt.Sprintf("Jira-состояние: %s (%s)", m.jiraStateLabel(branch), m.jiraReasonLabel(branch))
}

func (m Model) jiraReasonLabel(branch model.BranchInfo) string {
	switch branch.JiraReason {
	case model.JiraStatusReasonRegexKeyOnly:
		return "ключ извлечен только из regex"
	case model.JiraStatusReasonNoRegexMatch:
		return "regex не совпал"
	case model.JiraStatusReasonNoMapping:
		return "нет link mapping"
	case model.JiraStatusReasonNoGroupConfig:
		return "group не настроена"
	case model.JiraStatusReasonInvalidRequest:
		return "некорректный запрос"
	case model.JiraStatusReasonTemporarilyDown:
		return "временный сбой"
	case model.JiraStatusReasonAuthRequired:
		return "требуется авторизация (401)"
	case model.JiraStatusReasonForbidden:
		return "доступ запрещен (403)"
	case model.JiraStatusReasonLoginRequired:
		return "нужен вход (redirect/login/html)"
	case model.JiraStatusReasonIssueNotFound:
		return "тикет не найден (404)"
	case model.JiraStatusReasonClientError:
		return "клиентская ошибка HTTP (4xx)"
	case model.JiraStatusReasonHTTPError:
		return "ошибка HTTP ответа"
	case model.JiraStatusReasonTransportError:
		return "ошибка транспорта"
	case model.JiraStatusReasonResponseParseErr:
		return "некорректный JSON-ответ Jira"
	case model.JiraStatusReasonBrowserUnavailableHTTPFallback:
		return "browser недоступен, использован HTTP fallback"
	case model.JiraStatusReasonBrowserUnavailableHTTPAuthRequired:
		return "browser недоступен, HTTP требует авторизацию"
	case model.JiraStatusReasonBrowserUnavailableHTTPError:
		return "browser недоступен, ошибка HTTP/сети"
	case model.JiraStatusReasonNone:
		return "нет"
	default:
		return "нет"
	}
}

func (m Model) jiraState(branch model.BranchInfo) model.JiraStatusState {
	if branch.JiraState != "" {
		return branch.JiraState
	}

	if valueOrDash(branch.JiraKey) == "-" || valueOrDash(branch.JiraTicketURL) == "-" {
		return model.JiraStatusStateUnmapped
	}

	return model.JiraStatusStateReady
}

func (m Model) jiraInfoStyle(branch model.BranchInfo) lipgloss.Style {
	return m.jiraStatusStyle(branch)
}

func (m Model) jiraContextStyle(branch model.BranchInfo) lipgloss.Style {
	state := m.jiraState(branch)
	// Create a fresh copy with explicit background to avoid mutating the global statusStyle var.
	base := lipgloss.NewStyle().Background(mcCyan).Foreground(mcBlack).Padding(0, 1)

	switch state {
	case model.JiraStatusStateAuth:
		return base.Foreground(lipgloss.Color("196")).Bold(true)
	case model.JiraStatusStateTransient:
		return base.Foreground(mcYellow).Bold(true)
	case model.JiraStatusStateError:
		return base.Foreground(lipgloss.Color("196"))
	case model.JiraStatusStateUnmapped:
		return base.Foreground(mcGray)
	case model.JiraStatusStateLoading:
		return base.Foreground(mcGray)
	default:
		switch jiraStatusSortBucket(branch.JiraStatus) {
		case 0:
			return base.Foreground(lipgloss.Color("46"))
		case 1:
			return base.Foreground(mcYellow)
		case 2:
			return base.Foreground(mcBrightCyan)
		case 3:
			return base.Foreground(mcLightGray)
		default:
			return base.Foreground(mcGray)
		}
	}
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

func onOff(v bool) string {
	if v {
		return "вкл"
	}
	return "выкл"
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

func (m Model) viewEventLogPanel(width, height int) string {
	logBg := lipgloss.Color("232")
	logFg := lipgloss.Color("250")
	dimFg := lipgloss.Color("241")
	borderFg := lipgloss.Color("238")

	style := lipgloss.NewStyle().
		Background(logBg).
		Foreground(logFg).
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderFg).
		Padding(0, 1).
		Width(width).
		Height(height)

	titleSt := lipgloss.NewStyle().
		Background(lipgloss.Color("236")).
		Foreground(logFg).
		Bold(true).
		Padding(0, 1)

	innerWidth := max(0, width-4)   // border + padding
	innerHeight := max(0, height-4) // border + padding + title line

	var lines []string
	lines = append(lines, titleSt.Width(innerWidth).Render(" ЛОГА СОБЫТИЙ "))

	if len(m.eventLog) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(dimFg).Background(logBg).Render("  (нет событий)"))
	} else {
		start := 0
		if len(m.eventLog) > innerHeight {
			start = len(m.eventLog) - innerHeight
		}
		for i, entry := range m.eventLog[start:] {
			idx := start + i
			numStr := lipgloss.NewStyle().Foreground(dimFg).Background(logBg).Render(fmt.Sprintf("%3d ", idx+1))
			entryStr := lipgloss.NewStyle().Foreground(logFg).Background(logBg).Render(truncate(entry, innerWidth-4))
			lines = append(lines, numStr+entryStr)
		}
	}

	content := strings.Join(lines, "\n")
	return style.Render(content)
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

func (m Model) isQuitKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "ctrl+c", "q", "f10":
		return true
	default:
		return msg.Type == tea.KeyF10
	}
}

func (m *Model) toggleProtectedFilter() {
	m.hideProtected = !m.hideProtected
	m.clampBranchCursor(m.activeRepo.RepoName)
	m.ensureBranchCursorVisible(m.activeRepo.RepoName)
	m.statusLine = fmt.Sprintf("Скрытое: %s", onOff(m.hideProtected))
}

func isInvertSelectionKey(msg tea.KeyMsg) bool {
	key := strings.ToLower(msg.String())
	switch key {
	case "*", "kp*", "kp_multiply", "keypad*", "keypad_multiply", "numpad*", "numpad_multiply":
		return true
	}

	return msg.Type == tea.KeyRunes && len(msg.Runes) == 1 && msg.Runes[0] == '*'
}

func isHomeNavigationKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyHome, tea.KeyCtrlHome, tea.KeyShiftHome, tea.KeyCtrlShiftHome, tea.KeyCtrlA:
		return true
	}

	key := normalizeNavigationKeyName(msg.String())
	if strings.HasSuffix(key, "+home") {
		return true
	}
	if isRawHomeNavigationSequence(key) {
		return true
	}
	switch key {
	case "home", "ctrl+a", "kp_home", "keypad_home", "numpad_home", "find", "pos1", "kp7", "kp_7", "keypad7", "keypad_7", "numpad7", "numpad_7":
		return true
	}

	return false
}

func isEndNavigationKey(msg tea.KeyMsg) bool {
	switch msg.Type {
	case tea.KeyEnd, tea.KeyCtrlEnd, tea.KeyShiftEnd, tea.KeyCtrlShiftEnd, tea.KeyCtrlE:
		return true
	}

	key := normalizeNavigationKeyName(msg.String())
	if strings.HasSuffix(key, "+end") {
		return true
	}
	if isRawEndNavigationSequence(key) {
		return true
	}
	switch key {
	case "end", "ctrl+e", "kp_end", "keypad_end", "numpad_end", "select", "kp1", "kp_1", "keypad1", "keypad_1", "numpad1", "numpad_1":
		return true
	}

	return false
}

func normalizeNavigationKeyName(key string) string {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	normalized = strings.ReplaceAll(normalized, " ", "_")
	if strings.HasPrefix(normalized, "esc[") {
		normalized = "[" + strings.TrimPrefix(normalized, "esc[")
	} else if strings.HasPrefix(normalized, "esco") {
		normalized = "o" + strings.TrimPrefix(normalized, "esco")
	}
	normalized = strings.TrimPrefix(normalized, "esc+")
	for strings.HasPrefix(normalized, "\x1b") {
		normalized = strings.TrimPrefix(normalized, "\x1b")
	}
	normalized = strings.TrimPrefix(normalized, "^")
	if strings.HasPrefix(normalized, "[[") {
		normalized = strings.TrimPrefix(normalized, "[")
	}
	return normalized
}

func isRawHomeNavigationSequence(key string) bool {
	switch key {
	case "[h", "oh", "[1~", "[7~":
		return true
	default:
		return false
	}
}

func isRawEndNavigationSequence(key string) bool {
	switch key {
	case "[f", "of", "[4~", "[8~":
		return true
	default:
		return false
	}
}

func (m *Model) startCreateLocalCopyFromCurrentRemoteBranch() tea.Cmd {
	if !m.canCreateLocalFromRemote() {
		if !m.canUseF7ForActiveRepo() {
			m.statusLine = "Создание локальной копии доступно только для репозиториев с path"
		} else {
			m.statusLine = "Создание локальной копии доступно только для удаленной ветки"
		}
		return nil
	}

	repo, ok := m.cfg.RepoByName(m.activeRepo.RepoName)
	if !ok {
		m.err = errors.New("репозиторий не найден в конфигурации")
		return nil
	}

	branch := m.currentBranch()
	if branch == nil {
		m.statusLine = "Ветка не выбрана"
		return nil
	}

	m.repoLoading[repo.Name] = true
	m.statusLine = fmt.Sprintf("Создание локальной копии %s...", branch.Name)

	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		err := m.clean.CreateLocalTrackingBranch(repo, branch.Name, branch.QualifiedName)
		return localCopyCompletedMsg{repoName: repo.Name, branch: branch.Name, err: err}
	})
}

func (m *Model) startFetchAndPullActiveRepo() tea.Cmd {
	if len(m.cfg.Repos) == 0 || m.repoIdx < 0 || m.repoIdx >= len(m.cfg.Repos) {
		m.statusLine = "Нет активного репозитория"
		return nil
	}

	repo := m.cfg.Repos[m.repoIdx]
	m.repoLoading[repo.Name] = true
	m.err = nil
	m.statusLine = fmt.Sprintf("Обновление %q через fetch + pull...", repo.Name)
	m.refreshLocked = true
	m.refreshAll = false
	for repoName := range m.refreshPending {
		delete(m.refreshPending, repoName)
	}
	m.refreshRepo = repo.Name
	m.refreshReqID = 0

	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		err := m.clean.FetchAndPullRepo(repo)
		return repoFetchPullCompletedMsg{repoName: repo.Name, err: err}
	})
}

func (m *Model) openConfirmIfPossible() bool {
	if len(m.selectedBranches(m.activeRepo.RepoName)) == 0 {
		return false
	}
	m.confirmType = confirmGenerate
	return true
}

func (m *Model) activateSelectedRepoFromCache() {
	repoName := m.selectedRepoName()
	if repoName == "" {
		return
	}

	rb, ok := m.repoData[repoName]
	if !ok {
		m.activeRepo = model.RepoBranches{RepoName: repoName}
		return
	}

	m.activeRepo = rb
	m.ensureRepoState(repoName)
	m.clampBranchCursor(repoName)
	m.ensureBranchCursorVisible(repoName)
}

func (m Model) hasLoadedBranches(repoName string) bool {
	_, ok := m.repoData[repoName]
	return ok
}

func (m Model) loadingSelectedRepo() bool {
	repoName := m.selectedRepoName()
	if repoName == "" {
		return false
	}

	return m.repoLoading[repoName]
}

func (m Model) canUseF7ForActiveRepo() bool {
	repo, ok := m.cfg.RepoByName(m.activeRepo.RepoName)
	if !ok {
		return false
	}
	return strings.TrimSpace(repo.Path) != ""
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

func (m Model) viewStartupScreen() string {
	usableW := max(40, m.width-4)
	usableH := max(12, m.height-2)

	// --- Progress header block
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

	// --- Event log block (takes remaining space)
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
	logBg := lipgloss.Color("17") // темно-синий (чуть темнее основного)
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
	innerH := max(1, height-4) // border top+bot, header, blank

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
