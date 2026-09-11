import { ChevronLeft, ChevronRight } from 'lucide-react'

export function Pagination({ page, totalPages, onPageChange }: { page: number; totalPages: number; onPageChange: (page: number) => void }) {
  if (totalPages <= 1) return null
  return <nav className="flex items-center justify-between gap-3 border-t border-border pt-4" aria-label="Pagination"><p className="text-sm text-secondary">Page <span className="font-semibold text-strong">{page}</span> of {totalPages}</p><div className="flex gap-2"><button className="grid h-10 w-10 place-items-center rounded-lg border border-border bg-white text-secondary hover:bg-subtle disabled:opacity-40" disabled={page === 1} onClick={() => onPageChange(page - 1)} aria-label="Previous page"><ChevronLeft className="h-4 w-4" /></button><button className="grid h-10 w-10 place-items-center rounded-lg border border-border bg-white text-secondary hover:bg-subtle disabled:opacity-40" disabled={page === totalPages} onClick={() => onPageChange(page + 1)} aria-label="Next page"><ChevronRight className="h-4 w-4" /></button></div></nav>
}
