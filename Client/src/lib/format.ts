import type { AssessmentLifecycle } from '../types/domain'

export function formatDateTime(value?: string) {
  if (!value) return 'Not scheduled'
  return new Intl.DateTimeFormat('en-IN', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

export function lifecycleTone(status: AssessmentLifecycle): 'neutral' | 'info' | 'success' | 'warning' | 'danger' {
  if (status === 'Completed') return 'success'
  if (status === 'Active' || status === 'Ready') return 'info'
  if (status === 'Awaiting approval' || status === 'Booked') return 'warning'
  if (status === 'Revoked' || status === 'Expired') return 'danger'
  return 'neutral'
}

export function minutesLabel(value: number) { return value >= 60 ? `${Math.floor(value / 60)}h ${value % 60 ? `${value % 60}m` : ''}`.trim() : `${value}m` }
