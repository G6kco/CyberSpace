import { Plus, Trash2 } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Button } from '../../components/ui/Button'
import { BusyLabel } from '../../components/ui/Feedback'
import type { AnswerInput, LevelInput, LevelQuestion, QuestionInput } from '../../types/levels'

export const emptyLevel: LevelInput = {
  name: '',
  title: '',
  instructions: '',
  durationMinutes: 60,
  questionsPerAttempt: 3,
  passPercent: 80,
}

// LevelSettingsForm edits what a level is: its name, timing, how many
// questions each student is dealt, and the pass mark.
export function LevelSettingsForm({
  initial,
  disabled,
  submitLabel,
  onSubmit,
}: {
  initial: LevelInput
  disabled?: boolean
  submitLabel: string
  onSubmit: (input: LevelInput) => Promise<void>
}) {
  const [value, setValue] = useState(initial)
  const [busy, setBusy] = useState(false)
  const set = <K extends keyof LevelInput>(key: K, v: LevelInput[K]) => setValue((current) => ({ ...current, [key]: v }))

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setBusy(true)
    try {
      await onSubmit(value)
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className="space-y-4">
      <fieldset disabled={disabled || busy} className="grid gap-4 sm:grid-cols-2">
        <Field label="Level name" hint="Shown to students, e.g. Level 2">
          <input className="input" required maxLength={120} value={value.name} onChange={(e) => set('name', e.target.value)} />
        </Field>
        <Field label="Assessment title" hint="Optional; defaults to the name">
          <input className="input" maxLength={200} value={value.title} onChange={(e) => set('title', e.target.value)} />
        </Field>
        <Field label="Duration (minutes)">
          <input className="input" type="number" required min={1} max={600} value={value.durationMinutes}
            onChange={(e) => set('durationMinutes', Number(e.target.value))} />
        </Field>
        <Field label="Questions per student" hint="Each student is dealt this many random questions from the bank">
          <input className="input" type="number" required min={1} max={50} value={value.questionsPerAttempt}
            onChange={(e) => set('questionsPerAttempt', Number(e.target.value))} />
        </Field>
        <Field label="Pass mark (%)" hint="Of the marks of the questions a student is dealt">
          <input className="input" type="number" required min={0.01} max={100} step={0.01} value={value.passPercent}
            onChange={(e) => set('passPercent', Number(e.target.value))} />
        </Field>
        <div className="sm:col-span-2">
          <Field label="Instructions" hint="Shown to students before and during the test">
            <textarea className="input min-h-28 resize-y" maxLength={20000} value={value.instructions}
              onChange={(e) => set('instructions', e.target.value)} />
          </Field>
        </div>
      </fieldset>
      {!disabled && (
        <div className="flex justify-end">
          <Button type="submit" disabled={busy}>{busy ? <BusyLabel>Saving</BusyLabel> : submitLabel}</Button>
        </div>
      )}
    </form>
  )
}

interface AnswerRow extends AnswerInput {
  key: number
}

let rowKey = 0

