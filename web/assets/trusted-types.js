import DOMPurify from 'dompurify'

const ALLOWED_SCRIPT_ORIGINS = new Set([
  'https://foragd.app',
  'https://foragd-dev.astralcars.com',
  'https://cloud.umami.is',
  'https://cdn.paddle.com',
  'https://challenges.cloudflare.com',
  'https://www.youtube.com',
  'https://src.litix.io',
  'https://public.profitwell.com',
  'https://static.cloudflareinsights.com',
  'https://cdn.jsdelivr.net',
])

if (window.trustedTypes?.createPolicy) {
  trustedTypes.createPolicy('default', {
    createHTML: (input) => DOMPurify.sanitize(input),
    createScript: (input) => input,
    createScriptURL: (input) => {
      let url
      try {
        url = new URL(input, document.baseURI)
      } catch {
        throw new TypeError(`Invalid script URL: ${input}`)
      }
      if (url.origin === location.origin || ALLOWED_SCRIPT_ORIGINS.has(url.origin)) {
        return url.href
      }
      // Log what was rejected so you can see exactly which URL is the problem.
      console.warn('Blocked script URL by Trusted Types default policy:', input)
      throw new TypeError(`Script URL not allowed: ${input}`)
    },
  })
}
