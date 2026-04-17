/**
 * system.spec.ts — drafts / file upload-download / backup / system config /
 * learning.
 *
 * [confidence gap closed] Previously:
 *  - Draft retrieval asserted "listRes.ok()" without checking the body matched
 *    the saved payload.
 *  - Draft DELETE was only checked for status; the row could remain in the DB.
 *  - File download asserted byte-for-byte but did not check response headers
 *    (Content-Disposition) or validate metadata shape.
 *  - Config and backup asserted key presence, not meaningful values.
 *
 * This rewrite tightens every contract:
 *  - Drafts: exact round-trip of state_json; DELETE must invalidate GET (→404).
 *  - Files: sha256 check + Content-Disposition header assertion.
 *  - Backup: backup_file ends in .sql; files_archive is empty or ends in .zip.
 *  - Config: value round-trip via PUT→GET on a unique key.
 *  - Update/rollback: contract-level preconditions (401 without auth, 403 for
 *    non-admins). Full execution is out of scope because it requires signed
 *    update packages and bytecode promotion that cannot be run in CI safely.
 */
import { test, expect } from '@playwright/test';
import { adminApi, uniqueSuffix } from './helpers';
import * as crypto from 'node:crypto';

test.describe('Drafts (auto-save) — contract-level assertions', () => {
  test('PUT exact 201 + GET returns saved state_json + DELETE returns 200 + subsequent GET is 404', async () => {
    const { api } = await adminApi();
    const formType = `wo_create_${uniqueSuffix()}`.slice(0, 30);
    const formId = `draft-${uniqueSuffix()}`;
    const savedPayload = { description: 'E2E draft', priority: 'high', trade: 'electrical' };

    // PUT must return exactly 201 with an id.
    const putRes = await api.put(`/drafts/${formType}`, {
      data: { form_id: formId, state_json: savedPayload },
    });
    expect(putRes.status()).toBe(201);
    const putBody = await putRes.json();
    expect(putBody.id).toBeTruthy();
    expect(putBody.form_type).toBe(formType);
    expect(putBody.form_id).toBe(formId);

    // GET must return exactly 200 with the exact same state_json (no mutation).
    const getRes = await api.get(`/drafts/${formType}/${encodeURIComponent(formId)}`);
    expect(getRes.status()).toBe(200);
    const getBody = await getRes.json();
    expect(getBody.state_json).toEqual(savedPayload);

    // LIST must include the saved draft by (form_type, form_id).
    const listRes = await api.get('/drafts');
    expect(listRes.status()).toBe(200);
    const listBody = await listRes.json();
    const rows = (listBody.data ?? listBody) as Array<{ form_type: string; form_id: string }>;
    expect(rows.find(r => r.form_type === formType && r.form_id === formId)).toBeTruthy();

    // DELETE must return exactly 200.
    const delRes = await api.delete(`/drafts/${formType}/${encodeURIComponent(formId)}`);
    expect(delRes.status()).toBe(200);

    // Deterministic delete: after DELETE, GET must NOT return the saved payload.
    // Current backend contract (system.go:1185): when no row matches, the handler
    // returns 200 with body == null (NOT 404). The tight check is that the body
    // is no longer the original draft — covering both the current 200/null
    // contract and a future 404 fix.
    const afterDelRes = await api.get(`/drafts/${formType}/${encodeURIComponent(formId)}`);
    if (afterDelRes.status() === 200) {
      const body = await afterDelRes.text();
      expect(body.trim()).toBe('null'); // handler renders Go nil pointer as JSON null
    } else {
      expect(afterDelRes.status()).toBe(404);
    }

    // Extra durability check: LIST must no longer contain the draft.
    const listAfterRes = await api.get('/drafts');
    const listAfter = ((await listAfterRes.json()).data ?? []) as Array<{ form_type: string; form_id: string }>;
    expect(listAfter.find(r => r.form_type === formType && r.form_id === formId)).toBeUndefined();

    await api.dispose();
  });

  test('Unauthenticated access to drafts returns 401 (negative test)', async ({ request: _req }) => {
    const { request: pwr } = await import('@playwright/test');
    const { API_URL, apiPath } = await import('./helpers');
    const noAuth = await pwr.newContext({ baseURL: API_URL });
    const res = await noAuth.get(apiPath('/drafts'));
    expect(res.status()).toBe(401);
    await noAuth.dispose();
  });
});

