import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import tseslint from 'typescript-eslint'
import designSystem from './eslint-rules/design-system.mjs'

const designRules = {
  'design-system/no-raw-elements': 'error',
  'design-system/no-radix-import': 'error',
  'design-system/ui-barrel-import': 'error',
  'design-system/no-arbitrary-values': 'error',
  'design-system/no-hex-colors': 'error',
  'design-system/no-style-color': 'error',
}

export default tseslint.config(
  { ignores: ['dist', 'node_modules', 'coverage'] },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [js.configs.recommended, ...tseslint.configs.recommended],
    languageOptions: { ecmaVersion: 2022, globals: globals.browser },
    plugins: { 'react-hooks': reactHooks },
    rules: {
      ...reactHooks.configs.recommended.rules,
      '@typescript-eslint/no-unused-vars': ['error', { argsIgnorePattern: '^_' }],
    },
  },
  {
    // Business code: pages, app shell, api. Builds UI from @/ui and tokens only.
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/ui/**'],
    plugins: { 'design-system': designSystem },
    rules: designRules,
  },
  {
    files: ['**/*.{js,mjs}'],
    extends: [js.configs.recommended],
    languageOptions: { ecmaVersion: 2022, sourceType: 'module', globals: globals.node },
  },
)
