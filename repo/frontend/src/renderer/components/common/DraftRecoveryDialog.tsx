/**
 * DraftRecoveryDialog — shown on app/page load when a draft checkpoint exists.
 *
 * Props:
 *   formType   — matches the draft's form_type stored in the server
 *   formId     — optional specific form ID
 *   onRestore  — called with the recovered state JSON so the parent can hydrate the form
 *   onDiscard  — called when the user chooses to discard the draft
 */

import React, { useEffect, useState } from 'react';
import api from '../../services/api';

interface Props {
  formType: string;
  formId?: string;
  onRestore: (state: unknown) => void;
  onDiscard: () => void;
}

interface DraftCheckpoint {
  id: string;
  form_type: string;
  form_id?: string;
  state_json: unknown;
  saved_at: string;
}

export const DraftRecoveryDialog: React.FC<Props> = ({
  formType,
  formId,
  onRestore,
  onDiscard,
}) => {
  const [draft, setDraft] = useState<DraftCheckpoint | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const path = formId
      ? `/drafts/${formType}/${formId}`
      : `/drafts/${formType}/default`;

    api
      .get<DraftCheckpoint>(path)
      .then((res) => setDraft(res.data))
      .catch(() => setDraft(null))
      .finally(() => setLoading(false));
  }, [formType, formId]);

  if (loading || !draft) return null;

  const savedAt = new Date(draft.saved_at).toLocaleString();

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="draft-recovery-title"
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0,0,0,0.5)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 9999,
      }}
    >
      <div
        style={{
          background: '#fff',
          borderRadius: 8,
          padding: '2rem',
          maxWidth: 420,
          width: '90%',
          boxShadow: '0 4px 24px rgba(0,0,0,0.2)',
        }}
      >
        <h2 id="draft-recovery-title" style={{ marginTop: 0 }}>
          Unsaved Draft Found
        </h2>
        <p>
          A draft checkpoint was saved at <strong>{savedAt}</strong>. Would you
          like to restore it?
        </p>
        <div style={{ display: 'flex', gap: '1rem', justifyContent: 'flex-end' }}>
          <button
            onClick={() => {
              setDraft(null);
              onDiscard();
            }}
            style={{ padding: '0.5rem 1rem', cursor: 'pointer' }}
          >
            Discard
          </button>
          <button
            onClick={() => {
              setDraft(null);
              onRestore(draft.state_json);
            }}
            style={{
              padding: '0.5rem 1rem',
              cursor: 'pointer',
              background: '#1976d2',
              color: '#fff',
              border: 'none',
              borderRadius: 4,
            }}
          >
            Restore Draft
          </button>
        </div>
      </div>
    </div>
  );
};
