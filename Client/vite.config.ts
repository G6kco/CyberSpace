import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  // Must match the hostname in the server's FRONTEND_URL and
  // CORS_ORIGIN_ALLOWED: localhost and 127.0.0.1 are different origins, and
  // the session cookie is only sent back to the one that set it.
  server: { host: 'localhost' },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    css: true,
  },
})
