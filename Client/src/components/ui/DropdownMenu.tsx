import { MoreHorizontal } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'

export function DropdownMenu({ label = 'Open actions', children }: { label?: string; children: (close: () => void) => ReactNode }) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const listener = (event: MouseEvent) => { if (!root.current?.contains(event.target as Node)) setOpen(false) }
    const keydown = (event: KeyboardEvent) => { if (event.key === 'Escape') setOpen(false) }
    document.addEventListener('mousedown', listener)
    document.addEventListener('keydown', keydown)
    return () => { document.removeEventListener('mousedown', listener); document.removeEventListener('keydown', keydown) }
  }, [])
  return <div ref={root} className="relative inline-block"><button onClick={() => setOpen((value) => !value)} className="grid h-10 w-10 place-items-center rounded-lg border border-border bg-white text-secondary hover:bg-subtle hover:text-strong" aria-label={label} aria-expanded={open} aria-haspopup="menu"><MoreHorizontal className="h-5 w-5" /></button>{open && <div role="menu" className="absolute right-0 z-20 mt-1 w-56 rounded-lg border border-border bg-white p-1.5 shadow-lg">{children(() => setOpen(false))}</div>}</div>
}

export function DropdownItem({ children, onClick, danger = false }: { children: ReactNode; onClick: () => void; danger?: boolean }) {
  return <button role="menuitem" onClick={onClick} className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm font-medium hover:bg-subtle ${danger ? 'text-danger' : 'text-strong'}`}>{children}</button>
}
