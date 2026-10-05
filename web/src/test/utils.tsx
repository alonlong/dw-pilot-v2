import { render } from '@testing-library/react'
import { QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter, useLocation } from 'react-router'
import { vi } from 'vitest'
import type { Me, ReportDetail, Role } from '@/api'
import { AppRoutes } from '@/app/AppRoutes'
import { createQueryClient } from '@/app/queryClient'

export interface MockResponse {
  status?: number
  body?: unknown
}

export interface Call {
  method: string
  path: string
  search: string
  body: unknown
}

/** `"GET /api/reports"` → response (or a function of the request). Unlisted routes answer 500. */
export type Routes = Record<string, MockResponse | ((call: Call) => MockResponse)>

export function mockApi(routes: Routes) {
  const calls: Call[] = []
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), 'http://localhost')
    const method = (init?.method ?? 'GET').toUpperCase()
    const call: Call = {
      method,
      path: url.pathname,
      search: url.search,
      body: typeof init?.body === 'string' ? JSON.parse(init.body) : undefined,
    }
    calls.push(call)
    const route = routes[`${method} ${url.pathname}`]
    const res = typeof route === 'function' ? route(call) : (route ?? { status: 500, body: { error: { code: 'unmocked', message: `${method} ${url.pathname}` } } })
    const status = res.status ?? 200
    return new Response(status === 204 ? null : JSON.stringify(res.body ?? {}), {
      status,
      headers: { 'Content-Type': 'application/json' },
    })
  })
  vi.stubGlobal('fetch', fetchMock)
  return { calls, fetchMock }
}

export const apiError = (status: number, code: string, message = code): MockResponse => ({
  status,
  body: { error: { code, message } },
})

export function meAs(role: Role): Me {
  return { id: 1, email: `${role}@dlpv1.test`, name: `测试${role}`, role, tenant: 'dlpv1', tenantName: '试点租户一' }
}

export function detail(overrides: Partial<ReportDetail> = {}): ReportDetail {
  return {
    id: 7,
    jobId: 'J-24001',
    jobName: '滨江一号',
    businessDate: '2026-09-02',
    status: '未提交',
    submittedAt: null,
    locked: false,
    totalHours: 16,
    daysLate: 0,
    crew: [
      { worker: '张三', hours: 8 },
      { worker: '李四', hours: 8 },
    ],
    notes: [],
    can: { editCrew: true, submit: true, lock: false, addNote: true },
    ...overrides,
  }
}

export const jobs = [
  { id: 'J-24001', name: '滨江一号', address: '滨江路 1 号' },
  { id: 'J-24002', name: '北湖二期', address: '北湖大道 8 号' },
]

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="location">{loc.pathname + loc.search}</div>
}

export function renderApp(path: string) {
  const queryClient = createQueryClient({ retry: false })
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[path]}>
        <AppRoutes />
        <LocationProbe />
      </MemoryRouter>
    </QueryClientProvider>,
  )
  return { ...utils, queryClient }
}
