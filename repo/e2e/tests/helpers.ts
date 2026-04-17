/**
 * helpers.ts — shared Playwright test helpers.
 *
 * No mocks. Every helper drives the real API (via `apiRequest`) or the real UI
 * (via Playwright's Page object). These helpers exist only to reduce test
 * boilerplate — they don't short-circuit any application logic.
 */
import { APIRequestContext, Page, expect, request, APIResponse } from '@playwright/test';

// Base URL for API requests. Set to just the HOST (no /api/v1) because
// Playwright's baseURL + leading-slash paths drop the pathname segment.
// The wrapper below automatically prefixes every request with /api/v1.
export const API_URL = process.env.API_URL || 'http://backend:8080';
export const ADMIN_USER = 'admin';
export const ADMIN_PASS = 'AdminPass1234';
const API_PREFIX = '/api/v1';

/** Thin wrapper over APIRequestContext that prefixes every relative path
 *  with `/api/v1`. Absolute URLs (http://...) are left untouched. */
export interface ApiClient {
  get(path: string, options?: Parameters<APIRequestContext['get']>[1]): Promise<APIResponse>;
  post(path: string, options?: Parameters<APIRequestContext['post']>[1]): Promise<APIResponse>;
  put(path: string, options?: Parameters<APIRequestContext['put']>[1]): Promise<APIResponse>;
  delete(path: string, options?: Parameters<APIRequestContext['delete']>[1]): Promise<APIResponse>;
  raw: APIRequestContext;
  dispose(): Promise<void>;
}

function prefix(path: string): string {
  if (/^https?:\/\//i.test(path)) return path;
  return `${API_PREFIX}${path.startsWith('/') ? path : '/' + path}`;
}

function wrap(raw: APIRequestContext): ApiClient {
  return {
    get: (path, options) => raw.get(prefix(path), options),
    post: (path, options) => raw.post(prefix(path), options),
    put: (path, options) => raw.put(prefix(path), options),
    delete: (path, options) => raw.delete(prefix(path), options),
    raw,
    dispose: () => raw.dispose(),
  };
}

/** Unique suffix so parallel runs / reruns don't collide on unique columns. */
export function uniqueSuffix(): string {
  return `${Date.now()}${Math.floor(Math.random() * 1000)}`;
}

/** Create an authenticated API client using the admin credentials. */
export async function adminApi(): Promise<{ api: ApiClient; token: string }> {
  const raw = await request.newContext({ baseURL: API_URL });
  const res = await raw.post(`${API_PREFIX}/auth/login`, {
    data: { username: ADMIN_USER, password: ADMIN_PASS },
  });
  if (!res.ok()) throw new Error(`admin login failed: ${res.status()} ${await res.text()}`);
  const body = await res.json();
  const token = body.token as string;
  await raw.dispose();
  const authed = await request.newContext({
    baseURL: API_URL,
    extraHTTPHeaders: { Authorization: `Bearer ${token}` },
  });
  return { api: wrap(authed), token };
}

/** Log the given credentials into the UI. Waits until the URL navigates away
 *  from /login — which happens on success (→ /#/) OR when the force-password-
 *  change gate is shown (also at /#/, rendered by ProtectedRoute). */
export async function uiLogin(page: Page, username: string, password: string): Promise<void> {
  await page.goto('/#/login');
  await page.getByPlaceholder('Enter username').fill(username);
  await page.getByPlaceholder('Enter password').fill(password);
  await page.getByRole('button', { name: /sign in/i }).click();
  await page.waitForURL((url) => !url.hash.includes('/login'), { timeout: 15_000 });
  // Either the Dashboard h1 "Dashboard" OR the force-change "You must change your password"
  // paragraph must appear within a reasonable time.
  await expect(
    page.getByText(/^Dashboard$|you must change your password/i).first(),
  ).toBeVisible({ timeout: 15_000 });
}

/** Automate the first-time password rotation so subsequent flows work without
 *  hitting the force-password-change gate. Returns the new password. */
export async function uiCompleteForcedPasswordChange(page: Page, oldPassword: string): Promise<string> {
  const newPassword = `Rotated${oldPassword}!`;
  const pwInputs = page.locator('input[type="password"]');
  await pwInputs.nth(0).fill(oldPassword);
  await pwInputs.nth(1).fill(newPassword);
  await pwInputs.nth(2).fill(newPassword);
  await page.getByRole('button', { name: /change password/i }).click();
  // After reload, we should be on the dashboard.
  await expect(page.getByRole('heading', { name: /^dashboard$/i })).toBeVisible({ timeout: 15_000 });
  return newPassword;
}

/** Create a user via API and return its credentials. */
export async function createUser(
  api: ApiClient,
  role: 'system_admin' | 'inventory_pharmacist' | 'front_desk' | 'maintenance_tech' | 'learning_coordinator',
): Promise<{ id: string; username: string; password: string }> {
  const username = `${role}_${uniqueSuffix()}`.slice(0, 30);
  const password = 'TestPass1234AA';
  const res = await api.post('/users', { data: { username, password, role } });
  if (!res.ok()) throw new Error(`createUser failed: ${res.status()} ${await res.text()}`);
  const body = await res.json();
  return { id: body.id, username, password };
}

/** Create a SKU via API and return its id. */
export async function createSKU(
  api: ApiClient,
  override: Partial<{ name: string; unit_of_measure: string; category: string; reorder_point: number }> = {},
): Promise<string> {
  const payload = {
    name: override.name ?? `SKU-${uniqueSuffix()}`,
    unit_of_measure: override.unit_of_measure ?? 'box',
    category: override.category ?? 'general',
    reorder_point: override.reorder_point ?? 10,
  };
  const res = await api.post('/skus', { data: payload });
  if (!res.ok()) throw new Error(`createSKU failed: ${res.status()} ${await res.text()}`);
  const body = await res.json();
  return body.id;
}

/** Log out via UI (clears localStorage and navigates to login). */
export async function uiLogout(page: Page): Promise<void> {
  await page.evaluate(() => {
    localStorage.removeItem('medops_token');
    localStorage.removeItem('medops_user');
  });
  await page.goto('/#/login');
}

/** Helper exposing the API_PREFIX for tests that need to construct absolute URLs. */
export function apiPath(path: string): string {
  return prefix(path);
}
