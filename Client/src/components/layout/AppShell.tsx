import { Bell, LogOut, Menu, PanelLeftClose, PanelLeftOpen, ShieldCheck, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { NavLink, Outlet, useLocation } from 'react-router-dom'
import { useAuth } from '../../features/auth/AuthContext'
import { navigation } from '../../routes/navigation'

const pageTitles: Record<string, string> = {
  '/student/learning': 'Learning Materials',
  '/student/assessments': 'Assessments',
  '/student/lobby': 'Test Portal',
  '/student/attempts': 'Assessment',
  '/admin/monitoring': 'Assessment Monitoring',
  '/admin/assessments': 'Question Bank',
  '/admin/students': 'Students',
}

export function AppShell() {
  const { user, signOut } = useAuth()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [profileOpen, setProfileOpen] = useState(false)
  const closeRef = useRef<HTMLButtonElement>(null)
  useEffect(() => { setMobileOpen(false); setProfileOpen(false) }, [location.pathname])
  useEffect(() => {
    if (mobileOpen) closeRef.current?.focus()
    const listener = (event: KeyboardEvent) => { if (event.key === 'Escape') setMobileOpen(false) }
    document.addEventListener('keydown', listener)
    return () => document.removeEventListener('keydown', listener)
  }, [mobileOpen])
  if (!user) return null
  const currentTitle = Object.entries(pageTitles).find(([path]) => location.pathname.startsWith(path))?.[1] ?? 'CyberSpace'
  const sidebar = <div className="flex h-full flex-col">
    <div className={`flex h-18 items-center border-b border-slate-800 px-4 ${collapsed ? 'justify-center' : 'gap-3'}`}>
      <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-primary text-white"><ShieldCheck className="h-5 w-5" aria-hidden="true" /></div>
      {!collapsed && <div><div className="font-semibold tracking-tight text-white">CyberSpace</div><div className="text-xs text-slate-400">College security lab</div></div>}
    </div>
    <nav className="flex-1 px-3 py-5" aria-label={`${user.role} navigation`}>
      {!collapsed && <p className="px-3 pb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">{user.role === 'admin' ? 'Administration' : 'Student workspace'}</p>}
      <div className="space-y-1">{navigation[user.role].map(({ label, to, icon: Icon }) => <NavLink key={to} to={to} title={collapsed ? label : undefined} className={({ isActive }) => `flex min-h-11 items-center rounded-lg text-sm font-medium transition ${collapsed ? 'justify-center px-2' : 'gap-3 px-3'} ${isActive ? 'bg-primary text-white' : 'text-slate-300 hover:bg-slate-800 hover:text-white'}`}><Icon className="h-5 w-5 shrink-0" aria-hidden="true" />{!collapsed && <span>{label}</span>}</NavLink>)}</div>
    </nav>
    <div className="border-t border-slate-800 p-3">
      <div className={`flex items-center ${collapsed ? 'justify-center' : 'gap-3 px-2 py-2'}`}>
        <div className="grid h-9 w-9 shrink-0 place-items-center rounded-full bg-blue-100 text-sm font-bold text-blue-700">{user.initials}</div>
        {!collapsed && <div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold text-white">{user.name}</p><p className="truncate text-xs capitalize text-slate-400">{user.role === 'admin' ? 'Administrator' : 'Student'}</p></div>}
      </div>
      {!collapsed && <button onClick={signOut} className="mt-1 flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium text-slate-300 hover:bg-slate-800 hover:text-white"><LogOut className="h-4 w-4" />Sign out</button>}
    </div>
  </div>

  return <div className="min-h-screen bg-app">
    <aside className={`fixed inset-y-0 left-0 z-40 hidden bg-sidebar transition-[width] duration-200 lg:block ${collapsed ? 'w-20' : 'w-64'}`}>{sidebar}<button onClick={() => setCollapsed((value) => !value)} className="absolute -right-3 top-23 grid h-7 w-7 place-items-center rounded-full border border-border bg-white text-secondary shadow-sm hover:text-strong" aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}>{collapsed ? <PanelLeftOpen className="h-4 w-4" /> : <PanelLeftClose className="h-4 w-4" />}</button></aside>
    {mobileOpen && <div className="fixed inset-0 z-50 lg:hidden"><button className="absolute inset-0 bg-slate-950/50" onClick={() => setMobileOpen(false)} aria-label="Close navigation overlay" /><aside className="relative h-full w-[min(86vw,280px)] bg-sidebar shadow-xl"><button ref={closeRef} onClick={() => setMobileOpen(false)} className="absolute right-3 top-4 z-10 rounded-md p-2 text-slate-300 hover:bg-slate-800" aria-label="Close navigation"><X className="h-5 w-5" /></button>{sidebar}</aside></div>}
    <div className={`transition-[padding] duration-200 ${collapsed ? 'lg:pl-20' : 'lg:pl-64'}`}>
      <header className="sticky top-0 z-30 flex h-18 items-center border-b border-border bg-white/95 px-4 backdrop-blur sm:px-6">
        <button onClick={() => setMobileOpen(true)} className="mr-3 rounded-lg p-2 text-secondary hover:bg-subtle lg:hidden" aria-label="Open navigation"><Menu className="h-5 w-5" /></button>
        <div className="min-w-0 flex-1"><p className="truncate text-xs font-medium text-secondary">CyberSpace / {user.role === 'admin' ? 'Administration' : 'Student'}</p><h1 className="truncate text-lg font-semibold text-strong">{currentTitle}</h1></div>
        <div className="flex items-center gap-1 sm:gap-2"><button className="relative rounded-lg p-2.5 text-secondary hover:bg-subtle" aria-label="Notifications"><Bell className="h-5 w-5" /><span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-primary ring-2 ring-white" /></button><div className="relative"><button onClick={() => setProfileOpen((value) => !value)} className="flex items-center gap-2 rounded-lg p-1.5 hover:bg-subtle" aria-expanded={profileOpen} aria-haspopup="menu"><div className="grid h-9 w-9 place-items-center rounded-full bg-primary-light text-sm font-bold text-primary-dark">{user.initials}</div><div className="hidden text-left sm:block"><p className="text-sm font-semibold text-strong">{user.name}</p><p className="text-xs capitalize text-secondary">{user.role === 'admin' ? 'Administrator' : 'Student'}</p></div></button>{profileOpen && <div role="menu" className="absolute right-0 mt-2 w-52 rounded-lg border border-border bg-white p-1.5 shadow-lg"><div className="border-b border-border px-2.5 py-2 text-xs text-secondary">{user.email}</div><button role="menuitem" onClick={signOut} className="mt-1 flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-sm font-medium text-strong hover:bg-subtle"><LogOut className="h-4 w-4" />Sign out</button></div>}</div></div>
      </header>
      <main className="mx-auto max-w-[1500px] p-4 sm:p-6 lg:p-8"><Outlet /></main>
    </div>
  </div>
}
