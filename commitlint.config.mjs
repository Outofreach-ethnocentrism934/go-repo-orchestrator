export default {
  extends: ['@commitlint/config-conventional'],
  ignores: [
    (message) => message.includes('dependabot'),
    (message) => message.includes('CodeRabbit'),
  ],
  rules: {
    // Disable max line length for the commit body because bots and URLs often exceed 100 chars
    'body-max-line-length': [0],
  }
};
