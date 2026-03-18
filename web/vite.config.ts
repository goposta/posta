import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue()],
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          codemirror: ['codemirror', '@codemirror/lang-html', '@codemirror/lang-json', '@codemirror/theme-one-dark', '@codemirror/view', '@codemirror/state'],
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
