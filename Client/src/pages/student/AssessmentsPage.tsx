import { ArrowRight, CalendarClock, CheckCircle2, Clock3, DoorOpen, Hourglass, Target, Trophy } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { Button } from '../../components/ui/Button'
import { EmptyState, ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { usePolling } from '../../hooks/usePolling'
import { attemptOutcome } from '../../lib/assessment'
import { formatDateTime } from '../../lib/format'
import { studentApi } from '../../services/assessments'
import type { StudentOverview } from '../../types/assessment'

export function AssessmentsPage() {
  const { data, error, loading, refresh } = usePolling(studentApi.overview, 30000)
  const navigate = useNavigate()

  if (loading) return <LoadingSkeleton rows={4} />
  if (!data) return <ErrorState message={error || 'Your assessment record could not be loaded.'} onRetry={refresh} />

  return (
    <div className="space-y-6">
      <div>
        <p className="eyebrow">Level assessments</p>
        <h2 className="page-heading">Your assessments</h2>
        <p className="mt-2 max-w-2xl text-secondary">
          Each level is one practical sitting: a Wireshark capture to analyse and an Nmap target to scan. Pass a
          level to unlock the next one.
        </p>
      </div>

      <StatusCard overview={data} onEnter={() => navigate('/student/lobby')} />

      <section className="surface">
        <div className="border-b border-border p-4 sm:p-5">
          <h3 className="font-semibold text-strong">Attempt history</h3>
        </div>
        <div className="p-4 sm:p-5">
          {data.history.length === 0 ? (
            <EmptyState title="No attempts yet" description="Your attempts and results will appear here." />
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full min-w-[640px] text-left text-sm">
                <thead className="text-xs uppercase tracking-wide text-muted">
                  <tr>
                    <th className="pb-3 font-semibold">Level</th>
                    <th className="pb-3 font-semibold">Result</th>
                    <th className="pb-3 font-semibold">Score</th>
                    <th className="pb-3 font-semibold">Started</th>
                    <th className="pb-3 font-semibold">Ended</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {data.history.map((attempt) => {
                    const outcome = attemptOutcome(attempt)
                    return (
                      <tr key={attempt.id}>
                        <td className="py-3 font-medium text-strong">{attempt.level}</td>
                        <td className="py-3">
                          <StatusBadge tone={outcome.tone}>{outcome.label}</StatusBadge>
                          {attempt.reason && attempt.status === 'revoked' && (
                            <p className="mt-1 text-xs text-secondary">{attempt.reason}</p>
                          )}
                        </td>
                        <td className="py-3 text-secondary">
                          {attempt.status === 'revoked' ? '—' : `${attempt.score} / ${attempt.maxScore}`}
                        </td>
                        <td className="py-3 text-secondary">{formatDateTime(attempt.startedAt ?? undefined)}</td>
                        <td className="py-3 text-secondary">{formatDateTime(attempt.endedAt ?? undefined)}</td>
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

function StatusCard({ overview, onEnter }: { overview: StudentOverview; onEnter: () => void }) {
  const assessment = overview.assessment

  if (overview.status === 'in_progress' && overview.activeAttemptId) {
    return (
      <Banner icon={<Hourglass className="h-6 w-6" />} title="Your assessment is in progress" tone="info">
        <p>Return to your workspace. The timer keeps running while you are away.</p>
        <Link
          to={`/student/attempts/${overview.activeAttemptId}`}
          className="mt-4 inline-flex min-h-10 items-center gap-2 rounded-lg bg-primary px-3.5 text-sm font-semibold text-white hover:bg-primary-dark"
        >
          Resume assessment
          <ArrowRight className="h-4 w-4" />
        </Link>
      </Banner>
    )
  }
  if (overview.status === 'completed') {
    return (
      <Banner icon={<Trophy className="h-6 w-6" />} title="You have passed every level" tone="success">
        <p>New levels will appear here when they are published.</p>
      </Banner>
    )
  }
  if (overview.status === 'unavailable' || !assessment) {
    return (
      <Banner icon={<Target className="h-6 w-6" />} title="No assessment is open" tone="neutral">
        <p>No level assessment has been published yet.</p>
      </Banner>
    )
  }

  return (
    <section className="surface p-5 sm:p-6">
      <div className="grid gap-6 lg:grid-cols-[minmax(0,1fr)_260px]">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge tone="info">{assessment.level}</StatusBadge>
            {overview.status === 'cooldown' && <StatusBadge tone="warning">Cooldown</StatusBadge>}
          </div>
          <h3 className="mt-3 text-xl font-semibold text-strong">{assessment.title}</h3>
          <div className="mt-3 flex flex-wrap gap-x-6 gap-y-2 text-sm text-secondary">
            <span className="flex items-center gap-2">
              <Clock3 className="h-4 w-4" />
              {Math.round(assessment.durationSeconds / 60)} minutes
            </span>
            <span className="flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4" />
              Pass mark {assessment.passScore} / {assessment.totalPoints}
            </span>
          </div>
          <ol className="mt-5 space-y-2 text-sm text-secondary">
            <li>1. Enter the test portal at your booked slot and stay on the page.</li>
            <li>2. The administrator takes attendance and starts the test.</li>
            <li>3. You get one Wireshark and one Nmap question. Submit each flag as you find it.</li>
          </ol>
        </div>
        <div className="flex flex-col justify-center rounded-lg bg-app p-4">
          {overview.status === 'cooldown' ? (
            <>
              <p className="flex items-center gap-2 text-sm font-semibold text-strong">
                <CalendarClock className="h-4 w-4 text-warning" />
                Retake available after
              </p>
              <p className="mt-1 text-sm text-secondary">{formatDateTime(overview.cooldownUntil ?? undefined)}</p>
              <p className="mt-3 text-xs text-secondary">Book a new slot for after this time.</p>
            </>
          ) : (
            <>
              <p className="text-sm text-secondary">Ready when your slot begins.</p>
              <Button className="mt-3" onClick={onEnter}>
                <DoorOpen className="h-4 w-4" />
                Enter test portal
              </Button>
            </>
          )}
        </div>
      </div>
    </section>
  )
}

function Banner({ icon, title, tone, children }: { icon: React.ReactNode; title: string; tone: 'info' | 'success' | 'neutral'; children: React.ReactNode }) {
  const colors = {
    info: 'border-blue-200 bg-primary-light text-blue-900',
    success: 'border-emerald-200 bg-emerald-50 text-emerald-900',
    neutral: 'border-border bg-white text-strong',
  }[tone]
  return (
    <section className={`flex items-start gap-4 rounded-xl border p-5 ${colors}`}>
      <div className="mt-0.5 shrink-0">{icon}</div>
      <div className="text-sm">
        <h3 className="text-lg font-semibold">{title}</h3>
        <div className="mt-1">{children}</div>
      </div>
    </section>
  )
}
