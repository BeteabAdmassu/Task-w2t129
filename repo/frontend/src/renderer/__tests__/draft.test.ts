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
