import DOMPurify from 'dompurify'
if (window.trustedTypes && trustedTypes.createPolicy) {
  trustedTypes.createPolicy('default', {
    createHTML: (input) => {
      return DOMPurify.sanitize(input)
    },
    createScript: (input) => input,
  })
}
