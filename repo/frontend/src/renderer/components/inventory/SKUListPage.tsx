import React, { useState, FormEvent, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { skusAPI } from '../../services/api';
import { useFetch } from '../../hooks/useFetch';
import type { SKU, PaginatedResponse } from '../../types';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';
import EmptyState from '../common/EmptyState';
import DataTable from '../common/DataTable';
import Pagination from '../common/Pagination';
import Modal from '../common/Modal';
import ContextMenu from '../common/ContextMenu';

const SKUListPage: React.FC = () => {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);
  const pageSize = 20;

  const { data, loading, error, refetch } = useFetch<PaginatedResponse<SKU>>(
    () => skusAPI.list({ search: search || undefined, page, page_size: pageSize }),
    [search, page]
  );

  const skus = data?.data || [];
  const total = data?.total || 0;

  // Create modal
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({
    name: '', ndc: '', upc: '', description: '', unit_of_measure: 'each',
    low_stock_threshold: 10, storage_location: '', is_active: true,
  });
  const [createErrors, setCreateErrors] = useState<Record<string, string>>({});
  const [createLoading, setCreateLoading] = useState(false);
  const [createSuccess, setCreateSuccess] = useState('');

  // Context menu
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; sku: SKU } | null>(null);

  // Operation feedback
  const [opError, setOpError] = useState<string | null>(null);
  const [opSuccess, setOpSuccess] = useState<string | null>(null);

  const [searchTimeout, setSearchTimeout] = useState<ReturnType<typeof setTimeout> | null>(null);

  const handleSearchChange = (value: string) => {
    setSearch(value);
    if (searchTimeout) clearTimeout(searchTimeout);
    setSearchTimeout(setTimeout(() => {
      setPage(1);
    }, 300));
  };

  const validateCreate = (): boolean => {
    const errs: Record<string, string> = {};
    if (!createForm.name.trim()) errs.name = 'Name is required';
    if (!createForm.unit_of_measure.trim()) errs.unit_of_measure = 'Unit of measure is required';
    if (!createForm.storage_location.trim()) errs.storage_location = 'Storage location is required';
    if (createForm.low_stock_threshold < 0) errs.low_stock_threshold = 'Threshold cannot be negative';
    setCreateErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    if (!validateCreate()) return;
    setCreateLoading(true);
    setCreateSuccess('');
    try {
      await skusAPI.create({
        name: createForm.name.trim(),
        ndc: createForm.ndc.trim() || undefined,
        upc: createForm.upc.trim() || undefined,
        description: createForm.description.trim(),
        unit_of_measure: createForm.unit_of_measure.trim(),
        low_stock_threshold: createForm.low_stock_threshold,
        storage_location: createForm.storage_location.trim(),
        is_active: createForm.is_active,
      });
      setCreateSuccess('SKU created successfully');
      setCreateForm({ name: '', ndc: '', upc: '', description: '', unit_of_measure: 'each', low_stock_threshold: 10, storage_location: '', is_active: true });
      setCreateErrors({});
      refetch();
      setTimeout(() => {
        setShowCreate(false);
        setCreateSuccess('');
      }, 1500);
    } catch (err: any) {
      setCreateErrors({ _form: err.response?.data?.error || 'Failed to create SKU' });
    } finally {
      setCreateLoading(false);
    }
  };

  const handleDeactivate = async (sku: SKU) => {
    setOpError(null);
    setOpSuccess(null);
    try {
      await skusAPI.update(sku.id, { is_active: !sku.is_active });
      setOpSuccess(`SKU ${sku.is_active ? 'deactivated' : 'activated'} successfully`);
      refetch();
      setTimeout(() => setOpSuccess(null), 3000);
    } catch (err: any) {
      setOpError(err.response?.data?.error || 'Operation failed');
    }
  };

  const columns = [
    { key: 'name', header: 'Name', sortable: true },
    { key: 'ndc', header: 'NDC', sortable: true, render: (s: SKU) => s.ndc || '-' },
    { key: 'upc', header: 'UPC', sortable: true, render: (s: SKU) => s.upc || '-' },
    { key: 'unit_of_measure', header: 'Unit', sortable: true },
    { key: 'storage_location', header: 'Storage Location', sortable: true },
    {
      key: 'is_active', header: 'Status', sortable: true,
      render: (s: SKU) => (
        <span style={{
          padding: '0.15rem 0.5rem',
          borderRadius: 12,
          fontSize: '0.8rem',
          backgroundColor: s.is_active ? '#e8f5e9' : '#fdecea',
          color: s.is_active ? '#2e7d32' : '#c62828',
        }}>
          {s.is_active ? 'Active' : 'Inactive'}
        </span>
      ),
    },
  ];

  const inputStyle = (hasError: boolean): React.CSSProperties => ({
    width: '100%',
    padding: '0.6rem',
    fontSize: '0.9rem',
    border: `1px solid ${hasError ? '#dc3545' : '#ccc'}`,
    borderRadius: 4,
    boxSizing: 'border-box',
  });

  if (loading && skus.length === 0) return <LoadingSpinner message="Loading SKUs..." />;

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h1 style={{ margin: 0, fontSize: '1.5rem', color: '#333' }}>SKU Management</h1>
        <button
          onClick={() => setShowCreate(true)}
          style={{
            padding: '0.5rem 1.25rem',
            fontSize: '0.9rem',
            backgroundColor: '#1a237e',
            color: '#fff',
            border: 'none',
            borderRadius: 4,
            cursor: 'pointer',
            fontWeight: 500,
          }}
        >
          + Create SKU
        </button>
      </div>

      {/* Search bar */}
      <div style={{ marginBottom: '1rem' }}>
        <input
          type="text"
          value={search}
          onChange={(e) => handleSearchChange(e.target.value)}
          placeholder="Search SKUs by name, NDC, or UPC..."
          style={{
            width: '100%',
            maxWidth: 500,
            padding: '0.6rem 1rem',
            fontSize: '0.9rem',
            border: '1px solid #ccc',
            borderRadius: 4,
            boxSizing: 'border-box',
          }}
        />
      </div>

      {opError && <ErrorMessage message={opError} />}
      {opSuccess && (
        <div style={{ padding: '0.75rem', backgroundColor: '#e8f5e9', border: '1px solid #c8e6c9', borderRadius: 4, color: '#2e7d32', fontSize: '0.9rem', marginBottom: '1rem' }}>
          {opSuccess}
        </div>
      )}
      {error && <ErrorMessage message={error} onRetry={refetch} />}

      {!error && skus.length === 0 && !loading ? (
        <EmptyState
          message={search ? 'No SKUs match your search' : 'No SKUs found'}
          actionLabel={search ? undefined : 'Create SKU'}
          onAction={search ? undefined : () => setShowCreate(true)}
        />
      ) : (
        <>
          <DataTable
            columns={columns}
            data={skus}
            onRowClick={(sku) => navigate(`/skus/${sku.id}`)}
            onContextMenu={(sku, e) => setCtxMenu({ x: e.clientX, y: e.clientY, sku })}
          />
          <Pagination page={page} pageSize={pageSize} total={total} onPageChange={setPage} />
        </>
      )}

      {/* Context menu */}
      {ctxMenu && (
        <ContextMenu
          x={ctxMenu.x}
          y={ctxMenu.y}
          onClose={() => setCtxMenu(null)}
          items={[
            { label: 'Edit SKU', onClick: () => navigate(`/skus/${ctxMenu.sku.id}`) },
            { label: 'View Batches', onClick: () => navigate(`/skus/${ctxMenu.sku.id}`) },
            { label: ctxMenu.sku.is_active ? 'Deactivate' : 'Activate', onClick: () => handleDeactivate(ctxMenu.sku) },
          ]}
        />
      )}

      {/* Create SKU modal */}
      {showCreate && (
        <Modal title="Create SKU" onClose={() => { setShowCreate(false); setCreateErrors({}); setCreateSuccess(''); }} width={550}>
          <form onSubmit={handleCreate}>
            {createErrors._form && (
              <div style={{ padding: '0.5rem', backgroundColor: '#fdecea', borderRadius: 4, color: '#721c24', fontSize: '0.85rem', marginBottom: '1rem' }}>
                {createErrors._form}
              </div>
            )}
            {createSuccess && (
              <div style={{ padding: '0.5rem', backgroundColor: '#e8f5e9', borderRadius: 4, color: '#2e7d32', fontSize: '0.85rem', marginBottom: '1rem' }}>
                {createSuccess}
              </div>
            )}

            <div style={{ marginBottom: '1rem' }}>
              <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Name *</label>
              <input type="text" value={createForm.name} onChange={(e) => setCreateForm((p) => ({ ...p, name: e.target.value }))} style={inputStyle(!!createErrors.name)} placeholder="SKU name" />
              {createErrors.name && <span style={{ fontSize: '0.8rem', color: '#dc3545' }}>{createErrors.name}</span>}
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
              <div>
                <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>NDC</label>
                <input type="text" value={createForm.ndc} onChange={(e) => setCreateForm((p) => ({ ...p, ndc: e.target.value }))} style={inputStyle(false)} placeholder="Optional" />
              </div>
              <div>
                <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>UPC</label>
                <input type="text" value={createForm.upc} onChange={(e) => setCreateForm((p) => ({ ...p, upc: e.target.value }))} style={inputStyle(false)} placeholder="Optional" />
              </div>
            </div>

            <div style={{ marginBottom: '1rem' }}>
              <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Description</label>
              <textarea value={createForm.description} onChange={(e) => setCreateForm((p) => ({ ...p, description: e.target.value }))} style={{ ...inputStyle(false), minHeight: 60, resize: 'vertical' }} placeholder="Description" />
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
              <div>
                <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Unit of Measure *</label>
                <select value={createForm.unit_of_measure} onChange={(e) => setCreateForm((p) => ({ ...p, unit_of_measure: e.target.value }))} style={{ ...inputStyle(!!createErrors.unit_of_measure), cursor: 'pointer' }}>
                  <option value="each">Each</option>
                  <option value="box">Box</option>
                  <option value="bottle">Bottle</option>
                  <option value="vial">Vial</option>
                  <option value="tablet">Tablet</option>
                  <option value="ml">mL</option>
                  <option value="mg">mg</option>
                  <option value="pack">Pack</option>
                </select>
                {createErrors.unit_of_measure && <span style={{ fontSize: '0.8rem', color: '#dc3545' }}>{createErrors.unit_of_measure}</span>}
              </div>
              <div>
                <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Low Stock Threshold</label>
                <input type="number" value={createForm.low_stock_threshold} onChange={(e) => setCreateForm((p) => ({ ...p, low_stock_threshold: parseInt(e.target.value) || 0 }))} style={inputStyle(!!createErrors.low_stock_threshold)} min="0" />
                {createErrors.low_stock_threshold && <span style={{ fontSize: '0.8rem', color: '#dc3545' }}>{createErrors.low_stock_threshold}</span>}
              </div>
            </div>

            <div style={{ marginBottom: '1.5rem' }}>
              <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Storage Location *</label>
              <input type="text" value={createForm.storage_location} onChange={(e) => setCreateForm((p) => ({ ...p, storage_location: e.target.value }))} style={inputStyle(!!createErrors.storage_location)} placeholder="e.g. Shelf A-1" />
              {createErrors.storage_location && <span style={{ fontSize: '0.8rem', color: '#dc3545' }}>{createErrors.storage_location}</span>}
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
              <button type="button" onClick={() => { setShowCreate(false); setCreateErrors({}); setCreateSuccess(''); }}
                style={{ padding: '0.5rem 1rem', border: '1px solid #ccc', borderRadius: 4, backgroundColor: '#fff', cursor: 'pointer' }}>
                Cancel
              </button>
              <button type="submit" disabled={createLoading}
                style={{ padding: '0.5rem 1.25rem', backgroundColor: createLoading ? '#6c757d' : '#1a237e', color: '#fff', border: 'none', borderRadius: 4, cursor: createLoading ? 'not-allowed' : 'pointer', fontWeight: 500 }}>
                {createLoading ? 'Creating...' : 'Create SKU'}
              </button>
            </div>
          </form>
        </Modal>
      )}
    </div>
  );
};

export default SKUListPage;
