import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { ToastProvider } from '../components/ui/Toast'
import { AuthProvider } from '../features/auth/AuthContext'
import { AppRoutes } from '../routes/AppRoutes'

export { setSession } from './session'

export function renderApp(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AuthProvider>
        <ToastProvider>
          <AppRoutes />
        </ToastProvider>
      </AuthProvider>
    </MemoryRouter>,
  )
}
