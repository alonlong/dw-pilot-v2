import type {
  CrewEntry,
  DateRange,
  ErrorBody,
  Job,
  Me,
  ReportDetail,
  ReportFilter,
  ReportRow,
  SummaryRow,
} from './types'

/**
 * Every failed call becomes an ApiError. `code` is the server's error.code when the body
 * had one; otherwise `network` (no response at all) or `unknown` (a response that is not
 * the documented error shape, e.g. a proxy's 502 page).
 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

function isErrorBody(v: unknown): v is ErrorBody {
  if (typeof v !== 'object' || v === null || !('error' in v)) return false
  const e = (v as { error: unknown }).error
  return typeof e === 'object' && e !== null && typeof (e as { code: unknown }).code === 'string'
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  let res: Response
  try {
    res = await fetch(path, {
      method,
      credentials: 'include',
      headers: body === undefined ? { Accept: 'application/json' } : { Accept: 'application/json', 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
  } catch (err) {
    throw new ApiError(0, 'network', err instanceof Error ? err.message : String(err))
  }
  if (res.status === 204) return undefined as T

  const text = await res.text()
  let data: unknown = undefined
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = undefined
    }
  }
  if (!res.ok) {
    if (isErrorBody(data)) throw new ApiError(res.status, data.error.code, data.error.message)
    throw new ApiError(res.status, 'unknown', `HTTP ${res.status}`)
  }
  if (data === undefined) throw new ApiError(res.status, 'unknown', '响应不是 JSON')
  return data as T
}

function query(params: Record<string, string | undefined>): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) if (v) q.set(k, v)
  const s = q.toString()
  return s ? `?${s}` : ''
}

export const api = {
  login: (email: string, password: string) => request<Me>('POST', '/api/login', { email, password }),
  logout: () => request<void>('POST', '/api/logout'),
  me: () => request<Me>('GET', '/api/me'),
  jobs: () => request<{ jobs: Job[] }>('GET', '/api/jobs').then((r) => r.jobs),
  reports: (f: ReportFilter = {}) =>
    request<{ rows: ReportRow[] }>('GET', `/api/reports${query({ job: f.job, from: f.from, to: f.to })}`).then((r) => r.rows),
  createReport: (jobId: string, businessDate: string) =>
    request<ReportDetail>('POST', '/api/reports', { jobId, businessDate }),
  report: (id: number) => request<ReportDetail>('GET', `/api/reports/${id}`),
  updateCrew: (id: number, crew: CrewEntry[]) => request<ReportDetail>('PUT', `/api/reports/${id}/crew`, { crew }),
  submit: (id: number) => request<ReportDetail>('POST', `/api/reports/${id}/submit`),
  lock: (id: number) => request<ReportDetail>('POST', `/api/reports/${id}/lock`),
  addNote: (id: number, body: string) => request<ReportDetail>('POST', `/api/reports/${id}/notes`, { body }),
  summary: (r: DateRange = {}) =>
    request<{ rows: SummaryRow[] }>('GET', `/api/summary${query({ from: r.from, to: r.to })}`).then((x) => x.rows),
}
