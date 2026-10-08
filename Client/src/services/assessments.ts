import { apiBase, apiRequest } from './api'
import type {
  ActionResult,
  AdminAttempt,
  AttemptSummary,
  AttemptView,
  BankAssessment,
  LobbyEntry,
  StudentOverview,
  StudentRecord,
  SubmitResult,
} from '../types/assessment'

export const studentApi = {
  overview: () => apiRequest<StudentOverview>('/api/v1/student/assessment'),
  heartbeat: () => apiRequest<StudentOverview>('/api/v1/student/lobby/heartbeat', { method: 'POST' }),
  leaveLobby: () => apiRequest<void>('/api/v1/student/lobby', { method: 'DELETE' }),
  attempt: (attemptId: number) => apiRequest<AttemptView>(`/api/v1/student/attempts/${attemptId}`),
  submit: (attemptId: number, questionId: number, answer: string) =>
    apiRequest<SubmitResult>(`/api/v1/student/attempts/${attemptId}/answers`, { method: 'POST', body: { questionId, answer } }),
  finish: (attemptId: number) => apiRequest<AttemptSummary>(`/api/v1/student/attempts/${attemptId}/finish`, { method: 'POST' }),
  // A plain link: the browser downloads the file with the session cookie.
  pcapUrl: (attemptId: number, questionId: number) =>
    `${apiBase}/api/v1/student/attempts/${attemptId}/questions/${questionId}/pcap`,
}

export const adminApi = {
  lobby: () => apiRequest<{ students: LobbyEntry[] }>('/api/v1/admin/lobby').then((r) => r.students),
  markAttendance: (studentIds: string[]) =>
    apiRequest<{ results: ActionResult[] }>('/api/v1/admin/lobby/attendance', { method: 'POST', body: { studentIds } }).then((r) => r.results),
  cancelAttendance: (studentId: string) =>
    apiRequest<ActionResult>(`/api/v1/admin/lobby/attendance/${encodeURIComponent(studentId)}`, { method: 'DELETE' }),
  start: (studentIds: string[]) =>
    apiRequest<{ results: ActionResult[] }>('/api/v1/admin/lobby/start', { method: 'POST', body: { studentIds } }).then((r) => r.results),
  attempts: (scope: 'active' | 'recent') =>
    apiRequest<{ attempts: AdminAttempt[] }>(`/api/v1/admin/attempts?scope=${scope}`).then((r) => r.attempts),
  attempt: (attemptId: number) => apiRequest<AttemptView>(`/api/v1/admin/attempts/${attemptId}`),
  end: (attemptId: number) => apiRequest<AttemptSummary>(`/api/v1/admin/attempts/${attemptId}/end`, { method: 'POST' }),
  revoke: (attemptId: number, reason: string) =>
    apiRequest<AttemptSummary>(`/api/v1/admin/attempts/${attemptId}/revoke`, { method: 'POST', body: { reason } }),
  restartLab: (attemptId: number) => apiRequest<void>(`/api/v1/admin/attempts/${attemptId}/lab/restart`, { method: 'POST' }),
  questionBank: () => apiRequest<{ assessments: BankAssessment[] }>('/api/v1/admin/question-bank').then((r) => r.assessments),
  students: (search: string) =>
    apiRequest<{ students: StudentRecord[] }>(`/api/v1/admin/students?search=${encodeURIComponent(search)}`).then((r) => r.students),
}
