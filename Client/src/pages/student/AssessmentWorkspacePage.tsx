import { AlertTriangle, ArrowLeft, Check, Clock3, Download, Flag, Network, Server } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Button } from '../../components/ui/Button'
import { ConfirmDialog } from '../../components/ui/ConfirmDialog'
import { ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { useToast } from '../../components/ui/Toast'
import { useNow, usePolling } from '../../hooks/usePolling'
import { attemptOutcome, formatCountdown, labLabel, poolName, remainingMs, serverOffset } from '../../lib/assessment'
import { formatDateTime } from '../../lib/format'
import { ApiError, errorMessage } from '../../services/api'
import { studentApi } from '../../services/assessments'
import type { AttemptView, DealtQuestion, FlagSlot, LabView } from '../../types/assessment'

export function AssessmentWorkspacePage() {
  const attemptId = Number(useParams().attemptId)
  const { notify } = useToast()
  const load = useMemo(() => () => studentApi.attempt(attemptId), [attemptId])
  // Polling picks up the lab becoming ready and an administrator ending
  // or revoking the attempt.
  const { data: attempt, error, loading, refresh } = usePolling(load, 8000)
  const now = useNow()
  const [finishOpen, setFinishOpen] = useState(false)
  // Stable, because the dialog refocuses whenever its handler changes and
  // the countdown re-renders this page every second.
  const cancelFinish = useCallback(() => setFinishOpen(false), [])

  const offset = useMemo(() => (attempt ? serverOffset(attempt.serverTime) : 0), [attempt])
  const remaining = attempt ? remainingMs(attempt.deadlineAt, offset, now) : 0
  const inProgress = attempt?.status === 'in_progress'

  // At zero the server's expiry job scores the attempt within seconds.
  const expiredRefresh = useRef(false)
  useEffect(() => {
    if (inProgress && remaining === 0 && !expiredRefresh.current) {
      expiredRefresh.current = true
      window.setTimeout(() => void refresh(), 6000)
    }
  }, [inProgress, remaining, refresh])

  if (!Number.isInteger(attemptId) || attemptId <= 0) return <ErrorState message="This attempt does not exist." />
  if (loading) return <LoadingSkeleton rows={6} />
  if (!attempt) return <ErrorState message={error || 'This attempt could not be loaded.'} onRetry={refresh} />
  if (!inProgress) return <AttemptResult attempt={attempt} />

  const questions = attempt.questions ?? []
  const solved = questions.flatMap((q) => q.flags).filter((f) => f.solved).length
  const totalFlags = questions.flatMap((q) => q.flags).length

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Link to="/student/assessments" className="inline-flex items-center gap-2 text-sm font-semibold text-secondary hover:text-primary">
          <ArrowLeft className="h-4 w-4" />
          Assessments
        </Link>
        <div className="flex items-center gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-sm font-medium text-amber-900">
          <AlertTriangle className="h-4 w-4" />
          Scan only the target IP you are given
        </div>
      </div>

      <section className="surface grid gap-4 p-4 sm:p-5 lg:grid-cols-[1fr_auto_auto_auto] lg:items-center">
        <div>
          <div className="flex items-center gap-2">
            <StatusBadge tone="info">In progress</StatusBadge>
            <span className="text-sm text-secondary">{attempt.level}</span>
          </div>
          <h2 className="mt-2 text-xl font-semibold text-strong">{attempt.title}</h2>
        </div>
        <Stat label="Score" value={`${attempt.score} / ${attempt.maxScore}`} detail={`Pass ${attempt.passScore}`} />
        <Stat label="Flags" value={`${solved} / ${totalFlags}`} detail="solved" />
        <div className={`rounded-lg border px-4 py-2 text-center ${remaining < 5 * 60000 ? 'border-red-200 bg-red-50' : 'border-border bg-app'}`}>
          <div className="text-xs font-semibold uppercase tracking-wide text-secondary">Time remaining</div>
          <div className="mt-1 flex items-center justify-center gap-2 font-mono text-lg font-semibold text-strong" aria-live="off">
            <Clock3 className="h-4 w-4 text-primary" />
            {formatCountdown(remaining)}
          </div>
        </div>
      </section>
      {error && <p role="status" className="text-sm text-warning">Connection problem: {error} Retrying…</p>}

      <div className="grid gap-4 xl:grid-cols-2">
        {questions.map((question) => (
          <QuestionCard
            key={question.id}
            attemptId={attempt.id}
            question={question}
            lab={question.pool === 'nmap' ? attempt.lab : null}
            onChange={refresh}
          />
        ))}
      </div>

      <div className="flex justify-end">
        <Button variant="danger" onClick={() => setFinishOpen(true)}>
          Finish and submit
        </Button>
      </div>

      <ConfirmDialog
        open={finishOpen}
        title="Finish this assessment?"
        description={`You have solved ${solved} of ${totalFlags} flags for ${attempt.score} points. Finishing submits your attempt for scoring and stops your lab. You cannot return to it.`}
        confirmLabel="Finish and submit"
        tone="danger"
        onCancel={cancelFinish}
        onConfirm={async () => {
          try {
            await studentApi.finish(attempt.id)
            notify('Assessment submitted.')
          } catch (err) {
            notify(errorMessage(err), 'error')
          } finally {
            setFinishOpen(false)
            await refresh()
          }
        }}
      />
    </div>
  )
}

