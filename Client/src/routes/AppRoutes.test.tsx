import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { renderApp, setSession } from '../test/renderApp'

describe('authentication and role routing', () => {
  beforeEach(() => setSession(null))

  it('redirects a signed-out user to login', () => {
    renderApp('/student/learning')
    expect(screen.getByRole('heading', { name: 'Welcome back' })).toBeInTheDocument()
  })

  it('restores a persisted student session', async () => {
    setSession('student')
    renderApp('/student/learning')
    expect(await screen.findByRole('heading', { name: /Build skills before your next assessment/i })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'student navigation' })).toBeInTheDocument()
  })

  it('blocks students from administrator routes', () => {
    setSession('student')
    renderApp('/admin/monitoring')
    expect(screen.getByRole('heading', { name: 'Access denied' })).toBeInTheDocument()
  })

  it('enters the selected administrator demo role', async () => {
    const user = userEvent.setup()
    renderApp('/login')
    await user.click(screen.getByRole('button', { name: 'Administrator' }))
    await user.click(screen.getByRole('button', { name: 'Sign in as administrator' }))
    expect(await screen.findByRole('heading', { name: /Student assessment monitoring/i })).toBeInTheDocument()
  })
})
