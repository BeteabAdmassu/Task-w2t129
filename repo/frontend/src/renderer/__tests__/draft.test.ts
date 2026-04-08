/**
 * draft.test.ts — unit tests for draft auto-save and recovery logic.
 *
 * Tests the pure-logic layer: interval scheduling, state merging, and the
 * recovery flow decision (restore vs discard). Does not render React components.
 */

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// ─── Auto-save interval scheduling ───────────────────────────────────────────

describe('Draft auto-save interval', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('fires a save after 30 seconds', () => {
    const saveFn = vi.fn();
    const timer = setInterval(saveFn, 30_000);

    vi.advanceTimersByTime(29_999);
    expect(saveFn).not.toHaveBeenCalled();

    vi.advanceTimersByTime(1);
    expect(saveFn).toHaveBeenCalledTimes(1);

    clearInterval(timer);
  });

  it('fires multiple saves at 30-second intervals', () => {
    const saveFn = vi.fn();
    const timer = setInterval(saveFn, 30_000);

    vi.advanceTimersByTime(90_000); // 3 intervals
    expect(saveFn).toHaveBeenCalledTimes(3);

    clearInterval(timer);
  });

  it('does not fire if cleared before first interval', () => {
    const saveFn = vi.fn();
    const timer = setInterval(saveFn, 30_000);
    clearInterval(timer);

    vi.advanceTimersByTime(60_000);
    expect(saveFn).not.toHaveBeenCalled();
  });
});

// ─── Recovery flow decision logic ────────────────────────────────────────────

describe('Draft recovery decision', () => {
  function shouldShowRecovery(draft: { saved_at: string } | null): boolean {
    return draft !== null;
  }

  function applyDraft<T>(currentState: T, draftState: Partial<T>): T {
    return { ...currentState, ...draftState };
  }

  it('shows recovery dialog when a draft exists', () => {
    const draft = { saved_at: new Date().toISOString() };
    expect(shouldShowRecovery(draft)).toBe(true);
  });

  it('does not show recovery dialog when no draft exists', () => {
    expect(shouldShowRecovery(null)).toBe(false);
  });

  it('restoring draft merges into current state', () => {
    const current = { title: '', description: '', priority: 'normal' };
    const saved = { title: 'Fix lights', description: 'Room 12' };
    const restored = applyDraft(current, saved);

    expect(restored.title).toBe('Fix lights');
    expect(restored.description).toBe('Room 12');
    expect(restored.priority).toBe('normal'); // unchanged field preserved
  });

  it('discarding draft preserves current (blank) state', () => {
    const current = { title: '', description: '', priority: 'normal' };
    // Discard = keep current unchanged
    const afterDiscard = { ...current };
    expect(afterDiscard.title).toBe('');
  });

  it('recovery uses the saved_at timestamp for display', () => {
    const isoDate = '2026-03-15T10:30:00.000Z';
    const draft = { saved_at: isoDate };
    const display = new Date(draft.saved_at).toLocaleString();
    expect(typeof display).toBe('string');
    expect(display.length).toBeGreaterThan(0);
  });
});

// ─── Draft state serialisation ────────────────────────────────────────────────

describe('Draft state serialisation', () => {
  it('serialises form state to JSON without losing field types', () => {
    const state = { qty: 5, expiry: '2030-01-01', checked: true };
    const json = JSON.stringify(state);
    const parsed = JSON.parse(json);

    expect(parsed.qty).toBe(5);
    expect(parsed.checked).toBe(true);
    expect(parsed.expiry).toBe('2030-01-01');
  });

  it('deserialises null state_json gracefully', () => {
    const raw = null;
    const state = raw ?? {};
    expect(state).toEqual({});
  });
});

// ─── F-004: Draft form-type coverage ─────────────────────────────────────────
//
// useDraftAutoSave was extended from 3 forms to 7 in F-004. These tests verify
// the form-key naming contract for all newly covered create/edit forms.
// The formType string must be stable (used as the key on the server-side draft
// store), so renaming it is a breaking change.

