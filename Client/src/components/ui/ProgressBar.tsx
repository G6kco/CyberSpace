export function ProgressBar({ value, label }: { value: number; label?: string }) {
  const safe = Math.max(0, Math.min(100, value))
  return <div>
    {label && <div className="mb-1.5 flex justify-between text-sm"><span className="text-secondary">{label}</span><span className="font-semibold text-strong">{safe}%</span></div>}
    <div className="h-2 overflow-hidden rounded-full bg-slate-100" role="progressbar" aria-valuemin={0} aria-valuemax={100} aria-valuenow={safe} aria-label={label ?? 'Progress'}>
      <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${safe}%` }} />
    </div>
  </div>
}
