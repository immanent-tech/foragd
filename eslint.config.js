import js from '@eslint/js'
import { defineConfig } from 'eslint/config'
import globals from 'globals'
import eslintPluginPrettierRecommended from 'eslint-plugin-prettier/recommended'

export default defineConfig([
  {
    files: ['**/*.{js,mjs,cjs}'],
    plugins: { js },
    extends: ['js/recommended'],
    languageOptions: { globals: globals.browser },
  },
  eslintPluginPrettierRecommended,
])
