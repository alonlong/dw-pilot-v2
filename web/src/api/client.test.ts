import { describe, expect, it, vi } from 'vitest'
import { api, ApiError } from './client'
import { errorMessage } from './errors'

function respond(status: number, body: string | null) {
  const fetchMock = vi.fn(async () => new Response(body, { status }))
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

describe('api client', () => {
  it('sends credentials and JSON, and builds query strings from set params only', async () => {
    const fetchMock = respond(200, '{"rows":[]}')
    await api.reports({ job: 'J-1', from: '', to: '2026-09-30' })
    const [url, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(url).toBe('/api/reports?job=J-1&to=2026-09-30')
    expect(init.credentials).toBe('include')
  })

  it('posts a JSON body', async () => {
    const fetchMock = respond(201, '{"id":1}')
    await api.createReport('J-1', '2026-09-01')
    const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit]
    expect(init.method).toBe('POST')
    expect(JSON.parse(init.body as string)).toEqual({ jobId: 'J-1', businessDate: '2026-09-01' })
    expect((init.headers as Record<string, string>)['Content-Type']).toBe('application/json')
  })

  it('returns undefined for 204', async () => {
    respond(204, null)
    await expect(api.logout()).resolves.toBeUndefined()
  })

  it('turns the error body into an ApiError', async () => {
    respond(422, '{"error":{"code":"negative_hours","message":"工时不能为负数：张三=-1"}}')
    const err = await api.updateCrew(1, []).catch((e: unknown) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 422, code: 'negative_hours' })
    expect(errorMessage(err)).toBe('工时不能为负数。')
  })

  it('maps a non-JSON failure to unknown and a network failure to network', async () => {
    respond(502, '<html>Bad Gateway</html>')
    await expect(api.me()).rejects.toMatchObject({ code: 'unknown', status: 502 })
    vi.stubGlobal('fetch', vi.fn(async () => Promise.reject(new TypeError('Failed to fetch'))))
    const err = await api.me().catch((e: unknown) => e)
    expect(err).toMatchObject({ code: 'network', status: 0 })
    expect(errorMessage(err)).toContain('无法连接服务器')
  })
})
