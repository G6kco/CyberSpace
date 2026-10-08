export const apiBase = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

// ApiError carries the server's error code so pages can react to specific
// refusals (for example an attempt that has already closed).
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'DELETE'
  body?: unknown
}

// apiRequest sends the session cookie with every call. The browser adds the
// Origin header to these cross-origin requests, which the server requires
// for anything that changes state.
export async function apiRequest<T>(path: string, { method = 'GET', body }: RequestOptions = {}): Promise<T> {
  const response = await fetch(`${apiBase}${path}`, {
    method,
    credentials: 'include',
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })

  if (!response.ok) {
    let code = 'request_failed'
    let message = 'The request could not be completed.'
    try {
      const payload = (await response.json()) as { code?: string; message?: string }
      code = payload.code ?? code
      message = payload.message ?? message
    } catch {
      // Not every failure has a JSON body (for example a CORS refusal).
    }
    throw new ApiError(response.status, code, message)
  }

  if (response.status === 204 || response.status === 202) return undefined as T
  return (await response.json()) as T
}

export function errorMessage(error: unknown, fallback = 'Something went wrong. Try again.'): string {
  return error instanceof ApiError ? error.message : fallback
}
