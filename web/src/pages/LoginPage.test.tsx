import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { apiError, jobs, meAs, mockApi, renderApp } from '@/test/utils'

describe('LoginPage', () => {
  it('redirects an anonymous visitor to /login and back after signing in', async () => {
    let signedIn = false
    const { calls } = mockApi({
      'GET /api/me': () => (signedIn ? { body: meAs('field') } : apiError(401, 'unauthenticated')),
      'POST /api/login': () => {
        signedIn = true
        return { body: meAs('field') }
      },
      'GET /api/jobs': { body: { jobs } },
      'GET /api/reports': { body: { rows: [] } },
    })
    renderApp('/reports?job=J-24001')

    expect(await screen.findByRole('heading', { name: '施工日报' })).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/login')

    await userEvent.type(screen.getByRole('textbox', { name: /邮箱/ }), 'field@dlpv1.test')
    await userEvent.type(screen.getByLabelText(/密码/), 'secret')
    await userEvent.click(screen.getByRole('button', { name: '登录' }))

    expect(await screen.findByRole('heading', { name: '日报台账' })).toBeInTheDocument()
    expect(screen.getByTestId('location')).toHaveTextContent('/reports?job=J-24001')
    expect(calls.find((c) => c.path === '/api/login')?.body).toEqual({ email: 'field@dlpv1.test', password: 'secret' })
  })

  it('shows the Chinese message for bad_credentials', async () => {
    mockApi({
      'GET /api/me': apiError(401, 'unauthenticated'),
      'POST /api/login': apiError(401, 'bad_credentials'),
    })
    renderApp('/login')
    await userEvent.type(await screen.findByRole('textbox', { name: /邮箱/ }), 'x@y.test')
    await userEvent.type(screen.getByLabelText(/密码/), 'wrong')
    await userEvent.click(screen.getByRole('button', { name: '登录' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('邮箱或密码错误')
  })

  it('validates empty fields without calling the API', async () => {
    const { calls } = mockApi({ 'GET /api/me': apiError(401, 'unauthenticated') })
    renderApp('/login')
    await userEvent.click(await screen.findByRole('button', { name: '登录' }))
    expect(screen.getByText('请输入邮箱')).toBeInTheDocument()
    expect(screen.getByText('请输入密码')).toBeInTheDocument()
    expect(calls.some((c) => c.path === '/api/login')).toBe(false)
  })

  it('shows a loading state while the session is checked', () => {
    mockApi({})
    // fetch never resolves within this synchronous assertion
    renderApp('/login')
    expect(screen.getByRole('status')).toHaveTextContent('正在确认登录状态')
  })

  it('sends a signed-in user straight to the register', async () => {
    mockApi({
      'GET /api/me': { body: meAs('admin') },
      'GET /api/jobs': { body: { jobs } },
      'GET /api/reports': { body: { rows: [] } },
    })
    renderApp('/login')
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/reports'))
  })
})
