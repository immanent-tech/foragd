import DOMPurify from 'dompurify'

const ALLOWED_SCRIPT_ORIGINS = new Set(['https://cloud.umami.is'])

if (window.trustedTypes && trustedTypes.createPolicy) {
  trustedTypes.createPolicy('default', {
    createHTML: (input) => {
      return DOMPurify.sanitize(input)
    },
    createScript: (input) => input,
    createScriptURL: (input) => {
      const url = new URL(input, document.baseURI)
      if (url.origin === location.origin || ALLOWED_SCRIPT_ORIGINS.has(url.origin)) {
        return url.href
      }
      // Returning undefined makes the browser throw / report a violation
      return undefined
    },
  })
}
