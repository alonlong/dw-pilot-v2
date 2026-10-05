import { defineConfig, devices } from '@playwright/test'

// Runs against an already started server (docker compose, a preview slot, or a local
// `go run ./api/cmd/server`); it does not start one itself. Reporters are chosen on the
// command line (`--reporter=list,html` in CI), so none is fixed here.
export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  timeout: 30_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: process.env.E2E_BASE_URL || 'http://localhost:8080',
    locale: 'zh-CN',
    timezoneId: 'Asia/Shanghai',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
