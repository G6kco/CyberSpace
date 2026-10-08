import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { apiCalls, mockApi } from '../../test/api'
import { renderApp, setSession } from '../../test/renderApp'

const student = (patch: Record<string, unknown> = {}) => ({
  id: '01STUDENT', name: 'Nila Joseph', email: 'nila@bitsathy.ac.in', registerNumber: '7376241CS101', status: 'active',
  level: 'Level 1', online: true, lastSeen: new Date().toISOString(), attendanceMarked: false, activeAttemptId: null,
  eligibility: 'eligible', ...patch,
})

const liveAttempt = {
  id: 7, level: 'Level 1', title: 'Level 1 Practical', status: 'in_progress', result: 'pending', score: 50, maxScore: 100, passScore: 80,
  startedAt: new Date().toISOString(), deadlineAt: new Date(Date.now() + 3000000).toISOString(), endedAt: null,
  student: { id: '01STUDENT', name: 'Nila Joseph', email: 'nila@bitsathy.ac.in', registerNumber: '7376241CS101', status: 'active' },
  lab: { status: 'running', ip: '192.168.20.182' },
  questions: [{ id: 11, code: 'wireshark-02', pool: 'wireshark', difficulty: 'hard', prompt: '', points: 50, score: 50, hasPcap: false, flags: [] }],
}

describe('administrator monitoring', () => {
  beforeEach(() => setSession('admin'))

  it('takes attendance for selected students in the portal', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/admin/lobby', { students: [student()] })
    mockApi('POST', '/api/v1/admin/lobby/attendance', { results: [{ studentId: '01STUDENT', ok: true }] })
    renderApp('/admin/monitoring')

    await user.click(await screen.findByRole('checkbox', { name: 'Select Nila Joseph' }))
    await user.click(screen.getByRole('button', { name: /Mark present \(1\)/ }))

    expect(await screen.findByText('Attendance: 1 of 1 succeeded.')).toBeInTheDocument()
    expect(apiCalls('POST', '/api/v1/admin/lobby/attendance')[0].body).toEqual({ studentIds: ['01STUDENT'] })
  })

  it('requires confirmation before starting the test', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/admin/lobby', { students: [student({ attendanceMarked: true })] })
    mockApi('POST', '/api/v1/admin/lobby/start', { results: [{ studentId: '01STUDENT', ok: true, attemptId: 7 }] })
    renderApp('/admin/monitoring')

    await user.click(await screen.findByRole('checkbox', { name: 'Select Nila Joseph' }))
    expect(screen.getByRole('button', { name: /Mark present \(0\)/ })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: /Start test \(1\)/ }))
    const dialog = screen.getByRole('alertdialog', { name: 'Start the test?' })
    await user.click(within(dialog).getByRole('button', { name: 'Start test' }))

    expect(await screen.findByText('Start: 1 of 1 succeeded.')).toBeInTheDocument()
    expect(apiCalls('POST', '/api/v1/admin/lobby/start')[0].body).toEqual({ studentIds: ['01STUDENT'] })
  })

  it('reports students the server refused', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/admin/lobby', { students: [student({ attendanceMarked: true })] })
    mockApi('POST', '/api/v1/admin/lobby/start', { results: [{ studentId: '01STUDENT', ok: false, error: 'Attendance has not been taken for this student.' }] })
    renderApp('/admin/monitoring')

    await user.click(await screen.findByRole('checkbox', { name: 'Select Nila Joseph' }))
    await user.click(screen.getByRole('button', { name: /Start test \(1\)/ }))
    await user.click(within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Start test' }))

    expect(await screen.findByText(/Nila Joseph: Attendance has not been taken/)).toBeInTheDocument()
  })

  it('requires a reason to revoke a live attempt', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/admin/lobby', { students: [] })
    mockApi('GET', '/api/v1/admin/attempts?scope=active', { attempts: [liveAttempt] })
    mockApi('POST', '/api/v1/admin/attempts/7/revoke', { ...liveAttempt, status: 'revoked' })
    renderApp('/admin/monitoring')

    await user.click(await screen.findByRole('tab', { name: /Live attempts/ }))
    expect(await screen.findByText('192.168.20.182')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Open actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Revoke attempt' }))

    const dialog = screen.getByRole('alertdialog', { name: 'Revoke this attempt?' })
    const confirm = within(dialog).getByRole('button', { name: 'Revoke attempt' })
    expect(confirm).toBeDisabled()
    await user.type(within(dialog).getByRole('textbox'), 'Used a phone')
    await user.click(confirm)

    expect(await screen.findByText('Attempt revoked.')).toBeInTheDocument()
    expect(apiCalls('POST', '/api/v1/admin/attempts/7/revoke')[0].body).toEqual({ reason: 'Used a phone' })
  })
})
