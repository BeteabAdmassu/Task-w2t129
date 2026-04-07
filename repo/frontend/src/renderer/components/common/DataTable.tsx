import React, { useState } from 'react';

interface Column<T> {
  key: string;
  header: string;
  render?: (item: T) => React.ReactNode;
  sortable?: boolean;
}

interface Props<T> {
  columns: Column<T>[];
  data: T[];
  onRowClick?: (item: T) => void;
  onContextMenu?: (item: T, e: React.MouseEvent) => void;
  keyField?: string;
}

function DataTable<T extends Record<string, any>>({ columns, data, onRowClick, onContextMenu, keyField = 'id' }: Props<T>) {
  const [sortKey, setSortKey] = useState<string>('');
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc');

  const sorted = [...data].sort((a, b) => {
    if (!sortKey) return 0;
    const aVal = a[sortKey], bVal = b[sortKey];
    const cmp = aVal < bVal ? -1 : aVal > bVal ? 1 : 0;
    return sortDir === 'asc' ? cmp : -cmp;
  });

  const handleSort = (key: string) => {
    if (sortKey === key) setSortDir(d => d === 'asc' ? 'desc' : 'asc');
    else { setSortKey(key); setSortDir('asc'); }
  };

  return (
    <div style={{ overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.9rem' }}>
        <thead>
          <tr style={{ backgroundColor: '#f5f5f5', borderBottom: '2px solid #ddd' }}>
            {columns.map(col => (
              <th key={col.key} onClick={() => col.sortable && handleSort(col.key)}
                style={{ padding: '0.75rem', textAlign: 'left', cursor: col.sortable ? 'pointer' : 'default', userSelect: 'none', whiteSpace: 'nowrap' }}>
                {col.header} {sortKey === col.key ? (sortDir === 'asc' ? ' ▲' : ' ▼') : ''}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {sorted.map((item) => (
            <tr key={item[keyField]}
              onClick={() => onRowClick?.(item)}
              onContextMenu={(e) => { e.preventDefault(); onContextMenu?.(item, e); }}
              style={{ borderBottom: '1px solid #eee', cursor: onRowClick ? 'pointer' : 'default' }}
              onMouseEnter={e => (e.currentTarget.style.backgroundColor = '#f9f9f9')}
              onMouseLeave={e => (e.currentTarget.style.backgroundColor = 'transparent')}>
              {columns.map(col => (
                <td key={col.key} style={{ padding: '0.75rem' }}>
                  {col.render ? col.render(item) : String(item[col.key] ?? '')}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export default DataTable;
