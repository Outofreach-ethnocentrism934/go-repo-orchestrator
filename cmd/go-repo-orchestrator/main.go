package main

import (
	"os"
	"strings"

	"github.com/agelxnash/go-repo-orchestrator/internal/app"
	"github.com/agelxnash/go-repo-orchestrator/internal/cli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	logger, err := app.NewProductionLogger()
	if err != nil {
		_, _ = os.Stderr.WriteString("init logger: " + err.Error() + "\n")
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	if err := cli.NewRootCommand(version, commit, date, logger).Execute(); err != nil {
		_, _ = os.Stderr.WriteString(formatUserError(err) + "\n")
		os.Exit(1)
	}
}

func formatUserError(err error) string {
	if err == nil {
		return ""
	}

	msg := strings.TrimSpace(err.Error())
	msg = strings.ReplaceAll(msg, "\n", " ")

	replacements := []struct {
		from string
		to   string
	}{
		{"read config:", "Ошибка чтения конфигурации:"},
		{"resolve config path:", "Ошибка пути к конфигурации:"},
		{"create config directory:", "Ошибка создания директории конфигурации:"},
		{"prepare repository:", "Ошибка подготовки репозитория:"},
		{"scan directory:", "Ошибка сканирования директории:"},
		{"marshal config:", "Ошибка формирования конфигурации:"},
		{"write config:", "Ошибка записи конфигурации:"},
	}

	for _, r := range replacements {
		msg = strings.ReplaceAll(msg, r.from, r.to)
	}

	if strings.TrimSpace(msg) == "" {
		return "Ошибка: неизвестная ошибка"
	}

	if strings.HasPrefix(msg, "Ошибка") {
		return msg
	}

	return "Ошибка: " + msg
}