function Stat({ label, value, detail }: { label: string; value: string; detail: string }) {
  return (
    <div className="rounded-lg border border-border bg-app px-4 py-2 text-center">
      <div className="text-xs font-semibold uppercase tracking-wide text-secondary">{label}</div>
      <div className="mt-1 text-lg font-semibold text-strong">{value}</div>
      <div className="text-xs text-secondary">{detail}</div>
    </div>
  )
}

function QuestionCard({ attemptId, question, lab, onChange }: { attemptId: number; question: DealtQuestion; lab: LabView | null; onChange: () => Promise<void> }) {
  return (
    <section className="surface flex flex-col" aria-label={`${poolName[question.pool]} question`}>
      <header className="flex flex-wrap items-center justify-between gap-2 border-b border-border p-4 sm:p-5">
        <div className="flex items-center gap-2">
          {question.pool === 'wireshark' ? <Network className="h-5 w-5 text-primary" /> : <Server className="h-5 w-5 text-primary" />}
          <h3 className="font-semibold text-strong">{poolName[question.pool]}</h3>
          <StatusBadge tone={question.difficulty === 'hard' ? 'warning' : 'neutral'}>{question.difficulty}</StatusBadge>
        </div>
        <span className="text-sm font-medium text-secondary">
          {question.score} / {question.points} points
        </span>
      </header>
      <div className="space-y-5 p-4 sm:p-5">
        <p className="whitespace-pre-line text-sm leading-6 text-strong">{question.prompt}</p>

        {question.pool === 'wireshark' && question.hasPcap && (
          <a
            href={studentApi.pcapUrl(attemptId, question.id)}
            className="inline-flex min-h-10 items-center gap-2 rounded-lg border border-border px-3.5 text-sm font-semibold text-strong hover:bg-subtle"
          >
            <Download className="h-4 w-4" />
            Download capture file
          </a>
        )}
        {question.pool === 'nmap' && <TargetPanel lab={lab} />}

        <div className="space-y-3">
          {question.flags.map((flag, index) => (
            <FlagForm key={flag.id} attemptId={attemptId} flag={flag} index={index} onChange={onChange} />
          ))}
        </div>
      </div>
    </section>
  )
}

function TargetPanel({ lab }: { lab: LabView | null }) {
  const state = labLabel(lab?.status)
  return (
    <div className="rounded-lg border border-border bg-app p-4">
      <div className="flex items-center justify-between">
        <span className="text-sm text-secondary">Target</span>
        <StatusBadge tone={state.tone}>{state.label}</StatusBadge>
      </div>
      <p className="mt-2 font-mono text-lg font-semibold text-strong">
        {lab?.status === 'running' && lab.ip ? lab.ip : 'Available once the target is running'}
      </p>
      {lab?.status === 'failed' && (
        <p className="mt-2 text-sm text-danger">The target failed to start. Tell the administrator; they can restart it.</p>
      )}
    </div>
  )
}

