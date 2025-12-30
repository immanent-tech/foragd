import js from '@eslint/js'
import eslintConfigPrettier from 'eslint-config-prettier/flat'
import { defineConfig } from 'eslint/config'
import globals from 'globals'

export default defineConfig([
  eslintConfigPrettier,
  {
    files: ['web/*.{js,mjs,cjs}'],
    plugins: { js },
    extends: ['js/recommended'],
  },
  {
    files: ['web/*.{js,mjs,cjs}'],
    languageOptions: { globals: globals.browser },
  },
])
