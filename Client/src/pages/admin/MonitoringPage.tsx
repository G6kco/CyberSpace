import { Activity, Ban, CheckCircle2, ClipboardCheck, Eye, Play, RotateCcw, Square, UserCheck, UserMinus, Users } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { Button } from '../../components/ui/Button'
import { ConfirmDialog } from '../../components/ui/ConfirmDialog'
import { Drawer } from '../../components/ui/Drawer'
import { DropdownItem, DropdownMenu } from '../../components/ui/DropdownMenu'
import { EmptyState, ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { SearchInput } from '../../components/ui/SearchInput'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { useToast } from '../../components/ui/Toast'
import { useNow, usePolling } from '../../hooks/usePolling'
import { attemptOutcome, eligibilityLabel, formatCountdown, labLabel, poolName } from '../../lib/assessment'
import { formatDateTime } from '../../lib/format'
import { errorMessage } from '../../services/api'
import { adminApi } from '../../services/assessments'
import type { ActionResult, AdminAttempt, AttemptView, LobbyEntry } from '../../types/assessment'

type Tab = 'lobby' | 'live' | 'results'

export function MonitoringPage() {
  const [tab, setTab] = useState<Tab>('lobby')
  const tabs: Array<{ id: Tab; label: string; icon: typeof Users }> = [
    { id: 'lobby', label: 'Test portal', icon: Users },
    { id: 'live', label: 'Live attempts', icon: Activity },
    { id: 'results', label: 'Results', icon: ClipboardCheck },
  ]
  return (
    <div className="space-y-6">
      <div>
        <p className="eyebrow">Operational control</p>
        <h2 className="page-heading">Assessment monitoring</h2>
        <p className="mt-2 max-w-2xl text-secondary">
          Take attendance of students in the test portal, start their test, and supervise attempts and labs as they run.
        </p>
      </div>
      <div role="tablist" aria-label="Monitoring views" className="flex gap-1 rounded-lg border border-border bg-white p-1">
        {tabs.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            role="tab"
            aria-selected={tab === id}
            onClick={() => setTab(id)}
            className={`flex min-h-10 flex-1 items-center justify-center gap-2 rounded-md text-sm font-semibold transition ${tab === id ? 'bg-primary text-white' : 'text-secondary hover:bg-subtle'}`}
          >
            <Icon className="h-4 w-4" />
            {label}
          </button>
        ))}
      </div>
      {tab === 'lobby' && <LobbyPanel />}
      {tab === 'live' && <AttemptsPanel scope="active" />}
      {tab === 'results' && <AttemptsPanel scope="recent" />}
    </div>
  )
}

// ---- Test portal -----------------------------------------------------------

