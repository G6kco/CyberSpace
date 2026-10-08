import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { apiCalls, mockApi } from '../../test/api'
import { renderApp, setSession } from '../../test/renderApp'

const level = { level: 'Level 1', levelNumber: 1, title: 'Level 1 Practical', durationSeconds: 3600, passScore: 80, totalPoints: 100, instructions: '' }

function overview(patch: Record<string, unknown> = {}) {
  return {
    status: 'eligible', assessment: level, cooldownUntil: null, attendanceMarked: false, inLobby: false,
    activeAttemptId: null, history: [], serverTime: new Date().toISOString(), ...patch,
  }
}

function attempt(patch: Record<string, unknown> = {}) {
  const now = Date.now()
  return {
    id: 7, level: 'Level 1', title: 'Level 1 Practical', status: 'in_progress', result: 'pending',
    score: 0, maxScore: 100, passScore: 80,
    startedAt: new Date(now - 60000).toISOString(), deadlineAt: new Date(now + 3540000).toISOString(), endedAt: null,
    serverTime: new Date(now).toISOString(),
    lab: { status: 'running', ip: '192.168.20.182' },
    questions: [
      {
        id: 11, code: 'wireshark-02', pool: 'wireshark', difficulty: 'hard', prompt: 'Find the true message.', points: 50, score: 0, hasPcap: true,
        flags: [{ id: 111, prompt: 'Submit the flag.', points: 50, solved: false, submissions: 0 }],
      },
      {
        id: 21, code: 'nmap-01', pool: 'nmap', difficulty: 'easy', prompt: 'Scan the target.', points: 50, score: 0, hasPcap: false,
        flags: [{ id: 211, prompt: 'Submit the flag.', points: 50, solved: false, submissions: 0 }],
      },
    ],
    ...patch,
  }
}

describe('student assessment flow', () => {
  beforeEach(() => setSession('student'))

  it('lets an eligible student enter the test portal', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/student/assessment', overview())
    mockApi('POST', '/api/v1/student/lobby/heartbeat', overview({ inLobby: true }))
    renderApp('/student/assessments')

    await user.click(await screen.findByRole('button', { name: /Enter test portal/ }))

    expect(await screen.findByText('You are in the test portal')).toBeInTheDocument()
    expect(screen.getByText('Waiting for attendance')).toBeInTheDocument()
    expect(apiCalls('POST', '/api/v1/student/lobby/heartbeat')).not.toHaveLength(0)
  })

  it('moves a student from the portal into the attempt once started', async () => {
    mockApi('POST', '/api/v1/student/lobby/heartbeat', overview({ status: 'in_progress', activeAttemptId: 7, attendanceMarked: true }))
    mockApi('GET', '/api/v1/student/attempts/7', attempt())
    renderApp('/student/lobby')

    expect(await screen.findByRole('region', { name: 'Wireshark question' })).toBeInTheDocument()
    expect(screen.getByText('192.168.20.182')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Download capture file/ })).toHaveAttribute(
      'href', 'http://localhost:8080/api/v1/student/attempts/7/questions/11/pcap',
    )
  })

  it('submits a flag and reports the points', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/student/attempts/7', attempt())
    mockApi('POST', '/api/v1/student/attempts/7/answers', { correct: true, points: 50, score: 50 })
    renderApp('/student/attempts/7')

    const nmap = await screen.findByRole('region', { name: 'Nmap question' })
    await user.type(within(nmap).getByRole('textbox'), 'f7f4d1')
    await user.click(within(nmap).getByRole('button', { name: /Submit/ }))

    expect(await within(nmap).findByText('Correct: +50 points.')).toBeInTheDocument()
    expect(apiCalls('POST', '/api/v1/student/attempts/7/answers')[0].body).toEqual({ questionId: 211, answer: 'f7f4d1' })
  })

  it('reports an incorrect flag without revealing anything', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/student/attempts/7', attempt())
    mockApi('POST', '/api/v1/student/attempts/7/answers', { correct: false, points: 0, score: 0 })
    renderApp('/student/attempts/7')

    const wireshark = await screen.findByRole('region', { name: 'Wireshark question' })
    await user.type(within(wireshark).getByRole('textbox'), 'guess')
    await user.click(within(wireshark).getByRole('button', { name: /Submit/ }))
    expect(await within(wireshark).findByText('Incorrect. Try again.')).toBeInTheDocument()
  })

  it('asks for confirmation before finishing', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/student/attempts/7', attempt())
    mockApi('POST', '/api/v1/student/attempts/7/finish', attempt({ status: 'evaluated', result: 'failed' }))
    renderApp('/student/attempts/7')

    await user.click(await screen.findByRole('button', { name: 'Finish and submit' }))
    const dialog = screen.getByRole('alertdialog', { name: 'Finish this assessment?' })
    await user.click(within(dialog).getByRole('button', { name: 'Finish and submit' }))
    expect(apiCalls('POST', '/api/v1/student/attempts/7/finish')).toHaveLength(1)
  })

  it('shows only the result of a finished attempt', async () => {
    mockApi('GET', '/api/v1/student/attempts/7', attempt({ status: 'evaluated', result: 'passed', score: 90, questions: null, lab: null, endedAt: new Date().toISOString() }))
    renderApp('/student/attempts/7')

    expect(await screen.findByText('Passed')).toBeInTheDocument()
    expect(screen.getByText('90 / 100')).toBeInTheDocument()
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument()
  })

  it('shows when a student in cooldown can retake', async () => {
    mockApi('GET', '/api/v1/student/assessment', overview({ status: 'cooldown', cooldownUntil: '2026-10-08T10:00:00Z' }))
    renderApp('/student/assessments')

    expect(await screen.findByText('Retake available after')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Enter test portal/ })).not.toBeInTheDocument()
  })
})
