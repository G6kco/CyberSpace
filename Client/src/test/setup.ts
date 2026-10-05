import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, vi } from 'vitest'
import { clearSession, currentSession } from './session'

afterEach(() => {
  cleanup()
  clearSession()
})

// Stub the API rather than the auth service, so services/auth.ts runs for real
// in tests: its 401 handling and initials derivation stay covered.
const json = (body: unknown, status: number) =>
  new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })

vi.stubGlobal(
  'fetch',
  vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)

    if (url.endsWith('/api/v1/me')) {
      const user = currentSession()
      if (!user) return json({ code: 'unauthenticated' }, 401)
      // The server sends no display initials; the client derives them.
      const { id, name, email, role } = user
      return json({ id, name, email, role }, 200)
    }

    if (url.endsWith('/api/v1/auth/session') && init?.method === 'DELETE') {
      clearSession()
      return new Response(null, { status: 204 })
    }

    return json({ code: 'not_found' }, 404)
  }),
)

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }),
})