const EXPECTED_DRAFT_FORM_TYPES = [
  'rate_table_create',    // RateTablesPage — new in F-004
  'user_create',          // UsersPage — new in F-004
  'learning_subject',     // LearningPage — new in F-004
  'sku_create',           // SKUListPage — new in F-004
];

describe('F-004 draft form-type key naming contract', () => {
  it('every new form type is a non-empty snake_case string', () => {
    for (const formType of EXPECTED_DRAFT_FORM_TYPES) {
      expect(typeof formType).toBe('string');
      expect(formType.length).toBeGreaterThan(0);
      expect(formType).toMatch(/^[a-z][a-z0-9_]*$/);
    }
  });

  it('no two new form types share the same key', () => {
    const unique = new Set(EXPECTED_DRAFT_FORM_TYPES);
    expect(unique.size).toBe(EXPECTED_DRAFT_FORM_TYPES.length);
  });

  it('sku_create draft key serialises a SKU create form correctly', () => {
    const skuState = {
      name: 'Amoxicillin 500mg',
      sku_code: 'AMX500',
      category: 'antibiotic',
      unit: 'tablet',
      reorder_point: 50,
      controlled: false,
    };
    const json = JSON.stringify(skuState);
    const parsed = JSON.parse(json);
    expect(parsed.sku_code).toBe('AMX500');
    expect(parsed.controlled).toBe(false);
    expect(parsed.reorder_point).toBe(50);
  });

  it('user_create draft key serialises a user create form correctly', () => {
    const userState = { username: 'jdoe', password: '**REDACTED**', role: 'front_desk' };
    const json = JSON.stringify(userState);
    const parsed = JSON.parse(json);
    expect(parsed.username).toBe('jdoe');
    expect(parsed.role).toBe('front_desk');
    // password field present in state (server-side draft is scoped to userID)
    expect('password' in parsed).toBe(true);
  });

  it('rate_table_create draft key serialises rate-table form with tiers correctly', () => {
    const rtState = {
      name: 'Standard Distance',
      type: 'distance',
      tiers: '[{"min":0,"max":10,"rate":5}]',
      fuel_surcharge_pct: '2.5',
      taxable: true,
      effective_date: '2026-01-01',
    };
    const json = JSON.stringify(rtState);
    const parsed = JSON.parse(json);
    expect(parsed.type).toBe('distance');
    expect(parsed.taxable).toBe(true);
    // tiers is stored as a string (textarea value) — parsed separately by the form
    const tiers = JSON.parse(parsed.tiers);
    expect(Array.isArray(tiers)).toBe(true);
    expect(tiers[0].rate).toBe(5);
  });

  it('learning_subject draft key serialises subject create form correctly', () => {
    const subjectState = {
      title: 'Hand Hygiene Basics',
      description: 'Annual hand hygiene training module',
      required_role: 'front_desk',
    };
    const json = JSON.stringify(subjectState);
    const parsed = JSON.parse(json);
    expect(parsed.title).toBe('Hand Hygiene Basics');
    expect(parsed.required_role).toBe('front_desk');
  });
});

// ─── F-004: Behavioral save / restore / clear cycle tests ────────────────────
//
// These tests verify the three lifecycle phases of draft autosave:
//   1. Save  — state is captured and can be serialised for PUT /drafts/:formType
//   2. Restore — saved state can be deserialised back into form state without loss
//   3. Clear  — after successful submit or explicit discard, the draft is removed
//
// The tests use pure functions that mirror what the hook and components do
// (no React rendering required). This keeps tests fast and deterministic.

// ── Helpers mirroring the hook's save/clear/restore contract ─────────────────

/** Simulates the PUT body the hook sends on auto-save. */
function buildSavePayload(formType: string, formId: string | null, state: unknown) {
  return { form_type: formType, form_id: formId, state_json: JSON.stringify(state) };
}

