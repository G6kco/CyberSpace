import { ArrowLeft, LockKeyhole, MapPinOff } from 'lucide-react'
import { Link } from 'react-router-dom'
import { Button } from '../components/ui/Button'
import { useAuth } from '../features/auth/AuthContext'
import { roleLanding } from '../routes/navigation'

export function UnauthorizedPage() { const { user } = useAuth(); return <main className="grid min-h-screen place-items-center bg-app p-5"><div className="max-w-md rounded-xl border border-border bg-white p-8 text-center shadow-sm"><LockKeyhole className="mx-auto h-9 w-9 text-danger" /><h1 className="mt-4 text-2xl font-semibold text-strong">Access denied</h1><p className="mt-2 text-secondary">Your current role does not have permission to open this page.</p><Link to={user ? roleLanding[user.role] : '/login'}><Button className="mt-6"><ArrowLeft className="h-4 w-4" />Return to your workspace</Button></Link></div></main> }

export function NotFoundPage() { return <main className="grid min-h-screen place-items-center bg-app p-5"><div className="max-w-md text-center"><MapPinOff className="mx-auto h-10 w-10 text-muted" /><h1 className="mt-4 text-2xl font-semibold text-strong">Page not found</h1><p className="mt-2 text-secondary">The page may have moved or the address may be incorrect.</p><Link to="/"><Button className="mt-6">Return home</Button></Link></div></main> }
