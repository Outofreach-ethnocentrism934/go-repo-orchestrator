# go-repo-orchestrator

[![Release workflow](https://github.com/AgelxNash/go-repo-orchestrator/actions/workflows/release.yaml/badge.svg)](https://github.com/AgelxNash/go-repo-orchestrator/actions/workflows/release.yaml)
[![CI](https://github.com/AgelxNash/go-repo-orchestrator/actions/workflows/ci.yml/badge.svg)](https://github.com/AgelxNash/go-repo-orchestrator/actions/workflows/ci.yml)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-fe5196.svg)](https://www.conventionalcommits.org/en/v1.0.0/)
[![Latest release](https://img.shields.io/github/v/release/AgelxNash/go-repo-orchestrator)](https://github.com/AgelxNash/go-repo-orchestrator/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/AgelxNash/go-repo-orchestrator)](https://github.com/AgelxNash/go-repo-orchestrator/blob/main/go.mod)
[![License](https://img.shields.io/github/license/AgelxNash/go-repo-orchestrator)](https://github.com/AgelxNash/go-repo-orchestrator/blob/main/LICENSE)

Локальная TUI-утилита для безопасной подготовки удаления Git-веток через генерацию скриптов (`.sh`/`.bat`).

## Назначение

`go-repo-orchestrator` не удаляет ветки напрямую из интерфейса. Приложение формирует скрипт удаления, который пользователь запускает самостоятельно.

## Ключевые возможности

- TUI в стиле FAR/MC для работы с репозиториями и ветками.
- Режимы источников репозитория: `url`, `path`, `url+path`.
- Безопасные правила отбора веток (current/default/`branch.keep` защищены).
- Генерация команд:
  - локальные ветки: `git branch -d` / `git branch -D`
  - удаленные ветки: `git push <remote> --delete <branch>`
- Опциональная Jira status интеграция по regex key extraction и batch search API.
- Опциональный Playwright transport для Jira-групп с `playwright: true` (включается только если хотя бы одна Jira-группа его требует).
- Логирование в stdout для runtime отключено намеренно, чтобы не ломать TUI-вывод.

## Требования

- Go `1.24+`
- `git` в `PATH`

## Быстрый старт

`--config` обязателен для всех CLI-команд.

```bash
go run ./cmd/go-repo-orchestrator --config ./config.example.yaml
```

Генерация нового конфига:

```bash
go run ./cmd/go-repo-orchestrator generate --config ./my-repo.gbc.yaml
```

Справка CLI:

```bash
go run ./cmd/go-repo-orchestrator --help
```

## Установка

Из исходников:

```bash
make build
./bin/go-repo-orchestrator --config ./config.example.yaml
```

Через `go install`:

```bash
go install github.com/agelxnash/go-repo-orchestrator/cmd/go-repo-orchestrator@latest
```

Для релизных бинарников используйте GitHub Releases этого репозитория.

## Горячие клавиши (основные)

- `F2` — показать/скрыть нижнюю info-панель
- `F3` — поиск
- `F4` — scope веток (`локальные` → `удаленные` → `все`)
- `F5` — обновление (контекстно по табу)
- `r` — алиас `F5`
- `F6` — сортировка
- `F7`:
  - в табе `Репозитории`: `fetch + pull` активного репозитория
  - в табе `Ветки`: локальная tracking-копия удаленной ветки (доступно не для `url`-only репозитория)
- `F8` — генерация скрипта
- `g` — алиас `F8`
- `F9` — скрыть/показать защищенные ветки (таб `Ветки`)
- `Enter`:
  - в табе `Репозитории`: открыть таб `Ветки` для активного репозитория
  - в табе `Ветки`: checkout локальной ветки
- `Space` / `Insert` — выбрать ветку
- `F10` / `q` / `Ctrl+C` — выход

## Формат конфига

См. `config.example.yaml`.

Коротко по ключам:

- `repos[].name`, `repos[].url`, `repos[].path`
- `repos[].branch.keep` — regex защищенных веток
- `repos[].branch.jira` — regex извлечения Jira key
- `jira[]` — Jira-группы (`group`, `url`, `playwright`, `type`, `token`/`login`)
- `browser.cdp_url` — подключение к внешнему Chromium через CDP

Для ENV override используется префикс `GBC_` (например, `GBC_STATE_DIR`).

## Каталог состояния

- Linux/macOS: `$HOME/.local/state/go-repo-orchestrator`
- Fallback: `.go-repo-orchestrator-state`

Workspace managed clone:

- `<state-dir>/workspace/<repo-name>__<url-hash>/`

## Генерируемые скрипты

Скрипты удаления создаются с префиксом имени проекта:

- `go-repo-orchestrator-<repo>-delete-<session>-<timestamp>.sh`
- `go-repo-orchestrator-<repo>-delete-<session>-<timestamp>.bat`

## Release и подписи (maintainers)

Релизный workflow: `.github/workflows/release.yaml`.

- Триггер: push тега `v*`
- Используется GoReleaser + GPG signing checksum-файла
- Версия по умолчанию для dev-сборок: `dev`; релизные метаданные прошиваются через `ldflags` в `.goreleaser.yaml`
- В workflow есть preflight-проверки (тег, секреты, `go test`, `go vet`, `goreleaser check`)

GitHub Secrets для релиза:

- `GPG_PRIVATE_KEY` — приватный ключ для подписи
- `GPG_FINGERPRINT` — fingerprint ключа
- `GPG_PASSPHRASE` — passphrase ключа (опционально, если ключ защищен passphrase)

Проверка подписи после скачивания релизных артефактов:

```bash
gpg --verify checksums.txt.sig checksums.txt
sha256sum -c checksums.txt
```

## Подготовка окружения для контрибьютора

Рекомендуемый минимальный onboarding перед первым push:

```bash
make commitlint-install
make golangci-lint-install
make setup-hooks
```

Что настраивается:

- `make commitlint-install` — устанавливает бинарь `commitlint` (если он еще не установлен).
- `make golangci-lint-install` — устанавливает бинарь `golangci-lint` (если он еще не установлен).
- `make setup-hooks` — настраивает `core.hooksPath=.githooks` и включает локальные хуки:
  - `commit-msg`: проверка формата сообщения коммита (Conventional Commits).
  - `pre-commit`: быстрые quality gates (`make fmt-check` и `make vet`).
  - `pre-push`: полный локальный quality gate (`make check`) перед отправкой в remote.

## Локальная проверка качества

```bash
gofmt -w ./cmd ./internal
go test $(go list ./cmd/... ./internal/...)
go vet $(go list ./cmd/... ./internal/...)
go build -o ./bin/go-repo-orchestrator ./cmd/go-repo-orchestrator
golangci-lint run --timeout=5m $(go list ./cmd/... ./internal/...)
```

Эквивалентно через `Makefile`:

```bash
make test
make build
make check
```

Быстрые шаги вручную (как в хуках):

```bash
make fmt-check
make vet
make check
```

## Contributing

Для правил по Conventional Commits, требованиям к PR title и обязательным проверкам перед merge см. `CONTRIBUTING.md`.
