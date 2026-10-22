import { UserPlus } from 'lucide-react'
import { useEffect, useState, type FormEvent } from 'react'
import { Button } from '../../components/ui/Button'
import { BusyLabel, EmptyState, ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { useToast } from '../../components/ui/Toast'
import { formatDateTime } from '../../lib/format'
import { errorMessage } from '../../services/api'
import { usersApi } from '../../services/users'
import type { AdminAccount } from '../../types/users'

export function AdministratorsPage() {
  const [admins, setAdmins] = useState<AdminAccount[] | null>(null)
  const [error, setError] = useState('')
  const [reload, setReload] = useState(0)
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [busy, setBusy] = useState(false)
  const { notify } = useToast()

  useEffect(() => {
    let cancelled = false
    usersApi
      .admins()
      .then((result) => { if (!cancelled) { setAdmins(result); setError('') } })
      .catch((err) => { if (!cancelled) setError(errorMessage(err, 'Administrators could not be loaded.')) })
    return () => { cancelled = true }
  }, [reload])

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setBusy(true)
    try {
      const added = await usersApi.createAdmin({ name, email })
      notify(`${added.name} can now sign in as an administrator.`)
      setName('')
      setEmail('')
      setReload((n) => n + 1)
    } catch (err) {
      notify(errorMessage(err, 'The administrator could not be added.'), 'error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-6">
      <div>
        <p className="eyebrow">Accounts</p>
        <h2 className="page-heading">Administrators</h2>
        <p className="mt-2 max-w-2xl text-secondary">Administrators sign in with Google like students. Add each one here before their first sign-in.</p>
      </div>

      <section className="surface p-4 sm:p-5" aria-labelledby="add-admin-heading">
        <h3 id="add-admin-heading" className="font-semibold text-strong">Add an administrator</h3>
        <form onSubmit={submit} className="mt-4 grid gap-4 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
          <label className="block text-sm">
            <span className="font-medium text-strong">Name</span>
            <input className="input mt-1.5" required maxLength={150} value={name} onChange={(e) => setName(e.target.value)} disabled={busy} />
          </label>
          <label className="block text-sm">
            <span className="font-medium text-strong">Google Workspace email</span>
            <input className="input mt-1.5" type="email" required maxLength={254} value={email} onChange={(e) => setEmail(e.target.value)} disabled={busy} />
          </label>
          <Button type="submit" disabled={busy}>
            {busy ? <BusyLabel>Adding</BusyLabel> : <><UserPlus className="h-4 w-4" />Add administrator</>}
          </Button>
        </form>
      </section>

      <section className="surface p-4 sm:p-5">
        {error ? (
          <ErrorState message={error} onRetry={() => setReload((n) => n + 1)} />
        ) : !admins ? (
          <LoadingSkeleton rows={3} />
        ) : admins.length === 0 ? (
          <EmptyState title="No administrators" description="Add one above." />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[620px] text-left text-sm">
              <thead className="text-xs uppercase tracking-wide text-muted">
                <tr>
                  <th className="pb-3 font-semibold">Administrator</th>
                  <th className="pb-3 font-semibold">Account</th>
                  <th className="pb-3 font-semibold">Last sign-in</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {admins.map((admin) => (
                  <tr key={admin.id}>
                    <td className="py-3">
                      <p className="font-medium text-strong">{admin.name}</p>
                      <p className="text-xs text-secondary">{admin.email}</p>
                    </td>
                    <td className="py-3">
                      <StatusBadge tone={admin.status === 'active' ? 'success' : 'neutral'}>{admin.status}</StatusBadge>
                    </td>
                    <td className="py-3 text-secondary">{admin.lastLoginAt ? formatDateTime(admin.lastLoginAt) : 'Never'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  )
}
