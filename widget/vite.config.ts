import { defineConfig } from 'vite'

// One self-contained IIFE, written straight into the Go embed directory so
// `go build` serves it at /widget/v1/posta-chat.js.
export default defineConfig({
  build: {
    target: 'es2020',
    outDir: '../internal/web/widget',
    emptyOutDir: false,
    copyPublicDir: false,
    sourcemap: false,
    lib: {
      entry: 'src/main.tsx',
      formats: ['iife'],
      name: 'PostaChatWidget',
      fileName: () => 'posta-chat.js',
    },
  },
})
