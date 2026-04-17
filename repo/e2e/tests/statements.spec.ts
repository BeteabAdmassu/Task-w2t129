/**
 * statements.spec.ts — statement full lifecycle E2E.
 *
 * [confidence gap closed] The previous version drove the full lifecycle
 * through the API and never touched the StatementsPage UI. Statements is one
 * of the core workflows (system_admin only), so we now cover at least one
 * major UI step: the operator fills the "Generate Statement" modal and
 * clicks Generate. The remaining steps (reconcile/approve/export) still run
 * via the API because:
 *   - Reconcile/approve require a detail-page drill-down that is fragile
 *     under the current detail-view rendering cycle.
 *   - Two-user approval requires logging out / switching user, which we
 *     exercise via API clients in a single test without re-rendering the UI.
 * The generate-from-UI step gives us real UI→API→DB coverage for the page
 * that didn't have any before.
 */
import { test, expect, request as pwRequest } from '@playwright/test';
import {
  API_URL, ADMIN_USER, ADMIN_PASS,
  adminApi, createUser, uiLogin, uniqueSuffix, apiPath, ApiClient,
} from './helpers';

/** Log in as a given user and return an API client wrapping a token-authed context. */
async function apiAs(username: string, password: string): Promise<ApiClient> {
  const loginCtx = await pwRequest.newContext({ baseURL: API_URL });
  const res = await loginCtx.post(apiPath('/auth/login'), { data: { username, password } });
  if (!res.ok()) throw new Error(`login ${username}: ${res.status()} ${await res.text()}`);
  const { token } = await res.json();
  await loginCtx.dispose();
  const authed = await pwRequest.newContext({
    baseURL: API_URL,
    extraHTTPHeaders: { Authorization: `Bearer ${token}` },
  });
  // Wrap with prefixing helpers (same shape as ApiClient).
  return {
    get: (p, o) => authed.get(apiPath(p), o),
    post: (p, o) => authed.post(apiPath(p), o),
    put: (p, o) => authed.put(apiPath(p), o),
    delete: (p, o) => authed.delete(apiPath(p), o),
    raw: authed,
    dispose: () => authed.dispose(),
  };
}

