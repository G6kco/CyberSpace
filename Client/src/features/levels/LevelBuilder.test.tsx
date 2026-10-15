import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it } from 'vitest'
import { apiCalls, mockApi } from '../../test/api'
import { renderApp, setSession } from '../../test/renderApp'

const draft = (patch: Record<string, unknown> = {}) => ({
  id: 2, sequence: 2, name: 'Level 2', title: 'Web scenarios', instructions: '', durationMinutes: 60,
  questionsPerAttempt: 2, passPercent: 60, status: 'draft', built: true, questionCount: 0, attempts: 0, questions: [], ...patch,
})

const question = (id: number, marks: number, image = '') => ({
  id, code: `q-0${id}`, prompt: `Scenario ${id}`, image, marks,
  answers: [{ id: id * 10, prompt: 'Flag', marks }], files: [],
})

describe('level builder', () => {
  beforeEach(() => setSession('admin'))

  it('creates a level and opens it', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/admin/levels', { levels: [{ ...draft(), id: 1, sequence: 1, name: 'Level 1', built: false, status: 'published' }] })
    mockApi('POST', '/api/v1/admin/levels', draft())
    mockApi('GET', '/api/v1/admin/levels/2', draft())
    renderApp('/admin/levels')

    expect(await screen.findByText('Imported')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: /New level/ }))
    const form = screen.getByRole('dialog')
    expect(within(form).getByLabelText(/Level name/)).toHaveValue('Level 2')
    await user.clear(within(form).getByLabelText(/Questions per student/))
    await user.type(within(form).getByLabelText(/Questions per student/), '2')
    await user.click(within(form).getByRole('button', { name: 'Create level' }))

    expect(await screen.findByRole('heading', { name: 'Level 2' })).toBeInTheDocument()
    expect(apiCalls('POST', '/api/v1/admin/levels')[0].body).toMatchObject({ name: 'Level 2', questionsPerAttempt: 2, passPercent: 80 })
  })

  it('adds a question with its image and weighted answers', async () => {
    const user = userEvent.setup()
    mockApi('GET', '/api/v1/admin/levels/2', draft())
    mockApi('POST', '/api/v1/admin/levels/2/questions', draft({ questionCount: 1, questions: [question(1, 30, 'scenario2-web')] }))
    renderApp('/admin/levels/2')

    // Publishing waits until the bank holds as many questions as are dealt.
    expect(await screen.findByRole('button', { name: /Publish/ })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: /Add question/ }))
    const form = screen.getByRole('dialog')
    await user.type(within(form).getByLabelText(/^Question/), 'Log in as admin.')
    await user.type(within(form).getByLabelText(/Docker image/), 'scenario2-web')
    await user.type(within(form).getByLabelText(/Expected answer/), 'FLAG{{web}')
    await user.click(within(form).getByRole('button', { name: /Add answer/ }))
    const prompts = within(form).getAllByLabelText(/prompt$/)
    await user.type(prompts[1], 'Database name')
    await user.type(within(form).getAllByLabelText(/Expected answer/)[1], 'shop')
    const marks = within(form).getAllByLabelText('Marks')
    await user.clear(marks[1])
    await user.type(marks[1], '20')
    expect(within(form).getByText('Question worth 30 marks')).toBeInTheDocument()
    await user.click(within(form).getByRole('button', { name: 'Save question' }))

    expect(await screen.findByText('Question added.')).toBeInTheDocument()
    expect(apiCalls('POST', '/api/v1/admin/levels/2/questions')[0].body).toEqual({
      prompt: 'Log in as admin.',
      image: 'scenario2-web',
      answers: [
        { prompt: 'Flag', answer: 'FLAG{web}', marks: 10 },
        { prompt: 'Database name', answer: 'shop', marks: 20 },
      ],
    })
  })

  it('locks a published level and offers unpublishing until it is used', async () => {
    mockApi('GET', '/api/v1/admin/levels/2', draft({ status: 'published', questionCount: 2, questions: [question(1, 30), question(2, 20)] }))
    renderApp('/admin/levels/2')

    expect(await screen.findByRole('button', { name: /Unpublish to edit/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /Add question/ })).not.toBeInTheDocument()
    expect(screen.getByLabelText(/Level name/)).toBeDisabled()
    expect(screen.getByRole('region', { name: 'Readiness' })).toHaveTextContent('50')
  })
})
