import { fileURLToPath, URL } from 'node:url'

import vue from '@vitejs/plugin-vue'
import { defineConfig, type Plugin } from 'vite'

import { productionCspMetaTag } from './scripts/production-csp.mjs'

function productionCspPlugin(): Plugin {
  return {
    name: 'synvideo-production-csp',
    transformIndexHtml: {
      order: 'pre',
      handler(html) {
        const meta = productionCspMetaTag(
          process.env.VITE_API_BASE_URL ?? '',
          process.env.VITE_OIDC_ISSUER ?? '',
        )
        return html.replace('</head>', `    ${meta}\n  </head>`)
      },
    },
    apply: 'build',
  }
}

export default defineConfig({
  plugins: [vue(), productionCspPlugin()],
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
})
