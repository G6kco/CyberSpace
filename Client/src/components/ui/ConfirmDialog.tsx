import { useEffect, useRef, useState } from 'react'
import { X } from 'lucide-react'
import { Button } from './Button'

interface Props {
  open: boolean
  title: string
  description: string
  confirmLabel: string
  tone?: 'primary' | 'danger'
  requireReason?: boolean
  // reasonRequired makes the reason mandatory: confirm stays disabled until
  // one is entered.
  reasonRequired?: boolean
  onCancel: () => void
  onConfirm: (reason?: string) => void | Promise<void>
}

export function ConfirmDialog({ open, title, description, confirmLabel, tone = 'primary', requireReason, reasonRequired, onCancel, onConfirm }: Props) {
  const panelRef = useRef<HTMLDivElement>(null)
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)
  useEffect(() => {
    if (!open) { setReason(''); return }
    const previous = document.activeElement as HTMLElement | null
    panelRef.current?.focus()
    const listener = (event: KeyboardEvent) => { if (event.key === 'Escape' && !busy) onCancel() }
    document.addEventListener('keydown', listener)
    return () => { document.removeEventListener('keydown', listener); previous?.focus() }
  }, [open, onCancel, busy])
  if (!open) return null
  return <div className="fixed inset-0 z-[70] flex items-center justify-center bg-slate-950/45 p-4" role="presentation" onMouseDown={(e) => { if (e.target === e.currentTarget && !busy) onCancel() }}><div ref={panelRef} tabIndex={-1} role="alertdialog" aria-modal="true" aria-labelledby="confirm-title" aria-describedby="confirm-description" className="w-full max-w-md rounded-xl border border-border bg-white p-5 shadow-xl outline-none"><div className="flex items-start justify-between gap-4"><div><h2 id="confirm-title" className="text-lg font-semibold text-strong">{title}</h2><p id="confirm-description" className="mt-1.5 text-sm leading-6 text-secondary">{description}</p></div><button onClick={onCancel} disabled={busy} className="rounded-md p-1.5 text-muted hover:bg-subtle" aria-label="Close dialog"><X className="h-5 w-5" /></button></div>{(requireReason || reasonRequired) && <label className="mt-4 block text-sm font-medium text-strong">Reason {!reasonRequired && <span className="text-secondary">(optional)</span>}<textarea value={reason} onChange={(e) => setReason(e.target.value)} className="input mt-2 min-h-24 resize-y" placeholder="Add an administrative note" /></label>}<div className="mt-5 flex justify-end gap-2"><Button variant="secondary" disabled={busy} onClick={onCancel}>Cancel</Button><Button variant={tone} disabled={busy || (reasonRequired && !reason.trim())} onClick={async () => { setBusy(true); try { await onConfirm(reason) } finally { setBusy(false) } }}>{confirmLabel}</Button></div></div></div>
}
