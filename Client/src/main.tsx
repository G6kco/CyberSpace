import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { AuthProvider } from './features/auth/AuthContext'
import { AppRoutes } from './routes/AppRoutes'
import { ToastProvider } from './components/ui/Toast'
import './styles.css'
import { WebMcpBridge } from './lib/WebMcpBridge'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <ToastProvider><WebMcpBridge /><AppRoutes /></ToastProvider>
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
)
