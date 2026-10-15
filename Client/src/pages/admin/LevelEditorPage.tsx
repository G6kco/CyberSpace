import { AlertTriangle, ArrowLeft, Box, Download, FileUp, Lock, Pencil, Plus, Rocket, Trash2, Undo2 } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Button } from '../../components/ui/Button'
import { ConfirmDialog } from '../../components/ui/ConfirmDialog'
import { Drawer } from '../../components/ui/Drawer'
import { BusyLabel, EmptyState, ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { useToast } from '../../components/ui/Toast'
import { LevelSettingsForm, QuestionForm } from '../../features/levels/LevelForms'
import { formatBytes } from '../../lib/assessment'
import { errorMessage } from '../../services/api'
import { levelsApi } from '../../services/levels'
import type { Level, LevelQuestion } from '../../types/levels'

type Pending = { kind: 'publish' | 'unpublish' | 'delete' } | { kind: 'deleteQuestion'; question: LevelQuestion }
type Editing = { question?: LevelQuestion } | null

export function LevelEditorPage() {
  const levelId = Number(useParams().levelId)
  const [level, setLevel] = useState<Level | null>(null)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState<Editing>(null)
  const [pending, setPending] = useState<Pending | null>(null)
  const navigate = useNavigate()
  const { notify } = useToast()
  const closeEditor = useCallback(() => setEditing(null), [])
  const cancelPending = useCallback(() => setPending(null), [])

  const load = useCallback(() => {
    setError('')
    levelsApi.get(levelId).then(setLevel).catch((err) => setError(errorMessage(err, 'The level could not be loaded.')))
  }, [levelId])
  useEffect(load, [load])

  // run performs a change; every change answers with the level as it now
  // stands.
  const run = async (action: () => Promise<Level | void>, done: string) => {
    try {
      const next = await action()
      if (next) setLevel(next)
      notify(done)
      return true
    } catch (err) {
      notify(errorMessage(err), 'error')
      return false
    }
  }

  if (!Number.isInteger(levelId) || levelId <= 0) return <ErrorState message="This level does not exist." />
  if (!level && !error) return <LoadingSkeleton rows={6} />
  if (!level) return <ErrorState message={error} onRetry={load} />

  const questions = level.questions ?? []
  const editable = level.built && level.status === 'draft'
  const marks = questions.map((q) => q.marks).sort((a, b) => b - a)
  const dealt = level.questionsPerAttempt
  const highest = marks.slice(0, dealt).reduce((a, b) => a + b, 0)
  const lowest = marks.slice(-dealt).reduce((a, b) => a + b, 0)
  const ready = questions.length >= dealt && questions.every((q) => q.answers.length > 0)
  const missingFiles = questions.flatMap((q) => q.files.filter((f) => !f.present))

  return (
    <div className="space-y-6">
      <Link to="/admin/levels" className="inline-flex items-center gap-2 text-sm font-semibold text-secondary hover:text-primary">
        <ArrowLeft className="h-4 w-4" />
        Levels
      </Link>

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge tone="info">Level {level.sequence}</StatusBadge>
            <StatusBadge tone={level.status === 'published' ? 'success' : 'neutral'}>{level.status === 'published' ? 'Published' : 'Draft'}</StatusBadge>
          </div>
          <h2 className="page-heading mt-2">{level.name}</h2>
          <p className="mt-1 text-secondary">{level.title}</p>
        </div>
        {level.built && (
          <div className="flex flex-wrap gap-2">
            {level.status === 'draft' && (
              <>
                <Button variant="ghost" onClick={() => setPending({ kind: 'delete' })}><Trash2 className="h-4 w-4" />Delete</Button>
                <Button disabled={!ready} onClick={() => setPending({ kind: 'publish' })}><Rocket className="h-4 w-4" />Publish</Button>
              </>
            )}
            {level.status === 'published' && level.attempts === 0 && (
              <Button variant="secondary" onClick={() => setPending({ kind: 'unpublish' })}><Undo2 className="h-4 w-4" />Unpublish to edit</Button>
            )}
          </div>
        )}
      </div>

      {!level.built && (
        <Notice icon={<Lock className="h-5 w-5" />}>This level was imported on the server and deals one Wireshark and one Nmap question. It is shown here but cannot be edited.</Notice>
      )}
      {level.built && level.status === 'published' && (
        <Notice icon={<Lock className="h-5 w-5" />}>
          {level.attempts === 0
            ? 'Published levels are locked so every student sits the same bank. Unpublish it to make changes; this also withdraws any attendance taken for it.'
            : `${level.attempts} attempt(s) have been made, so this level is fixed.`}
        </Notice>
      )}
      {missingFiles.length > 0 && (
        <Notice icon={<AlertTriangle className="h-5 w-5" />} tone="warning">
          {missingFiles.length} file(s) are missing from the server's files directory: {missingFiles.map((f) => f.name).join(', ')}.
        </Notice>
      )}

      <section className="surface p-4 sm:p-5">
        <h3 className="font-semibold text-strong">Settings</h3>
        <div className="mt-4">
          <LevelSettingsForm
            key={`${level.id}-${level.status}`}
            initial={{
              name: level.name,
              title: level.title,
              instructions: level.instructions,
              durationMinutes: level.durationMinutes,
              questionsPerAttempt: level.questionsPerAttempt,
              passPercent: level.passPercent,
            }}
            disabled={!editable}
            submitLabel="Save settings"
            onSubmit={async (input) => { await run(() => levelsApi.update(level.id, input), 'Settings saved.') }}
          />
        </div>
      </section>

      {level.built && (
        <section className="surface grid gap-4 p-4 sm:grid-cols-4 sm:p-5" aria-label="Readiness">
          <Summary label="Questions in bank" value={String(questions.length)} detail={`${dealt} dealt to each student`} warn={questions.length < dealt} />
          <Summary label="Marks per student" value={questions.length >= dealt ? (lowest === highest ? `${highest}` : `${lowest} – ${highest}`) : '—'} detail="depends on the questions dealt" />
          <Summary label="Pass mark" value={`${level.passPercent}%`} detail={questions.length >= dealt ? `${Math.round(lowest * level.passPercent) / 100} – ${Math.round(highest * level.passPercent) / 100} marks` : 'of the marks dealt'} />
          <Summary label="Labs per student" value={`up to ${Math.min(dealt, questions.filter((q) => q.image).length)}`} detail="one container and IP each" />
        </section>
      )}

      <section className="space-y-3">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-strong">Questions</h3>
          {editable && (
            <Button onClick={() => setEditing({})}><Plus className="h-4 w-4" />Add question</Button>
          )}
        </div>
        {questions.length === 0 ? (
          <EmptyState title="No questions yet" description={`Add at least ${dealt} question(s); each student is dealt ${dealt} at random.`} />
        ) : (
          questions.map((question) => (
            <QuestionCard
              key={question.id}
              level={level}
              question={question}
              editable={editable}
              onEdit={() => setEditing({ question })}
              onDelete={() => setPending({ kind: 'deleteQuestion', question })}
              onUpload={(file) => run(() => levelsApi.uploadFile(level.id, question.id, file), `${file.name} uploaded.`)}
              onRemoveFile={(fileId) => run(() => levelsApi.removeFile(level.id, question.id, fileId), 'File removed.')}
            />
          ))
        )}
      </section>

      <Drawer
        open={editing !== null}
        title={editing?.question ? `Edit ${editing.question.code}` : 'New question'}
        description="Each question is a scenario with its own optional lab."
        onClose={closeEditor}
        width="max-w-2xl"
      >
        {editing && (
          <QuestionForm
            initial={editing.question}
            onCancel={closeEditor}
            onSubmit={async (input) => {
              const id = editing.question?.id
              const saved = await run(
                () => (id ? levelsApi.updateQuestion(level.id, id, input) : levelsApi.createQuestion(level.id, input)),
                id ? 'Question saved.' : 'Question added.',
              )
              if (saved) setEditing(null)
            }}
          />
        )}
      </Drawer>

      <ConfirmDialog
        open={pending !== null}
        title={
          pending?.kind === 'publish' ? 'Publish this level?'
            : pending?.kind === 'unpublish' ? 'Unpublish this level?'
              : pending?.kind === 'delete' ? 'Delete this level?'
                : 'Delete this question?'
        }
        description={
          pending?.kind === 'publish'
            ? `Students who have passed the levels before it can take it. It is locked while published; each student is dealt ${dealt} of its ${questions.length} questions.`
            : pending?.kind === 'unpublish'
              ? 'Students can no longer take it, and attendance taken for it is withdrawn. You can then edit and publish it again.'
              : pending?.kind === 'delete'
                ? 'The level, its questions, answers and files are removed. This cannot be undone.'
                : pending?.kind === 'deleteQuestion'
                  ? `${pending.question.code}, its answers and its files are removed.`
                  : ''
        }
        confirmLabel={pending?.kind === 'publish' ? 'Publish' : pending?.kind === 'unpublish' ? 'Unpublish' : 'Delete'}
        tone={pending?.kind === 'publish' || pending?.kind === 'unpublish' ? 'primary' : 'danger'}
        onCancel={cancelPending}
        onConfirm={async () => {
          if (!pending) return
          if (pending.kind === 'publish') await run(() => levelsApi.publish(level.id), 'Level published.')
          if (pending.kind === 'unpublish') await run(() => levelsApi.unpublish(level.id), 'Level unpublished.')
          if (pending.kind === 'deleteQuestion') await run(() => levelsApi.removeQuestion(level.id, pending.question.id), 'Question deleted.')
          if (pending.kind === 'delete' && (await run(() => levelsApi.remove(level.id), 'Level deleted.'))) navigate('/admin/levels')
          setPending(null)
        }}
      />
    </div>
  )
}

function QuestionCard({
  level,
  question,
  editable,
  onEdit,
  onDelete,
  onUpload,
  onRemoveFile,
}: {
  level: Level
  question: LevelQuestion
  editable: boolean
  onEdit: () => void
  onDelete: () => void
  onUpload: (file: File) => Promise<boolean>
  onRemoveFile: (fileId: number) => Promise<boolean>
}) {
  const input = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)

  return (
    <article className="surface" aria-label={`Question ${question.code}`}>
      <header className="flex flex-wrap items-center gap-3 border-b border-border p-4 sm:px-5">
        <span className="font-mono text-sm font-semibold text-strong">{question.code}</span>
        <StatusBadge tone="info">{question.marks} marks</StatusBadge>
        {question.image ? (
          <span className="flex items-center gap-1.5 text-sm text-secondary"><Box className="h-4 w-4" /><span className="font-mono">{question.image}</span></span>
        ) : (
          <span className="text-sm text-muted">No lab</span>
        )}
        {editable && (
          <div className="ml-auto flex gap-1">
            <Button variant="ghost" onClick={onEdit} aria-label={`Edit ${question.code}`}><Pencil className="h-4 w-4" /></Button>
            <Button variant="ghost" onClick={onDelete} aria-label={`Delete ${question.code}`}><Trash2 className="h-4 w-4" /></Button>
          </div>
        )}
      </header>
      <div className="space-y-4 p-4 text-sm sm:px-5">
        <p className="whitespace-pre-line text-strong">{question.prompt}</p>
        <ol className="space-y-1">
          {question.answers.map((a, i) => (
            <li key={a.id} className="flex justify-between gap-3 text-secondary">
              <span>{i + 1}. {a.prompt}</span>
              <span className="shrink-0 font-medium text-strong">{a.marks}</span>
            </li>
          ))}
        </ol>
        <div className="flex flex-wrap items-center gap-2">
          {question.files.map((file) => (
            <span key={file.id} className={`inline-flex items-center gap-1 rounded-lg border px-2 py-1 ${file.present ? 'border-border' : 'border-amber-300 bg-amber-50'}`}>
              <a href={levelsApi.fileUrl(level.id, question.id, file.id)} className="inline-flex items-center gap-1.5 font-medium text-strong hover:text-primary">
                <Download className="h-4 w-4" />
                {file.name}
              </a>
              <span className="text-secondary">{formatBytes(file.size)}</span>
              {editable && (
                <button className="rounded p-1 text-muted hover:bg-subtle hover:text-danger" aria-label={`Remove ${file.name}`}
                  onClick={() => void onRemoveFile(file.id)}>
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              )}
            </span>
          ))}
          {editable && (
            <>
              <input ref={input} type="file" className="sr-only" aria-label={`Upload a file for ${question.code}`}
                onChange={async (e) => {
                  const file = e.target.files?.[0]
                  e.target.value = ''
                  if (!file) return
                  setUploading(true)
                  await onUpload(file)
                  setUploading(false)
                }} />
              <Button variant="secondary" disabled={uploading} onClick={() => input.current?.click()}>
                {uploading ? <BusyLabel>Uploading</BusyLabel> : <><FileUp className="h-4 w-4" />Add file</>}
              </Button>
            </>
          )}
          {!editable && question.files.length === 0 && <span className="text-muted">No files</span>}
        </div>
      </div>
    </article>
  )
}

function Summary({ label, value, detail, warn }: { label: string; value: string; detail: string; warn?: boolean }) {
  return (
    <div>
      <div className="text-xs font-semibold uppercase tracking-wide text-secondary">{label}</div>
      <div className={`mt-1 text-lg font-semibold ${warn ? 'text-warning' : 'text-strong'}`}>{value}</div>
      <div className="text-xs text-secondary">{detail}</div>
    </div>
  )
}

function Notice({ icon, tone = 'info', children }: { icon: React.ReactNode; tone?: 'info' | 'warning'; children: React.ReactNode }) {
  const style = tone === 'warning' ? 'border-amber-200 bg-amber-50 text-amber-900' : 'border-border bg-app text-secondary'
  return (
    <div role={tone === 'warning' ? 'alert' : 'note'} className={`flex items-start gap-3 rounded-lg border p-4 text-sm ${style}`}>
      <span className="mt-0.5 shrink-0">{icon}</span>
      <p>{children}</p>
    </div>
  )
}
