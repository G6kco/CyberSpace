import { ArrowLeft, CheckCircle2, LoaderCircle, Radio, UserCheck } from 'lucide-react'
import { useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { usePolling } from '../../hooks/usePolling'
import { formatDateTime } from '../../lib/format'
import { studentApi } from '../../services/assessments'

// The test portal. While this page is open it sends a heartbeat every few
// seconds; the administrator sees the student as present only then. When
// the test is started the heartbeat response names the attempt, and the
// student is taken straight to it.
export function LobbyPage() {
  const { data, error, loading, refresh } = usePolling(studentApi.heartbeat, 5000)
  const navigate = useNavigate()

  useEffect(() => {
    if (data?.activeAttemptId) navigate(`/student/attempts/${data.activeAttemptId}`, { replace: true })
  }, [data?.activeAttemptId, navigate])

  // Leaving the page leaves the portal, so the administrator's list stays
  // accurate without waiting for the heartbeat to lapse.
  useEffect(() => () => void studentApi.leaveLobby().catch(() => undefined), [])

  if (loading) return <LoadingSkeleton rows={3} />
  if (!data) return <ErrorState message={error || 'The test portal could not be opened.'} onRetry={refresh} />

  const back = (
    <Link to="/student/assessments" className="inline-flex items-center gap-2 text-sm font-semibold text-secondary hover:text-primary">
      <ArrowLeft className="h-4 w-4" />
      Back to assessments
    </Link>
  )

  if (data.status !== 'eligible') {
    return (
      <div className="space-y-5">
        {back}
        <section className="surface p-6">
          <h2 className="page-heading">The test portal is not open to you</h2>
          <p className="mt-2 text-secondary">
            {data.status === 'cooldown'
              ? `You can retake after ${formatDateTime(data.cooldownUntil ?? undefined)}.`
              : data.status === 'completed'
                ? 'You have passed every level.'
                : 'There is no assessment for you to take right now.'}
          </p>
        </section>
      </div>
    )
  }

  return (
    <div className="space-y-5">
      {back}
      <section className="surface p-6 sm:p-8">
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge tone="info">{data.assessment?.level}</StatusBadge>
          <span className="flex items-center gap-1.5 text-sm font-medium text-success">
            <Radio className="h-4 w-4" />
            You are in the test portal
          </span>
        </div>
        <h2 className="page-heading mt-4">{data.assessment?.title}</h2>
        <p className="mt-2 max-w-2xl text-secondary">
          Keep this page open. When the administrator starts the test, your questions open automatically.
        </p>

        <div className="mt-6 grid gap-3 sm:grid-cols-2">
          <Step done icon={<CheckCircle2 className="h-5 w-5" />} title="Checked in" detail="The administrator can see you are here." />
          <Step
            done={data.attendanceMarked}
            icon={data.attendanceMarked ? <UserCheck className="h-5 w-5" /> : <LoaderCircle className="h-5 w-5 animate-spin" />}
            title={data.attendanceMarked ? 'Attendance taken' : 'Waiting for attendance'}
            detail={data.attendanceMarked ? 'Waiting for the test to start.' : 'The administrator is taking attendance.'}
          />
        </div>
        {error && <p role="status" className="mt-4 text-sm text-warning">Connection problem: {error} Retrying…</p>}
      </section>
    </div>
  )
}

function Step({ done, icon, title, detail }: { done: boolean; icon: React.ReactNode; title: string; detail: string }) {
  return (
    <div className={`flex items-start gap-3 rounded-lg border p-4 ${done ? 'border-emerald-200 bg-emerald-50' : 'border-border bg-app'}`}>
      <div className={done ? 'text-success' : 'text-primary'}>{icon}</div>
      <div>
        <p className="font-semibold text-strong">{title}</p>
        <p className="text-sm text-secondary">{detail}</p>
      </div>
    </div>
  )
}
