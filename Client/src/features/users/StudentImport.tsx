import { FileUp } from 'lucide-react'
import { useRef, useState } from 'react'
import { Button } from '../../components/ui/Button'
import { BusyLabel } from '../../components/ui/Feedback'
import { useToast } from '../../components/ui/Toast'
import { errorMessage } from '../../services/api'
import { usersApi } from '../../services/users'
import type { ImportResult } from '../../types/users'

// StudentImport uploads an .xlsx student list. Every sheet with Reg No,
// Student Name, Email and Department columns is read; uploading the same list
// again only applies what changed.
export function StudentImport({ onImported }: { onImported: () => void }) {
  const input = useRef<HTMLInputElement>(null)
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<ImportResult | null>(null)
  const { notify } = useToast()

  const upload = async (file: File) => {
    setBusy(true)
    try {
      const imported = await usersApi.importStudents(file)
      setResult(imported)
      notify(`${file.name} imported.`)
      onImported()
    } catch (err) {
      notify(errorMessage(err, 'The student list could not be imported.'), 'error')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="surface" aria-labelledby="student-import-heading">
      <div className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between sm:p-5">
        <div>
          <h3 id="student-import-heading" className="font-semibold text-strong">Import students</h3>
          <p className="mt-1 text-sm text-secondary">
            An .xlsx workbook with Reg No, Student Name, Email and Department columns. Every sheet is read; existing students are updated by email.
          </p>
        </div>
        <input ref={input} type="file" className="sr-only" aria-label="Student list workbook"
          accept=".xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          onChange={(e) => {
            const file = e.target.files?.[0]
            e.target.value = ''
            if (file) void upload(file)
          }} />
        <Button variant="secondary" className="shrink-0" disabled={busy} onClick={() => input.current?.click()}>
          {busy ? <BusyLabel>Importing</BusyLabel> : <><FileUp className="h-4 w-4" />Upload .xlsx</>}
        </Button>
      </div>
      {result && (
        <div className="border-t border-border p-4 sm:p-5" role="status" aria-label="Import result">
          <dl className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <Count label="Created" value={result.created} />
            <Count label="Updated" value={result.updated} />
            <Count label="Unchanged" value={result.unchanged} />
            <Count label="Skipped" value={result.skipped.length} warn={result.skipped.length > 0} />
          </dl>
          {result.skipped.length > 0 && (
            <div className="mt-4 max-h-72 overflow-auto">
              <table className="w-full min-w-[640px] text-left text-sm">
                <thead className="sticky top-0 bg-white text-xs uppercase tracking-wide text-muted">
                  <tr>
                    <th className="pb-2 font-semibold">Sheet</th>
                    <th className="pb-2 font-semibold">Row</th>
                    <th className="pb-2 font-semibold">Student</th>
                    <th className="pb-2 font-semibold">Reason</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {result.skipped.map((issue) => (
                    <tr key={`${issue.sheet}:${issue.row}`}>
                      <td className="py-2 text-secondary">{issue.sheet}</td>
                      <td className="py-2 text-secondary">{issue.row || '—'}</td>
                      <td className="py-2 text-strong">{[issue.registerNumber, issue.email].filter(Boolean).join(' · ') || '—'}</td>
                      <td className="py-2 text-secondary">{issue.reason}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </section>
  )
}

function Count({ label, value, warn }: { label: string; value: number; warn?: boolean }) {
  return (
    <div>
      <dt className="text-xs font-semibold uppercase tracking-wide text-secondary">{label}</dt>
      <dd className={`mt-1 text-lg font-semibold ${warn ? 'text-warning' : 'text-strong'}`}>{value}</dd>
    </div>
  )
}
