import { X } from 'lucide-react'
import { useEffect, useRef, type ReactNode } from 'react'

export function Drawer({ open, title, description, onClose, children, width = 'max-w-xl' }: { open: boolean; title: string; description?: string; onClose: () => void; children: ReactNode; width?: string }) {
  const panel = useRef<HTMLElement>(null)
  useEffect(() => {
    if (!open) return
    const previous = document.activeElement as HTMLElement | null
    panel.current?.focus()
    const keydown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
      if (event.key === 'Tab' && panel.current) {
        const focusable = Array.from(panel.current.querySelectorAll<HTMLElement>('button, a, input, select, textarea, [tabindex]:not([tabindex="-1"])')).filter((item) => !item.hasAttribute('disabled'))
        if (!focusable.length) return
        const first = focusable[0]
        const last = focusable[focusable.length - 1]
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
        if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
      }
    }
    document.addEventListener('keydown', keydown)
    return () => { document.removeEventListener('keydown', keydown); previous?.focus() }
  }, [open, onClose])
  if (!open) return null
  return <div className="fixed inset-0 z-[60]" role="presentation"><button className="absolute inset-0 bg-slate-950/40" onClick={onClose} aria-label="Close details" /><aside ref={panel} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby="drawer-title" className={`absolute inset-y-0 right-0 flex w-full ${width} flex-col bg-white shadow-2xl outline-none`}><header className="flex items-start justify-between gap-4 border-b border-border px-5 py-4"><div><h2 id="drawer-title" className="text-lg font-semibold text-strong">{title}</h2>{description && <p className="mt-1 text-sm text-secondary">{description}</p>}</div><button onClick={onClose} className="rounded-lg p-2 text-secondary hover:bg-subtle" aria-label="Close drawer"><X className="h-5 w-5" /></button></header><div className="flex-1 overflow-y-auto p-5">{children}</div></aside></div>
}
