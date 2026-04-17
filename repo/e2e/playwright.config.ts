import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright config for MedOps end-to-end tests.
 *
 * Tests run against the real frontend + backend + Postgres brought up by
 * docker-compose. No mocks, no stubs — every UI action drives a real API
 * request that writes to the real database.
 *
 * Environment variables:
 *   FRONTEND_URL (default: http://frontend:3000) — resolved on the compose network
 *   API_URL      (default: http://backend:8080/api/v1)  — direct API for setup/teardown
 */
const FRONTEND_URL = process.env.FRONTEND_URL || 'http://frontend:3000';

export default defineConfig({
  testDir: './tests',
  testMatch: /.*\.spec\.ts/,
  globalSetup: require.resolve('./tests/global-setup'),
  timeout: 60_000,
  expect: { timeout: 10_000 },
  // Tests share the same seeded database — run serially to avoid cross-test interference.
  fullyParallel: false,
  workers: 1,
  retries: 1,
  reporter: [['list'], ['json', { outputFile: 'test-results/results.json' }]],
  outputDir: 'test-results/artifacts',
  use: {
    baseURL: FRONTEND_URL,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
    actionTimeout: 10_000,
    navigationTimeout: 20_000,
    // Frontend uses HashRouter — paths are fragments, e.g. "/#/login"
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
});
