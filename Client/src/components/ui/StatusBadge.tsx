import type { ReactNode } from 'react'

type Tone = 'neutral' | 'info' | 'success' | 'warning' | 'danger'

export function StatusBadge({ children, tone = 'neutral' }: { children: ReactNode; tone?: Tone }) {
  const tones: Record<Tone, string> = {
    neutral: 'bg-slate-100 text-slate-700 border-slate-200',
    info: 'bg-blue-50 text-blue-700 border-blue-200',
    success: 'bg-emerald-50 text-emerald-700 border-emerald-200',
    warning: 'bg-amber-50 text-amber-800 border-amber-200',
    danger: 'bg-red-50 text-red-700 border-red-200',
  }
  return <span className={`inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-semibold ${tones[tone]}`}>{children}</span>
}
