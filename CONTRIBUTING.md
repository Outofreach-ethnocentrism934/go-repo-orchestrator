# Contributing

## Conventional Commits

В репозитории используется спецификация [Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).

Формат сообщения коммита:

```text
<type>[optional scope][!]: <description>
```

Примеры валидных сообщений:

- `feat: add Jira status badges`
- `fix(tui): handle empty branch list`
- `refactor(git)!: remove legacy remote sync path`
- `docs: update contributing flow`

Рекомендуемые типы: `feat`, `fix`, `docs`, `refactor`, `test`, `build`, `ci`, `chore`, `perf`, `revert`, `style`.

### Локальная автоматическая проверка (Git Hooks)

Чтобы включить проверку сообщений коммитов локально (до их пуша), выполните в корне проекта:

```bash
make setup-hooks
```

Это добавит локальный Git-hook (`commit-msg`), который будет применять Go-утилиту `commitlint` для проверки формата сообщения при каждом `git commit`.

## Требование к PR title

Заголовок Pull Request должен соответствовать тому же формату Conventional Commits.

Примеры валидных PR title:

- `feat: add conventional commits validation workflow`
- `ci: enforce golangci and commit format checks`

## Какие проверки блокируют merge

Для Pull Request запускаются обязательные проверки:

- `pr-title` — проверка заголовка PR на Conventional Commits.
- `commit-messages` — проверка сообщений коммитов в PR на Conventional Commits.
- `go-checks` — проверка форматирования, тестов, `go vet` и `golangci-lint` (конфиг `.golangci.yml`).

Если любая из этих проверок падает, PR нельзя мерджить.

## Branch protection (настройка вручную)

В GitHub settings для целевой ветки включите branch protection и добавьте Required status checks:

- `pr-title`
- `commit-messages`
- `go-checks`

Без этого workflow будет показывать ошибки, но merge может остаться доступным в зависимости от настроек репозитория.