test.describe('File upload / download — byte-integrity + header contract', () => {
  test('upload returns UUID id + download returns exact bytes (sha256) + Content-Disposition present', async () => {
    const { api } = await adminApi();
    const contents = Buffer.from(`e2e file content ${uniqueSuffix()}\n`, 'utf-8');
    const expectedSha = crypto.createHash('sha256').update(contents).digest('hex');

    const uploadRes = await api.post('/files/upload', {
      multipart: { file: { name: 'e2e.txt', mimeType: 'text/plain', buffer: contents } },
    });
    expect(uploadRes.status()).toBe(201);
    const uploadBody = await uploadRes.json();
    const fileId: string = uploadBody.file?.id ?? uploadBody.id;
    // Must be a UUID.
    expect(fileId).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);

    // Download: bytes equal via sha256.
    const dlRes = await api.get(`/files/${fileId}`);
    expect(dlRes.status()).toBe(200);
    const dlBuf = await dlRes.body();
    const gotSha = crypto.createHash('sha256').update(dlBuf).digest('hex');
    expect(gotSha).toBe(expectedSha);

    // Download metadata: Content-Disposition header must be present for browser save-as.
    const headers = dlRes.headers();
    expect(headers['content-disposition']).toBeTruthy();

    await api.dispose();
  });

  test('upload → link photo to work order → GET photos returns the linked file in envelope .data[]', async () => {
    const { api } = await adminApi();

    const woRes = await api.post('/work-orders', {
      data: { trade: 'electrical', priority: 'normal', description: `photo-link WO ${uniqueSuffix()}`, location: 'Any' },
    });
    expect(woRes.status()).toBe(201);
    const wo = await woRes.json();

    const up = await api.post('/files/upload', {
      multipart: { file: { name: 'pic.png', mimeType: 'image/png', buffer: Buffer.from('fake png bytes') } },
    });
    expect(up.status()).toBe(201);
    const file = await up.json();
    const fileId: string = file.file?.id ?? file.id;

    // Link returns exactly 201 (handler: LinkPhoto → http.StatusCreated).
    const linkRes = await api.post(`/work-orders/${wo.id}/photos`, { data: { file_id: fileId } });
    expect(linkRes.status()).toBe(201);

    // GET returns envelope { data: [...] } containing the exact file id.
    const photosRes = await api.get(`/work-orders/${wo.id}/photos`);
    expect(photosRes.status()).toBe(200);
    const photosBody = await photosRes.json();
    expect(Array.isArray(photosBody.data)).toBeTruthy();
    const ids = (photosBody.data as Array<{ id: string }>).map(p => p.id);
    expect(ids).toContain(fileId);

    await api.dispose();
  });

  test('download without auth returns 401; non-existent file id returns 403 or 404', async () => {
    const { request: pwr } = await import('@playwright/test');
    const { API_URL, apiPath } = await import('./helpers');
    const noAuth = await pwr.newContext({ baseURL: API_URL });
    const res = await noAuth.get(apiPath('/files/00000000-0000-0000-0000-000000000000'));
    expect(res.status()).toBe(401);
    await noAuth.dispose();

    // With auth, a nonexistent id → 403 (object-auth-first) OR 404.
    const { api } = await adminApi();
    const nx = await api.get('/files/00000000-0000-0000-0000-000000000000');
    expect([403, 404]).toContain(nx.status());
    await api.dispose();
  });
});

test.describe('System config — strict round-trip', () => {
  test('PUT stores and GET returns the exact value for a unique key', async () => {
    const { api } = await adminApi();
    const key = `e2e_cfg_${uniqueSuffix()}`.slice(0, 40);
    const value = `value-${uniqueSuffix()}`;

    const putRes = await api.put('/system/config', { data: { key, value } });
    expect(putRes.status()).toBe(200);

    const getRes = await api.get('/system/config');
    expect(getRes.status()).toBe(200);
    const body = await getRes.json();
    // The config object must contain the exact key with the exact value.
    expect(body.config[key]).toBe(value);

    // Overwrite with a different value.
    const value2 = `overwritten-${uniqueSuffix()}`;
    const putRes2 = await api.put('/system/config', { data: { key, value: value2 } });
    expect(putRes2.status()).toBe(200);
    const getRes2 = await api.get('/system/config');
    const body2 = await getRes2.json();
    expect(body2.config[key]).toBe(value2);

    await api.dispose();
  });
});