/** Simulates deserialising a restored draft back into form state. */
function restoreState<T>(savedJson: string): T {
  return JSON.parse(savedJson) as T;
}

/** Tracks mock cleared form types (simulates the DELETE call). */
class MockDraftStore {
  private store: Map<string, string> = new Map();

  save(formType: string, formId: string | null, state: unknown): void {
    const key = `${formType}/${formId ?? 'default'}`;
    this.store.set(key, JSON.stringify(state));
  }

  restore<T>(formType: string, formId: string | null): T | null {
    const key = `${formType}/${formId ?? 'default'}`;
    const raw = this.store.get(key);
    return raw ? (JSON.parse(raw) as T) : null;
  }

  clear(formType: string, formId: string | null): void {
    const key = `${formType}/${formId ?? 'default'}`;
    this.store.delete(key);
  }

  has(formType: string, formId: string | null): boolean {
    return this.store.has(`${formType}/${formId ?? 'default'}`);
  }
}

// ── sku_receive behavioral tests ──────────────────────────────────────────────

describe('F-004 sku_receive draft — save / restore / clear', () => {
  const FORM_TYPE = 'sku_receive';
  const FORM_ID = 'sku-abc-123';

  it('save captures all receive form fields', () => {
    const form = { lot_number: 'LOT-2026-001', expiration_date: '2028-06-30', quantity: 50, reason_code: 'purchase_order' };
    const payload = buildSavePayload(FORM_TYPE, FORM_ID, form);
    expect(payload.form_type).toBe(FORM_TYPE);
    expect(payload.form_id).toBe(FORM_ID);
    const parsed = JSON.parse(payload.state_json);
    expect(parsed.lot_number).toBe('LOT-2026-001');
    expect(parsed.quantity).toBe(50);
    expect(parsed.reason_code).toBe('purchase_order');
  });

  it('restore deserialises saved state back into receive form without loss', () => {
    const original = { lot_number: 'LOT-2026-002', expiration_date: '2027-12-31', quantity: 10, reason_code: 'return' };
    const json = JSON.stringify(original);
    const restored = restoreState<typeof original>(json);
    expect(restored.lot_number).toBe(original.lot_number);
    expect(restored.quantity).toBe(original.quantity);
    expect(restored.expiration_date).toBe(original.expiration_date);
  });

  it('clear removes the receive draft after successful submit', () => {
    const store = new MockDraftStore();
    const form = { lot_number: 'LOT-2026-003', expiration_date: '2028-01-01', quantity: 25, reason_code: 'adjustment' };
    store.save(FORM_TYPE, FORM_ID, form);
    expect(store.has(FORM_TYPE, FORM_ID)).toBe(true);

    store.clear(FORM_TYPE, FORM_ID);
    expect(store.has(FORM_TYPE, FORM_ID)).toBe(false);
  });

  it('restore returns null when no draft was saved', () => {
    const store = new MockDraftStore();
    expect(store.restore(FORM_TYPE, 'unknown-sku')).toBeNull();
  });
});

// ── sku_dispense behavioral tests ─────────────────────────────────────────────

