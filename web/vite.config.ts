import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      output: {
        // Split the two heavyweight editors into their own chunks so they are
        // not pulled into the initial bundle.
        //
        // Vite 8 bundles with Rolldown, whose manualChunks only accepts a
        // function — the object form silently used before now throws. These
        // groups are the Rolldown-native equivalent, matched on module id:
        // `codemirror` covers the `codemirror` package and the `@codemirror/*`
        // scope, `grapesjs` covers grapesjs and grapesjs-preset-newsletter.
        advancedChunks: {
          groups: [
            { name: 'codemirror', test: /[\\/]node_modules[\\/](?:@codemirror[\\/]|codemirror[\\/])/ },
            { name: 'grapesjs', test: /[\\/]node_modules[\\/]grapesjs/ },
          ],
        },
      },
    },
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      '/api/v1': {
        target: 'http://localhost:9000',
        changeOrigin: true,
      },
    },
  },
})
