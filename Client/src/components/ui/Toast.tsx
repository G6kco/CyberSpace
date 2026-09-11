import { CheckCircle2, X } from 'lucide-react'
import { createContext, useCallback, useContext, useMemo, useState, type ReactNode } from 'react'

interface ToastItem { id: number; message: string; tone: 'success' | 'error' }
interface ToastContextValue { notify: (message: string, tone?: ToastItem['tone']) => void }
const ToastContext = createContext<ToastContextValue | null>(null)

export function ToastProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const notify = useCallback((message: string, tone: ToastItem['tone'] = 'success') => {
    const id = Date.now()
    setToasts((items) => [...items, { id, message, tone }])
    window.setTimeout(() => setToasts((items) => items.filter((item) => item.id !== id)), 4200)
  }, [])
  const value = useMemo(() => ({ notify }), [notify])
  return <ToastContext.Provider value={value}>{children}<div className="fixed bottom-4 right-4 z-[80] flex w-[calc(100%-2rem)] max-w-sm flex-col gap-2" aria-live="polite">{toasts.map((toast) => <div key={toast.id} className={`flex items-start gap-3 rounded-lg border bg-white p-4 shadow-lg ${toast.tone === 'error' ? 'border-red-200' : 'border-emerald-200'}`}><CheckCircle2 className={`mt-0.5 h-5 w-5 shrink-0 ${toast.tone === 'error' ? 'text-danger' : 'text-success'}`} /><p className="flex-1 text-sm font-medium text-strong">{toast.message}</p><button onClick={() => setToasts((items) => items.filter((item) => item.id !== toast.id))} className="rounded p-1 text-muted hover:bg-subtle" aria-label="Dismiss notification"><X className="h-4 w-4" /></button></div>)}</div></ToastContext.Provider>
}

export function useToast() { const value = useContext(ToastContext); if (!value) throw new Error('useToast must be used within ToastProvider'); return value }