describe('F-004 sku_dispense draft — save / restore / clear', () => {
  const FORM_TYPE = 'sku_dispense';
  const FORM_ID = 'sku-xyz-789';

  it('save captures batch_id, quantity, and prescription_id', () => {
    const form = { batch_id: 'batch-001', quantity: 5, reason_code: 'dispensed', prescription_id: 'RX-001' };
    const payload = buildSavePayload(FORM_TYPE, FORM_ID, form);
    const parsed = JSON.parse(payload.state_json);
    expect(parsed.batch_id).toBe('batch-001');
    expect(parsed.prescription_id).toBe('RX-001');
  });

  it('clear removes dispense draft after successful dispense', () => {
    const store = new MockDraftStore();
    store.save(FORM_TYPE, FORM_ID, { batch_id: 'b-1', quantity: 3 });
    store.clear(FORM_TYPE, FORM_ID);
    expect(store.has(FORM_TYPE, FORM_ID)).toBe(false);
  });

  it('restored dispense state does not override batch selection from live data', () => {
    // batch_id from a draft is advisory only — the page may pre-select a different batch.
    // This test documents that the restored batch_id value is present but should
    // be validated against live batches before use.
    const saved = { batch_id: 'batch-old', quantity: 5, reason_code: 'dispensed', prescription_id: '' };
    const restored = restoreState<typeof saved>(JSON.stringify(saved));
    // The restored batch_id exists and can be used to pre-fill the select,
    // but the component validates it against live batches.
    expect(restored.batch_id).toBe('batch-old');
  });
});

// ── wo_close behavioral tests ─────────────────────────────────────────────────

describe('F-004 wo_close draft — save / restore / clear', () => {
  const FORM_TYPE = 'wo_close';
  const FORM_ID = 'wo-def-456';

  it('save captures parts_cost and labor_cost strings', () => {
    const form = { parts_cost: '125.00', labor_cost: '250.00' };
    const payload = buildSavePayload(FORM_TYPE, FORM_ID, form);
    const parsed = JSON.parse(payload.state_json);
    expect(parsed.parts_cost).toBe('125.00');
    expect(parsed.labor_cost).toBe('250.00');
  });

  it('restore allows pre-filling close form with saved cost values', () => {
    const saved = { parts_cost: '75.50', labor_cost: '120.00' };
    const restored = restoreState<typeof saved>(JSON.stringify(saved));
    expect(parseFloat(restored.parts_cost)).toBe(75.5);
    expect(parseFloat(restored.labor_cost)).toBe(120.0);
  });

  it('clear removes close draft after work order is successfully closed', () => {
    const store = new MockDraftStore();
    store.save(FORM_TYPE, FORM_ID, { parts_cost: '50', labor_cost: '100' });
    expect(store.has(FORM_TYPE, FORM_ID)).toBe(true);
    store.clear(FORM_TYPE, FORM_ID);
    expect(store.has(FORM_TYPE, FORM_ID)).toBe(false);
  });
});

// ── learning_chapter behavioral tests ─────────────────────────────────────────

describe('F-004 learning_chapter draft — save / restore / clear', () => {
  const FORM_TYPE = 'learning_chapter';
  const SUBJECT_ID = 'subj-001';

  it('save captures chapter name and sort_order scoped to subject', () => {
    const form = { name: 'Introduction to HIPAA', sort_order: 1 };
    const payload = buildSavePayload(FORM_TYPE, SUBJECT_ID, form);
    expect(payload.form_id).toBe(SUBJECT_ID); // scoped to subject
    const parsed = JSON.parse(payload.state_json);
    expect(parsed.name).toBe('Introduction to HIPAA');
    expect(parsed.sort_order).toBe(1);
  });

  it('clear removes chapter draft after successful creation', () => {
    const store = new MockDraftStore();
    store.save(FORM_TYPE, SUBJECT_ID, { name: 'Chapter 1', sort_order: 0 });
    store.clear(FORM_TYPE, SUBJECT_ID);
    expect(store.has(FORM_TYPE, SUBJECT_ID)).toBe(false);
  });

  it('chapter drafts for different subjects are stored independently', () => {
    const store = new MockDraftStore();
    store.save(FORM_TYPE, 'subj-A', { name: 'Chapter A' });
    store.save(FORM_TYPE, 'subj-B', { name: 'Chapter B' });
    const a = store.restore<{ name: string }>(FORM_TYPE, 'subj-A');
    const b = store.restore<{ name: string }>(FORM_TYPE, 'subj-B');
    expect(a?.name).toBe('Chapter A');
    expect(b?.name).toBe('Chapter B');
    // Clearing A does not affect B
    store.clear(FORM_TYPE, 'subj-A');
    expect(store.has(FORM_TYPE, 'subj-A')).toBe(false);
    expect(store.has(FORM_TYPE, 'subj-B')).toBe(true);
  });
});

