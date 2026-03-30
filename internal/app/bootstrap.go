package app

import (
	"os"
	"path/filepath"
	"time"

	"github.com/agelxnash/go-repo-orchestrator/internal/config"

	"go.uber.org/zap"
)

const (
	DefaultConfigPath = "config.yaml"
	DefaultEnvPrefix  = "GBC"
	DefaultGitTimeout = 30 * time.Second
)

// RuntimeOptions описывает опции инициализации runtime-зависимостей.
type RuntimeOptions struct {
	StateDir      string
	WorkspaceDir  string
	GitTimeout    time.Duration
	BrowserCDPURL string
	JiraGroups    []config.JiraConfig
	Logger        *zap.Logger
}

// DefaultRuntimeOptions возвращает предсказуемый набор runtime-настроек.
func DefaultRuntimeOptions(stateDir string) RuntimeOptions {
	if stateDir == "" {
		stateDir = DefaultStateDir()
	}

	return RuntimeOptions{
		StateDir:     stateDir,
		WorkspaceDir: DefaultWorkspaceDir(stateDir),
		GitTimeout:   DefaultGitTimeout,
	}
}

// DefaultStateDir возвращает директорию состояния по умолчанию.
func DefaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".go-repo-orchestrator-state"
	}

	return filepath.Join(home, ".local", "state", "go-repo-orchestrator")
}

// DefaultWorkspaceDir возвращает директорию managed clone репозиториев.
func DefaultWorkspaceDir(stateDir string) string {
	if stateDir == "" {
		stateDir = DefaultStateDir()
	}

	return filepath.Join(stateDir, "workspace")
}

// NewProductionLogger создает стандартный (пустой) production logger,
// чтобы фоновые предупреждения (например, ошибки Jira 400 Bad Request)
// не ломали отрисовку TUI, записывая данные в stdout поверх интерфейса.
func NewProductionLogger() (*zap.Logger, error) {
	return zap.NewNop(), nil
}
