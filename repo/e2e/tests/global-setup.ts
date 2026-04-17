/**
 * global-setup.ts — runs once before any Playwright test.
 *
 * Ensures the seeded admin user has `must_change_password=false` so UI tests
 * don't get gated on first-login password rotation. The rotation is
 * idempotent: if admin already has the password rotated, we just re-apply the
 * same change which the backend short-circuits (400 "must differ") — we
 * ignore that case.
 *
 * No state is leaked across tests: we always restore the original password so
 * `ADMIN_PASS` in helpers.ts is always valid.
 */
import { request } from '@playwright/test';

const API_URL = process.env.API_URL || 'http://backend:8080';
const ADMIN_USER = 'admin';
const ADMIN_PASS = 'AdminPass1234';
const TEMP_PASS = 'Temp_PW_Rotation_9871';

async function main() {
  const ctx = await request.newContext({ baseURL: API_URL });
  const loginRes = await ctx.post('/api/v1/auth/login', {
    data: { username: ADMIN_USER, password: ADMIN_PASS },
  });
  if (!loginRes.ok()) {
    console.log(`global-setup: admin login failed (${loginRes.status()}) — skipping rotation`);
    await ctx.dispose();
    return;
  }
  const { token, user } = await loginRes.json();

  if (user?.must_change_password) {
    console.log('global-setup: admin has must_change_password=true; rotating');
    // First change to a temp password, then back to the known original.
    const r1 = await ctx.put('/api/v1/auth/password', {
      data: { old_password: ADMIN_PASS, new_password: TEMP_PASS },
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!r1.ok()) {
      console.log('global-setup: first rotation failed:', r1.status(), await r1.text());
      await ctx.dispose();
      return;
    }
    // Re-login with temp password (new token needed)
    const loginTmp = await ctx.post('/api/v1/auth/login', {
      data: { username: ADMIN_USER, password: TEMP_PASS },
    });
    const { token: tmpTok } = await loginTmp.json();
    const r2 = await ctx.put('/api/v1/auth/password', {
      data: { old_password: TEMP_PASS, new_password: ADMIN_PASS },
      headers: { Authorization: `Bearer ${tmpTok}` },
    });
    if (!r2.ok()) {
      console.log('global-setup: restore rotation failed:', r2.status(), await r2.text());
    } else {
      console.log('global-setup: admin must_change_password cleared, password restored');
    }
  } else {
    console.log('global-setup: admin already has must_change_password=false — no-op');
  }
  await ctx.dispose();
}

export default main;