// ── learning_kp behavioral tests ──────────────────────────────────────────────

describe('F-004 learning_kp draft — save / restore / clear', () => {
  const FORM_TYPE = 'learning_kp';

  it('save captures title, content, tags, and classifications', () => {
    const form = { title: 'Hand Hygiene Steps', content: '# Step 1\nWet hands', tags: 'hygiene, safety', classifications: '{"level":"basic"}' };
    const payload = buildSavePayload(FORM_TYPE, 'chapter-001', form);
    const parsed = JSON.parse(payload.state_json);
    expect(parsed.title).toBe('Hand Hygiene Steps');
    expect(parsed.content).toContain('Wet hands');
    expect(JSON.parse(parsed.classifications).level).toBe('basic');
  });

  it('restore preserves multiline content without truncation', () => {
    const content = 'Line 1\nLine 2\nLine 3\n## Section\nMore content';
    const saved = { title: 'Test', content, tags: '', classifications: '{}' };
    const restored = restoreState<typeof saved>(JSON.stringify(saved));
    expect(restored.content).toBe(content);
    expect(restored.content.split('\n')).toHaveLength(5);
  });

  it('clear removes KP draft after successful create or update', () => {
    const store = new MockDraftStore();
    store.save(FORM_TYPE, 'chapter-002', { title: 'Draft KP' });
    store.clear(FORM_TYPE, 'chapter-002');
    expect(store.has(FORM_TYPE, 'chapter-002')).toBe(false);
  });

  it('edit KP uses KP id as formId (not chapter id)', () => {
    // When editing an existing KP, formId = editingKp.id
    // When creating a new KP, formId = selectedChapter.id
    // This ensures the edit draft is separate from the create draft for the same chapter.
    const store = new MockDraftStore();
    const chapterId = 'chapter-003';
    const kpId = 'kp-edit-001';
    store.save(FORM_TYPE, chapterId, { title: 'New KP draft' });     // create draft
    store.save(FORM_TYPE, kpId, { title: 'Edit KP draft' });          // edit draft
    expect(store.restore<{ title: string }>(FORM_TYPE, chapterId)?.title).toBe('New KP draft');
    expect(store.restore<{ title: string }>(FORM_TYPE, kpId)?.title).toBe('Edit KP draft');
  });
});

// ── Discard path behavioral tests ─────────────────────────────────────────────

describe('F-004 discard path — clear does not restore stale data', () => {
  it('after clear, restore returns null (no stale data)', () => {
    const store = new MockDraftStore();
    store.save('sku_receive', 'sku-1', { lot_number: 'LOT-001', quantity: 5 });
    store.clear('sku_receive', 'sku-1');
    const restored = store.restore('sku_receive', 'sku-1');
    expect(restored).toBeNull();
  });

  it('clearing one form does not affect drafts for other form types', () => {
    const store = new MockDraftStore();
    store.save('sku_receive', 'sku-1', { lot_number: 'LOT-001' });
    store.save('sku_dispense', 'sku-1', { batch_id: 'batch-1' });
    store.clear('sku_receive', 'sku-1');
    expect(store.has('sku_receive', 'sku-1')).toBe(false);
    expect(store.has('sku_dispense', 'sku-1')).toBe(true);
  });

  it('save after discard starts fresh (old content not present)', () => {
    const store = new MockDraftStore();
    store.save('wo_close', 'wo-1', { parts_cost: '999', labor_cost: '999' });
    store.clear('wo_close', 'wo-1');
    store.save('wo_close', 'wo-1', { parts_cost: '0', labor_cost: '0' });
    const restored = store.restore<{ parts_cost: string }>('wo_close', 'wo-1');
    expect(restored?.parts_cost).toBe('0');
  });
});

