import { AlertTriangle, CheckCircle2, FileWarning } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { EmptyState, ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { SearchInput } from '../../components/ui/SearchInput'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { poolName } from '../../lib/assessment'
import { errorMessage } from '../../services/api'
import { adminApi } from '../../services/assessments'
import type { BankAssessment, BankQuestion, Pool } from '../../types/assessment'

// The question bank of every published level. It is read-only here: the
// bank is imported on the server (cmd/import-questions) and answers are
// stored only as keyed hashes, so they cannot be shown.
export function AssessmentManagementPage() {
  const [bank, setBank] = useState<BankAssessment[] | null>(null)
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')
  const [pool, setPool] = useState<Pool | 'all'>('all')

  const load = () => {
    setError('')
    adminApi.questionBank().then(setBank).catch((err) => setError(errorMessage(err, 'The question bank could not be loaded.')))
  }
  useEffect(load, [])

  if (!bank && !error) return <LoadingSkeleton rows={5} />
  if (!bank) return <ErrorState message={error} onRetry={load} />

  return (
    <div className="space-y-6">
      <div>
        <p className="eyebrow">Assessment catalogue</p>
        <h2 className="page-heading">Question bank</h2>
        <p className="mt-2 max-w-2xl text-secondary">
          Each attempt deals one random question from each pool. Expected flags are stored as keyed hashes and are never shown.
        </p>
      </div>
      <div className="grid gap-3 sm:grid-cols-[1fr_200px]">
        <SearchInput value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search questions" />
        <label>
          <span className="sr-only">Pool</span>
          <select className="input" value={pool} onChange={(e) => setPool(e.target.value as Pool | 'all')}>
            <option value="all">All pools</option>
            <option value="wireshark">Wireshark</option>
            <option value="nmap">Nmap</option>
          </select>
        </label>
      </div>
      {bank.length === 0 ? (
        <EmptyState title="No published assessments" description="Import the question bank on the server to publish Level 1." />
      ) : (
        bank.map((assessment) => <AssessmentBank key={assessment.level} assessment={assessment} search={search} pool={pool} />)
      )}
    </div>
  )
}

function AssessmentBank({ assessment, search, pool }: { assessment: BankAssessment; search: string; pool: Pool | 'all' }) {
  const questions = useMemo(() => {
    const term = search.toLowerCase()
    return assessment.questions.filter(
      (q) => (pool === 'all' || q.pool === pool) && (!term || `${q.code} ${q.prompt} ${q.flagPrompts.join(' ')}`.toLowerCase().includes(term)),
    )
  }, [assessment.questions, search, pool])
  const missing = assessment.questions.filter((q) => q.pool === 'wireshark' && !q.pcapPresent)
  const count = (p: Pool, d: string) => assessment.questions.filter((q) => q.pool === p && q.difficulty === d).length

  return (
    <section className="surface">
      <header className="flex flex-wrap items-start justify-between gap-4 border-b border-border p-4 sm:p-5">
        <div>
          <div className="flex items-center gap-2">
            <StatusBadge tone="info">{assessment.level}</StatusBadge>
            <StatusBadge tone="success">Published</StatusBadge>
          </div>
          <h3 className="mt-2 text-lg font-semibold text-strong">{assessment.title}</h3>
          <p className="mt-1 text-sm text-secondary">
            {Math.round(assessment.durationSeconds / 60)} minutes · pass {assessment.passScore} / {assessment.totalPoints}
          </p>
        </div>
        <dl className="grid grid-cols-2 gap-x-6 gap-y-1 text-sm">
          {(['wireshark', 'nmap'] as const).map((p) => (
            <div key={p} className="contents">
              <dt className="text-secondary">{poolName[p]}</dt>
              <dd className="font-medium text-strong">{count(p, 'easy')} easy · {count(p, 'hard')} hard</dd>
            </div>
          ))}
        </dl>
      </header>
      {missing.length > 0 && (
        <div role="alert" className="flex items-start gap-3 border-b border-amber-200 bg-amber-50 p-4 text-sm text-amber-900">
          <FileWarning className="mt-0.5 h-5 w-5 shrink-0" />
          <p>
            {missing.length} capture file(s) are missing from the server's capture directory:{' '}
            <span className="font-mono">{missing.map((q) => q.pcapFile).join(', ')}</span>. Students dealt these questions cannot download them.
          </p>
        </div>
      )}
      <div className="divide-y divide-border">
        {questions.length === 0 ? (
          <div className="p-5"><EmptyState title="No questions match" description="Adjust the search or pool filter." /></div>
        ) : (
          questions.map((q) => <QuestionRow key={q.id} question={q} />)
        )}
      </div>
    </section>
  )
}

function QuestionRow({ question }: { question: BankQuestion }) {
  return (
    <details className="group p-4 sm:px-5">
      <summary className="flex cursor-pointer list-none flex-wrap items-center gap-3">
        <span className="font-mono text-sm font-semibold text-strong">{question.code}</span>
        <StatusBadge tone={question.difficulty === 'hard' ? 'warning' : 'neutral'}>{question.difficulty}</StatusBadge>
        <span className="text-sm text-secondary">{question.points} pts · {question.flagPrompts.length} flag(s)</span>
        <span className="flex items-center gap-1.5 text-sm text-secondary">
          {question.pool === 'wireshark' ? (
            <>
              {question.pcapPresent ? <CheckCircle2 className="h-4 w-4 text-success" /> : <AlertTriangle className="h-4 w-4 text-warning" />}
              <span className="font-mono">{question.pcapFile}</span>
            </>
          ) : (
            <>image <span className="font-mono">{question.image}</span></>
          )}
        </span>
        <span className="ml-auto text-xs text-muted">dealt {question.timesDealt}×</span>
      </summary>
      <div className="mt-3 space-y-3 text-sm">
        <p className="whitespace-pre-line text-strong">{question.prompt}</p>
        <ol className="list-decimal space-y-1 pl-5 text-secondary">
          {question.flagPrompts.map((prompt, i) => <li key={i}>{prompt}</li>)}
        </ol>
      </div>
    </details>
  )
}
