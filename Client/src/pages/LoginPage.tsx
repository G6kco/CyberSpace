import { LockKeyhole, ShieldCheck } from 'lucide-react'
import { Navigate } from 'react-router-dom'
import { Button } from '../components/ui/Button'
import { useAuth } from '../features/auth/AuthContext'
import { roleLanding } from '../routes/navigation'

function GoogleMark() {
  return (
    <svg className="h-4 w-4" viewBox="0 0 48 48" aria-hidden="true">
      <path fill="#4285F4" d="M45.1 24.5c0-1.6-.1-3.1-.4-4.5H24v8.5h11.8c-.5 2.7-2.1 5-4.4 6.6v5.5h7.1c4.2-3.8 6.6-9.5 6.6-16.1z" />
      <path fill="#34A853" d="M24 46c6 0 11-2 14.5-5.4l-7.1-5.5c-2 1.3-4.5 2.1-7.4 2.1-5.7 0-10.6-3.9-12.3-9.1H4.5v5.7C8.1 41.1 15.4 46 24 46z" />
      <path fill="#FBBC05" d="M11.7 28.1c-.4-1.3-.7-2.7-.7-4.1s.3-2.8.7-4.1v-5.7H4.5C2.9 17.3 2 20.5 2 24s.9 6.7 2.5 9.8l7.2-5.7z" />
      <path fill="#EA4335" d="M24 10.4c3.2 0 6.1 1.1 8.4 3.3l6.3-6.3C34.9 3.9 29.9 2 24 2 15.4 2 8.1 6.9 4.5 14.2l7.2 5.7c1.7-5.2 6.6-9.5 12.3-9.5z" />
    </svg>
  )
}

export function LoginPage() {
  const { user, loading, error, signIn } = useAuth()

  if (user) return <Navigate to={roleLanding[user.role]} replace />

  return (
    <div className="grid min-h-screen bg-white lg:grid-cols-[0.9fr_1.1fr]">
      <section className="hidden bg-sidebar px-12 py-10 text-white lg:flex lg:flex-col">
        <div className="flex items-center gap-3">
          <div className="grid h-10 w-10 place-items-center rounded-lg bg-primary">
            <ShieldCheck className="h-6 w-6" />
          </div>
          <div>
            <div className="text-lg font-semibold">CyberSpace</div>
            <div className="text-xs text-slate-400">College security lab</div>
          </div>
        </div>
        <div className="my-auto max-w-xl">
          <div className="mb-7 grid h-14 w-14 place-items-center rounded-xl border border-slate-700 bg-slate-800">
            <LockKeyhole className="h-7 w-7 text-blue-300" />
          </div>
          <h1 className="max-w-lg text-4xl font-semibold leading-tight tracking-tight">
            Learn securely. Assess confidently.
          </h1>
          <p className="mt-5 max-w-lg text-lg leading-8 text-slate-300">
            A controlled learning and assessment workspace for authorized college
            cybersecurity training.
          </p>
          <div className="mt-10 border-l-2 border-primary pl-4 text-sm leading-6 text-slate-400">
            All assessment environments shown in this prototype are isolated
            simulations. Use is limited to approved academic activities.
          </div>
        </div>
        <p className="text-xs text-slate-500">Authorized college access only</p>
      </section>

      <main className="flex items-center justify-center bg-app px-5 py-10 sm:px-8">
        <div className="w-full max-w-md">
          <div className="mb-8 flex items-center gap-3 lg:hidden">
            <div className="grid h-10 w-10 place-items-center rounded-lg bg-primary text-white">
              <ShieldCheck className="h-6 w-6" />
            </div>
            <div>
              <div className="font-semibold text-strong">CyberSpace</div>
              <div className="text-xs text-secondary">College security lab</div>
            </div>
          </div>

          <div className="rounded-xl border border-border bg-white p-6 shadow-sm sm:p-8">
            <h2 className="text-2xl font-semibold tracking-tight text-strong">
              Welcome back
            </h2>
            <p className="mt-2 text-sm leading-6 text-secondary">
              Sign in with your college Google account to reach your learning
              workspace.
            </p>

            {error && (
              <p className="mt-5 rounded-lg bg-red-50 px-3 py-2 text-sm font-medium text-danger" role="alert">
                {error}
              </p>
            )}

            <Button
              type="button"
              variant="secondary"
              onClick={signIn}
              disabled={loading}
              className="mt-7 w-full"
            >
              <GoogleMark />
              Continue with Google
            </Button>

            <p className="mt-6 border-t border-border pt-5 text-xs leading-5 text-secondary">
              Access is limited to accounts on your college's Google Workspace
              domain. Your account must already be registered by an
              administrator before you can sign in.
            </p>
          </div>
        </div>
      </main>
    </div>
  )
}
