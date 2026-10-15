import dns from 'node:dns'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// Node resolves localhost to ::1 first, so Vite would listen on IPv6 only and
// a browser reaching localhost over 127.0.0.1 is refused. IPv4 first matches
// the API, which listens on 127.0.0.1.
dns.setDefaultResultOrder('ipv4first')

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
