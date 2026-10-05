import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import type { Role } from '@/api'
import { apiError, jobs, meAs, mockApi, renderApp } from '@/test/utils'

const base = {
  'GET /api/jobs': { body: { jobs } },
  'GET /api/reports': { body: { rows: [] } },
}

describe('Layout', () => {
  it('shows user name, role, tenant and a logout button', async () => {
    mockApi({ ...base, 'GET /api/me': { body: meAs('manager') } })
    renderApp('/reports')
    const banner = await screen.findByRole('banner')
    expect(within(banner).getByText('测试manager')).toBeInTheDocument()
    expect(within(banner).getByText('项目经理')).toBeInTheDocument()
    expect(within(banner).getByText('试点租户一')).toBeInTheDocument()
    expect(within(banner).getByRole('button', { name: '登出' })).toBeInTheDocument()
  })

  it.each<[Role, boolean]>([
    ['admin', true],
    ['manager', true],
    ['finance', true],
    ['field', false],
  ])('summary nav for %s: %s', async (role, visible) => {
    mockApi({ ...base, 'GET /api/me': { body: meAs(role) } })
    renderApp('/reports')
    const nav = await screen.findByRole('navigation', { name: '主导航' })
    expect(within(nav).getByRole('link', { name: '日报台账' })).toBeInTheDocument()
    expect(within(nav).queryByRole('link', { name: '工时汇总' }) !== null).toBe(visible)
  })

  it('logs out and returns to /login', async () => {
    let signedIn = true
    const { calls } = mockApi({
      ...base,
      'GET /api/me': () => (signedIn ? { body: meAs('field') } : apiError(401, 'unauthenticated')),
      'POST /api/logout': () => {
        signedIn = false
        return { status: 204 }
      },
    })
    renderApp('/reports')
    await userEvent.click(await screen.findByRole('button', { name: '登出' }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/login'))
    expect(calls.some((c) => c.method === 'POST' && c.path === '/api/logout')).toBe(true)
  })

  it('sends the user to /login when a call reports the session expired', async () => {
    let expired = false
    mockApi({
      'GET /api/me': () => (expired ? apiError(401, 'unauthenticated') : { body: meAs('field') }),
      'GET /api/jobs': { body: { jobs } },
      'GET /api/reports': () => {
        expired = true
        return apiError(401, 'unauthenticated')
      },
    })
    renderApp('/reports')
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/login'))
  })

  it('shows an error with retry when the session check fails', async () => {
    mockApi({ 'GET /api/me': apiError(503, 'tenant_outdated') })
    renderApp('/reports')
    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('系统正在升级数据')
    expect(within(alert).getByRole('button', { name: '重试' })).toBeInTheDocument()
  })
})