function FlagForm({ attemptId, flag, index, onChange }: { attemptId: number; flag: FlagSlot; index: number; onChange: () => Promise<void> }) {
  const [answer, setAnswer] = useState('')
  const [busy, setBusy] = useState(false)
  const [feedback, setFeedback] = useState<{ ok: boolean; text: string } | null>(null)
  const inputId = `flag-${flag.id}`

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    if (!answer.trim()) {
      setFeedback({ ok: false, text: 'Enter an answer first.' })
      return
    }
    setBusy(true)
    try {
      const result = await studentApi.submit(attemptId, flag.id, answer)
      setFeedback(result.correct ? { ok: true, text: `Correct: +${result.points} points.` } : { ok: false, text: 'Incorrect. Try again.' })
      if (result.correct) setAnswer('')
      await onChange()
    } catch (err) {
      setFeedback({ ok: false, text: errorMessage(err) })
      if (err instanceof ApiError && (err.code === 'attempt_closed' || err.code === 'already_solved')) await onChange()
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className={`rounded-lg border p-3 ${flag.solved ? 'border-emerald-200 bg-emerald-50' : 'border-border'}`}>
      <div className="flex items-start justify-between gap-3">
        <label htmlFor={inputId} className="text-sm font-medium text-strong">
          {index === 0 ? 'Flag' : `Q${index}`}: {flag.prompt}
        </label>
        <span className="shrink-0 text-xs font-semibold text-secondary">{flag.points} pts</span>
      </div>
      {flag.solved ? (
        <p className="mt-2 flex items-center gap-1.5 text-sm font-semibold text-success">
          <Check className="h-4 w-4" />
          Solved
        </p>
      ) : (
        <div className="mt-2 flex gap-2">
          <input
            id={inputId}
            className="input font-mono"
            value={answer}
            maxLength={256}
            autoComplete="off"
            spellCheck={false}
            onChange={(e) => setAnswer(e.target.value)}
            placeholder="Your answer"
          />
          <Button type="submit" disabled={busy}>
            <Flag className="h-4 w-4" />
            Submit
          </Button>
        </div>
      )}
      {feedback && !flag.solved && (
        <p role="status" className={`mt-2 text-sm font-medium ${feedback.ok ? 'text-success' : 'text-warning'}`}>
          {feedback.text}
        </p>
      )}
      {!flag.solved && flag.submissions > 0 && <p className="mt-1 text-xs text-muted">{flag.submissions} attempt(s) so far</p>}
    </form>
  )
}

function AttemptResult({ attempt }: { attempt: AttemptView }) {
  const outcome = attemptOutcome(attempt)
  return (
    <div className="space-y-5">
      <Link to="/student/assessments" className="inline-flex items-center gap-2 text-sm font-semibold text-secondary hover:text-primary">
        <ArrowLeft className="h-4 w-4" />
        Back to assessments
      </Link>
      <section className="surface p-6">
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge tone={outcome.tone}>{outcome.label}</StatusBadge>
          <span className="text-sm text-secondary">{attempt.level}</span>
        </div>
        <h2 className="page-heading mt-4">{attempt.title}</h2>
        <dl className="mt-6 grid gap-4 sm:grid-cols-3">
          <div>
            <dt className="text-sm text-secondary">Score</dt>
            <dd className="mt-1 text-lg font-semibold">
              {attempt.status === 'revoked' ? '—' : `${attempt.score} / ${attempt.maxScore}`}
            </dd>
          </div>
          <div>
            <dt className="text-sm text-secondary">Pass mark</dt>
            <dd className="mt-1 text-lg font-semibold">{attempt.passScore}</dd>
          </div>
          <div>
            <dt className="text-sm text-secondary">Ended</dt>
            <dd className="mt-1 font-medium">{formatDateTime(attempt.endedAt ?? undefined)}</dd>
          </div>
        </dl>
        {attempt.reason && <p className="mt-6 rounded-lg bg-app p-4 text-sm text-secondary">{attempt.reason}</p>}
        {outcome.label === 'Not passed' || attempt.status === 'revoked' ? (
          <p className="mt-4 text-sm text-secondary">You can book a new slot after the one-day cooldown.</p>
        ) : null}
      </section>
    </div>
  )
}
