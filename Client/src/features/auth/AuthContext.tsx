import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { endSession, getCurrentUser, startGoogleLogin } from '../../services/auth'
import type { User } from '../../types/domain'

interface AuthContextValue {
  user: User | null
  loading: boolean
  error: string | null
  signIn: () => void
  signOut: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // The session lives in an HTTP-only cookie the client cannot read, so the
  // server is asked who the user is on every page load.
  useEffect(() => {
    getCurrentUser()
      .then(setUser)
      .catch(() => setError('Could not check your session'))
      .finally(() => setLoading(false))
  }, [])

  async function signOut() {
    try {
      await endSession()
      setUser(null)
      setError(null)
    } catch {
      setError('Could not sign out')
    }
  }

  return (
    <AuthContext.Provider
      value={{ user, loading, error, signIn: startGoogleLogin, signOut }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
