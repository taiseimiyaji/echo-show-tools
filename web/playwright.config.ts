import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: true,
  retries: 0,
  use: { baseURL: 'http://127.0.0.1:18081', viewport: { width: 960, height: 480 }, trace: 'retain-on-failure' },
  webServer: {
    command: 'go run ../cmd/server -demo -addr 127.0.0.1:18081 -web dist',
    url: 'http://127.0.0.1:18081/api/sessions',
    reuseExistingServer: false,
    timeout: 120000,
  },
});
