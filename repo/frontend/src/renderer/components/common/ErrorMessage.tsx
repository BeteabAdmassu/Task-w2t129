import React from 'react';

interface Props {
  message: string;
  onRetry?: () => void;
}

const ErrorMessage: React.FC<Props> = ({ message, onRetry }) => (
  <div style={{ padding: '1rem', backgroundColor: '#fdecea', border: '1px solid #f5c6cb', borderRadius: 8, margin: '1rem 0' }}>
    <p style={{ color: '#721c24', margin: 0 }}>{message}</p>
    {onRetry && (
      <button onClick={onRetry} style={{ marginTop: '0.5rem', padding: '0.25rem 1rem', cursor: 'pointer', backgroundColor: '#dc3545', color: '#fff', border: 'none', borderRadius: 4 }}>
        Retry
      </button>
    )}
  </div>
);

export default ErrorMessage;
