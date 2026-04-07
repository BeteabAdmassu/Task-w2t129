import React from 'react';

interface Props {
  title: string;
  children: React.ReactNode;
  onClose: () => void;
  width?: number;
}

const Modal: React.FC<Props> = ({ title, children, onClose, width = 500 }) => (
  <div style={{
    position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.5)', display: 'flex',
    alignItems: 'center', justifyContent: 'center', zIndex: 999,
  }} onClick={onClose}>
    <div style={{
      backgroundColor: '#fff', borderRadius: 8, width, maxWidth: '90vw', maxHeight: '90vh',
      overflow: 'auto', boxShadow: '0 4px 24px rgba(0,0,0,0.2)',
    }} onClick={e => e.stopPropagation()}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '1rem 1.5rem', borderBottom: '1px solid #eee' }}>
        <h3 style={{ margin: 0 }}>{title}</h3>
        <button onClick={onClose} style={{ background: 'none', border: 'none', fontSize: '1.5rem', cursor: 'pointer', color: '#666' }}>&times;</button>
      </div>
      <div style={{ padding: '1.5rem' }}>{children}</div>
    </div>
  </div>
);

export default Modal;
