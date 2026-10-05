/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath } from 'node:url'

// The Go server (api/cmd/server) listens on ADDR, :8080 by default.
const apiTarget = process.env.API_TARGET || 'http://localhost:8080'

// Shared by `vite` and `vite preview`. Anchored on '/api/' so that static files whose
// names merely start with "api" are not swallowed by the proxy. changeOrigin stays off:
// the API rejects writes whose Origin differs from Host (bad_origin), and the browser's
// Origin is the dev server, so Host must stay the dev server too.
const proxy = {
  '^/api/': { target: apiTarget, changeOrigin: false },
  '/healthz': { target: apiTarget, changeOrigin: false },
}

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: { proxy },
  preview: { proxy },
  build: { outDir: 'dist' },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}', 'eslint-rules/**/*.test.mjs'],
    css: false,
  },
})
