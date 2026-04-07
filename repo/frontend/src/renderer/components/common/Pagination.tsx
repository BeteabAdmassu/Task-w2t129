import React from 'react';

interface Props {
  page: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
}

const Pagination: React.FC<Props> = ({ page, pageSize, total, onPageChange }) => {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));

  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '0.5rem', padding: '1rem 0' }}>
      <button onClick={() => onPageChange(page - 1)} disabled={page <= 1}
        style={{ padding: '0.5rem 1rem', cursor: page <= 1 ? 'not-allowed' : 'pointer', border: '1px solid #ddd', borderRadius: 4, backgroundColor: '#fff' }}>
        Previous
      </button>
      <span style={{ padding: '0 1rem', color: '#666' }}>
        Page {page} of {totalPages} ({total} items)
      </span>
      <button onClick={() => onPageChange(page + 1)} disabled={page >= totalPages}
        style={{ padding: '0.5rem 1rem', cursor: page >= totalPages ? 'not-allowed' : 'pointer', border: '1px solid #ddd', borderRadius: 4, backgroundColor: '#fff' }}>
        Next
      </button>
    </div>
  );
};

export default Pagination;
