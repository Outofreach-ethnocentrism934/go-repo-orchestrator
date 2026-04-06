export default {
  extends: ['@commitlint/config-conventional'],
  ignores: [
    (message) => message.includes('dependabot'),
    (message) => message.includes('CodeRabbit'),
  ],
  rules: {
    // Максимальная длина заголовка коммита: 100 символов
    // Синхронизировано с .commitlint.yml (Go commitlint)
    'header-max-length': [2, 'always', 100],

    // Отключаем ограничение длины body (боты и URL часто превышают 100)
    'body-max-line-length': [0],
  }
};
