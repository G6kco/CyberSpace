import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ToastProvider } from '../components/ui/Toast'
import { AuthProvider } from '../features/auth/AuthContext'
import { demoUsers } from '../mocks/data'
import { AppRoutes } from '../routes/AppRoutes'
import type { Role } from '../types/domain'

export function setSession(role: Role | null) {
  localStorage.clear()
  if (role) localStorage.setItem('cyberspace-demo-session', JSON.stringify(demoUsers[role]))
}

export function renderApp(path: string) {
  return render(<MemoryRouter initialEntries={[path]}><AuthProvider><ToastProvider><AppRoutes /></ToastProvider></AuthProvider></MemoryRouter>)
}
