/**
 * useDraftAutoSave — auto-saves form state as a draft checkpoint every 30 seconds.
 *
 * Usage:
 *   const { clearDraft } = useDraftAutoSave('work_order', formId, formState);
 *   // On successful submit: clearDraft()
 *
 * The hook calls PUT /api/v1/drafts/:formType with the serialised state.
 * On component unmount the interval is cleared automatically.
 */

import { useEffect, useRef } from 'react';
import { api } from '../services/api';

const AUTOSAVE_INTERVAL_MS = 30_000; // 30 seconds

export function useDraftAutoSave(
  formType: string,
  formId: string | null,
  state: unknown,
): { clearDraft: () => void } {
  const stateRef = useRef(state);
  stateRef.current = state;

  useEffect(() => {
    if (!formType) return;

    const save = async () => {
      try {
        await api.put(`/drafts/${formType}`, {
          form_id: formId,
          state_json: stateRef.current,
        });
      } catch {
        // Non-fatal — draft save failure should not interrupt the user
      }
    };

    const timer = setInterval(save, AUTOSAVE_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [formType, formId]);

  const clearDraft = async () => {
    try {
      await api.delete(`/drafts/${formType}/${formId ?? 'default'}`);
    } catch {
      // ignore
    }
  };

  return { clearDraft };
}
