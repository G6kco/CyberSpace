import { AlertCircle, Inbox, LoaderCircle } from 'lucide-react'
import type { ReactNode } from 'react'
import { Button } from './Button'

export function LoadingSkeleton({ rows = 4 }: { rows?: number }) {
  return <div className="space-y-3" aria-label="Loading content">{Array.from({ length: rows }).map((_, index) => <div key={index} className="h-20 animate-pulse rounded-lg border border-border bg-white p-4"><div className="h-4 w-1/3 rounded bg-slate-200" /><div className="mt-3 h-3 w-2/3 rounded bg-slate-100" /></div>)}</div>
}

export function EmptyState({ title, description, action }: { title: string; description: string; action?: ReactNode }) {
  return <div className="rounded-xl border border-dashed border-border bg-white px-6 py-12 text-center"><Inbox className="mx-auto h-8 w-8 text-muted" aria-hidden="true" /><h3 className="mt-3 font-semibold text-strong">{title}</h3><p className="mx-auto mt-1 max-w-md text-sm text-secondary">{description}</p>{action && <div className="mt-4">{action}</div>}</div>
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return <div className="rounded-xl border border-red-200 bg-red-50 p-5"><div className="flex items-start gap-3"><AlertCircle className="mt-0.5 h-5 w-5 text-danger" /><div><h3 className="font-semibold text-red-900">Unable to load this section</h3><p className="mt-1 text-sm text-red-700">{message}</p>{onRetry && <Button variant="secondary" className="mt-3" onClick={onRetry}>Try again</Button>}</div></div></div>
}

export function BusyLabel({ children }: { children: ReactNode }) { return <><LoaderCircle className="h-4 w-4 animate-spin" />{children}</> }
