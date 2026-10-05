import { expect, type APIRequestContext, type Browser, type Page } from '@playwright/test'

export type Role = 'admin' | 'manager' | 'field' | 'finance'

/** Tenant whose test accounts are used: dlpv1 by default (a preview slot), dev for docker compose. */
export const tenant = process.env.E2E_TENANT || 'dlpv1'
const password = process.env.E2E_PASSWORD || ''

export const email = (role: Role) => `${role}@${tenant}.test`

/** Golden snapshot facts (api/testdata/golden): its reports all fall in this range. */
export const golden = { from: '2026-09-01', to: '2026-09-10', reports: 30, late: 6 }

function requirePassword(): string {
  if (!password) throw new Error('E2E_PASSWORD 未设置：测试账号的密码（与 TENANT_TEST_PASSWORD 相同）')
  return password
}

/** Logs in through the login page and waits for the register to show. */
export async function login(page: Page, role: Role) {
  await page.goto('/login')
  await page.getByLabel('邮箱').fill(email(role))
  await page.getByLabel('密码').fill(requirePassword())
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page.getByRole('heading', { name: '日报台账' })).toBeVisible()
}

/** A fresh browser context logged in as `role`; closed by the caller via page.context().close(). */
export async function pageAs(browser: Browser, role: Role): Promise<Page> {
  const context = await browser.newContext()
  const page = await context.newPage()
  await login(page, role)
  return page
}

/**
 * A business date no snapshot report and, in practice, no earlier run uses: a random day in
 * years 2100–9999. Write tests create their own reports on such a date so that they can be
 * re-run against the same tenant without a reset, and the golden range stays untouched.
 */
export function uniqueDate(): string {
  const start = Date.UTC(2100, 0, 1)
  const end = Date.UTC(9999, 11, 31)
  const day = 86_400_000
  const t = start + Math.floor(Math.random() * ((end - start) / day)) * day
  return new Date(t).toISOString().slice(0, 10)
}

/** Creates a report with one crew entry through the API as the field user; returns its id. */
export async function createReportViaAPI(request: APIRequestContext, jobId: string): Promise<number> {
  const res = await request.post('/api/login', { data: { email: email('field'), password: requirePassword() } })
  expect(res.ok(), await res.text()).toBe(true)
  const created = await request.post('/api/reports', { data: { jobId, businessDate: uniqueDate() } })
  expect(created.status(), await created.text()).toBe(201)
  const { id } = (await created.json()) as { id: number }
  const crew = await request.put(`/api/reports/${id}/crew`, { data: { crew: [{ worker: '冒烟工人', hours: 6 }] } })
  expect(crew.ok(), await crew.text()).toBe(true)
  return id
}
