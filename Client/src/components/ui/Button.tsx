import type { ButtonHTMLAttributes, ReactNode } from 'react'

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost'

export function Button({ variant = 'primary', className = '', children, ...props }: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; children: ReactNode }) {
  const variants: Record<Variant, string> = {
    primary: 'bg-primary text-white border-primary hover:bg-primary-dark hover:border-primary-dark',
    secondary: 'bg-white text-strong border-border hover:bg-subtle',
    danger: 'bg-danger text-white border-danger hover:bg-red-700',
    ghost: 'bg-transparent text-secondary border-transparent hover:bg-subtle hover:text-strong',
  }
  return <button className={`inline-flex min-h-10 items-center justify-center gap-2 rounded-lg border px-3.5 py-2 text-sm font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-55 ${variants[variant]} ${className}`} {...props}>{children}</button>
}
