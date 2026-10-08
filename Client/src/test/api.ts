// A per-test registry of stubbed API responses. Tests register what the
// server would answer; the fetch stub in setup.ts consults it, so the real
// services and pages run unchanged.

const replyMark = Symbol('reply')
type Reply = { [replyMark]: true; status: number; body?: unknown }
type Handler = (body: unknown) => unknown

// reply answers with a specific status, for example an error.
export function reply(status: number, body?: unknown): Reply {
  return { [replyMark]: true, status, body }
}

interface Call {
  method: string
  path: string
  body: unknown
}

const routes = new Map<string, Handler>()
const calls: Call[] = []

// mockApi answers METHOD path (path includes any query string) with a
// value, or a handler of the request body. A reply(...) controls the
// status; anything else is a 200 JSON body.
export function mockApi(method: string, path: string, handler: Handler | unknown) {
  routes.set(`${method} ${path}`, typeof handler === 'function' ? (handler as Handler) : () => handler)
}

export function apiCalls(method?: string, path?: string): Call[] {
  return calls.filter((c) => (!method || c.method === method) && (!path || c.path === path))
}

export function resetApi() {
  routes.clear()
  calls.length = 0
}

export function answer(method: string, path: string, body: unknown): Response | null {
  calls.push({ method, path, body })
  const handler = routes.get(`${method} ${path}`)
  if (!handler) return null
  const result = handler(body)
  const out: Reply = result && typeof result === 'object' && replyMark in result ? (result as Reply) : reply(200, result)
  if (out.body === undefined) return new Response(null, { status: out.status === 200 ? 204 : out.status })
  return new Response(JSON.stringify(out.body), { status: out.status, headers: { 'Content-Type': 'application/json' } })
}
