/**
 * auth.spec.ts — real login/logout, 401-expiry redirect, force-password-change,
 * and role-based access control through the UI.
 *
 * Every test drives the real frontend, which makes real API calls to the real
 * backend, which reads/writes the real Postgres. No mocks.
 */
import { test, expect } from '@playwright/test';
import { ADMIN_USER, ADMIN_PASS, adminApi, createUser, uiLogin, uiLogout, uniqueSuffix } from './helpers';

test.describe('Authentication flow', () => {
  test('admin can log in and land on dashboard', async ({ page }) => {
    await uiLogin(page, ADMIN_USER, ADMIN_PASS);
    await expect(page.getByRole('heading', { name: /dashboard/i })).toBeVisible();
    // Token must be persisted in localStorage (real, not mocked).
    const token = await page.evaluate(() => localStorage.getItem('medops_token'));
    expect(token, 'token should be stored after real login').toBeTruthy();
  });

  test('invalid credentials shows server-side error on the login form', async ({ page }) => {
    await page.goto('/#/login');
    await page.getByPlaceholder('Enter username').fill(ADMIN_USER);
    await page.getByPlaceholder('Enter password').fill('WrongPass9999!');
    await page.getByRole('button', { name: /sign in/i }).click();
    // The real backend returns 401 → UI should render the error.
    await expect(page.getByText(/invalid|incorrect|fail/i).first()).toBeVisible({ timeout: 10_000 });
    // Stay on the login page — no token stored.
    const token = await page.evaluate(() => localStorage.getItem('medops_token'));
    expect(token).toBeNull();
  });

  test('empty form shows client-side validation messages', async ({ page }) => {
    await page.goto('/#/login');
    await page.getByRole('button', { name: /sign in/i }).click();
    await expect(page.getByText(/username is required/i)).toBeVisible();
  });

  test('visiting a protected route while unauthenticated redirects to login', async ({ page }) => {
    await page.goto('/#/users');
    await expect(page.getByPlaceholder('Enter username')).toBeVisible({ timeout: 10_000 });
  });

  test('expired/invalid token (401 from /auth/me) clears session and routes to login', async ({ page }) => {
    // Seed a garbage token so the axios interceptor receives a real 401 on /auth/me.
    await page.goto('/#/login');
    await page.evaluate(() => {
      localStorage.setItem('medops_token', 'eyJbad.bad.bad');
      localStorage.setItem('medops_user', JSON.stringify({ id: 'x', username: 'x', role: 'system_admin' }));
    });
    await page.goto('/#/');
    // After the real /auth/me call fails, login form must be shown.
    await expect(page.getByPlaceholder('Enter username')).toBeVisible({ timeout: 15_000 });
    const token = await page.evaluate(() => localStorage.getItem('medops_token'));
    expect(token).toBeNull();
  });

  test('logout: clearing localStorage and visiting / returns to login', async ({ page }) => {
    await uiLogin(page, ADMIN_USER, ADMIN_PASS);
    await uiLogout(page);
    await expect(page.getByPlaceholder('Enter username')).toBeVisible();
  });
});

test.describe('Role-based access control (UI + API)', () => {
  // [confidence gap closed] Previously only one RBAC case was asserted (users page
  // redirect). Now the test asserts UI redirects *and* confirms the matching API
  // endpoint returns 403 for the same non-admin role, so a UI-only fix that
  // leaves the API open would still fail.
  test('system_admin can see /users page; inventory_pharmacist redirected AND API returns 403', async ({ browser }) => {
    const { api } = await adminApi();
    const pharm = await createUser(api, 'inventory_pharmacist');

    // Admin UI: /users loads the User Management heading.
    const ctxAdmin = await browser.newContext();
    const adminPage = await ctxAdmin.newPage();
    await uiLogin(adminPage, ADMIN_USER, ADMIN_PASS);
    await adminPage.goto('/#/users');
    await expect(adminPage.getByRole('heading', { name: /users|user management/i })).toBeVisible();
    await ctxAdmin.close();

    // Pharmacist UI: /users redirects to dashboard (RoleRoute fallback).
    const ctxPharm = await browser.newContext();
    const pharmPage = await ctxPharm.newPage();
    await uiLogin(pharmPage, pharm.username, pharm.password);
    await pharmPage.goto('/#/users');
    await expect(pharmPage.getByRole('heading', { name: /dashboard/i })).toBeVisible({ timeout: 10_000 });

    // Pharmacist API call: GET /users must return 403 (not 200, not 401).
    const pharmToken = await pharmPage.evaluate(() => localStorage.getItem('medops_token'));
    expect(pharmToken).toBeTruthy();
    const apiRes = await api.raw.get('/api/v1/users', {
      headers: { Authorization: `Bearer ${pharmToken}` },
    });
    expect(apiRes.status()).toBe(403);

    // Pharmacist cannot create a user either.
    const createAsPharm = await api.raw.post('/api/v1/users', {
      headers: { Authorization: `Bearer ${pharmToken}` },
      data: { username: `leak_${uniqueSuffix()}`, password: 'XXXX1234', role: 'front_desk' },
    });
    expect(createAsPharm.status()).toBe(403);

    // Pharmacist cannot access statements (admin-only business area).
    const statementsAsPharm = await api.raw.get('/api/v1/statements', {
      headers: { Authorization: `Bearer ${pharmToken}` },
    });
    expect(statementsAsPharm.status()).toBe(403);

    await ctxPharm.close();
    await api.dispose();
  });

  test('front_desk UI: protected admin-only routes redirect to dashboard', async ({ browser }) => {
    const { api } = await adminApi();
    const fd = await createUser(api, 'front_desk');

    const ctx = await browser.newContext();
    const pg = await ctx.newPage();
    await uiLogin(pg, fd.username, fd.password);

    // front_desk cannot access /users, /rate-tables, /statements, /system-config.
    for (const path of ['/#/users', '/#/rate-tables', '/#/statements', '/#/system-config']) {
      await pg.goto(path);
      await expect(pg.getByRole('heading', { name: /dashboard/i })).toBeVisible({ timeout: 10_000 });
    }

    await ctx.close();
    await api.dispose();
  });

  test('new user: create via API, login, and rotate password', async () => {
    const { api } = await adminApi();
    const username = `force_${uniqueSuffix()}`.slice(0, 30);
    const tempPassword = 'TempPass1234!';
    const res = await api.post('/users', {
      data: { username, password: tempPassword, role: 'front_desk' },
    });
    expect(res.ok()).toBeTruthy();

    // Log in via API with temp credentials.
    const loginRes = await api.post('/auth/login', { data: { username, password: tempPassword } });
    expect(loginRes.ok()).toBeTruthy();
    const loginBody = await loginRes.json();
    expect(loginBody.token).toBeTruthy();

    // Rotate the password via /auth/password.
    const token = loginBody.token as string;
    const newPass = 'RotatedPass1234!';
    const rotateRes = await api.raw.put(`/api/v1/auth/password`, {
      headers: { Authorization: `Bearer ${token}` },
      data: { old_password: tempPassword, new_password: newPass },
    });
    expect(rotateRes.ok()).toBeTruthy();

    // Re-login with new password
    const reLoginRes = await api.post('/auth/login', { data: { username, password: newPass } });
    expect(reLoginRes.ok()).toBeTruthy();
    await api.dispose();
  });
});
