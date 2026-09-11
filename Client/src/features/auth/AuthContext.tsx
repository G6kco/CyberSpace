import { createContext, useContext, useMemo, useState, type ReactNode } from 'react'
import { demoUsers } from '../../mocks/data'
import type { Role, User } from '../../types/domain'

const SESSION_KEY = 'cyberspace-demo-session'

interface AuthContextValue {
  user: User | null
  signIn: (role: Role) => void
  signOut: () => void
}

const AuthContext = createContext<AuthContextValue | null>(null)

function loadSession(): User | null {
  try {
    const value = localStorage.getItem(SESSION_KEY)
    return value ? (JSON.parse(value) as User) : null
  } catch {
    return null
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(loadSession)
  const value = useMemo<AuthContextValue>(() => ({
    user,
    signIn: (role) => {
      const next = demoUsers[role]
      localStorage.setItem(SESSION_KEY, JSON.stringify(next))
      setUser(next)
    },
    signOut: () => {
      localStorage.removeItem(SESSION_KEY)
      setUser(null)
    },
  }), [user])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const value = useContext(AuthContext)
  if (!value) throw new Error('useAuth must be used inside AuthProvider')
  return value
}
