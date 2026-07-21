import { defineConfig } from '@maizzle/framework'

export default defineConfig({
  parallel: true,
  plaintext: true,
  components: {
    source: [{ path: 'components', prefix: 'Custom' }],
  },
  html: {
    minify: true,
  },
  url: {
    base: process.env.APP_BASEURL || 'https://foragd.app',
  },
})
