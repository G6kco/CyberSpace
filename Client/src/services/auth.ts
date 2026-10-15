import type { Role, User } from '../types/domain'
import { apiBase } from './api'

// The server returns the identity it trusts. It does not send display
// initials, so they are derived here to keep the avatar working without
// widening the API surface.
function initialsOf(name: string): string {
  const parts = name
    .split(/\s+/)
    .filter(Boolean)
    .filter((part) => !/^(dr|mr|mrs|ms|prof)\.?$/i.test(part))

  if (parts.length === 0) return '?'
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase()
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
}

interface ApiUser {
  id: string
  name: string
  email: string
  role: Role
}

function toUser(payload: ApiUser): User {
  return { ...payload, initials: initialsOf(payload.name) }
}

// Sign-in is a full page navigation, not a fetch: the browser must follow the
// redirect to Google and come back to the API so the session cookie is set on
// the API's own origin.
export function startGoogleLogin() {
  window.location.assign(`${apiBase}/api/v1/auth/google`)
}

export async function getCurrentUser(): Promise<User | null> {
  const response = await fetch(`${apiBase}/api/v1/me`, {
    credentials: 'include',
  })
  if (response.status === 401) return null
  if (!response.ok) throw new Error('Could not check sign-in status')
  return toUser((await response.json()) as ApiUser)
}

export async function endSession(): Promise<void> {
  const response = await fetch(`${apiBase}/api/v1/auth/session`, {
    method: 'DELETE',
    credentials: 'include',
  })
  if (!response.ok) throw new Error('Could not sign out')
}
