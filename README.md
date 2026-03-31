<div align="center">
  <h1>🚀 go-repo-orchestrator</h1>
  <p><b>Локальная TUI-утилита для оркестрации репозиториев, аудита задач и безопасной работы с Git-ветками</b></p>
  
  [![Go version](https://img.shields.io/github/go-mod/go-version/AgelxNash/go-repo-orchestrator)](https://github.com/AgelxNash/go-repo-orchestrator/blob/main/go.mod)
  [![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-fe5196.svg)](https://www.conventionalcommits.org/en/v1.0.0/)
  ![Go Report Card](https://goreportcard.com/badge/github.com/AgelxNash/go-repo-orchestrator)
  [![CI](https://github.com/AgelxNash/go-repo-orchestrator/actions/workflows/ci.yml/badge.svg)](https://github.com/AgelxNash/go-repo-orchestrator/actions/workflows/ci.yml)
  [![Release workflow](https://github.com/AgelxNash/go-repo-orchestrator/actions/workflows/release.yaml/badge.svg)](https://github.com/AgelxNash/go-repo-orchestrator/actions/workflows/release.yaml)
  [![Latest release](https://img.shields.io/github/v/release/AgelxNash/go-repo-orchestrator)](https://github.com/AgelxNash/go-repo-orchestrator/releases)
  [![Go Reference](https://pkg.go.dev/badge/github.com/AgelxNash/go-repo-orchestrator.svg)](https://pkg.go.dev/github.com/AgelxNash/go-repo-orchestrator)
  [![License](https://img.shields.io/github/license/AgelxNash/go-repo-orchestrator)](https://github.com/AgelxNash/go-repo-orchestrator/blob/main/LICENSE)
</div>

<br/>

> **go-repo-orchestrator** — это ваш персональный пульт управления хаосом в микросервисах. Инструмент решает проблемы долгого онбординга, позволяет проводить сквозной поиск по веткам, следить за реальными статусами задач из Jira и генерировать безопасные скрипты для удаления мусорных веток в интерактивном TUI.

<div align="center">
  <br>
  <video src="https://github.com/AgelxNash/go-repo-orchestrator/raw/main/.github/assets/demo.webm?raw=true" width="100%" controls></video>
  <br>
  <i>Демонстрация работы приложения</i>
</div>

---

## 📖 Оглавление
- [🔍 Проблема и Решение](#-проблема-и-решение)
- [✨ Скрытые фичи и возможности](#-скрытые-фичи-и-возможности)
- [🚀 Быстрый старт](#-быстрый-старт)
- [💻 Установка](#-установка)
- [⌨️ Горячие клавиши](#️-горячие-клавиши)
- [⚙️ Конфигурация](#️-конфигурация)
- [🤝 Contributing](#-contributing)

---

## 🔍 Проблема и Решение

При работе с десятками микросервисов разработчик тратит кучу времени на рутину: долгий онбординг, сложные переключения (cd) между папками, потерю контекста статусов из таск-трекера и страх случайно удалить нужную ветку командой `git branch -D`.

**go-repo-orchestrator решает эти боли, объединяя управление всеми репозиториями в одном интерфейсе.**

| Боль | ❌ Обычный подход | ✅ go-repo-orchestrator |
|---|---|---|
| **Долгий онбординг** | Ручное клонирование 50+ репо по папкам | Запуск с общим `config.yaml` автоматически вытягивает и раскладывает все проекты |
| **Поиск и навигация** | Бесконечные `cd`, `git branch`, IDE | Глобальный поиск веток и переход (Checkout) кликом мыши или нажатием `Enter` |
| **Зависающие задачи** | Искать статусы вручную | Сквозной мониторинг статусов из Jira напрямую в TUI (легко найти отмененные задачи) |
| **Смена Jira/Проектов** | Неудобно отслеживать разные домены | Поддержка одновременной работы сразу с **несколькими** инстансами Jira |
| **Опасность `git branch -D`**| Страх удалить чужой код / production | TUI-выбор с предпросмотром, генерацией скрипта и `branch.keep` regex защитой |

### 🔒 Безопасное удаление по умолчанию
Деструктивные команды не выполняются "под капотом". Вы выбираете ветки в TUI, после чего генерируется `.sh` / `.bat` скрипт, который вы сможете проанализировать и запустить вручную. К тому же, утилита аппаратно защищает текущую активную ветку и системные ветки (по умолчанию: `main|master|prod|release`).

---

## ✨ Скрытые фичи и возможности

- 🚀 **Мгновенный онбординг (Workspace-менеджер)**: Поделитесь одним YAML-файлом конфигурации с командой. При запуске оркестратор сам склонирует все недостающие репозитории и разложит их по правильным директориям, экономя часы новичкам.
- 👁️ **Глубокая интеграция с Jira**: 
  - Оркестратор мониторит не только факт того, что ветка слита, но и **реальные статусы задач**. 
  - Вы сразу поймете, что задача висит "В ревью", даже если Merge Request ещё никто не открыл.
  - Мгновенное выявление заброшенных или отмененных из-за смены приоритетов задач для очистки локального мусора.
  - **Multi-Jira**: Работа с несколькими Jira-инстансами одновременно (идеально для ситуаций, когда часть проектов "переезжает" на новые сервера).
- 🔀 **Удобная навигация и Checkout**: Смена активных веток одним нажатием `Enter`. Вам больше не нужно "прыгать" через терминалы и IDE по папкам.
- 🔎 **Сквозной поиск**: Глобальный поиск нужных веток и репозиториев прямо из TUI. Незаменимая фича, когда в рамках одной задачи вы модифицируете 5-7 разных репозиториев.
- 🗑️ **Генерация команд очистки**: Формирует пачки `git branch -D` для локальных веток и `git push <remote> --delete` для удаленных с учетом строгих правил безопасности.
- 🌐 **CDP & Playwright-мосты**: Дополнительный транспорт через Chromium для сложных Jira-групп с защитой (Cloudflare/Captchas).

---

## 🚀 Быстрый старт

**Требования:**
- Go `1.24+`
- Установленный `git` в `$PATH`

Существует два основных сценария работы с конфигурацией оркестратора:

### Сценарий 1: С готовым конфингом (Онбординг)
Идеально, если в вашей команде уже есть подготовленный файл конфигурации. Оркестратор автоматически скачает (через `git clone`) недостающие репозитории по URL в нужную структуру.

```bash
# 1. Скачиваем конфигурацию (в качестве примера возьмем базовый шаблон)
curl -O https://raw.githubusercontent.com/AgelxNash/go-repo-orchestrator/main/config.example.yaml

# 2. Запускаем оркестратор
go run ./cmd/go-repo-orchestrator --config config.example.yaml
```

### Сценарий 2: Генерация нового конфига
Если вы внедряете утилиту в свой проект с нуля, создайте чистый шаблон для заполнения:

```bash
# 1. Генерируем новый файл
go run ./cmd/go-repo-orchestrator generate --config ./my-repo.gbc.yaml

# 2. Отредактируйте my-repo.gbc.yaml, добавив свои пути и настройки.

# 3. Запускаем оркестратор
go run ./cmd/go-repo-orchestrator --config ./my-repo.gbc.yaml
```

Для просмотра полной справки по CLI:
```bash
go run ./cmd/go-repo-orchestrator --help
```

---

## 💻 Установка

**Из исходников:**
```bash
make build
./bin/go-repo-orchestrator --config ./config.example.yaml
```

**Через go install:**
```bash
go install github.com/agelxnash/go-repo-orchestrator/cmd/go-repo-orchestrator@latest
```

*Для скачивания готовых бинарных файлов посетите раздел [GitHub Releases](https://github.com/AgelxNash/go-repo-orchestrator/releases).*

---

## ⌨️ Горячие клавиши

#### Основные
- **`F2`** — Показать/скрыть нижнюю панель информации.
- **`F3`** — Поиск (Глобально по веткам и проектам).
- **`F4`** — Область веток (`Локальные` → `Удаленные` → `Все`).
- **`F5`** (или `r`) — Обновить контекст текущей вкладки.
- **`F6`** — Сортировка.
- **`F8`** (или `g`) — Генерация скрипта.
- **`F10`** (или `q` / `Ctrl+C`) — Выход из приложения.

#### Вкладка «Репозитории»
- **`Enter`**: Открыть ветки для активного репозитория.
- **`F7`**: Выполнить `fetch + pull` для активного репозитория.

#### Вкладка «Ветки»
- **`Enter`**: Сделать `checkout` на выбранную ветку без необходимости открывать терминал.
- **`Пробел`** / **`Insert`**: Отметить ветку для скрипта удаления.
- **`F7`**: Создать локальную tracking-копию удаленной ветки (если репозиторий не в режиме `url`-only).
- **`F9`**: Скрыть/показать защищенные ветки.

---

## ⚙️ Конфигурация

Все настройки по умолчанию и пример заполнения находятся в шаблоне `config.example.yaml`, который вы можете сгенерировать командой `generate` (см. Быстрый старт). Обязательным требованием является передача файла конфигурации через флаг `--config`.

**Ключевые секции конфига:**
- `repos[].name`, `repos[].url`, `repos[].path` — базовые настройки репозитория (поддержка `url`, `path`, `url+path`).
- `repos[].branch.keep` — regex-выражение для системных/защищенных веток.
- `repos[].branch.jira` — regex-выражение для извлечения Jira ключа (например, `[A-Z]+-\d+`).
- `jira[]` — настройки интеграции с Jira (`group`, `url`, `playwright`, `type`, `token`/`login`).
- `browser.cdp_url` — подключение к внешнему Chromium через CDP.

*Можно переопределять конфигурацию через переменные окружения с префиксом `GBC_` (например, `GBC_STATE_DIR`).*

### Каталог состояния
Утилита хранит свое состояние (и выкачанные workspace-репозитории) в:
- **Linux/macOS:** `$HOME/.local/state/go-repo-orchestrator`
- **Fallback:** `.go-repo-orchestrator-state`

Клонированные workspace хранятся по пути: `<state-dir>/workspace/<repo-name>__<url-hash>/`.

### О генерации скриптов очистки
Генерируемые `.sh`/`.bat` создаются в вашей текущей рабочей директории терминала в формате:
- `go-repo-orchestrator-<repo>-delete-<session>-<timestamp>.sh`

---

## 🤝 Contributing

Будем рады вашему вкладу! Для ознакомления с правилами *Conventional Commits*, требованиями к *Pull Requests* и обязательными проверками, пожалуйста, прочитайте [CONTRIBUTING.md](CONTRIBUTING.md).

<details>
<summary><b>🛠️ Подготовка окружения (Onboarding)</b></summary>

Быстрый onboarding перед первым коммитом:

```bash
make commitlint-install
make golangci-lint-install
make setup-hooks
```
*Команды установят необходимые утилиты (`commitlint`, `golangci-lint`) и пропишут настройки `core.hooksPath=.githooks` для локальных quality gates (`pre-commit` и `pre-push`).*

**Быстрые локальные проверки (manual call):**
```bash
make fmt-check
make vet
make check
```
</details>

<details>
<summary><b>📦 Информация о релизах (Для Maintainers)</b></summary>

Релизный workflow находится в `.github/workflows/release.yaml`.
- **Триггер:** push тега с префиксом `v*`.
- Используется `GoReleaser` + `GPG signing` checksum-файла.

**Проверка подписи скачанных артефактов в релизе:**
```bash
gpg --verify checksums.txt.sig checksums.txt
sha256sum -c checksums.txt
```
</details>

---

## 📈 Roadmap

- Введение полноценного i18n-слоя для CLI/TUI и пользовательских сообщений.
- Потенциальная опция автооткрытия IDE (VS Code / JetBrains) для выбранного репозитория и/или ветки из интерфейса.

---

## ⭐ Star History

<div align="center">
  <a href="https://www.star-history.com/?repos=AgelxNash%2Fgo-repo-orchestrator&type=date&legend=top-left">
    <picture>
      <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/image?repos=AgelxNash/go-repo-orchestrator&type=date&theme=dark&legend=top-left" />
      <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/image?repos=AgelxNash/go-repo-orchestrator&type=date&legend=top-left" />
      <img alt="Star History Chart" src="https://api.star-history.com/image?repos=AgelxNash/go-repo-orchestrator&type=date&legend=top-left" />
    </picture>
  </a>
</div>

## 👥 Contributors

<div align="center">
  <a href="https://github.com/AgelxNash/go-repo-orchestrator/graphs/contributors">
    <img src="https://contrib.rocks/image?repo=AgelxNash/go-repo-orchestrator" alt="Contributors" />
  </a>
  <br/>
  <i>Made with <a href="https://contrib.rocks">contrib.rocks</a>.</i>
</div>

---

## 📄 License

**MIT** © [AgelxNash](https://github.com/AgelxNash)

<div align="center">
  <br/>
  <a href="https://github.com/AgelxNash/go-repo-orchestrator">
    <img src="https://github-view-counter.vercel.app/api?username=AgelxNash/go-repo-orchestrator&label=views&color=0969da&labelColor=555555" alt="Views"/>
  </a>
</div>
