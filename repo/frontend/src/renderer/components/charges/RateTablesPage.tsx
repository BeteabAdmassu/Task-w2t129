import React, { useState, useEffect, useRef } from 'react';
import { chargesAPI } from '../../services/api';
import type { RateTable } from '../../types';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';
import EmptyState from '../common/EmptyState';
import DataTable from '../common/DataTable';
import Modal from '../common/Modal';

const inputStyle: React.CSSProperties = {
  width: '100%', padding: '0.5rem', border: '1px solid #ccc', borderRadius: 4, fontSize: '0.9rem', boxSizing: 'border-box',
};
const btnPrimary: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#1976d2', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const btnDisabled: React.CSSProperties = { ...btnPrimary, opacity: 0.6, cursor: 'not-allowed' };
const btnSecondary: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#6c757d', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const selectStyle: React.CSSProperties = { ...inputStyle, appearance: 'auto' as const };
const successStyle: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#d4edda', border: '1px solid #c3e6cb', borderRadius: 4, color: '#155724', marginBottom: '1rem',
};

const typeColors: Record<string, string> = { distance: '#1976d2', weight: '#ff9800', volume: '#9c27b0' };

const RateTablesPage: React.FC = () => {
  const [rateTables, setRateTables] = useState<RateTable[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchRateTables = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await chargesAPI.listRateTables();
      setRateTables(res.data.data || res.data);
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to load rate tables');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRateTables();
  }, []);

  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState({
    name: '', type: 'distance' as string, tiers: '[{"min": 0, "max": 10, "rate": 5}]',
    fuel_surcharge_pct: '0', taxable: false, effective_date: '',
  });
  const [formErr, setFormErr] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [successMsg, setSuccessMsg] = useState('');

  // CSV import
  const csvInputRef = useRef<HTMLInputElement>(null);
  const [importLoading, setImportLoading] = useState(false);
  const [importMsg, setImportMsg] = useState('');

  const handleCreate = async () => {
    if (!form.name.trim()) { setFormErr('Name is required'); return; }
    if (!form.type) { setFormErr('Type is required'); return; }
    let tiers: Array<{ min: number; max: number; rate: number }>;
    try {
      tiers = JSON.parse(form.tiers);
      if (!Array.isArray(tiers)) throw new Error('not array');
      for (const t of tiers) {
        if (typeof t.min !== 'number' || typeof t.max !== 'number' || typeof t.rate !== 'number') {
          throw new Error('Each tier must have numeric min, max, and rate');
        }
        if (t.min < 0 || t.max < t.min || t.rate < 0) {
          throw new Error('Invalid tier values: min >= 0, max >= min, rate >= 0');
        }
      }
    } catch (e: any) {
      setFormErr('Invalid tiers JSON: ' + e.message);
      return;
    }
    const fuelPct = parseFloat(form.fuel_surcharge_pct);
    if (isNaN(fuelPct) || fuelPct < 0) { setFormErr('Fuel surcharge must be a non-negative number'); return; }
    if (!form.effective_date) { setFormErr('Effective date is required'); return; }

    setSubmitting(true);
    setFormErr('');
    try {
      await chargesAPI.createRateTable({
        name: form.name.trim(),
        type: form.type,
        tiers,
        fuel_surcharge_pct: fuelPct,
        taxable: form.taxable,
        effective_date: form.effective_date,
      });
      setSuccessMsg('Rate table created successfully');
      setShowCreate(false);
      setForm({ name: '', type: 'distance', tiers: '[{"min": 0, "max": 10, "rate": 5}]', fuel_surcharge_pct: '0', taxable: false, effective_date: '' });
      fetchRateTables();
      setTimeout(() => setSuccessMsg(''), 3000);
    } catch (e: any) {
      setFormErr(e.response?.data?.error || 'Failed to create rate table');
    } finally {
      setSubmitting(false);
    }
  };

  const handleCSVImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setImportLoading(true);
    setImportMsg('');
    const fd = new FormData();
    fd.append('file', file);
    try {
      await chargesAPI.importCSV(fd);
      setImportMsg('CSV imported successfully');
      fetchRateTables();
      setTimeout(() => setImportMsg(''), 3000);
    } catch (err: any) {
      setImportMsg('Import failed: ' + (err.response?.data?.error || err.message));
    } finally {
      setImportLoading(false);
      if (csvInputRef.current) csvInputRef.current.value = '';
    }
  };

  const columns = [
    { key: 'name', header: 'Name', sortable: true },
    {
      key: 'type', header: 'Type', sortable: true,
      render: (rt: RateTable) => (
        <span style={{
          padding: '0.2rem 0.6rem', borderRadius: 12, fontSize: '0.8rem', fontWeight: 600,
          backgroundColor: (typeColors[rt.type] || '#666') + '20', color: typeColors[rt.type] || '#666',
          textTransform: 'capitalize' as const,
        }}>{rt.type}</span>
      ),
    },
    { key: 'fuel_surcharge_pct', header: 'Fuel Surcharge', sortable: true, render: (rt: RateTable) => `${rt.fuel_surcharge_pct}%` },
    {
      key: 'taxable', header: 'Taxable',
      render: (rt: RateTable) => <span style={{ color: rt.taxable ? '#28a745' : '#999' }}>{rt.taxable ? 'Yes' : 'No'}</span>,
    },
    { key: 'effective_date', header: 'Effective Date', sortable: true, render: (rt: RateTable) => new Date(rt.effective_date).toLocaleDateString() },
  ];

  // Expand row to show tiers
  const [expandedId, setExpandedId] = useState<string | null>(null);

  return (
    <div style={{ padding: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h2 style={{ margin: 0 }}>Rate Tables</h2>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <input ref={csvInputRef} type="file" accept=".csv" style={{ display: 'none' }} onChange={handleCSVImport} />
          <button onClick={() => csvInputRef.current?.click()} style={btnSecondary} disabled={importLoading}>
            {importLoading ? 'Importing...' : 'Import CSV'}
          </button>
          <button onClick={() => setShowCreate(true)} style={btnPrimary}>+ New Rate Table</button>
        </div>
      </div>

      {successMsg && <div style={successStyle}>{successMsg}</div>}
      {importMsg && <div style={{ padding: '0.5rem 1rem', backgroundColor: importMsg.startsWith('Import failed') ? '#fdecea' : '#d4edda', color: importMsg.startsWith('Import failed') ? '#721c24' : '#155724', borderRadius: 4, marginBottom: '1rem' }}>{importMsg}</div>}

      {loading && <LoadingSpinner />}
      {error && <ErrorMessage message={error} onRetry={fetchRateTables} />}
      {!loading && !error && (!rateTables || rateTables.length === 0) && (
        <EmptyState message="No rate tables found" actionLabel="Create Rate Table" onAction={() => setShowCreate(true)} />
      )}
      {!loading && !error && rateTables && rateTables.length > 0 && (
        <>
          <DataTable<RateTable>
            columns={columns}
            data={rateTables}
            onRowClick={(rt) => setExpandedId(expandedId === rt.id ? null : rt.id)}
          />
          {expandedId && rateTables.find(rt => rt.id === expandedId) && (
            <div style={{ margin: '0.5rem 0 1rem', padding: '1rem', backgroundColor: '#f9f9f9', borderRadius: 8, border: '1px solid #e0e0e0' }}>
              <h4 style={{ margin: '0 0 0.5rem' }}>Tiers for: {rateTables.find(rt => rt.id === expandedId)!.name}</h4>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                <thead>
                  <tr style={{ borderBottom: '2px solid #ddd', backgroundColor: '#f0f0f0' }}>
                    <th style={{ padding: '0.5rem', textAlign: 'left' }}>Min</th>
                    <th style={{ padding: '0.5rem', textAlign: 'left' }}>Max</th>
                    <th style={{ padding: '0.5rem', textAlign: 'left' }}>Rate</th>
                  </tr>
                </thead>
                <tbody>
                  {rateTables.find(rt => rt.id === expandedId)!.tiers.map((t, i) => (
                    <tr key={i} style={{ borderBottom: '1px solid #eee' }}>
                      <td style={{ padding: '0.5rem' }}>{t.min}</td>
                      <td style={{ padding: '0.5rem' }}>{t.max}</td>
                      <td style={{ padding: '0.5rem' }}>${t.rate.toFixed(2)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      {/* Create Modal */}
      {showCreate && (
        <Modal title="New Rate Table" onClose={() => { setShowCreate(false); setFormErr(''); }} width={560}>
          {formErr && <div style={{ color: '#dc3545', marginBottom: '0.75rem', fontSize: '0.85rem' }}>{formErr}</div>}
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Name *</label>
            <input value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} style={inputStyle} placeholder="Rate table name" />
          </div>
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Type *</label>
            <select value={form.type} onChange={e => setForm({ ...form, type: e.target.value })} style={selectStyle}>
              <option value="distance">Distance</option>
              <option value="weight">Weight</option>
              <option value="volume">Volume</option>
            </select>
          </div>
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Tiers (JSON Array) *</label>
            <textarea value={form.tiers} onChange={e => setForm({ ...form, tiers: e.target.value })}
              rows={5} style={{ ...inputStyle, fontFamily: 'monospace', resize: 'vertical' }}
              placeholder='[{"min": 0, "max": 10, "rate": 5}]' />
            <div style={{ fontSize: '0.75rem', color: '#999', marginTop: 2 }}>Each tier: {`{"min": number, "max": number, "rate": number}`}</div>
          </div>
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '0.75rem' }}>
            <div>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Fuel Surcharge (%)</label>
              <input type="number" min="0" step="0.1" value={form.fuel_surcharge_pct}
                onChange={e => setForm({ ...form, fuel_surcharge_pct: e.target.value })} style={inputStyle} />
            </div>
            <div>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Effective Date *</label>
              <input type="date" value={form.effective_date}
                onChange={e => setForm({ ...form, effective_date: e.target.value })} style={inputStyle} />
            </div>
          </div>
          <div style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}>
              <input type="checkbox" checked={form.taxable} onChange={e => setForm({ ...form, taxable: e.target.checked })} />
              <span style={{ fontWeight: 500 }}>Taxable</span>
            </label>
          </div>
          <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
            <button onClick={() => { setShowCreate(false); setFormErr(''); }} style={btnSecondary}>Cancel</button>
            <button onClick={handleCreate} disabled={submitting} style={submitting ? btnDisabled : btnPrimary}>
              {submitting ? 'Creating...' : 'Create Rate Table'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default RateTablesPage;
