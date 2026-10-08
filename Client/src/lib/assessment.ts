import type { AttemptSummary, LabStatus, StudentStatus } from '../types/assessment'

export type Tone = 'neutral' | 'info' | 'success' | 'warning' | 'danger'

// The client clock may be off; the server sends its time with every attempt,
// and countdowns are computed against that offset.
export function serverOffset(serverTime: string): number {
  return new Date(serverTime).getTime() - Date.now()
}

export function remainingMs(deadlineAt: string | null, offset: number, now: number): number {
  if (!deadlineAt) return 0
  return Math.max(0, new Date(deadlineAt).getTime() - (now + offset))
}

export function formatCountdown(ms: number): string {
  const total = Math.floor(ms / 1000)
  const h = Math.floor(total / 3600)
  const m = Math.floor((total % 3600) / 60)
  const s = total % 60
  return [h, m, s].map((v) => String(v).padStart(2, '0')).join(':')
}

export function attemptOutcome(attempt: Pick<AttemptSummary, 'status' | 'result'>): { label: string; tone: Tone } {
  if (attempt.status === 'in_progress' || attempt.status === 'starting') return { label: 'In progress', tone: 'info' }
  if (attempt.status === 'revoked') return { label: 'Revoked', tone: 'danger' }
  if (attempt.status === 'evaluated') {
    return attempt.result === 'passed' ? { label: 'Passed', tone: 'success' } : { label: 'Not passed', tone: 'danger' }
  }
  return { label: attempt.status, tone: 'neutral' }
}

export function labLabel(status: LabStatus | undefined): { label: string; tone: Tone } {
  switch (status) {
    case 'running': return { label: 'Running', tone: 'success' }
    case 'queued':
    case 'provisioning': return { label: 'Starting', tone: 'info' }
    case 'stopping':
    case 'stopped': return { label: 'Stopped', tone: 'neutral' }
    case 'failed': return { label: 'Failed', tone: 'danger' }
    default: return { label: 'Not started', tone: 'neutral' }
  }
}

export function eligibilityLabel(status: StudentStatus): { label: string; tone: Tone } {
  switch (status) {
    case 'eligible': return { label: 'Eligible', tone: 'success' }
    case 'cooldown': return { label: 'Cooldown', tone: 'warning' }
    case 'in_progress': return { label: 'In progress', tone: 'info' }
    case 'completed': return { label: 'All levels passed', tone: 'success' }
    default: return { label: 'No assessment', tone: 'neutral' }
  }
}

export const poolName = { wireshark: 'Wireshark', nmap: 'Nmap' } as const
