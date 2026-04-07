import React from 'react';

interface Props {
  message?: string;
  actionLabel?: string;
  onAction?: () => void;
}

const EmptyState: React.FC<Props> = ({ message = 'No items found', actionLabel, onAction }) => (
  <div style={{ textAlign: 'center', padding: '3rem 1rem', color: '#999' }}>
    <p style={{ fontSize: '1.1rem' }}>{message}</p>
    {actionLabel && onAction && (
      <button onClick={onAction} style={{ marginTop: '1rem', padding: '0.5rem 1.5rem', cursor: 'pointer', backgroundColor: '#1976d2', color: '#fff', border: 'none', borderRadius: 4 }}>
        {actionLabel}
      </button>
    )}
  </div>
);

export default EmptyState;
