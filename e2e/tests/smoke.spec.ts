import { expect, test, type Page } from '@playwright/test'
import { createReportViaAPI, email, golden, login, pageAs, uniqueDate } from './helpers'

const nav = (page: Page) => page.getByRole('navigation', { name: '主导航' })

test('SMOKE-1 项目经理登录后看到台账和汇总入口', async ({ page }) => {
  await login(page, 'manager')
  await expect(page).toHaveURL(/\/reports$/)
  await expect(page.getByRole('banner')).toContainText('项目经理')
  await expect(nav(page).getByRole('link', { name: '工时汇总' })).toBeVisible()
  // Managers lock reports; they do not create them.
  await expect(page.getByRole('button', { name: '新建日报' })).toHaveCount(0)
})

test('SMOKE-2 现场登录后可以新建日报，没有汇总入口', async ({ page }) => {
  await login(page, 'field')
  await expect(page.getByRole('banner')).toContainText('现场')
  await expect(page.getByRole('button', { name: '新建日报' })).toBeVisible()
  await expect(nav(page).getByRole('link', { name: '工时汇总' })).toHaveCount(0)
})

test('SMOKE-3 台账显示快照的 30 份日报和迟交标记', async ({ page }) => {
  await login(page, 'manager')
  // Limit to the snapshot's dates: write tests add reports outside this range.
  await page.goto(`/reports?from=${golden.from}&to=${golden.to}`)
  const table = page.getByRole('table', { name: `日报台账，共 ${golden.reports} 份` })
  await expect(table).toBeVisible()
  // One header row plus one row per report.
  await expect(table.getByRole('row')).toHaveCount(golden.reports + 1)
  await expect(table.getByText(/^迟交 \d+ 天$/)).toHaveCount(golden.late)
  const lateRow = table.getByRole('row').filter({ has: page.getByRole('link', { name: '查看 北岸物流仓库（虚构） 2026-09-04 的日报' }) })
  await expect(lateRow.getByText('迟交 3 天')).toBeVisible()
})

test('SMOKE-4 从台账打开日报详情', async ({ page }) => {
  await login(page, 'finance')
  await page.goto(`/reports?from=${golden.from}&to=${golden.to}`)
  await page.getByRole('link', { name: '查看 青石社区卫生站（虚构） 2026-09-02 的日报' }).click()
  await expect(page).toHaveURL(/\/reports\/\d+$/)
  await expect(page.getByRole('heading', { name: '青石社区卫生站（虚构） · 2026-09-02' })).toBeVisible()
  await expect(page.getByText('迟交 2 天')).toBeVisible()
  await expect(page.getByRole('heading', { name: '工时' })).toBeVisible()
  await expect(page.getByRole('heading', { name: '备注' })).toBeVisible()
  // Finance is read-only.
  await expect(page.getByRole('status').filter({ hasText: '只读' })).toBeVisible()
  await expect(page.getByRole('button', { name: '保存工时' })).toHaveCount(0)
})

test('SMOKE-5 现场新建日报、填写工时并提交', async ({ page }) => {
  const date = uniqueDate()
  await login(page, 'field')

  await page.getByRole('button', { name: '新建日报' }).click()
  const dialog = page.getByRole('dialog', { name: '新建日报' })
  await dialog.getByLabel('工地').selectOption('J-24001')
  await dialog.getByLabel('业务日期').fill(date)
  await dialog.getByRole('button', { name: '创建' }).click()

  await expect(page).toHaveURL(/\/reports\/\d+$/)
  await expect(page.getByRole('heading', { name: `滨江示范小学扩建（虚构） · ${date}` })).toBeVisible()
  await expect(page.getByRole('main').getByText('未提交', { exact: true })).toBeVisible()

  const crew = page.getByRole('form', { name: '编辑工时' })
  await crew.getByRole('button', { name: '添加工人' }).click()
  await crew.getByLabel('第 1 行工人').fill('冒烟测试员')
  await crew.getByLabel('第 1 行工时').fill('7.5')
  await crew.getByRole('button', { name: '保存工时' }).click()
  await expect(page.getByRole('region', { name: '工时' })).toContainText('合计 7.5 小时')

  await page.getByRole('button', { name: '提交日报' }).click()
  const confirm = page.getByRole('dialog', { name: '提交这份日报？' })
  await confirm.getByRole('button', { name: '确认提交' }).click()
  await expect(confirm).toHaveCount(0)
  await expect(page.getByRole('main').getByText('已提交', { exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: '提交日报' })).toHaveCount(0)
})

test('SMOKE-6 现场访问汇总页显示无权限', async ({ page }) => {
  await login(page, 'field')
  await page.goto('/summary')
  await expect(page.getByRole('heading', { name: '工时汇总' })).toBeVisible()
  await expect(page.getByRole('status').filter({ hasText: '无权限' })).toBeVisible()
  await expect(page.getByRole('table')).toHaveCount(0)
})

test('SMOKE-7 项目经理锁定日报后，日报对所有角色只读', async ({ browser, request }) => {
  const id = await createReportViaAPI(request, 'J-24002')

  const manager = await pageAs(browser, 'manager')
  await manager.goto(`/reports/${id}`)
  await manager.getByRole('button', { name: '锁定' }).click()
  const confirm = manager.getByRole('dialog', { name: '锁定这份日报？' })
  await confirm.getByRole('button', { name: '确认锁定' }).click()
  await expect(confirm).toHaveCount(0)
  await expect(manager.getByRole('status').filter({ hasText: '只读：已锁定' })).toBeVisible()
  await expect(manager.getByRole('main').getByText('已锁定', { exact: true })).toBeVisible()
  await expect(manager.getByRole('button', { name: '锁定' })).toHaveCount(0)
  await manager.context().close()

  // The field user who could edit it before now sees a read-only report.
  const field = await pageAs(browser, 'field')
  await field.goto(`/reports/${id}`)
  await expect(field.getByRole('status').filter({ hasText: '只读：已锁定' })).toBeVisible()
  await expect(field.getByRole('table', { name: '工人工时' })).toContainText('冒烟工人')
  await expect(field.getByRole('form', { name: '编辑工时' })).toHaveCount(0)
  await expect(field.getByRole('button', { name: '提交日报' })).toHaveCount(0)
  // Notes stay open after locking.
  await expect(field.getByLabel('追加备注')).toBeVisible()
  await field.context().close()
})

test('SMOKE-8 登录失败显示错误', async ({ page }) => {
  await page.goto('/login')
  await page.getByLabel('邮箱').fill(email('field'))
  await page.getByLabel('密码').fill('wrong-password')
  await page.getByRole('button', { name: '登录' }).click()
  await expect(page.getByRole('alert').filter({ hasText: '登录失败' })).toBeVisible()
  await expect(page).toHaveURL(/\/login$/)
})
