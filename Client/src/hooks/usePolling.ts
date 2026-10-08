import { useCallback, useEffect, useRef, useState } from 'react'
import { errorMessage } from '../services/api'

interface Polled<T> {
  data: T | null
  error: string
  loading: boolean
  refresh: () => Promise<void>
}

// usePolling loads data now and then every intervalMs while the component is
// mounted. A failed refresh keeps the last good data and reports the error,
// so a brief network blip does not blank a live view.
export function usePolling<T>(load: () => Promise<T>, intervalMs: number): Polled<T> {
  const [data, setData] = useState<T | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const loadRef = useRef(load)
  loadRef.current = load

  const refresh = useCallback(async () => {
    try {
      setData(await loadRef.current())
      setError('')
    } catch (err) {
      setError(errorMessage(err, 'Could not load the latest data.'))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
    const timer = window.setInterval(() => void refresh(), intervalMs)
    return () => window.clearInterval(timer)
  }, [refresh, intervalMs])

  return { data, error, loading, refresh }
}

// useNow ticks every second, for countdowns.
export function useNow(): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 1000)
    return () => window.clearInterval(timer)
  }, [])
  return now
}
