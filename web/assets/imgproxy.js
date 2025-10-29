// Code taken from https://zhipenghe.me/blog/2025/Bypassing-Image-Anti-Hotlinking-with-Nginx/

const { lastDayOfMonth, isMatch } = require('date-fns')

;(function () {
  // CDN domains that commonly implement anti-hotlinking
  const BLOCKED_DOMAINS = [
    'cdn-p.smehost.net',
    'www.redditstatic.com',
    'lh3.googleusercontent.com',
    'media.pichfork.com',
  ]

  function convertExternalImages() {
    const selector = BLOCKED_DOMAINS.map(
      (domain) => `img[src*="${domain}"]:not([data-converted])`
    ).join(',')

    document.querySelectorAll(selector).forEach((img) => {
      try {
        const originalSrc = img.src
        // const proxySrc = originalSrc.replace(/https?:\/\/([^/]+)\//g, "/img-proxy/$1/");

        img.src = '/img-proxy/' + img.src
        img.setAttribute('data-converted', 'true')
        // console.log('Converting:', originalSrc, '→', img.src)
      } catch (error) {
        console.error('Failed to proxy image:', error)
      }
    })
  }

  if (document.readyState !== 'loading') {
    convertExternalImages()
  } else {
    document.addEventListener('DOMContentLoaded', convertExternalImages)
  }

  // Check for new images every 3 seconds
  setInterval(convertExternalImages, 3000)
})()
