export default {
  extends: ['@commitlint/config-conventional'],
  ignores: [
    (message) => message.includes('dependabot'),
    (message) => message.includes('CodeRabbit'),
  ],
  rules: {
    // Минимальная длина заголовка коммита: 10 символов
    // Синхронизировано с .commitlint.yml (Go commitlint)
    'header-min-length': [2, 'always', 10],

    // Максимальная длина заголовка коммита: 100 символов
    // Синхронизировано с .commitlint.yml (Go commitlint)
    'header-max-length': [2, 'always', 100],

    // Максимальная длина строки в footer: 100 символов
    // Синхронизировано с .commitlint.yml (Go commitlint)
    'footer-max-line-length': [2, 'always', 100],

    // Отключаем ограничение длины body (боты и URL часто превышают 100)
    'body-max-line-length': [0],

    // Разрешённые типы коммитов
    // Синхронизировано с .commitlint.yml (Go commitlint)
    'type-enum': [2, 'always', [
      'feat',
      'fix',
      'docs',
      'style',
      'refactor',
      'perf',
      'test',
      'build',
      'ci',
      'chore',
      'revert',
    ]],
  }
};