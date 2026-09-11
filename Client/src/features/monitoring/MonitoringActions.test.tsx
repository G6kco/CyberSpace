import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { renderApp, setSession } from '../../test/renderApp'

describe('administrator monitoring actions', () => {
  beforeEach(() => setSession('admin'))

  it('requires confirmation before approving a waiting assessment', async () => {
    const user = userEvent.setup()
    renderApp('/admin/monitoring')
    const students = await screen.findAllByText('Nila Joseph')
    const row = students.map((student) => student.closest('tr')).find(Boolean)
    expect(row).not.toBeNull()
    await user.click(within(row as HTMLElement).getByRole('button', { name: 'Open actions' }))
    await user.click(screen.getByRole('menuitem', { name: 'Approve and start' }))
    expect(screen.getByRole('alertdialog', { name: 'Approve and start assessment?' })).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Approve and start' }))
    expect(await screen.findByText('Assessment approved and started.')).toBeInTheDocument()
  })
})
