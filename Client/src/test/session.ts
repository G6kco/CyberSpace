import { demoUsers } from '../mocks/data'
import type { Role, User } from '../types/domain'

// The signed-in user the stubbed API reports. Tests set this instead of
// seeding storage, because the real session is a server-side cookie.
let signedIn: User | null = null

export function setSession(role: Role | null) {
  signedIn = role ? demoUsers[role] : null
}

export function currentSession(): User | null {
  return signedIn
}

export function clearSession() {
  signedIn = null
}
