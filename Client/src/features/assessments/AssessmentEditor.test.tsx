import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { renderApp, setSession } from '../../test/renderApp'

describe('assessment editor', () => {
  beforeEach(() => setSession('admin'))

  it('runs frontend YAML validation without executing configuration', async () => {
    const user = userEvent.setup()
    renderApp('/admin/assessments/new')
    await user.click(screen.getByRole('button', { name: /Lab environment/ }))
    await user.click(screen.getByRole('button', { name: 'Validate YAML' }))
    expect(screen.getByText(/Basic frontend checks passed/)).toBeInTheDocument()
  })

  it('adds and reorders questions', async () => {
    const user = userEvent.setup()
    renderApp('/admin/assessments/new')
    await user.click(screen.getByRole('button', { name: /Questions/ }))
    await user.click(screen.getByRole('button', { name: 'Add question' }))
    await user.click(screen.getByRole('button', { name: 'Add question' }))
    expect(screen.getByText('Question 2')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Move question 2 up' })).toBeEnabled()
  })
})
