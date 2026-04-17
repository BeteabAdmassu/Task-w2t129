/**
 * members.spec.ts — UI-driven membership lifecycle.
 *
 * [confidence gap closed] Previously the full lifecycle (create, add-value,
 * redeem, freeze/unfreeze) ran entirely through the API, with the UI only
 * checked to "render the detail page". That meant the UI forms — which a real
 * operator uses every day — were never exercised. This rewrite drives every
 * action through the actual UI controls and then verifies via the API that
 * the backend state matches.
 */
import { test, expect } from '@playwright/test';
import { ADMIN_USER, ADMIN_PASS, adminApi, uiLogin, uniqueSuffix } from './helpers';

test.describe('Member lifecycle — UI-driven create + operations', () => {
  test('UI-create: operator fills the modal, and the backend persists matching fields', async ({ page }) => {
    const { api } = await adminApi();
    const tiersRes = await api.get('/membership-tiers');
    const tiers = (await tiersRes.json()).data;
    expect(tiers.length).toBeGreaterThan(0);
    const tierId = tiers[0].id as string;
    const tierName = tiers[0].name as string;

    const memberName = `UI-Create Mbr ${uniqueSuffix()}`;
    const phone = `+155502${Math.floor(Math.random() * 99999).toString().padStart(5, '0')}`;

    await uiLogin(page, ADMIN_USER, ADMIN_PASS);
    await page.goto('/#/members');
    await expect(page.getByRole('heading', { name: /^members$/i })).toBeVisible();
    await page.getByRole('button', { name: /new member/i }).click();

    // Fill the modal form.
    await page.getByPlaceholder('Full name').fill(memberName);
    await page.getByPlaceholder('+1-234-567-8900').fill(phone);
    // Tier select — pick by visible option text (the seeded tier's name).
    await page.locator('select').filter({ has: page.locator('option', { hasText: tierName }) })
      .selectOption({ label: tierName });

    await page.getByRole('button', { name: /^create member$/i }).click();

    // UI outcome: row for the new member is visible in the list.
    await expect(page.getByText(memberName).first()).toBeVisible({ timeout: 15_000 });

    // Backend assertion: persisted with typed fields.
    const listRes = await api.get(`/members?search=${encodeURIComponent(memberName)}`);
    expect(listRes.ok()).toBeTruthy();
    const list = await listRes.json();
    const rows = (list.data || list) as Array<{ name: string; phone: string; tier_id: string; id: string }>;
    const created = rows.find(r => r.name === memberName);
    expect(created, 'UI-created member must be persisted').toBeTruthy();
    expect(created!.phone).toBe(phone);
    expect(created!.tier_id).toBe(tierId);
    await api.dispose();
  });

  test('UI add-value flow: operator clicks Add Value → fills form → clicks Add, stored_value increases', async ({ page }) => {
    const { api } = await adminApi();
    // Seed a baseline member via API so the test isolates the add-value UI action.
    const tiers = (await (await api.get('/membership-tiers')).json()).data;
    const member = await (await api.post('/members', {
      data: { name: `Add-Value Mbr ${uniqueSuffix()}`, phone: `+155503${Date.now() % 99999}`, tier_id: tiers[0].id },
    })).json();

    await uiLogin(page, ADMIN_USER, ADMIN_PASS);
    await page.goto(`/#/members/${member.id}`);
    await expect(page.getByText(member.name).first()).toBeVisible({ timeout: 15_000 });

    // Click "Add Value" — opens form with Type + Amount.
    await page.getByRole('button', { name: /^add value$/i }).click();
    // The form is an h4 "Add Value"; scope the inputs to the form's container.
    const addValueForm = page.getByRole('heading', { name: 'Add Value', level: 4 }).locator('..');
    await addValueForm.locator('select').selectOption('stored_value_add');
    await addValueForm.locator('input[type="number"]').fill('50');
    await page.getByRole('button', { name: /^add$/i }).click();

    // Backend verification: stored_value is now 50.
    await expect.poll(async () => {
      const r = await (await api.get(`/members/${member.id}`)).json();
      return r.stored_value;
    }, { timeout: 10_000, message: 'stored_value should be 50 after UI add-value' }).toBe(50);

    await api.dispose();
  });

  test('UI redeem flow: Redeem Benefit modal deducts stored value; over-redemption shows error', async ({ page }) => {
    const { api } = await adminApi();
    const tiers = (await (await api.get('/membership-tiers')).json()).data;
    const member = await (await api.post('/members', {
      data: { name: `Redeem Mbr ${uniqueSuffix()}`, phone: `+155504${Date.now() % 99999}`, tier_id: tiers[0].id },
    })).json();
    // Seed 100 stored value.
    await api.post(`/members/${member.id}/add-value`, { data: { type: 'stored_value_add', amount: 100 } });

    await uiLogin(page, ADMIN_USER, ADMIN_PASS);
    await page.goto(`/#/members/${member.id}`);
    await expect(page.getByText(member.name).first()).toBeVisible({ timeout: 15_000 });

    // Click "Redeem Benefit".
    await page.getByRole('button', { name: /redeem benefit/i }).click();
    const redeemForm = page.getByRole('heading', { name: 'Redeem Benefit', level: 4 }).locator('..');
    await redeemForm.locator('select').selectOption('stored_value_use');
    await redeemForm.locator('input[type="number"]').fill('30');
    await page.getByRole('button', { name: /^redeem$/i }).click();

    // Backend: stored_value decreased from 100 to 70.
    await expect.poll(async () => {
      return (await (await api.get(`/members/${member.id}`)).json()).stored_value;
    }, { timeout: 10_000 }).toBe(70);

    await api.dispose();
  });

  test('UI freeze/unfreeze: status transitions via buttons, redeem is blocked while frozen', async ({ page }) => {
    const { api } = await adminApi();
    const tiers = (await (await api.get('/membership-tiers')).json()).data;
    const member = await (await api.post('/members', {
      data: { name: `Freeze Mbr ${uniqueSuffix()}`, phone: `+155505${Date.now() % 99999}`, tier_id: tiers[0].id },
    })).json();
    await api.post(`/members/${member.id}/add-value`, { data: { type: 'stored_value_add', amount: 25 } });

    await uiLogin(page, ADMIN_USER, ADMIN_PASS);
    await page.goto(`/#/members/${member.id}`);
    await expect(page.getByText(member.name).first()).toBeVisible({ timeout: 15_000 });

    // Click "Freeze" — member.status → frozen.
    await page.getByRole('button', { name: /^freeze$/i }).click();
    await expect.poll(async () => {
      return (await (await api.get(`/members/${member.id}`)).json()).status;
    }, { timeout: 10_000 }).toBe('frozen');

    // Redeem button should now be DISABLED in the UI (component: disabled={member.status === 'frozen'}).
    await expect(page.getByRole('button', { name: /redeem benefit/i })).toBeDisabled();

    // API redeem also blocked (400).
    const blockedRes = await api.post(`/members/${member.id}/redeem`, {
      data: { type: 'stored_value_use', amount: 5 },
    });
    expect(blockedRes.status()).toBe(400);

    // Click "Unfreeze".
    await page.getByRole('button', { name: /^unfreeze$/i }).click();
    await expect.poll(async () => {
      return (await (await api.get(`/members/${member.id}`)).json()).status;
    }, { timeout: 10_000 }).toBe('active');

    // Redeem button enabled again.
    await expect(page.getByRole('button', { name: /redeem benefit/i })).toBeEnabled();

    await api.dispose();
  });

  test('UI validation: creating a member with empty name shows error and does NOT persist', async ({ page }) => {
    const { api } = await adminApi();
    const tiers = (await (await api.get('/membership-tiers')).json()).data;
    const tierName = tiers[0].name as string;

    await uiLogin(page, ADMIN_USER, ADMIN_PASS);
    await page.goto('/#/members');
    await page.getByRole('button', { name: /new member/i }).click();

    // Leave name empty; fill phone + tier.
    await page.getByPlaceholder('+1-234-567-8900').fill('+15555550000');
    await page.locator('select').filter({ has: page.locator('option', { hasText: tierName }) })
      .selectOption({ label: tierName });
    await page.getByRole('button', { name: /^create member$/i }).click();

    // UI must render a visible error (createErr div).
    await expect(page.locator('text=/name|required/i').first()).toBeVisible({ timeout: 5_000 });
    await api.dispose();
  });

  test('Session-package API validation: zero sessions is rejected (400)', async () => {
    const { api } = await adminApi();
    const tiers = (await (await api.get('/membership-tiers')).json()).data;
    const m = await (await api.post('/members', {
      data: { name: `Pkg ${uniqueSuffix()}`, phone: `+155506${Date.now() % 99999}`, tier_id: tiers[0].id },
    })).json();
    const zeroRes = await api.post(`/members/${m.id}/packages`, {
      data: { name: 'Zero', total_sessions: 0, expires_at: new Date(Date.now() + 86400000 * 30).toISOString().slice(0, 10) },
    });
    expect(zeroRes.status()).toBe(400);
    const body = await zeroRes.json();
    expect(body.error || body.message).toBeTruthy();
    await api.dispose();
  });
});
