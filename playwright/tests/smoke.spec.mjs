import { test, expect } from '@playwright/test';

test('home page is navigable by semantic controls', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/Санта|Santa/i);
  await expect(page.getByRole('main')).toBeVisible();
  const links = page.getByRole('link');
  await expect(links.first()).toBeVisible();
  await page.keyboard.press('Tab');
  await expect(page.locator(':focus')).toBeVisible();
});

test('mobile page has no horizontal overflow', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto('/');
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
});
