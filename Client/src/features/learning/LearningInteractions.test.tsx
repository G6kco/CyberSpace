import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { renderApp, setSession } from '../../test/renderApp'

describe('learning progress', () => {
  beforeEach(() => setSession('student'))

  it('marks a lesson complete through the repository', async () => {
    const user = userEvent.setup()
    renderApp('/student/learning/net-foundations')
    const buttons = await screen.findAllByRole('button', { name: 'Mark complete' })
    await user.click(buttons[0])
    expect(await screen.findByText('Lesson marked complete.')).toBeInTheDocument()
  })
})
