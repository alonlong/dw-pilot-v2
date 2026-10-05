import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { ReportDetail, Role } from '@/api'
import { apiError, detail, meAs, mockApi, renderApp } from '@/test/utils'

const routes = (role: Role, report: ReportDetail = detail()) => ({
  'GET /api/me': { body: meAs(role) },
  'GET /api/reports/7': { body: report },
})

const notes = [
  { id: 2, body: '下午下雨，停工两小时', authorName: '王五', createdAt: '2026-09-02T09:00:00Z' },
  { id: 1, body: '钢筋到场', authorName: '赵六', createdAt: '2026-09-02T01:00:00Z' },
]

describe('ReportDetailPage', () => {
  it('renders the header, crew and notes (newest first)', async () => {
    mockApi(routes('field', detail({ notes, daysLate: 1, status: '已提交', submittedAt: '2026-09-03T10:00:00Z' })))
    renderApp('/reports/7')
    expect(await screen.findByRole('heading', { level: 1, name: '滨江一号 · 2026-09-02' })).toBeInTheDocument()
    expect(screen.getByText('已提交')).toBeInTheDocument()
    expect(screen.getByText('迟交 1 天')).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: '第 1 行工人' })).toHaveValue('张三')
    const items = within(screen.getByRole('list', { name: /备注列表/ })).getAllByRole('listitem')
    expect(items.map((li) => li.querySelector('p')?.textContent)).toEqual(['下午下雨，停工两小时', '钢筋到场'])
  })

  it('shows a loading state', async () => {
    mockApi({ ...routes('field'), 'GET /api/reports/7': () => new Promise(() => {}) as never })
    renderApp('/reports/7')
    expect(await screen.findByText('正在加载日报')).toBeInTheDocument()
  })

  it('shows empty crew and empty notes states read-only for finance', async () => {
    mockApi(
      routes('finance', detail({ crew: [], totalHours: 0, can: { editCrew: false, submit: false, lock: false, addNote: false } })),
    )
    renderApp('/reports/7')
    expect(await screen.findByText('没有工时记录')).toBeInTheDocument()
    expect(screen.getByText('还没有备注')).toBeInTheDocument()
    expect(screen.getByText(/你的角色（财务）只能查看工时/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '保存工时' })).toBeNull()
    expect(screen.queryByRole('button', { name: '追加' })).toBeNull()
    expect(screen.queryByRole('button', { name: '提交日报' })).toBeNull()
    expect(screen.queryByRole('button', { name: '锁定' })).toBeNull()
  })

  it('shows not_found as its own state', async () => {
    mockApi({ ...routes('field'), 'GET /api/reports/7': apiError(404, 'not_found') })
    renderApp('/reports/7')
    expect(await screen.findByText('找不到这份日报')).toBeInTheDocument()
  })

  it('shows other errors with the mapped message', async () => {
    mockApi({ ...routes('field'), 'GET /api/reports/7': apiError(403, 'forbidden') })
    renderApp('/reports/7')
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('日报加载失败')
    expect(alert).toHaveTextContent('你的角色没有权限执行这个操作')
  })

  it('shows actions according to can', async () => {
    mockApi(routes('manager', detail({ can: { editCrew: false, submit: false, lock: true, addNote: true } })))
    renderApp('/reports/7')
    expect(await screen.findByRole('button', { name: '锁定' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '提交日报' })).toBeNull()
    expect(screen.getByRole('button', { name: '追加' })).toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: '第 1 行工人' })).toBeNull()
  })

  it('is read-only when locked and says why, but still allows notes', async () => {
    mockApi(
      routes(
        'admin',
        detail({ status: '已锁定', locked: true, can: { editCrew: false, submit: false, lock: false, addNote: true } }),
      ),
    )
    renderApp('/reports/7')
    expect(await screen.findByText('只读：已锁定')).toBeInTheDocument()
    expect(screen.getByText(/任何角色都不能再修改工时或提交/)).toBeInTheDocument()
    expect(screen.getByRole('table', { name: '工人工时' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '保存工时' })).toBeNull()
    expect(screen.getByRole('button', { name: '追加' })).toBeInTheDocument()
  })

  it('saves crew edits and validates negative hours locally', async () => {
    const saved = detail({ crew: [{ worker: '张三', hours: 9 }, { worker: '李四', hours: 8 }], totalHours: 17 })
    const { calls } = mockApi({ ...routes('field'), 'PUT /api/reports/7/crew': { body: saved } })
    renderApp('/reports/7')
    const hours = await screen.findByRole('spinbutton', { name: '第 1 行工时' })

    await userEvent.clear(hours)
    await userEvent.type(hours, '-1')
    await userEvent.click(screen.getByRole('button', { name: '保存工时' }))
    expect(screen.getByText('工时不能为负数')).toBeInTheDocument()
    expect(calls.some((c) => c.method === 'PUT')).toBe(false)

    await userEvent.clear(hours)
    await userEvent.type(hours, '9')
    await userEvent.click(screen.getByRole('button', { name: '保存工时' }))
    await waitFor(() => expect(screen.getByText('17')).toBeInTheDocument())
    expect(calls.find((c) => c.method === 'PUT')?.body).toEqual({ crew: saved.crew })
  })

  it('shows the server message when saving crew fails', async () => {
    mockApi({ ...routes('field'), 'PUT /api/reports/7/crew': apiError(409, 'locked') })
    renderApp('/reports/7')
    await userEvent.click(await screen.findByRole('button', { name: '添加工人' }))
    await userEvent.type(screen.getByRole('textbox', { name: '第 3 行工人' }), '王五')
    await userEvent.type(screen.getByRole('spinbutton', { name: '第 3 行工时' }), '4')
    await userEvent.click(screen.getByRole('button', { name: '保存工时' }))
    expect(await screen.findByText('这份日报已锁定，不能再修改工时或提交。')).toBeInTheDocument()
  })

  it('submits after confirmation', async () => {
    const { calls } = mockApi({
      ...routes('field'),
      'POST /api/reports/7/submit': {
        body: detail({ status: '已提交', submittedAt: '2026-09-02T10:00:00Z', can: { editCrew: true, submit: false, lock: false, addNote: true } }),
      },
    })
    renderApp('/reports/7')
    await userEvent.click(await screen.findByRole('button', { name: '提交日报' }))
    const dialog = await screen.findByRole('dialog', { name: '提交这份日报？' })
    await userEvent.click(within(dialog).getByRole('button', { name: '确认提交' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
    expect(screen.getByText('已提交')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '提交日报' })).toBeNull()
    expect(calls.some((c) => c.method === 'POST' && c.path === '/api/reports/7/submit')).toBe(true)
  })

  it('shows already_submitted inside the confirm dialog', async () => {
    mockApi({ ...routes('field'), 'POST /api/reports/7/submit': apiError(409, 'already_submitted') })
    renderApp('/reports/7')
    await userEvent.click(await screen.findByRole('button', { name: '提交日报' }))
    const dialog = await screen.findByRole('dialog')
    await userEvent.click(within(dialog).getByRole('button', { name: '确认提交' }))
    expect(await within(dialog).findByRole('alert')).toHaveTextContent('已经提交过')
  })

  it('appends a note and checks its length', async () => {
    const { calls } = mockApi({
      ...routes('field'),
      'POST /api/reports/7/notes': { status: 201, body: detail({ notes: [notes[0]!] }) },
    })
    renderApp('/reports/7')
    const box = await screen.findByRole('textbox', { name: '追加备注' })
    await userEvent.click(screen.getByRole('button', { name: '追加' }))
    expect(screen.getByText('备注不能为空')).toBeInTheDocument()

    await userEvent.type(box, '下午下雨，停工两小时')
    await userEvent.click(screen.getByRole('button', { name: '追加' }))
    expect(await screen.findByText('下午下雨，停工两小时', { selector: 'li p' })).toBeInTheDocument()
    expect(box).toHaveValue('')
    expect(calls.find((c) => c.path === '/api/reports/7/notes')?.body).toEqual({ body: '下午下雨，停工两小时' })
  })
})