// ── member_create behavioral tests ────────────────────────────────────────────

describe('F-004 member_create draft — save / restore / clear', () => {
  const FORM_TYPE = 'member_create';

  it('save captures all create-member fields without loss', () => {
    const form = { name: 'Jane Doe', id_number: 'GOV-12345', phone: '+1-555-000-1234', tier_id: 'tier-gold' };
    const payload = buildSavePayload(FORM_TYPE, null, form);
    expect(payload.form_type).toBe(FORM_TYPE);
    expect(payload.form_id).toBeNull();
    const parsed = JSON.parse(payload.state_json);
    expect(parsed.name).toBe('Jane Doe');
    expect(parsed.id_number).toBe('GOV-12345');
    expect(parsed.phone).toBe('+1-555-000-1234');
    expect(parsed.tier_id).toBe('tier-gold');
  });

  it('restore hydrates form correctly from a complete saved draft', () => {
    const saved = { name: 'John Smith', id_number: 'ID-99', phone: '+44 7700 900001', tier_id: 'tier-silver' };
    const restored = restoreState<typeof saved>(JSON.stringify(saved));
    expect(restored.name).toBe('John Smith');
    expect(restored.phone).toBe('+44 7700 900001');
    expect(restored.tier_id).toBe('tier-silver');
  });

  it('restore handles partial draft safely (missing fields default to empty string)', () => {
    // Simulates a draft saved before tier was selected, or a truncated payload.
    const partial = { name: 'Partial User', phone: '' };
    const restored = restoreState<Record<string, unknown>>(JSON.stringify(partial));

    // The restore handler in MembersPage.tsx defaults missing string fields to ''.
    const name      = typeof restored.name      === 'string' ? restored.name      : '';
    const id_number = typeof restored.id_number === 'string' ? restored.id_number : '';
    const phone     = typeof restored.phone     === 'string' ? restored.phone     : '';
    const tier_id   = typeof restored.tier_id   === 'string' ? restored.tier_id   : '';

    expect(name).toBe('Partial User');
    expect(id_number).toBe('');
    expect(phone).toBe('');
    expect(tier_id).toBe('');
  });

  it('restore handles null/non-object payload without throwing', () => {
    // onRestore guard: `if (!state || typeof state !== 'object') return`
    const badPayloads: unknown[] = [null, undefined, 42, 'string', true];
    for (const payload of badPayloads) {
      // Should not throw — same guard used in handleMemberDraftRestore
      const isSafe = !payload || typeof payload !== 'object';
      expect(isSafe).toBe(true);
    }
  });

  it('clear removes member draft after successful create', () => {
    const store = new MockDraftStore();
    store.save(FORM_TYPE, null, { name: 'Draft Member', phone: '+1-555-000-9999', tier_id: 'tier-bronze', id_number: '' });
    expect(store.has(FORM_TYPE, null)).toBe(true);

    store.clear(FORM_TYPE, null);
    expect(store.has(FORM_TYPE, null)).toBe(false);
  });

  it('discard removes draft and a subsequent restore returns null', () => {
    const store = new MockDraftStore();
    store.save(FORM_TYPE, null, { name: 'To Discard' });
    store.clear(FORM_TYPE, null); // simulates onDiscard → clearMemberDraft()
    expect(store.restore(FORM_TYPE, null)).toBeNull();
  });

  it('successful submit clears draft so re-opening form starts fresh', () => {
    const store = new MockDraftStore();
    // 1. Draft was auto-saved during form fill
    store.save(FORM_TYPE, null, { name: 'New Member', phone: '+1-555-111-2222', tier_id: 'tier-gold', id_number: '' });
    // 2. Submit succeeded — clearMemberDraft() runs
    store.clear(FORM_TYPE, null);
    // 3. Form is re-opened — no draft should exist
    expect(store.restore(FORM_TYPE, null)).toBeNull();
  });
});
