import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  reporter: [['line'], ['html', { outputFolder: '../artifacts/playwright-report', open: 'never' }]],
  use: {
    baseURL: process.env.SANTA_TEST_URL || 'http://127.0.0.1:5080',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure'
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }]
});
