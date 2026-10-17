import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, test } from 'vitest'
import type { Role, SummaryRow } from '@/api'
import { apiError, meAs, mockApi, renderApp } from '@/test/utils'

const rows: SummaryRow[] = [
  { jobId: 'J-24001', jobName: '滨江一号', totalHours: 120.5, lateCount: 0 },
  { jobId: 'J-24002', jobName: '北湖二期', totalHours: 64, lateCount: 3 },
]

const routes = (role: Role, summary: SummaryRow[] = rows) => ({
  'GET /api/me': { body: meAs(role) },
  'GET /api/summary': { body: { rows: summary } },
})

describe('SummaryPage', () => {
  it('renders totals and late counts per job', async () => {
    mockApi(routes('finance'))
    renderApp('/summary')
    const table = await screen.findByRole('table', { name: /按工地汇总/ })
    const body = within(table).getAllByRole('row').slice(1)
    expect(within(body[0]!).getByText('120.5')).toBeInTheDocument()
    expect(within(body[1]!).getByText('迟交 3 份')).toBeInTheDocument()
  })

  it('shows a loading state', async () => {
    mockApi({ ...routes('manager'), 'GET /api/summary': () => new Promise(() => {}) as never })
    renderApp('/summary')
    expect(await screen.findByText('正在加载汇总')).toBeInTheDocument()
  })

  it('shows the empty state', async () => {
    mockApi(routes('admin', []))
    renderApp('/summary')
    expect(await screen.findByText('还没有可汇总的日报')).toBeInTheDocument()
  })

  test('AC-1 shows the range empty message without a table', async () => {
    const { calls } = mockApi(routes('manager', []))
    renderApp('/summary?from=2030-01-01&to=2030-01-02')
    expect(await screen.findByText('所选日期范围内没有已提交的日报')).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(calls.find((c) => c.path === '/api/summary')?.search).toBe('?from=2030-01-01&to=2030-01-02')
  })

  it('shows the error state', async () => {
    mockApi({ ...routes('admin'), 'GET /api/summary': apiError(500, 'unknown_code') })
    renderApp('/summary')
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('汇总加载失败')
    expect(alert).toHaveTextContent('操作失败（unknown_code）')
  })

  it('passes the date range to the API', async () => {
    const { calls } = mockApi(routes('manager'))
    renderApp('/summary?from=2026-09-01')
    await screen.findByRole('table')
    expect(calls.find((c) => c.path === '/api/summary')?.search).toBe('?from=2026-09-01')
    await userEvent.type(screen.getByLabelText('结束日期'), '2026-09-30')
    await waitFor(() =>
      expect(calls.filter((c) => c.path === '/api/summary').at(-1)?.search).toBe('?from=2026-09-01&to=2026-09-30'),
    )
  })

  it('tells field users they have no access and does not call the API', async () => {
    const { calls } = mockApi(routes('field'))
    renderApp('/summary')
    expect(await screen.findByText('无权限')).toBeInTheDocument()
    expect(screen.getByText(/你的角色（现场）不能查看工时汇总/)).toBeInTheDocument()
    expect(calls.some((c) => c.path === '/api/summary')).toBe(false)
  })
})
