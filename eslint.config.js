import js from '@eslint/js'
import { defineConfig } from 'eslint/config'
import globals from 'globals'

export default defineConfig([
  {
    files: ['**/*.{js,mjs,cjs}'],
    plugins: { js },
    extends: ['js/recommended'],
    languageOptions: { globals: globals.browser },
  },
])

import eslintPluginPrettierRecommended from 'eslint-plugin-prettier/recommended'

module.exports = [
  // Any other config imports go at the top
  eslintPluginPrettierRecommended,
]
