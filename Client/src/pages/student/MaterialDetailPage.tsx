import { ArrowLeft, BookOpen, Check, Circle, Clock3, Target } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Button } from '../../components/ui/Button'
import { ErrorState, LoadingSkeleton } from '../../components/ui/Feedback'
import { ProgressBar } from '../../components/ui/ProgressBar'
import { StatusBadge } from '../../components/ui/StatusBadge'
import { useToast } from '../../components/ui/Toast'
import { platformRepository } from '../../services/repositories'
import type { LearningMaterial } from '../../types/domain'

export function MaterialDetailPage() {
  const { materialId } = useParams()
  const { notify } = useToast()
  const [material, setMaterial] = useState<LearningMaterial | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [busyLesson, setBusyLesson] = useState('')
  useEffect(() => { const load = () => platformRepository.listLearningMaterials().then((items) => { const found = items.find((item) => item.id === materialId); if (!found) throw new Error('Material not found'); setMaterial(found) }).catch(() => setError('This learning material could not be found.')).finally(() => setLoading(false)); void load(); const refresh = (event: Event) => { const detail = (event as CustomEvent<{ materialId: string }>).detail; if (detail.materialId === materialId) void load() }; document.addEventListener('cyberspace:learning-updated', refresh); return () => document.removeEventListener('cyberspace:learning-updated', refresh) }, [materialId])
  const progress = useMemo(() => material ? Math.round(material.lessons.filter((item) => item.completed).length / material.lessons.length * 100) : 0, [material])
  if (loading) return <LoadingSkeleton rows={5} />
  if (error || !material) return <ErrorState message={error || 'Material not found.'} />
  const update = async (lessonId: string, completed: boolean) => {
    setBusyLesson(lessonId)
    try { setMaterial(await platformRepository.updateLesson(material.id, lessonId, completed)); notify(completed ? 'Lesson marked complete.' : 'Lesson marked incomplete.') }
    catch { notify('Lesson progress could not be updated.', 'error') }
    finally { setBusyLesson('') }
  }
  return <div className="space-y-6"><Link to="/student/learning" className="inline-flex items-center gap-2 text-sm font-semibold text-secondary hover:text-primary"><ArrowLeft className="h-4 w-4" />Back to learning materials</Link><section className="surface overflow-hidden"><div className="border-b border-border p-5 sm:p-7"><div className="flex flex-wrap items-center gap-2"><StatusBadge tone="info">{material.category}</StatusBadge><StatusBadge>{material.difficulty}</StatusBadge></div><div className="mt-4 grid gap-6 lg:grid-cols-[1fr_300px]"><div><h2 className="page-heading">{material.title}</h2><p className="mt-3 max-w-3xl leading-7 text-secondary">{material.description}</p><div className="mt-4 flex flex-wrap gap-5 text-sm text-secondary"><span className="flex items-center gap-2"><Clock3 className="h-4 w-4" />{Math.floor(material.duration / 60)}h {material.duration % 60}m</span><span className="flex items-center gap-2"><BookOpen className="h-4 w-4" />{material.lessons.length} lessons</span></div></div><div className="rounded-lg border border-border bg-app p-4"><ProgressBar value={progress} label="Course progress" /><p className="mt-3 text-sm text-secondary">{material.lessons.filter((lesson) => lesson.completed).length} of {material.lessons.length} lessons complete</p></div></div></div><div className="grid gap-7 p-5 sm:p-7 lg:grid-cols-[minmax(0,1fr)_320px]"><div><h3 className="font-semibold text-strong">Lessons</h3><div className="mt-3 divide-y divide-border rounded-lg border border-border">{material.lessons.map((lesson, index) => <div key={lesson.id} className="flex items-center gap-3 p-4"><div className={`grid h-8 w-8 shrink-0 place-items-center rounded-full ${lesson.completed ? 'bg-emerald-50 text-success' : 'bg-slate-100 text-secondary'}`}>{lesson.completed ? <Check className="h-4 w-4" /> : <span className="text-sm font-semibold">{index + 1}</span>}</div><div className="min-w-0 flex-1"><p className="font-medium text-strong">{lesson.title}</p><p className="mt-0.5 text-sm text-secondary">{lesson.duration} minutes</p></div><Button variant={lesson.completed ? 'ghost' : 'secondary'} disabled={busyLesson === lesson.id} onClick={() => update(lesson.id, !lesson.completed)}>{lesson.completed ? 'Undo' : 'Mark complete'}</Button></div>)}</div></div><aside className="space-y-5"><div><h3 className="flex items-center gap-2 font-semibold text-strong"><Circle className="h-4 w-4 text-primary" />Prerequisites</h3><ul className="mt-3 space-y-2 text-sm leading-6 text-secondary">{material.prerequisites.map((item) => <li key={item}>• {item}</li>)}</ul></div><div className="border-t border-border pt-5"><h3 className="flex items-center gap-2 font-semibold text-strong"><Target className="h-4 w-4 text-primary" />Learning outcomes</h3><ul className="mt-3 space-y-2 text-sm leading-6 text-secondary">{material.outcomes.map((item) => <li key={item}>• {item}</li>)}</ul></div></aside></div></section></div>
}
