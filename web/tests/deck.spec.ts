import { test, expect } from '@playwright/test';

const session = { id: 'term_fixture', repo: 'demo-repo', title: 'API task', agent: 'codex', status: 'running', connected: true, writable: true, lastMessage: '作業を進めています', lastActivityAt: 1788686645990 };
const snapshot = (sessions = [session], partial = false) => ({ sessions, meta: { partial, demo: true, fetchedAt: 1788686645990 } });

test('production build serves the real demo API and refreshes', async ({ page }, testInfo) => {
  const errors: string[] = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Agent Deck' })).toBeVisible();
  await expect(page.getByRole('article')).toHaveCount(2);
  await expect(page.getByText('Running', { exact: true })).toBeVisible();
  await expect(page.getByText('Done', { exact: true })).toBeVisible();
  await page.getByRole('checkbox').check();
  await expect(page.getByRole('article')).toHaveCount(3);
  await page.getByRole('checkbox').uncheck();
  const response = page.waitForResponse('/api/sessions');
  await page.getByRole('button', { name: 'Refresh' }).click();
  expect((await response).ok()).toBeTruthy();
  await expect(page.getByRole('button', { name: 'Refresh' })).toBeEnabled();
  await page.screenshot({ path: testInfo.outputPath('deck-960x480.png'), fullPage: true });
  expect(errors).toEqual([]);
});

test('empty and unknown-only states are distinct', async ({ page }) => {
  let data = snapshot([]);
  await page.route('**/api/sessions', route => route.fulfill({ json: data }));
  await page.goto('/');
  await expect(page.getByText('表示するエージェントはありません')).toBeVisible();
  data = snapshot([{ ...session, agent: 'unknown', lastMessage: '' }]);
  await page.getByRole('button', { name: 'Refresh' }).click();
  await expect(page.getByText('不明なTerminalは下の切り替えで確認できます。')).toBeVisible();
  await page.getByRole('checkbox').check();
  await expect(page.getByRole('article')).toHaveCount(1);
});

test('failed refresh marks old data and recovers to replacement handles', async ({ page }) => {
  let fail = false;
  let current = snapshot();
  await page.route('**/api/sessions', route => fail ? route.fulfill({ status: 503, json: { error: { code: 'orca_unavailable' } } }) : route.fulfill({ json: current }));
  await page.goto('/');
  await expect(page.getByRole('article')).toHaveCount(1);
  fail = true;
  await page.getByRole('button', { name: 'Refresh' }).click();
  await expect(page.getByRole('alert')).toContainText('情報が古い可能性');
  await expect(page.getByRole('article')).toHaveCount(1);
  fail = false;
  current = snapshot([{ ...session, id: 'term_restarted', title: 'Replacement task' }]);
  await page.getByRole('button', { name: 'Refresh' }).click();
  await expect(page.getByRole('alert')).toHaveCount(0);
  await expect(page.getByRole('heading', { name: 'Replacement task' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'API task', exact: true })).toHaveCount(0);
});

test('initial failure is not shown as an empty successful result', async ({ page }) => {
  await page.route('**/api/sessions', route => route.fulfill({ status: 504, json: { error: { code: 'timeout' } } }));
  await page.goto('/');
  await expect(page.getByRole('alert')).toContainText('タイムアウト');
  await expect(page.getByText('表示するエージェントはありません')).toHaveCount(0);
});

test('loading prevents duplicate requests and polling fetches again', async ({ page }) => {
  let release!: () => void;
  const ready = new Promise<void>(resolve => { release = resolve; });
  let count = 0;
  await page.route('**/api/sessions', async route => { count++; if (count === 1) await ready; await route.fulfill({ json: snapshot() }); });
  await page.goto('/');
  await expect(page.getByText('セッションを取得しています…')).toBeVisible();
  await expect(page.getByRole('button', { name: '更新中' })).toBeDisabled();
  expect(count).toBe(1);
  release();
  await expect(page.getByRole('article')).toHaveCount(1);
  await expect.poll(() => count, { timeout: 8000 }).toBeGreaterThan(1);
});

test('partial data and HTML output are rendered safely', async ({ page }) => {
  const message = '<img src=x onerror="window.hacked=true">';
  await page.route('**/api/sessions', route => route.fulfill({ json: snapshot([{ ...session, lastMessage: message }], true) }));
  await page.goto('/');
  await expect(page.getByRole('status')).toContainText('一部のホスト');
  await expect(page.getByText(message, { exact: true })).toBeVisible();
  await expect(page.locator('article img')).toHaveCount(0);
});

for (const viewport of [{ width: 960, height: 360 }, { width: 375, height: 667 }]) {
  test(`many sessions fit ${viewport.width}x${viewport.height} without horizontal overflow`, async ({ page }, testInfo) => {
    await page.setViewportSize(viewport);
    await page.route('**/api/sessions', route => route.fulfill({ json: snapshot(Array.from({ length: 12 }, (_, i) => ({ ...session, id: `term_${i}`, title: `Long session title ${i} ` + '詳細'.repeat(30) }))) }));
    await page.goto('/');
    await expect(page.getByRole('article')).toHaveCount(12);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
    await page.getByRole('article').last().scrollIntoViewIfNeeded();
    await expect(page.getByRole('button', { name: 'Refresh' })).toBeInViewport();
    await page.screenshot({ path: testInfo.outputPath(`deck-${viewport.width}x${viewport.height}.png`), fullPage: false });
  });
}
