import { useEffect, useState } from 'react'
import { EmptyState, ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { SearchInput } from '../../components/ui/SearchInput'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { attemptOutcome } from '../../lib/assessment'
import { formatDateTime } from '../../lib/format'
import { errorMessage } from '../../services/api'
import { adminApi } from '../../services/assessments'
import type { StudentRecord } from '../../types/assessment'

export function StudentsPage() {
  const [search, setSearch] = useState('')
  const [records, setRecords] = useState<StudentRecord[] | null>(null)
  const [error, setError] = useState('')
  const [reload, setReload] = useState(0)

  // The server filters; a short pause avoids a request per keystroke.
  useEffect(() => {
    let cancelled = false
    const timer = window.setTimeout(() => {
      adminApi
        .students(search.trim())
        .then((result) => { if (!cancelled) { setRecords(result); setError('') } })
        .catch((err) => { if (!cancelled) setError(errorMessage(err, 'Students could not be loaded.')) })
    }, 250)
    return () => { cancelled = true; window.clearTimeout(timer) }
  }, [search, reload])

  return (
    <div className="space-y-6">
      <div>
        <p className="eyebrow">Student records</p>
        <h2 className="page-heading">Students</h2>
        <p className="mt-2 max-w-2xl text-secondary">Level progress, latest attempt, and retake cooldown for every student account.</p>
      </div>
      <section className="surface">
        <div className="border-b border-border p-4 sm:p-5">
          <SearchInput value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search by name, email, or register number" maxLength={100} />
        </div>
        <div className="p-4 sm:p-5">
          {error ? (
            <ErrorState message={error} onRetry={() => setReload((n) => n + 1)} />
          ) : !records ? (
            <LoadingSkeleton rows={5} />
          ) : records.length === 0 ? (
            <EmptyState title="No students found" description="Student accounts are created by an administrator; none match this search." />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[820px] text-left text-sm">
                <thead className="text-xs uppercase tracking-wide text-muted">
                  <tr>
                    <th className="pb-3 font-semibold">Student</th>
                    <th className="pb-3 font-semibold">Account</th>
                    <th className="pb-3 font-semibold">Levels passed</th>
                    <th className="pb-3 font-semibold">Latest attempt</th>
                    <th className="pb-3 font-semibold">Retake</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {records.map((record) => {
                    const outcome = record.lastAttempt ? attemptOutcome(record.lastAttempt) : null
                    return (
                      <tr key={record.id}>
                        <td className="py-3">
                          <p className="font-medium text-strong">{record.name}</p>
                          <p className="text-xs text-secondary">{record.registerNumber ? `${record.registerNumber} · ` : ''}{record.email}</p>
                        </td>
                        <td className="py-3">
                          <StatusBadge tone={record.status === 'active' ? 'success' : 'neutral'}>{record.status}</StatusBadge>
                        </td>
                        <td className="py-3 font-medium text-strong">{record.levelsPassed}</td>
                        <td className="py-3">
                          {record.lastAttempt && outcome ? (
                            <>
                              <StatusBadge tone={outcome.tone}>{outcome.label}</StatusBadge>
                              <p className="mt-1 text-xs text-secondary">
                                {record.lastAttempt.level} · {record.lastAttempt.status === 'revoked' ? 'revoked' : `${record.lastAttempt.score}/${record.lastAttempt.maxScore}`}
                              </p>
                            </>
                          ) : (
                            <span className="text-secondary">—</span>
                          )}
                        </td>
                        <td className="py-3 text-secondary">
                          {record.cooldownUntil ? `After ${formatDateTime(record.cooldownUntil)}` : 'Available'}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </section>
    </div>
  )
}
