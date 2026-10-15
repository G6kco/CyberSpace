import { ChevronRight, Plus } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Button } from '../../components/ui/Button'
import { Drawer } from '../../components/ui/Drawer'
import { EmptyState, ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { useToast } from '../../components/ui/Toast'
import { LevelSettingsForm, emptyLevel } from '../../features/levels/LevelForms'
import { errorMessage } from '../../services/api'
import { levelsApi } from '../../services/levels'
import type { Level } from '../../types/levels'

// Every level in the order students progress through them. New levels are
// built here and appended after the last one.
export function LevelsPage() {
  const [levels, setLevels] = useState<Level[] | null>(null)
  const [error, setError] = useState('')
  const [creating, setCreating] = useState(false)
  const navigate = useNavigate()
  const { notify } = useToast()
  const closeCreate = useCallback(() => setCreating(false), [])

  const load = () => {
    setError('')
    levelsApi.list().then(setLevels).catch((err) => setError(errorMessage(err, 'The levels could not be loaded.')))
  }
  useEffect(load, [])

  if (!levels && !error) return <LoadingSkeleton rows={4} />
  if (!levels) return <ErrorState message={error} onRetry={load} />

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="eyebrow">Level builder</p>
          <h2 className="page-heading">Levels</h2>
          <p className="mt-2 max-w-2xl text-secondary">
            Students take levels in this order. A level is a bank of scenario questions; each student is dealt a set number of them at random, each with its own lab and files.
          </p>
        </div>
        <Button onClick={() => setCreating(true)}>
          <Plus className="h-4 w-4" />
          New level
        </Button>
      </div>

      {levels.length === 0 ? (
        <EmptyState title="No levels yet" description="Create the first level to start building questions." />
      ) : (
        <ol className="surface divide-y divide-border">
          {levels.map((level) => (
            <li key={level.id}>
              <Link to={`/admin/levels/${level.id}`} className="flex flex-wrap items-center gap-x-6 gap-y-2 p-4 hover:bg-subtle sm:px-5">
                <span className="grid h-9 w-9 place-items-center rounded-full bg-app font-semibold text-strong">{level.sequence}</span>
                <div className="min-w-48 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="font-semibold text-strong">{level.name}</span>
                    <StatusBadge tone={level.status === 'published' ? 'success' : 'neutral'}>
                      {level.status === 'published' ? 'Published' : 'Draft'}
                    </StatusBadge>
                    {!level.built && <StatusBadge tone="info">Imported</StatusBadge>}
                  </div>
                  <p className="text-sm text-secondary">{level.title}</p>
                </div>
                <span className="text-sm text-secondary">
                  {level.built ? `${level.questionsPerAttempt} of ${level.questionCount} questions dealt` : `${level.questionCount} questions`}
                </span>
                <span className="text-sm text-secondary">{level.durationMinutes} min · pass {level.passPercent}%</span>
                <span className="text-sm text-secondary">{level.attempts} attempt(s)</span>
                <ChevronRight className="h-4 w-4 text-muted" />
              </Link>
            </li>
          ))}
        </ol>
      )}

      <Drawer open={creating} title="New level" description="You add questions after creating it." onClose={closeCreate}>
        <LevelSettingsForm
          initial={{ ...emptyLevel, name: `Level ${levels.length + 1}` }}
          submitLabel="Create level"
          onSubmit={async (input) => {
            try {
              const level = await levelsApi.create(input)
              notify('Level created.')
              navigate(`/admin/levels/${level.id}`)
            } catch (err) {
              notify(errorMessage(err), 'error')
            }
          }}
        />
      </Drawer>
    </div>
  )
}
