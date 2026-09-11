import { Eye, EyeOff, LockKeyhole, ShieldCheck } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { Button } from '../components/ui/Button'
import { useAuth } from '../features/auth/AuthContext'
import { roleLanding } from '../routes/navigation'
import type { Role } from '../types/domain'

export function LoginPage() {
  const { user, signIn } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [showPassword, setShowPassword] = useState(false)
  const [role, setRole] = useState<Role>('student')
  const [error, setError] = useState('')
  if (user) return <Navigate to={roleLanding[user.role]} replace />
  const submit = (event: FormEvent) => {
    event.preventDefault()
    const form = new FormData(event.currentTarget as HTMLFormElement)
    if (!form.get('email') || !form.get('password')) { setError('Enter both email and password to continue.'); return }
    signIn(role)
    const requested = (location.state as { from?: string } | null)?.from
    navigate(requested && requested.startsWith(`/${role}`) ? requested : roleLanding[role], { replace: true })
  }
  return <div className="grid min-h-screen bg-white lg:grid-cols-[0.9fr_1.1fr]">
    <section className="hidden bg-sidebar px-12 py-10 text-white lg:flex lg:flex-col"><div className="flex items-center gap-3"><div className="grid h-10 w-10 place-items-center rounded-lg bg-primary"><ShieldCheck className="h-6 w-6" /></div><div><div className="text-lg font-semibold">CyberSpace</div><div className="text-xs text-slate-400">College security lab</div></div></div><div className="my-auto max-w-xl"><div className="mb-7 grid h-14 w-14 place-items-center rounded-xl border border-slate-700 bg-slate-800"><LockKeyhole className="h-7 w-7 text-blue-300" /></div><h1 className="max-w-lg text-4xl font-semibold leading-tight tracking-tight">Learn securely. Assess confidently.</h1><p className="mt-5 max-w-lg text-lg leading-8 text-slate-300">A controlled learning and assessment workspace for authorized college cybersecurity training.</p><div className="mt-10 border-l-2 border-primary pl-4 text-sm leading-6 text-slate-400">All assessment environments shown in this prototype are isolated simulations. Use is limited to approved academic activities.</div></div><p className="text-xs text-slate-500">Frontend blueprint · Mock data only</p></section>
    <main className="flex items-center justify-center bg-app px-5 py-10 sm:px-8"><div className="w-full max-w-md"><div className="mb-8 flex items-center gap-3 lg:hidden"><div className="grid h-10 w-10 place-items-center rounded-lg bg-primary text-white"><ShieldCheck className="h-6 w-6" /></div><div><div className="font-semibold text-strong">CyberSpace</div><div className="text-xs text-secondary">College security lab</div></div></div><div className="rounded-xl border border-border bg-white p-6 shadow-sm sm:p-8"><h2 className="text-2xl font-semibold tracking-tight text-strong">Welcome back</h2><p className="mt-2 text-sm leading-6 text-secondary">Sign in to your college learning workspace.</p><form onSubmit={submit} className="mt-7 space-y-5" noValidate><label className="block text-sm font-medium text-strong">Email address<input name="email" type="email" autoComplete="email" defaultValue="maya.iyer@demo.cyberspace.edu" className="input mt-2" /></label><label className="block text-sm font-medium text-strong">Password<div className="relative mt-2"><input name="password" type={showPassword ? 'text' : 'password'} autoComplete="current-password" defaultValue="demo-password" className="input pr-11" /><button type="button" onClick={() => setShowPassword((value) => !value)} className="absolute right-2 top-1/2 -translate-y-1/2 rounded-md p-2 text-secondary hover:bg-subtle" aria-label={showPassword ? 'Hide password' : 'Show password'}>{showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}</button></div></label><div className="flex items-center justify-between gap-3"><label className="flex items-center gap-2 text-sm text-secondary"><input type="checkbox" defaultChecked className="h-4 w-4 rounded border-border accent-primary" />Remember me</label><button type="button" className="text-sm font-semibold text-primary hover:text-primary-dark">Forgot password?</button></div>{error && <p className="text-sm font-medium text-danger" role="alert">{error}</p>}<Button type="submit" className="w-full">Sign in as {role === 'admin' ? 'administrator' : 'student'}</Button></form><div className="my-6 flex items-center gap-3"><div className="h-px flex-1 bg-border" /><span className="text-xs font-medium uppercase tracking-wide text-muted">Demo role</span><div className="h-px flex-1 bg-border" /></div><div className="grid grid-cols-2 gap-2" role="group" aria-label="Choose demo role"><button onClick={() => setRole('student')} className={`rounded-lg border px-3 py-3 text-sm font-semibold transition ${role === 'student' ? 'border-primary bg-primary-light text-primary-dark' : 'border-border text-secondary hover:bg-subtle'}`}>Student</button><button onClick={() => setRole('admin')} className={`rounded-lg border px-3 py-3 text-sm font-semibold transition ${role === 'admin' ? 'border-primary bg-primary-light text-primary-dark' : 'border-border text-secondary hover:bg-subtle'}`}>Administrator</button></div></div><p className="mt-5 text-center text-xs leading-5 text-secondary">Any non-empty credentials work in this frontend-only prototype.</p></div></main>
  </div>
}