// QuestionForm writes one scenario: its prompt, an optional lab image, and
// its answers with their marks. Expected answers are write-only: an existing
// answer left blank keeps the stored one.
export function QuestionForm({
  initial,
  onSubmit,
  onCancel,
}: {
  initial?: LevelQuestion
  onSubmit: (input: QuestionInput) => Promise<void>
  onCancel: () => void
}) {
  const [prompt, setPrompt] = useState(initial?.prompt ?? '')
  const [image, setImage] = useState(initial?.image ?? '')
  const [answers, setAnswers] = useState<AnswerRow[]>(
    initial?.answers.length
      ? initial.answers.map((a) => ({ key: ++rowKey, id: a.id, prompt: a.prompt, answer: '', marks: a.marks }))
      : [{ key: ++rowKey, prompt: 'Flag', answer: '', marks: 10 }],
  )
  const [busy, setBusy] = useState(false)
  const total = answers.reduce((sum, a) => sum + (Number.isFinite(a.marks) ? a.marks : 0), 0)

  const update = (key: number, patch: Partial<AnswerRow>) =>
    setAnswers((rows) => rows.map((row) => (row.key === key ? { ...row, ...patch } : row)))

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setBusy(true)
    try {
      await onSubmit({
        prompt,
        image,
        answers: answers.map(({ id, prompt: p, answer, marks }) => ({ id, prompt: p, answer, marks })),
      })
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={submit} className="space-y-5">
      <fieldset disabled={busy} className="space-y-5">
        <Field label="Question" hint="The scenario as students read it">
          <textarea className="input min-h-32 resize-y" required maxLength={20000} value={prompt} onChange={(e) => setPrompt(e.target.value)} />
        </Field>
        <Field label="Docker image" hint="Each student dealt this question gets their own container on the lab network. Leave empty for no lab.">
          <input className="input font-mono" maxLength={160} value={image} placeholder="e.g. scenario2-web:latest"
            onChange={(e) => setImage(e.target.value)} />
        </Field>

        <div>
          <div className="flex items-baseline justify-between">
            <h3 className="text-sm font-semibold text-strong">Answers</h3>
            <span className="text-sm text-secondary">Question worth {Math.round(total * 100) / 100} marks</span>
          </div>
          <p className="mt-1 text-xs text-secondary">
            Answers are compared ignoring case and surrounding spaces. They are stored as keyed hashes and cannot be shown again.
          </p>
          <ol className="mt-3 space-y-3">
            {answers.map((row, index) => (
              <li key={row.key} className="rounded-lg border border-border p-3">
                <div className="grid gap-3 sm:grid-cols-[1fr_96px]">
                  <label className="text-xs font-semibold text-secondary">
                    Answer {index + 1} prompt
                    <input className="input mt-1" required maxLength={1000} value={row.prompt}
                      onChange={(e) => update(row.key, { prompt: e.target.value })} />
                  </label>
                  <label className="text-xs font-semibold text-secondary">
                    Marks
                    <input className="input mt-1" type="number" required min={0.01} max={1000} step={0.01} value={row.marks}
                      onChange={(e) => update(row.key, { marks: Number(e.target.value) })} />
                  </label>
                  <div className="flex items-end gap-2 sm:col-span-2">
                    <label className="flex-1 text-xs font-semibold text-secondary">
                      Expected answer
                      <input className="input mt-1 font-mono" maxLength={256} autoComplete="off" spellCheck={false}
                        required={!row.id} value={row.answer}
                        placeholder={row.id ? 'Leave blank to keep the stored answer' : 'e.g. FLAG{...}'}
                        onChange={(e) => update(row.key, { answer: e.target.value })} />
                    </label>
                    <Button variant="ghost" type="button" disabled={answers.length === 1}
                      aria-label={`Remove answer ${index + 1}`}
                      onClick={() => setAnswers((rows) => rows.filter((r) => r.key !== row.key))}>
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </li>
            ))}
          </ol>
          <Button variant="secondary" type="button" className="mt-3" disabled={answers.length >= 20}
            onClick={() => setAnswers((rows) => [...rows, { key: ++rowKey, prompt: '', answer: '', marks: 10 }])}>
            <Plus className="h-4 w-4" />
            Add answer
          </Button>
        </div>
      </fieldset>

      <div className="flex justify-end gap-2 border-t border-border pt-4">
        <Button variant="secondary" type="button" disabled={busy} onClick={onCancel}>Cancel</Button>
        <Button type="submit" disabled={busy}>{busy ? <BusyLabel>Saving</BusyLabel> : 'Save question'}</Button>
      </div>
    </form>
  )
}

function Field({ label, hint, children }: { label: string; hint?: string; children: React.ReactNode }) {
  return (
    <label className="block text-sm font-medium text-strong">
      {label}
      {hint && <span className="mt-0.5 block text-xs font-normal text-secondary">{hint}</span>}
      <div className="mt-1.5">{children}</div>
    </label>
  )
}
