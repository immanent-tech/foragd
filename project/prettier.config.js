// prettier.config.js, .prettierrc.js, prettier.config.mjs, or .prettierrc.mjs

/**
 * @see https://prettier.io/docs/configuration
 * @type {import("prettier").Config}
 */
const config = {
  trailingComma: 'es5',
  tabWidth: 4,
  semi: false,
  singleQuote: true,
  plugins: ['prettier-plugin-tailwindcss','prettier-plugin-toml'],
  tailwindStylesheet: './assets/styles.css',
}
export default config
