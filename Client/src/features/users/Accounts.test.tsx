import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { apiCalls, mockApi, reply } from '../../test/api'
import { renderApp, setSession } from '../../test/renderApp'

const admin = (id: number, name: string, email: string) => ({
  id, publicId: `01ADMIN${id}`, name, email, status: 'active', lastLoginAt: null, createdAt: '2026-10-22T08:00:00Z',
})

describe('accounts', () => {
  beforeEach(() => setSession('admin'))

  it('imports a student list and shows the rows it skipped', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/admin/students?search=', { students: [] })
    mockApi('POST', '/api/v1/admin/students/import', {
      created: 7438, updated: 0, unchanged: 0,
      skipped: [{ sheet: 'I Name List', row: 1392, registerNumber: '7376252IT315', email: 'rupeshj.it25@bitsathy.ac.in',
        reason: 'Conflicts with I Name List row 1403 (same email or register number, different details)' }],
    })
    renderApp('/admin/students')

    expect(await screen.findByText('No students found')).toBeInTheDocument()
    const file = new File(['xlsx'], 'students.xlsx', { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
    await user.upload(screen.getByLabelText('Student list workbook'), file)

    const result = await screen.findByRole('status', { name: 'Import result' })
    expect(within(result).getByText('7438')).toBeInTheDocument()
    expect(within(result).getByText(/Conflicts with I Name List row 1403/)).toBeInTheDocument()
    expect(apiCalls('POST', '/api/v1/admin/students/import')).toHaveLength(1)
    // The list reloads after the import (the search is debounced).
    await waitFor(() => expect(apiCalls('GET', '/api/v1/admin/students?search=').length).toBeGreaterThan(1))
  })

  it('adds an administrator and reports a refusal', async () => {
    const user = userEvent.setup()
    let admins = [admin(1, 'Dr Staff', 'staff@bitsathy.ac.in')]
    mockApi('GET', '/api/v1/admin/admins', () => ({ admins }))
    mockApi('POST', '/api/v1/admin/admins', (body: unknown) => {
      const input = body as { name: string; email: string }
      if (input.email === 'taken@bitsathy.ac.in') return reply(409, { code: 'conflict', message: 'an account with this email already exists' })
      admins = [...admins, admin(2, input.name, input.email)]
      return admins[1]
    })
    renderApp('/admin/administrators')

    expect(await screen.findByText('Dr Staff')).toBeInTheDocument()
    await user.type(screen.getByLabelText('Name'), 'Prof New')
    await user.type(screen.getByLabelText(/Google Workspace email/), 'new@bitsathy.ac.in')
    await user.click(screen.getByRole('button', { name: /Add administrator/ }))

    expect(await screen.findByText('Prof New can now sign in as an administrator.')).toBeInTheDocument()
    expect(await screen.findByText('new@bitsathy.ac.in')).toBeInTheDocument()
    expect(apiCalls('POST', '/api/v1/admin/admins')[0].body).toEqual({ name: 'Prof New', email: 'new@bitsathy.ac.in' })
    expect(screen.getByLabelText('Name')).toHaveValue('')

    await user.type(screen.getByLabelText('Name'), 'Again')
    await user.type(screen.getByLabelText(/Google Workspace email/), 'taken@bitsathy.ac.in')
    await user.click(screen.getByRole('button', { name: /Add administrator/ }))
    expect(await screen.findByText('an account with this email already exists')).toBeInTheDocument()
  })
})
