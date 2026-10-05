import { screen } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'
import { renderApp, setSession } from '../test/renderApp'

describe('authentication and role routing', () => {
  beforeEach(() => setSession(null))

  it('redirects a signed-out user to login', async () => {
    renderApp('/student/learning')
    expect(await screen.findByRole('heading', { name: 'Welcome back' })).toBeInTheDocument()
  })

  it('restores a session reported by the server', async () => {
    setSession('student')
    renderApp('/student/learning')
    expect(await screen.findByRole('heading', { name: /Build skills before your next assessment/i })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'student navigation' })).toBeInTheDocument()
  })

  it('derives display initials from the name the server sends', async () => {
    setSession('student')
    renderApp('/student/learning')
    expect(await screen.findAllByText('MI')).not.toHaveLength(0)
  })

  it('blocks students from administrator routes', async () => {
    setSession('student')
    renderApp('/admin/monitoring')
    expect(await screen.findByRole('heading', { name: 'Access denied' })).toBeInTheDocument()
  })

  it('offers Google sign-in instead of a password form', async () => {
    renderApp('/login')
    expect(await screen.findByRole('button', { name: /Continue with Google/i })).toBeEnabled()
    expect(screen.queryByLabelText(/password/i)).not.toBeInTheDocument()
  })
})
