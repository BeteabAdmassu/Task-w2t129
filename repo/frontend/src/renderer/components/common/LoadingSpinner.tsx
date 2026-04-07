import React from 'react';

interface Props {
  message?: string;
}

const LoadingSpinner: React.FC<Props> = ({ message = 'Loading...' }) => (
  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '2rem', flexDirection: 'column', gap: '1rem' }}>
    <div style={{
      width: 40, height: 40, border: '4px solid #e0e0e0', borderTop: '4px solid #1976d2',
      borderRadius: '50%', animation: 'spin 1s linear infinite',
    }} />
    <style>{`@keyframes spin { to { transform: rotate(360deg); } }`}</style>
    <span style={{ color: '#666' }}>{message}</span>
  </div>
);

export default LoadingSpinner;