function LobbyPanel() {
  const { data, error, loading, refresh } = usePolling(adminApi.lobby, 5000)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [search, setSearch] = useState('')
  const [confirmStart, setConfirmStart] = useState(false)
  const [results, setResults] = useState<{ title: string; items: ActionResult[] } | null>(null)
  const { notify } = useToast()
  // Stable, because the dialog refocuses whenever its handler changes and
  // this panel re-renders on every poll.
  const cancelStart = useCallback(() => setConfirmStart(false), [])

  const students = useMemo(() => {
    const term = search.toLowerCase()
    return (data ?? []).filter((s) => !term || `${s.name} ${s.email} ${s.registerNumber}`.toLowerCase().includes(term))
  }, [data, search])
  const byId = useMemo(() => new Map((data ?? []).map((s) => [s.id, s])), [data])
  const chosen = [...selected].map((id) => byId.get(id)).filter((s): s is LobbyEntry => Boolean(s))
  const toMark = chosen.filter((s) => s.online && !s.attendanceMarked && s.eligibility === 'eligible')
  const toStart = chosen.filter((s) => s.attendanceMarked && s.eligibility === 'eligible')

  const toggle = (id: string) => setSelected((current) => {
    const next = new Set(current)
    if (next.has(id)) next.delete(id)
    else next.add(id)
    return next
  })

  const report = (title: string, items: ActionResult[]) => {
    const failed = items.filter((r) => !r.ok || r.warning)
    notify(`${title}: ${items.length - items.filter((r) => !r.ok).length} of ${items.length} succeeded.`, failed.length ? 'error' : 'success')
    setResults(failed.length ? { title, items: failed } : null)
    setSelected(new Set())
  }

  const markPresent = async () => {
    try {
      report('Attendance', await adminApi.markAttendance(toMark.map((s) => s.id)))
    } catch (err) {
      notify(errorMessage(err), 'error')
    }
    await refresh()
  }

  const withdraw = async (student: LobbyEntry) => {
    try {
      await adminApi.cancelAttendance(student.id)
      notify(`Attendance withdrawn for ${student.name}.`)
    } catch (err) {
      notify(errorMessage(err), 'error')
    }
    await refresh()
  }

  if (loading) return <LoadingSkeleton rows={4} />
  if (!data) return <ErrorState message={error || 'The test portal could not be loaded.'} onRetry={refresh} />

  const online = data.filter((s) => s.online).length
  const marked = data.filter((s) => s.attendanceMarked).length

  return (
    <section className="surface">
      <div className="grid gap-3 border-b border-border p-4 sm:p-5 lg:grid-cols-[1fr_auto]">
        <div className="flex flex-wrap items-center gap-3 text-sm text-secondary">
          <span><strong className="text-strong">{online}</strong> in portal</span>
          <span><strong className="text-strong">{marked}</strong> attendance taken</span>
          {error && <span className="text-warning">Refresh failed: {error}</span>}
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="secondary" onClick={() => setSelected(new Set(data.filter((s) => s.online || s.attendanceMarked).map((s) => s.id)))}>
            Select all
          </Button>
          <Button variant="secondary" disabled={toMark.length === 0} onClick={markPresent}>
            <UserCheck className="h-4 w-4" />
            Mark present ({toMark.length})
          </Button>
          <Button disabled={toStart.length === 0} onClick={() => setConfirmStart(true)}>
            <Play className="h-4 w-4" />
            Start test ({toStart.length})
          </Button>
        </div>
        <div className="lg:col-span-2">
          <SearchInput value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search by name, email, or register number" />
        </div>
      </div>

      {results && (
        <div role="alert" className="border-b border-red-200 bg-red-50 p-4 text-sm text-red-900">
          <p className="font-semibold">{results.title}: some students need attention</p>
          <ul className="mt-2 space-y-1">
            {results.items.map((r) => (
              <li key={r.studentId}>
                {byId.get(r.studentId)?.name ?? r.studentId}: {r.error ?? r.warning}
              </li>
            ))}
          </ul>
        </div>
      )}

      <div className="p-4 sm:p-5">
        {students.length === 0 ? (
          <EmptyState title="Nobody is in the test portal" description="Students appear here when they open the test portal from their assessments page." />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[820px] text-left text-sm">
              <thead className="text-xs uppercase tracking-wide text-muted">
                <tr>
                  <th className="w-10 pb-3"><span className="sr-only">Select</span></th>
                  <th className="pb-3 font-semibold">Student</th>
                  <th className="pb-3 font-semibold">Level</th>
                  <th className="pb-3 font-semibold">Portal</th>
                  <th className="pb-3 font-semibold">Attendance</th>
                  <th className="pb-3 font-semibold">Status</th>
                  <th className="pb-3"><span className="sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {students.map((student) => {
                  const eligibility = eligibilityLabel(student.eligibility)
                  return (
                    <tr key={student.id}>
                      <td className="py-3">
                        <input
                          type="checkbox"
                          className="h-4 w-4"
                          checked={selected.has(student.id)}
                          onChange={() => toggle(student.id)}
                          aria-label={`Select ${student.name}`}
                        />
                      </td>
                      <td className="py-3">
                        <p className="font-medium text-strong">{student.name}</p>
                        <p className="text-xs text-secondary">{student.registerNumber || student.email}</p>
                      </td>
                      <td className="py-3 text-secondary">{student.level || '—'}</td>
                      <td className="py-3">
                        <StatusBadge tone={student.online ? 'success' : 'neutral'}>{student.online ? 'Online' : 'Offline'}</StatusBadge>
                      </td>
                      <td className="py-3">
                        {student.attendanceMarked ? <StatusBadge tone="info">Present</StatusBadge> : <span className="text-secondary">—</span>}
                      </td>
                      <td className="py-3"><StatusBadge tone={eligibility.tone}>{eligibility.label}</StatusBadge></td>
                      <td className="py-3 text-right">
                        {student.attendanceMarked && (
                          <Button variant="ghost" onClick={() => withdraw(student)} aria-label={`Withdraw attendance for ${student.name}`}>
                            <UserMinus className="h-4 w-4" />
                          </Button>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={confirmStart}
        title="Start the test?"
        description={`${toStart.length} student(s) will each be dealt one Wireshark and one Nmap question, their lab will start, and their one-hour timer begins now.`}
        confirmLabel="Start test"
        onCancel={cancelStart}
        onConfirm={async () => {
          try {
            report('Start', await adminApi.start(toStart.map((s) => s.id)))
          } catch (err) {
            notify(errorMessage(err), 'error')
          } finally {
            setConfirmStart(false)
            await refresh()
          }
        }}
      />
    </section>
  )
}

// ---- Attempts --------------------------------------------------------------

type PendingAction = { kind: 'end' | 'revoke' | 'lab'; attempt: AdminAttempt }

function AttemptsPanel({ scope }: { scope: 'active' | 'recent' }) {
  const load = useMemo(() => () => adminApi.attempts(scope), [scope])
  const { data, error, loading, refresh } = usePolling(load, scope === 'active' ? 5000 : 15000)
  const [search, setSearch] = useState('')
  const [pending, setPending] = useState<PendingAction | null>(null)
  const [inspected, setInspected] = useState<AttemptView | null>(null)
  const now = useNow()
  const { notify } = useToast()
  // Stable, because dialogs refocus whenever their handler changes and the
  // countdown re-renders this panel every second.
  const cancelPending = useCallback(() => setPending(null), [])
  const closeInspected = useCallback(() => setInspected(null), [])

  const attempts = useMemo(() => {
    const term = search.toLowerCase()
    return (data ?? []).filter((a) => !term || `${a.student.name} ${a.student.email} ${a.student.registerNumber}`.toLowerCase().includes(term))
  }, [data, search])

  const inspect = async (attempt: AdminAttempt) => {
    try {
      setInspected(await adminApi.attempt(attempt.id))
    } catch (err) {
      notify(errorMessage(err), 'error')
    }
  }

  const execute = async (reason?: string) => {
    if (!pending) return
    const { kind, attempt } = pending
    try {
      if (kind === 'end') await adminApi.end(attempt.id)
      if (kind === 'revoke') await adminApi.revoke(attempt.id, reason ?? '')
      if (kind === 'lab') await adminApi.restartLab(attempt.id)
      notify(kind === 'end' ? 'Attempt ended and scored.' : kind === 'revoke' ? 'Attempt revoked.' : 'Lab restart queued.')
    } catch (err) {
      notify(errorMessage(err), 'error')
    } finally {
      setPending(null)
      await refresh()
    }
  }

  if (loading) return <LoadingSkeleton rows={4} />
  if (!data) return <ErrorState message={error || 'Attempts could not be loaded.'} onRetry={refresh} />

  return (
    <section className="surface">
      <div className="flex flex-wrap items-center gap-3 border-b border-border p-4 sm:p-5">
        <div className="min-w-60 flex-1">
          <SearchInput value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search students" />
        </div>
        <span className="text-sm text-secondary">{attempts.length} attempt(s)</span>
        {error && <span className="text-sm text-warning">Refresh failed: {error}</span>}
      </div>
      <div className="p-4 sm:p-5">
        {attempts.length === 0 ? (
          <EmptyState
            title={scope === 'active' ? 'No attempts in progress' : 'No attempts yet'}
            description={scope === 'active' ? 'Started tests appear here.' : 'Finished attempts appear here.'}
          />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[900px] text-left text-sm">
              <thead className="text-xs uppercase tracking-wide text-muted">
                <tr>
                  <th className="pb-3 font-semibold">Student</th>
                  <th className="pb-3 font-semibold">Questions</th>
                  <th className="pb-3 font-semibold">Lab</th>
                  <th className="pb-3 font-semibold">Score</th>
                  <th className="pb-3 font-semibold">{scope === 'active' ? 'Time left' : 'Result'}</th>
                  <th className="pb-3 font-semibold">{scope === 'active' ? 'Started' : 'Ended'}</th>
                  <th className="pb-3"><span className="sr-only">Actions</span></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {attempts.map((attempt) => {
                  const lab = labLabel(attempt.lab?.status)
                  const outcome = attemptOutcome(attempt)
                  const active = attempt.status === 'in_progress'
                  const left = attempt.deadlineAt ? Math.max(0, new Date(attempt.deadlineAt).getTime() - now) : 0
                  return (
                    <tr key={attempt.id}>
                      <td className="py-3">
                        <p className="font-medium text-strong">{attempt.student.name}</p>
                        <p className="text-xs text-secondary">{attempt.student.registerNumber || attempt.student.email} · {attempt.level}</p>
                      </td>
                      <td className="py-3 text-secondary">
                        {attempt.questions.map((q) => (
                          <p key={q.id}>
                            {poolName[q.pool]} <span className="font-mono text-xs">{q.code}</span> · {q.score}/{q.points}
                          </p>
                        ))}
                      </td>
                      <td className="py-3">
                        <StatusBadge tone={lab.tone}>{lab.label}</StatusBadge>
                        {attempt.lab?.ip && <p className="mt-1 font-mono text-xs text-secondary">{attempt.lab.ip}</p>}
                      </td>
                      <td className="py-3 font-medium text-strong">{attempt.status === 'revoked' ? '—' : `${attempt.score} / ${attempt.maxScore}`}</td>
                      <td className="py-3">
                        {active ? (
                          <span className={`font-mono ${left < 5 * 60000 ? 'text-danger' : 'text-strong'}`}>{formatCountdown(left)}</span>
                        ) : (
                          <>
                            <StatusBadge tone={outcome.tone}>{outcome.label}</StatusBadge>
                            {attempt.reason && <p className="mt-1 text-xs text-secondary">{attempt.reason}</p>}
                          </>
                        )}
                      </td>
                      <td className="py-3 text-secondary">{formatDateTime((active ? attempt.startedAt : attempt.endedAt) ?? undefined)}</td>
                      <td className="py-3 text-right">
                        <DropdownMenu label="Open actions">
                          {(close) => (
                            <>
                              <DropdownItem onClick={() => { void inspect(attempt); close() }}><Eye className="h-4 w-4" />Inspect attempt</DropdownItem>
                              {active && <DropdownItem onClick={() => { setPending({ kind: 'lab', attempt }); close() }}><RotateCcw className="h-4 w-4" />Restart lab</DropdownItem>}
                              {active && <DropdownItem onClick={() => { setPending({ kind: 'end', attempt }); close() }}><Square className="h-4 w-4" />End and score</DropdownItem>}
                              {active && <DropdownItem danger onClick={() => { setPending({ kind: 'revoke', attempt }); close() }}><Ban className="h-4 w-4" />Revoke attempt</DropdownItem>}
                            </>
                          )}
                        </DropdownMenu>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <ConfirmDialog
        open={Boolean(pending)}
        title={pending?.kind === 'end' ? 'End this attempt?' : pending?.kind === 'revoke' ? 'Revoke this attempt?' : 'Restart the lab?'}
        description={
          pending?.kind === 'end'
            ? `${pending.attempt.student.name}'s attempt will be scored now with ${pending.attempt.score} points and the lab stopped.`
            : pending?.kind === 'revoke'
              ? `${pending.attempt.student.name}'s attempt will be closed without a score and counts as not passed. The reason is recorded and shown to the student.`
              : `A fresh lab will be queued for ${pending?.attempt.student.name ?? ''}. Only possible when the current lab has failed or stopped.`
        }
        confirmLabel={pending?.kind === 'end' ? 'End and score' : pending?.kind === 'revoke' ? 'Revoke attempt' : 'Restart lab'}
        tone={pending?.kind === 'lab' ? 'primary' : 'danger'}
        reasonRequired={pending?.kind === 'revoke'}
        onCancel={cancelPending}
        onConfirm={execute}
      />

      <Drawer open={Boolean(inspected)} title={inspected ? `Attempt #${inspected.id}` : ''} onClose={closeInspected}>
        {inspected && <AttemptDetails attempt={inspected} />}
      </Drawer>
    </section>
  )
}

function AttemptDetails({ attempt }: { attempt: AttemptView }) {
  const outcome = attemptOutcome(attempt)
  const lab = labLabel(attempt.lab?.status)
  return (
    <div className="space-y-5 text-sm">
      <dl className="grid grid-cols-2 gap-3 rounded-lg bg-app p-4">
        <Detail label="Status" value={<StatusBadge tone={outcome.tone}>{outcome.label}</StatusBadge>} />
        <Detail label="Score" value={`${attempt.score} / ${attempt.maxScore} (pass ${attempt.passScore})`} />
        <Detail label="Started" value={formatDateTime(attempt.startedAt ?? undefined)} />
        <Detail label="Deadline" value={formatDateTime(attempt.deadlineAt ?? undefined)} />
        <Detail label="Lab" value={<><StatusBadge tone={lab.tone}>{lab.label}</StatusBadge>{attempt.lab?.ip && <span className="ml-2 font-mono">{attempt.lab.ip}</span>}</>} />
        <Detail label="Ended" value={formatDateTime(attempt.endedAt ?? undefined)} />
      </dl>
      {attempt.reason && <p className="rounded-lg bg-app p-3 text-secondary">{attempt.reason}</p>}
      {(attempt.questions ?? []).map((q) => (
        <section key={q.id} className="rounded-lg border border-border p-4">
          <h3 className="font-semibold text-strong">
            {poolName[q.pool]} · <span className="font-mono text-xs">{q.code}</span> · {q.difficulty}
          </h3>
          <ul className="mt-3 space-y-2">
            {q.flags.map((f) => (
              <li key={f.id} className="flex items-start justify-between gap-3">
                <span className="text-secondary">{f.prompt}</span>
                <span className="shrink-0">
                  {f.solved ? <CheckCircle2 className="inline h-4 w-4 text-success" /> : <span className="text-muted">{f.submissions} tries</span>}
                </span>
              </li>
            ))}
          </ul>
        </section>
      ))}
    </div>
  )
}

function Detail({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <dt className="text-xs font-semibold uppercase tracking-wide text-muted">{label}</dt>
      <dd className="mt-1 font-medium text-strong">{value}</dd>
    </div>
  )
}