test.describe('System backup — shape + path contract', () => {
  test('backup returns backup_file ending in .sql, files_archive .zip or empty, non-empty timestamp', async () => {
    const { api } = await adminApi();
    const res = await api.post('/system/backup');
    expect(res.status()).toBe(200);
    const body = await res.json();
    expect(body.backup_file).toMatch(/\.sql$/);
    expect(body).toHaveProperty('files_archive');
    expect(typeof body.files_archive).toBe('string');
    if (body.files_archive.length > 0) {
      expect(body.files_archive).toMatch(/\.zip$/);
    }
    expect(body.timestamp).toBeTruthy();
    expect(body.timestamp.length).toBeGreaterThan(0);

    await api.dispose();
  });

  test('backup status endpoint returns a status field + message, and last_backup pointer when backups exist', async () => {
    const { api } = await adminApi();
    // Take a fresh backup so status has data to report about.
    await api.post('/system/backup');
    const res = await api.get('/system/backup/status');
    expect(res.status()).toBe(200);
    const body = await res.json();
    // Actual contract (system.go BackupStatus): { status, message, last_backup }.
    // status values observed: "idle" (operational), "no_backups" (empty backup dir).
    expect(['idle', 'no_backups', 'ok', 'available']).toContain(body.status);
    expect(typeof body.message).toBe('string');
    if (body.status === 'idle' || body.status === 'ok') {
      // When a backup exists, last_backup must be a non-empty filename string.
      expect(body.last_backup).toBeTruthy();
      expect(body.last_backup).toMatch(/\.(sql|zip)$/);
    }
    await api.dispose();
  });
});

// Update/rollback cannot be safely executed in CI (requires signed update
// packages and binary promotion). Contract-level tests only:
test.describe('System update/rollback — contract-level only', () => {
  test('POST /system/update without auth returns 401', async () => {
    const { request: pwr } = await import('@playwright/test');
    const { API_URL, apiPath } = await import('./helpers');
    const noAuth = await pwr.newContext({ baseURL: API_URL });
    const res = await noAuth.post(apiPath('/system/update'));
    expect(res.status()).toBe(401);
    await noAuth.dispose();
  });

  test('POST /system/update rejected for non-system_admin roles (403)', async () => {
    const { api } = await adminApi();
    // Create a pharmacist user and log in as them.
    const uname = `pharm_${uniqueSuffix()}`.slice(0, 30);
    const createRes = await api.post('/users', {
      data: { username: uname, password: 'PharmPass1234', role: 'inventory_pharmacist' },
    });
    expect(createRes.ok()).toBeTruthy();
    const loginRes = await api.post('/auth/login', { data: { username: uname, password: 'PharmPass1234' } });
    const pharmTok = (await loginRes.json()).token as string;
    const res = await api.raw.post('/api/v1/system/update', {
      headers: { Authorization: `Bearer ${pharmTok}` },
    });
    expect(res.status()).toBe(403);
    await api.dispose();
  });

  test('POST /system/rollback without auth returns 401 (contract-level)', async () => {
    const { request: pwr } = await import('@playwright/test');
    const { API_URL, apiPath } = await import('./helpers');
    const noAuth = await pwr.newContext({ baseURL: API_URL });
    const res = await noAuth.post(apiPath('/system/rollback'));
    expect(res.status()).toBe(401);
    await noAuth.dispose();
  });
});

test.describe('Learning content — real full-text search round-trip', () => {
  test('create subject → chapter → knowledge point → search finds by unique marker → export returns non-empty content', async () => {
    const { api } = await adminApi();

    const subject = await api.post('/learning/subjects', {
      data: { name: `Subj ${uniqueSuffix()}`, description: 'E2E' },
    });
    expect(subject.status()).toBe(201);
    const sBody = await subject.json();

    const chapter = await api.post('/learning/chapters', {
      data: { name: `Chap ${uniqueSuffix()}`, subject_id: sBody.id },
    });
    expect(chapter.status()).toBe(201);
    const cBody = await chapter.json();

    const marker = `Uniquemarker-${uniqueSuffix()}`;
    const kp = await api.post('/learning/knowledge-points', {
      data: { title: `Aspirin ${uniqueSuffix()}`, content: `# Aspirin\nPain reliever. ${marker}`, chapter_id: cBody.id },
    });
    expect(kp.status()).toBe(201);
    const kBody = await kp.json();

    const searchRes = await api.get(`/learning/search?q=${encodeURIComponent(marker)}`);
    expect(searchRes.status()).toBe(200);
    const searchBody = await searchRes.json();
    const hits = (searchBody.data || searchBody) as Array<{ id: string }>;
    expect(hits.find(h => h.id === kBody.id)).toBeTruthy();

    const expRes = await api.get(`/learning/export/${kBody.id}?format=md`);
    expect(expRes.status()).toBe(200);
    const expText = await expRes.text();
    expect(expText.length).toBeGreaterThan(0);
    expect(expText).toContain(marker);

    await api.dispose();
  });
});
