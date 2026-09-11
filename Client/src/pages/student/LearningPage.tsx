import { ArrowRight, BookOpen, CheckCircle2, Clock3, SlidersHorizontal } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import { Button } from '../../components/ui/Button'
import { EmptyState, ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { ProgressBar } from '../../components/ui/ProgressBar'
import { SearchInput } from '../../components/ui/SearchInput'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { platformRepository } from '../../services/repositories'
import type { Difficulty, LearningMaterial, MaterialCategory } from '../../types/domain'

const progressOf = (material: LearningMaterial) => Math.round(material.lessons.filter((lesson) => lesson.completed).length / material.lessons.length * 100)

export function LearningPage() {
  const [materials, setMaterials] = useState<LearningMaterial[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [search, setSearch] = useState('')
  const [category, setCategory] = useState<MaterialCategory | 'All'>('All')
  const [difficulty, setDifficulty] = useState<Difficulty | 'All'>('All')
  const [completion, setCompletion] = useState<'All' | 'Not started' | 'In progress' | 'Completed'>('All')
  const load = () => { setLoading(true); setError(''); platformRepository.listLearningMaterials().then(setMaterials).catch(() => setError('The mock learning repository did not respond.')).finally(() => setLoading(false)) }
  useEffect(() => { load(); const refresh = () => load(); document.addEventListener('cyberspace:learning-updated', refresh); return () => document.removeEventListener('cyberspace:learning-updated', refresh) }, [])
  const filtered = useMemo(() => materials.filter((material) => {
    const progress = progressOf(material)
    const state = progress === 100 ? 'Completed' : progress === 0 ? 'Not started' : 'In progress'
    return (!search || `${material.title} ${material.description} ${material.category}`.toLowerCase().includes(search.toLowerCase())) && (category === 'All' || material.category === category) && (difficulty === 'All' || material.difficulty === difficulty) && (completion === 'All' || completion === state)
  }), [materials, search, category, difficulty, completion])
  const latest = materials.find((material) => { const p = progressOf(material); return p > 0 && p < 100 })
  return <div className="space-y-6">
    <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between"><div><p className="eyebrow">Student learning</p><h2 className="page-heading">Build skills before your next assessment</h2><p className="mt-2 max-w-2xl text-secondary">Follow structured material, track lesson progress, and confirm prerequisites.</p></div>{latest && <Link to={`/student/learning/${latest.id}`}><Button>Continue learning <ArrowRight className="h-4 w-4" /></Button></Link>}</div>
    {latest && <section className="grid gap-5 rounded-xl border border-blue-200 bg-primary-light p-5 sm:grid-cols-[1fr_auto] sm:items-center"><div><div className="flex items-center gap-2 text-sm font-semibold text-primary-dark"><BookOpen className="h-4 w-4" />Continue where you left off</div><h3 className="mt-2 text-lg font-semibold text-strong">{latest.title}</h3><p className="mt-1 text-sm text-secondary">Next: {latest.lessons.find((lesson) => !lesson.completed)?.title}</p><div className="mt-4 max-w-lg"><ProgressBar value={progressOf(latest)} /></div></div><Link className="text-sm font-semibold text-primary hover:text-primary-dark" to={`/student/learning/${latest.id}`}>Open material <ArrowRight className="ml-1 inline h-4 w-4" /></Link></section>}
    <section className="rounded-xl border border-border bg-white"><div className="border-b border-border p-4 sm:p-5"><div className="flex items-center gap-2 font-semibold text-strong"><SlidersHorizontal className="h-4 w-4" />Find learning material</div><div className="mt-4 grid gap-3 md:grid-cols-[minmax(220px,1fr)_repeat(3,minmax(145px,0.45fr))]"><SearchInput value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search title or category" /><label><span className="sr-only">Category</span><select className="input" value={category} onChange={(e) => setCategory(e.target.value as typeof category)}><option>All</option>{Array.from(new Set(materials.map((item) => item.category))).map((item) => <option key={item}>{item}</option>)}</select></label><label><span className="sr-only">Difficulty</span><select className="input" value={difficulty} onChange={(e) => setDifficulty(e.target.value as typeof difficulty)}><option>All</option><option>Beginner</option><option>Intermediate</option><option>Advanced</option></select></label><label><span className="sr-only">Completion</span><select className="input" value={completion} onChange={(e) => setCompletion(e.target.value as typeof completion)}><option>All</option><option>Not started</option><option>In progress</option><option>Completed</option></select></label></div></div>
      <div className="p-4 sm:p-5">{loading ? <LoadingSkeleton rows={4} /> : error ? <ErrorState message={error} onRetry={load} /> : filtered.length === 0 ? <EmptyState title="No materials match" description="Try clearing one or more filters." action={<Button variant="secondary" onClick={() => { setSearch(''); setCategory('All'); setDifficulty('All'); setCompletion('All') }}>Clear filters</Button>} /> : <div className="divide-y divide-border">{filtered.map((material) => { const progress = progressOf(material); return <article key={material.id} className="grid gap-4 py-5 first:pt-0 last:pb-0 md:grid-cols-[1fr_180px_140px] md:items-center"><div><div className="flex flex-wrap items-center gap-2"><StatusBadge tone={material.difficulty === 'Advanced' ? 'warning' : material.difficulty === 'Intermediate' ? 'info' : 'neutral'}>{material.difficulty}</StatusBadge><span className="text-xs font-medium text-secondary">{material.category}</span></div><h3 className="mt-2 text-lg font-semibold text-strong"><Link className="hover:text-primary" to={`/student/learning/${material.id}`}>{material.title}</Link></h3><p className="mt-1 line-clamp-2 text-sm leading-6 text-secondary">{material.description}</p><div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-secondary"><span className="flex items-center gap-1.5"><Clock3 className="h-4 w-4" />{Math.floor(material.duration / 60)}h {material.duration % 60}m</span><span className="flex items-center gap-1.5"><BookOpen className="h-4 w-4" />{material.lessons.length} lessons</span></div></div><div><ProgressBar value={progress} label="Progress" /></div><Link to={`/student/learning/${material.id}`} className="inline-flex min-h-10 items-center justify-center gap-2 rounded-lg border border-border px-3 text-sm font-semibold text-strong hover:bg-subtle">{progress === 100 ? <><CheckCircle2 className="h-4 w-4 text-success" />Review</> : progress > 0 ? <>Continue <ArrowRight className="h-4 w-4" /></> : <>Start <ArrowRight className="h-4 w-4" /></>}</Link></article> })}</div>}</div>
    </section>
  </div>
}
