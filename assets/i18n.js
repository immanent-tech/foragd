const { pageData } = window.config

/**
 * A list of plural rules for all languages.
 * `plural_rules.go` defines the index for each of the 14 known plural rules.
 *
 * `pageData.PLURAL_RULE_LANG` is the index of the plural rule for the current language.
 * `pageData.PLURAL_RULE_FALLBACK` is the index of the plural rule for the default language,
 * to be used when a string is not translated in the current language.
 *
 * Each plural rule is a function that maps an amount `n` to the appropriate plural form index.
 * Which index means which rule is specific for each language and also defined in `plural_rules.go`.
 * The actual strings are in `pageData.PLURALSTRINGS_LANG` and `pageData.PLURALSTRINGS_FALLBACK`
 * respectively, which is an array indexed by the plural form index.
 *
 * Links to the language plural rule and form definitions:
 * https://codeberg.org/forgejo/forgejo/src/branch/forgejo/modules/translation/plural_rules.go
 * https://www.unicode.org/cldr/charts/46/supplemental/language_plural_rules.html
 * https://translate.codeberg.org/languages/$LANGUAGE_CODE/#information
 * https://github.com/WeblateOrg/language-data/blob/main/languages.csv
 */
const PLURAL_RULES = [
  // [ 0] Common 2-form, e.g. English, German
  function (n) {
    return n !== 1 ? 1 : 0
  },

  // [ 1] Bengali 2-form
  function (n) {
    return n > 1 ? 1 : 0
  },

  // [ 2] Icelandic 2-form
  function (n) {
    return n % 10 !== 1 || n % 100 === 11 ? 1 : 0
  },

  // [ 3] Filipino 2-form
  function (n) {
    return n !== 1 &&
      n !== 2 &&
      n !== 3 &&
      (n % 10 === 4 || n % 10 === 6 || n % 10 === 9)
      ? 1
      : 0
  },

  // [ 4] One form
  function (_) {
    return 0
  },

  // [ 5] Czech 3-form
  function (n) {
    return n === 1 ? 0 : n >= 2 && n <= 4 ? 1 : 2
  },

  // [ 6] Russian 3-form
  function (n) {
    return n % 10 === 1 && n % 100 !== 11
      ? 0
      : n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)
        ? 1
        : 2
  },

  // [ 7] Polish 3-form
  function (n) {
    return n === 1
      ? 0
      : n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)
        ? 1
        : 2
  },

  // [ 8] Latvian 3-form
  function (n) {
    return n % 10 === 0 || (n % 100 >= 11 && n % 100 <= 19)
      ? 0
      : n % 10 === 1 && n % 100 !== 11
        ? 1
        : 2
  },

  // [ 9] Lithunian 3-form
  function (n) {
    return n % 10 === 1 && (n % 100 < 11 || n % 100 > 19)
      ? 0
      : n % 10 >= 2 && n % 10 <= 9 && (n % 100 < 11 || n % 100 > 19)
        ? 1
        : 2
  },

  // [10] French 3-form
  function (n) {
    return n === 0 || n === 1 ? 0 : n !== 0 && n % 1000000 === 0 ? 1 : 2
  },

  // [11] Catalan 3-form
  function (n) {
    return n === 1 ? 0 : n !== 0 && n % 1000000 === 0 ? 1 : 2
  },

  // [12] Slovenian 4-form
  function (n) {
    return n % 100 === 1
      ? 0
      : n % 100 === 2
        ? 1
        : n % 100 === 3 || n % 100 === 4
          ? 2
          : 3
  },

  // [13] Arabic 6-form
  function (n) {
    return n === 0
      ? 0
      : n === 1
        ? 1
        : n === 2
          ? 2
          : n % 100 >= 3 && n % 100 <= 10
            ? 3
            : n % 100 >= 11
              ? 4
              : 5
  },
]

/**
 * Look up the correct localized plural form for amount `n` for the string with the translation key `key`.
 * If the current language does not contain a translation for this key, returns the text in the default language,
 * or `null` if `suppress_fallback` is set to `true`.
 */
export function GetPluralizedString(key, n, suppress_fallback) {
  const result =
    pageData.PLURALSTRINGS_LANG[key]?.[
      PLURAL_RULES[pageData.PLURAL_RULE_LANG](n)
    ]
  if (result || suppress_fallback) return result
  return pageData.PLURALSTRINGS_FALLBACK[key][
    PLURAL_RULES[pageData.PLURAL_RULE_FALLBACK](n)
  ]
}
