import React, { useEffect, useRef } from 'react';

interface MenuItem {
  label: string;
  onClick: () => void;
  disabled?: boolean;
}

interface Props {
  x: number;
  y: number;
  items: MenuItem[];
  onClose: () => void;
}

const ContextMenu: React.FC<Props> = ({ x, y, items, onClose }) => {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [onClose]);

  return (
    <div ref={ref} style={{
      position: 'fixed', top: y, left: x, backgroundColor: '#fff', border: '1px solid #ddd',
      borderRadius: 4, boxShadow: '0 2px 8px rgba(0,0,0,0.15)', zIndex: 1000, minWidth: 160, padding: '0.25rem 0',
    }}>
      {items.map((item, i) => (
        <div key={i} onClick={() => { if (!item.disabled) { item.onClick(); onClose(); } }}
          style={{
            padding: '0.5rem 1rem', cursor: item.disabled ? 'not-allowed' : 'pointer',
            color: item.disabled ? '#999' : '#333', backgroundColor: 'transparent',
          }}
          onMouseEnter={e => (e.currentTarget.style.backgroundColor = '#f0f0f0')}
          onMouseLeave={e => (e.currentTarget.style.backgroundColor = 'transparent')}>
          {item.label}
        </div>
      ))}
    </div>
  );
};

export default ContextMenu;
