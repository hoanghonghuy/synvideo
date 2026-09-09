import js from '@eslint/js'
import vue from 'eslint-plugin-vue'
import tseslint from 'typescript-eslint'
import vueTsEslintConfig from '@vue/eslint-config-typescript'

const nodeScriptGlobals = {
  URL: 'readonly',
  console: 'readonly',
  process: 'readonly',
} as const

export default [
  {
    name: 'synvideo/ignores',
    ignores: ['dist/**', 'coverage/**'],
  },
  {
    name: 'synvideo/scripts-node',
    files: ['scripts/**/*.{js,mjs,cjs}'],
    languageOptions: {
      ecmaVersion: 'latest',
      sourceType: 'module',
      globals: nodeScriptGlobals,
    },
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...vue.configs['flat/recommended'],
  ...vueTsEslintConfig(),
]
