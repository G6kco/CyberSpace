import { Search } from 'lucide-react'
import type { InputHTMLAttributes } from 'react'

export function SearchInput({ className = '', ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <label className={`relative block ${className}`}><span className="sr-only">Search</span><Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" aria-hidden="true" /><input type="search" className="input pl-9" {...props} /></label>
}
