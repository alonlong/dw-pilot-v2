import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, test } from 'vitest'
import type { ReportRow, Role } from '@/api'
import { apiError, detail, jobs, meAs, mockApi, renderApp } from '@/test/utils'

const rows: ReportRow[] = [
  { id: 1, jobId: 'J-24001', jobName: '滨江一号', businessDate: '2026-09-01', status: '已提交', totalHours: 24.5, daysLate: 0, noteCount: 2 },
  { id: 2, jobId: 'J-24002', jobName: '北湖二期', businessDate: '2026-09-01', status: '已锁定', totalHours: 16, daysLate: 2, noteCount: 0 },
  { id: 3, jobId: 'J-24001', jobName: '滨江一号', businessDate: '2026-09-02', status: '未提交', totalHours: 0, daysLate: 0, noteCount: 1 },
]

const routes = (role: Role, reportRows: ReportRow[] = rows) => ({
  'GET /api/me': { body: meAs(role) },
  'GET /api/jobs': { body: { jobs } },
  'GET /api/reports': { body: { rows: reportRows } },
})

describe('ReportsPage', () => {
  it('renders the register with status badges and the late marker', async () => {
    mockApi(routes('finance'))
    renderApp('/reports')
    const table = await screen.findByRole('table', { name: /日报台账/ })
    const bodyRows = within(table).getAllByRole('row').slice(1)
    expect(bodyRows).toHaveLength(3)
    expect(within(bodyRows[0]!).getByText('已提交')).toBeInTheDocument()
    expect(within(bodyRows[0]!).getByText('24.5')).toBeInTheDocument()
    expect(within(bodyRows[1]!).getByText('已锁定')).toBeInTheDocument()
    expect(within(bodyRows[1]!).getByText('迟交 2 天')).toBeInTheDocument()
    expect(within(bodyRows[2]!).getByText('未提交')).toBeInTheDocument()
    expect(within(bodyRows[1]!).getByRole('link', { name: /北湖二期 2026-09-01/ })).toHaveAttribute('href', '/reports/2')
  })

  test('AC-2 shows the 备注 column with noteCount for each row', async () => {
    // The last row omits noteCount, as an older server would.
    const legacy: ReportRow = {
      id: 3,
      jobId: 'J-24001',
      jobName: '滨江一号',
      businessDate: '2026-09-02',
      status: '未提交',
      totalHours: 0,
      daysLate: 0,
    }
    mockApi(routes('manager', [rows[0]!, rows[1]!, legacy]))
    renderApp('/reports')
    const table = await screen.findByRole('table', { name: /日报台账/ })
    const [header, ...bodyRows] = within(table).getAllByRole('row')
    const headers = within(header!).getAllByRole('columnheader')
    const noteIndex = headers.findIndex((h) => h.textContent === '备注')
    expect(noteIndex).toBeGreaterThanOrEqual(0)
    const noteCell = (row: HTMLElement) => within(row).getAllByRole('cell')[noteIndex]
    expect(bodyRows.map((r) => noteCell(r)?.textContent)).toEqual(['2', '0', '0'])
  })

  it('shows a loading state', async () => {
    mockApi({ ...routes('field'), 'GET /api/reports': () => new Promise(() => {}) as never })
    renderApp('/reports')
    expect(await screen.findByText('正在加载日报')).toBeInTheDocument()
  })

  it('shows the empty state', async () => {
    mockApi(routes('finance', []))
    renderApp('/reports')
    expect(await screen.findByText('还没有日报')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '新建日报' })).toBeNull()
  })

  it('shows a filtered empty state that clears the filter', async () => {
    mockApi(routes('finance', []))
    renderApp('/reports?job=J-24002')
    expect(await screen.findByText('没有符合筛选条件的日报')).toBeInTheDocument()
    await userEvent.click(screen.getAllByRole('button', { name: '清除筛选' })[1]!)
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent(/^\/reports$/))
  })

  it('shows the error state with the mapped message and retries', async () => {
    let fail = true
    mockApi({
      ...routes('field'),
      'GET /api/reports': () => (fail ? apiError(503, 'tenant_outdated') : { body: { rows } }),
    })
    renderApp('/reports')
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('日报加载失败')
    expect(alert).toHaveTextContent('系统正在升级数据')
    fail = false
    await userEvent.click(within(alert).getByRole('button', { name: '重试' }))
    expect(await screen.findByRole('table')).toBeInTheDocument()
  })

  it('passes the job and date filters to the API', async () => {
    const { calls } = mockApi(routes('finance'))
    renderApp('/reports')
    await screen.findByRole('table')
    await userEvent.selectOptions(screen.getByRole('combobox', { name: '工地' }), 'J-24002')
    await waitFor(() =>
      expect(calls.filter((c) => c.path === '/api/reports').at(-1)?.search).toBe('?job=J-24002'),
    )
  })

  it('flags a reversed date range instead of querying', async () => {
    const { calls } = mockApi(routes('finance'))
    renderApp('/reports?from=2026-09-10&to=2026-09-01')
    expect(await screen.findByText('结束日期不能早于开始日期')).toBeInTheDocument()
    expect(calls.some((c) => c.path === '/api/reports')).toBe(false)
  })

  it.each<[Role, boolean]>([
    ['admin', true],
    ['field', true],
    ['manager', false],
    ['finance', false],
  ])('new-report button for %s: %s', async (role, visible) => {
    mockApi(routes(role))
    renderApp('/reports')
    await screen.findByRole('table')
    await waitFor(() => expect(screen.queryByRole('button', { name: '新建日报' }) !== null).toBe(visible))
  })

  it('creates a report from the dialog and opens it', async () => {
    const { calls } = mockApi({
      ...routes('field'),
      'POST /api/reports': { status: 201, body: detail({ id: 42 }) },
      'GET /api/reports/42': { body: detail({ id: 42 }) },
    })
    renderApp('/reports')
    await userEvent.click(await screen.findByRole('button', { name: '新建日报' }))
    const dialog = await screen.findByRole('dialog', { name: '新建日报' })
    await userEvent.click(within(dialog).getByRole('button', { name: '创建' }))
    expect(within(dialog).getByText('请选择工地', { selector: 'p' })).toBeInTheDocument()

    await userEvent.selectOptions(within(dialog).getByRole('combobox', { name: /工地/ }), 'J-24001')
    await userEvent.type(within(dialog).getByLabelText(/业务日期/), '2026-09-03')
    await userEvent.click(within(dialog).getByRole('button', { name: '创建' }))

    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/reports/42'))
    expect(calls.find((c) => c.method === 'POST')?.body).toEqual({ jobId: 'J-24001', businessDate: '2026-09-03' })
  })

  it('shows already_exists in the dialog', async () => {
    mockApi({ ...routes('admin'), 'POST /api/reports': apiError(409, 'already_exists') })
    renderApp('/reports')
    await userEvent.click(await screen.findByRole('button', { name: '新建日报' }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.selectOptions(within(dialog).getByRole('combobox', { name: /工地/ }), 'J-24001')
    await userEvent.type(within(dialog).getByLabelText(/业务日期/), '2026-09-01')
    await userEvent.click(within(dialog).getByRole('button', { name: '创建' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('这个工地在这一天已经有日报了')
  })
})
