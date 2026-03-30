package tui

import "github.com/charmbracelet/lipgloss"

// Более мягкие цвета в стиле MC/FAR
const (
	// Основные - используем ANSI 256 для "винтажности"
	mcBlue      = lipgloss.Color("19")  // Тёмно-синий ANSI (вместо #0000AA)
	mcDarkBlue  = lipgloss.Color("18")  // Ещё темнее
	mcYellow    = lipgloss.Color("220") // Мягкий жёлтый (вместо #AAAA00)
	mcCyan      = lipgloss.Color("37")  // Приглушённый cyan (вместо #00AAAA)
	mcGray      = lipgloss.Color("247") // Серый (вместо #AAAAAA)
	mcWhite     = lipgloss.Color("255") // Почти белый
	mcBlack     = lipgloss.Color("0")   // Чёрный
	mcLightGray = lipgloss.Color("252") // Светло-серый (вместо #C0C0C0)

	// Дополнительные оттенки
	mcDarkGray   = lipgloss.Color("235")
	mcBrightCyan = lipgloss.Color("51")
	mcHeaderBlue = lipgloss.Color("24")
	mcRed        = lipgloss.Color("196")
)

var (
	// Основной фон приложения - тёмно-синий
	appStyle = lipgloss.NewStyle().
			Foreground(mcLightGray).
			Background(mcBlue)

	// Верхнее меню - светло-серый фон, чёрный текст
	topMenuStyle = lipgloss.NewStyle().
			Foreground(mcLightGray).
			Background(mcDarkBlue).
			Bold(true)

	tabActiveStyle = lipgloss.NewStyle().
			Foreground(mcBlack).
			Background(mcCyan).
			Bold(true).
			Padding(0, 2).
			Underline(true)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(mcGray).
				Background(mcDarkBlue).
				Padding(0, 2)

	// Заголовок окна
	titleStyle = lipgloss.NewStyle().
			Foreground(mcWhite).
			Background(mcDarkBlue).
			Padding(0, 1).
			Bold(true)

	// Панель по умолчанию
	panelStyle = lipgloss.NewStyle().
			Foreground(mcLightGray).
			Background(mcBlue).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mcGray).
			Padding(0, 1)

	// Активная панель - с яркой границей
	panelFocusedStyle = lipgloss.NewStyle().
				Foreground(mcWhite).
				Background(mcBlue).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(mcBrightCyan).
				Padding(0, 1)

	// Заголовок панели (РЕПОЗИТОРИИ, ВЕТКИ и т.д.)
	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(mcBlack).
			Background(mcCyan).
			Padding(0, 1).
			Underline(true)

	// Заголовки колонок в таблицах
	panelHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(mcWhite).
				Background(mcHeaderBlue).
				Padding(0, 1)

	// Инфо-панель (правая)
	infoStyle = lipgloss.NewStyle().
			Foreground(mcLightGray).
			Background(mcBlue).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(mcGray).
			Padding(0, 1)

	// Модальное окно
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(mcCyan).
			Foreground(mcWhite).
			Background(mcBlue).
			Padding(1, 2)

	// Строка статуса (нижняя)
	statusStyle = lipgloss.NewStyle().
			Foreground(mcBlack).
			Background(mcCyan).
			Padding(0, 1)

	// Панель горячих клавиш (самый низ)
	hotkeyStyle = lipgloss.NewStyle().
			Foreground(mcBlack).
			Background(mcLightGray)

	// Номера горячих клавиш
	hotkeyNumStyle = lipgloss.NewStyle().
			Foreground(mcBlack).
			Background(mcYellow).
			Bold(true).
			Padding(0, 1)

	// Текст горячих клавиш
	hotkeyTextStyle = lipgloss.NewStyle().
			Foreground(mcBlack).
			Background(mcLightGray).
			Padding(0, 1)

	// Неактивные номера горячих клавиш
	hotkeyInactiveNumStyle = lipgloss.NewStyle().
				Foreground(mcGray).
				Background(mcDarkGray).
				Bold(true).
				Padding(0, 1)

	// Неактивный текст горячих клавиш
	hotkeyInactiveTextStyle = lipgloss.NewStyle().
				Foreground(mcGray).
				Background(mcDarkGray).
				Padding(0, 1)

	// Приглушённый текст (подсказки)
	mutedStyle = lipgloss.NewStyle().
			Foreground(mcGray)

	// Ошибки
	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")) // Красный

	// Маркер выбранной ветки
	branchMarkerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("46")).
				Bold(true)

	// Предупреждения
	warnStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(mcYellow)

	// Выбранная строка
	selectedStyle = lipgloss.NewStyle().
			Foreground(mcBlack).
			Background(mcCyan).
			Bold(true)

	// Курсор (стрелка >)
	cursorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(mcYellow)

	// Статусы слияния (без фона, чтобы наследовать фон выделенной строки)
	mergedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")) // Зелёный

	unmergedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(mcYellow)

	unknownStyle = lipgloss.NewStyle().
			Foreground(mcGray)

	dirtyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196")) // Красный

	cleanStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")) // Зелёный

	jiraDoneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46"))

	jiraTestingStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(mcYellow)

	jiraActiveStyle = lipgloss.NewStyle().
			Foreground(mcBrightCyan)

	jiraOpenStyle = lipgloss.NewStyle().
			Foreground(mcLightGray)

	jiraMutedStyle = lipgloss.NewStyle().
			Foreground(mcGray)

	jiraAuthStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

	jiraWarningStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(mcYellow)
)