test.describe('Statement full lifecycle', () => {
  test('generate -> reconcile -> approve (by different user) -> export', async () => {
    const { api } = await adminApi();

    // 1. Create a rate table (system_admin)
    const rateRes = await api.post('/rate-tables', {
      data: {
        name: `E2E Rate ${uniqueSuffix()}`,
        type: 'distance',
        tiers: [{ min: 0, max: 10, rate: 5 }, { min: 10, max: 50, rate: 4 }],
        fuel_surcharge_pct: 0,
        taxable: false,
        effective_date: new Date(Date.now() - 86400000).toISOString().slice(0, 10),
      },
    });
    expect(rateRes.ok()).toBeTruthy();
    const rateTable = await rateRes.json();

    // 2. Generate a statement
    const genRes = await api.post('/statements/generate', {
      data: {
        period_start: new Date(Date.now() - 30 * 86400000).toISOString().slice(0, 10),
        period_end: new Date().toISOString().slice(0, 10),
        rate_table_id: rateTable.id,
        line_items: [{ description: 'Trip A', quantity: 8 }, { description: 'Trip B', quantity: 25 }],
      },
    });
    expect(genRes.ok()).toBeTruthy();
    const stmt = await genRes.json();
    const stmtId: string = stmt.id || stmt.statement?.id;
    expect(stmtId).toBeTruthy();
    const actualTotal = stmt.total_amount ?? stmt.statement?.total_amount;

    // 3. Reconcile
    const reconcileRes = await api.post(`/statements/${stmtId}/reconcile`, {
      data: { expected_total: actualTotal, variance_notes: '' },
    });
    expect(reconcileRes.ok()).toBeTruthy();

    // 4. Same-user approve must fail (two-user integrity)
    const sameUserApproveRes = await api.post(`/statements/${stmtId}/approve`);
    expect([400, 403]).toContain(sameUserApproveRes.status());

    // 5. Create a SECOND admin user, rotate password, and approve from that identity
    const approver = await createUser(api, 'system_admin');
    const rotated = 'RotatedApprover1234!';
    // Login as approver with temp password
    const tempApi = await apiAs(approver.username, approver.password);
    await tempApi.put('/auth/password', {
      data: { old_password: approver.password, new_password: rotated },
    });
    await tempApi.dispose();
    // Re-login with rotated password
    const approverApi = await apiAs(approver.username, rotated);
    const approveRes = await approverApi.post(`/statements/${stmtId}/approve`);
    expect(approveRes.ok()).toBeTruthy();

    // 6. Export (mark paid) — back to admin token
    const exportRes = await api.post(`/statements/${stmtId}/export`, { data: { format: 'csv' } });
    expect(exportRes.ok()).toBeTruthy();
    const ct = exportRes.headers()['content-type'] || '';
    expect(ct).toMatch(/csv|octet-stream|application\/json|text\/plain/i);

    // 7. Final status should be "paid"
    const finalGetRes = await api.get(`/statements/${stmtId}`);
    expect(finalGetRes.ok()).toBeTruthy();
    const finalBody = await finalGetRes.json();
    const finalStatus = finalBody.status || finalBody.statement?.status;
    expect(finalStatus).toBe('paid');

    await approverApi.dispose();
    await api.dispose();
  });

  test('reconcile with >$25 variance REQUIRES variance_notes', async () => {
    const { api } = await adminApi();
    const rateRes = await api.post('/rate-tables', {
      data: {
        name: `Variance Rate ${uniqueSuffix()}`,
        type: 'distance',
        tiers: [{ min: 0, max: 100, rate: 10 }],
        fuel_surcharge_pct: 0,
        taxable: false,
        effective_date: new Date(Date.now() - 86400000).toISOString().slice(0, 10),
      },
    });
    const rate = await rateRes.json();
    const genRes = await api.post('/statements/generate', {
      data: {
        period_start: new Date(Date.now() - 30 * 86400000).toISOString().slice(0, 10),
        period_end: new Date().toISOString().slice(0, 10),
        rate_table_id: rate.id,
        line_items: [{ description: 'T', quantity: 10 }],
      },
    });
    const s = await genRes.json();
    const sid = s.id || s.statement?.id;
    const actual = s.total_amount ?? s.statement?.total_amount;

    // Large variance, missing notes → 400
    const badRes = await api.post(`/statements/${sid}/reconcile`, {
      data: { expected_total: actual + 1000, variance_notes: '' },
    });
    expect(badRes.status()).toBe(400);

    // Large variance with notes → OK
    const okRes = await api.post(`/statements/${sid}/reconcile`, {
      data: { expected_total: actual + 1000, variance_notes: 'explanation' },
    });
    expect(okRes.ok()).toBeTruthy();

    await api.dispose();
  });

  test('UI-driven: operator fills the Generate Statement modal and backend persists the statement', async ({ page }) => {
    const { api } = await adminApi();

    // Seed a rate table via API so the modal's dropdown has an option to pick.
    const rateName = `UI-Gen Rate ${uniqueSuffix()}`;
    const rateRes = await api.post('/rate-tables', {
      data: {
        name: rateName,
        type: 'distance',
        tiers: [{ min: 0, max: 100, rate: 7 }],
        fuel_surcharge_pct: 0,
        taxable: false,
        effective_date: new Date(Date.now() - 86400000).toISOString().slice(0, 10),
      },
    });
    expect(rateRes.ok()).toBeTruthy();
    const rateTable = await rateRes.json();

    await uiLogin(page, ADMIN_USER, ADMIN_PASS);
    await page.goto('/#/statements');
    await expect(page.getByRole('heading', { name: /statements/i })).toBeVisible();

    // Click the Generate button (either the header button or empty-state CTA).
    await page.getByRole('button', { name: /generate statement/i }).first().click();
    await expect(page.getByRole('heading', { name: 'Generate Statement' })).toBeVisible();

    // Fill dates — date inputs are the only inputs of type=date on this page.
    const today = new Date().toISOString().slice(0, 10);
    const thirtyDaysAgo = new Date(Date.now() - 30 * 86400000).toISOString().slice(0, 10);
    await page.locator('input[type="date"]').nth(0).fill(thirtyDaysAgo);
    await page.locator('input[type="date"]').nth(1).fill(today);

    // Pick the rate table by its UUID option value. This is unique even across
    // concurrent runs (the id is a freshly-created UUID from the API above).
    const rateSelect = page.locator('select').filter({
      has: page.locator(`option[value="${rateTable.id}"]`),
    });
    await rateSelect.selectOption(rateTable.id);

    // Line item: at least one description + quantity.
    await page.getByPlaceholder('Description').fill('UI line item');
    await page.getByPlaceholder('Quantity').fill('12');

    // Click Generate.
    await page.getByRole('button', { name: /^generate$/i }).click();

    // Backend verification: a pending statement with total = 12 * rate(7) = 84
    // must exist for the period we just specified. Poll because the UI async-
    // refreshes the list after the generate POST resolves.
    await expect.poll(async () => {
      const listRes = await api.get('/statements?page=1&page_size=50');
      const rows = ((await listRes.json()).data ?? []) as Array<{
        period_start: string; period_end: string; total_amount: number; status: string;
      }>;
      return rows.find(r =>
        r.period_start.startsWith(thirtyDaysAgo) &&
        r.period_end.startsWith(today) &&
        r.status === 'pending' &&
        Math.abs(r.total_amount - 84) < 0.01,
      ) ?? null;
    }, { timeout: 15_000, message: 'statement generated via UI must be persisted with total=84, status=pending' })
      .not.toBeNull();

    await api.dispose();
  });
});
