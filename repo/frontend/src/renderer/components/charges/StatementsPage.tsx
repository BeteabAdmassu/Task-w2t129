import React, { useState, useEffect } from 'react';
import { chargesAPI } from '../../services/api';
import type { ChargeStatement, ChargeLineItem } from '../../types';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';
import EmptyState from '../common/EmptyState';
import DataTable from '../common/DataTable';
import Modal from '../common/Modal';
import { useAuth } from '../../hooks/useAuth';

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
const btnSuccess: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#28a745', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const btnWarning: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#ffc107', color: '#333', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const successStyle: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#d4edda', border: '1px solid #c3e6cb', borderRadius: 4, color: '#155724', marginBottom: '1rem',
};

const statusColors: Record<string, { bg: string; color: string }> = {
  draft: { bg: '#e0e0e0', color: '#555' },
  generated: { bg: '#cce5ff', color: '#004085' },
  reconciled: { bg: '#fff3cd', color: '#856404' },
  approved: { bg: '#d4edda', color: '#155724' },
  exported: { bg: '#d1ecf1', color: '#0c5460' },
};

const StatementsPage: React.FC = () => {
  const { user } = useAuth();

  // List state
  const [statements, setStatements] = useState<ChargeStatement[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchStatements = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await chargesAPI.listStatements({ page: 1, page_size: 50 });
      const payload = res.data;
      setStatements(payload.data || payload);
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to load statements');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchStatements();
  }, []);

  // Generate form
  const [showGenerate, setShowGenerate] = useState(false);
  const [genForm, setGenForm] = useState({ period_start: '', period_end: '' });
  const [genErr, setGenErr] = useState('');
  const [genSubmitting, setGenSubmitting] = useState(false);
  const [successMsg, setSuccessMsg] = useState('');

  // Detail / expanded view
  const [selectedStatement, setSelectedStatement] = useState<ChargeStatement | null>(null);
  const [lineItems, setLineItems] = useState<ChargeLineItem[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');

  // Reconcile
  const [showReconcile, setShowReconcile] = useState(false);
  const [reconcileNotes, setReconcileNotes] = useState('');
  const [reconcileErr, setReconcileErr] = useState('');

  // Action loading (reconcile, approve, export)
  const [actionLoading, setActionLoading] = useState(false);

  const showSuccess = (msg: string) => {
    setSuccessMsg(msg);
    setTimeout(() => setSuccessMsg(''), 3000);
  };

  // Generate statement
  const handleGenerate = async () => {
    if (!genForm.period_start) { setGenErr('Start date is required'); return; }
    if (!genForm.period_end) { setGenErr('End date is required'); return; }
    if (new Date(genForm.period_end) <= new Date(genForm.period_start)) { setGenErr('End date must be after start date'); return; }
    setGenSubmitting(true);
    setGenErr('');
    try {
      await chargesAPI.generateStatement({ period_start: genForm.period_start, period_end: genForm.period_end });
      showSuccess('Statement generated successfully');
      setShowGenerate(false);
      setGenForm({ period_start: '', period_end: '' });
      fetchStatements();
    } catch (e: any) {
      setGenErr(e.response?.data?.error || 'Failed to generate statement');
    } finally {
      setGenSubmitting(false);
    }
  };

  // View detail (click row to expand)
  const viewDetail = async (stmt: ChargeStatement) => {
    if (selectedStatement?.id === stmt.id) {
      setSelectedStatement(null);
      setLineItems([]);
      return;
    }
    setSelectedStatement(stmt);
    setDetailLoading(true);
    setDetailError('');
    setShowReconcile(false);
    try {
      const r = await chargesAPI.getStatement(stmt.id);
      const d = r.data;
      setSelectedStatement(d.statement || d);
      setLineItems(d.line_items || d.lineItems || d.items || []);
    } catch (e: any) {
      setDetailError(e.response?.data?.error || 'Failed to load statement details');
    } finally {
      setDetailLoading(false);
    }
  };

  // Reconcile
  const handleReconcile = async () => {
    if (!selectedStatement) return;
    if (selectedStatement.total_amount > 25 && reconcileNotes.trim().length === 0) {
      setReconcileErr('Variance notes are required when total exceeds $25');
      return;
    }
    if (reconcileNotes.trim().length === 0) {
      setReconcileErr('Variance notes are required');
      return;
    }
    setActionLoading(true);
    setReconcileErr('');
    try {
      await chargesAPI.reconcile(selectedStatement.id, { variance_notes: reconcileNotes.trim() });
      showSuccess('Statement reconciled');
      setShowReconcile(false);
      setReconcileNotes('');
      viewDetail(selectedStatement);
      fetchStatements();
    } catch (e: any) {
      setReconcileErr(e.response?.data?.error || 'Reconciliation failed');
    } finally {
      setActionLoading(false);
    }
  };

  // Determine approval step label and whether current user already approved
  const getApproveInfo = (stmt: ChargeStatement) => {
    const userId = user?.id || '';
    const alreadyApproved = stmt.approved_by_1 === userId || stmt.approved_by_2 === userId;
    let stepLabel = 'Approve - Step 1';
    if (stmt.approved_by_1 && !stmt.approved_by_2) {
      stepLabel = 'Approve - Step 2';
    }
    return { alreadyApproved, stepLabel };
  };

  // Approve
  const handleApprove = async () => {
    if (!selectedStatement) return;
    setActionLoading(true);
    try {
      await chargesAPI.approve(selectedStatement.id);
      showSuccess('Statement approved');
      // Refresh detail and list
      const r = await chargesAPI.getStatement(selectedStatement.id);
      const d = r.data;
      setSelectedStatement(d.statement || d);
      setLineItems(d.line_items || d.lineItems || d.items || []);
      fetchStatements();
    } catch (e: any) {
      setError(e.response?.data?.error || 'Approval failed');
    } finally {
      setActionLoading(false);
    }
  };

  // Export CSV
  const handleExport = async () => {
    if (!selectedStatement) return;
    setActionLoading(true);
    try {
      const r = await chargesAPI.exportStatement(selectedStatement.id);
      const url = window.URL.createObjectURL(new Blob([r.data]));
      const a = document.createElement('a');
      a.href = url;
      a.download = `statement-${selectedStatement.id.slice(0, 8)}.csv`;
      a.click();
      window.URL.revokeObjectURL(url);
      showSuccess('Statement exported as CSV');
    } catch (e: any) {
      setError(e.response?.data?.error || 'Export failed');
    } finally {
      setActionLoading(false);
    }
  };

  const columns = [
    { key: 'period_start', header: 'Period Start', sortable: true, render: (s: ChargeStatement) => new Date(s.period_start).toLocaleDateString() },
    { key: 'period_end', header: 'Period End', sortable: true, render: (s: ChargeStatement) => new Date(s.period_end).toLocaleDateString() },
    { key: 'total_amount', header: 'Total', sortable: true, render: (s: ChargeStatement) => `$${s.total_amount.toFixed(2)}` },
    {
      key: 'status', header: 'Status', sortable: true,
      render: (s: ChargeStatement) => {
        const sc = statusColors[s.status] || { bg: '#eee', color: '#333' };
        return <span style={{ padding: '0.2rem 0.6rem', borderRadius: 12, fontSize: '0.8rem', fontWeight: 600, backgroundColor: sc.bg, color: sc.color, textTransform: 'capitalize' as const }}>{s.status}</span>;
      },
    },
    { key: 'created_at', header: 'Created', sortable: true, render: (s: ChargeStatement) => new Date(s.created_at).toLocaleDateString() },
  ];

  const lineItemColumns = [
    { key: 'description', header: 'Description' },
    { key: 'quantity', header: 'Quantity', render: (li: ChargeLineItem) => li.quantity.toFixed(2) },
    { key: 'unit_price', header: 'Unit Price', render: (li: ChargeLineItem) => `$${li.unit_price.toFixed(2)}` },
    { key: 'surcharge', header: 'Surcharge', render: (li: ChargeLineItem) => `$${li.surcharge.toFixed(2)}` },
    { key: 'tax', header: 'Tax', render: (li: ChargeLineItem) => `$${li.tax.toFixed(2)}` },
    { key: 'total', header: 'Total', render: (li: ChargeLineItem) => <strong>${li.total.toFixed(2)}</strong> },
  ];

  return (
    <div style={{ padding: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h2 style={{ margin: 0 }}>Statements</h2>
        <button onClick={() => setShowGenerate(true)} style={btnPrimary}>+ Generate Statement</button>
      </div>

      {successMsg && <div style={successStyle}>{successMsg}</div>}

      {loading && <LoadingSpinner />}
      {error && <ErrorMessage message={error} onRetry={fetchStatements} />}
      {!loading && !error && statements.length === 0 && (
        <EmptyState message="No statements found" actionLabel="Generate Statement" onAction={() => setShowGenerate(true)} />
      )}
      {!loading && !error && statements.length > 0 && (
        <DataTable<ChargeStatement>
          columns={columns}
          data={statements}
          onRowClick={viewDetail}
        />
      )}

      {/* Expanded Detail */}
      {selectedStatement && (
        <div style={{ margin: '0.5rem 0 1rem', padding: '1.5rem', backgroundColor: '#f9f9f9', borderRadius: 8, border: '1px solid #e0e0e0' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
            <h3 style={{ margin: 0 }}>Statement #{selectedStatement.id.slice(0, 8)}</h3>
            <button onClick={() => { setSelectedStatement(null); setLineItems([]); }} style={{ background: 'none', border: 'none', fontSize: '1.3rem', cursor: 'pointer', color: '#666' }}>&times;</button>
          </div>

          {detailLoading && <LoadingSpinner message="Loading details..." />}
          {detailError && <ErrorMessage message={detailError} />}
          {!detailLoading && !detailError && (
            <>
              {/* Statement Info */}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '1.5rem' }}>
                <div><span style={{ fontWeight: 500, color: '#666' }}>Period:</span> {new Date(selectedStatement.period_start).toLocaleDateString()} - {new Date(selectedStatement.period_end).toLocaleDateString()}</div>
                <div><span style={{ fontWeight: 500, color: '#666' }}>Status:</span>{' '}
                  {(() => {
                    const sc = statusColors[selectedStatement.status] || { bg: '#eee', color: '#333' };
                    return <span style={{ padding: '0.2rem 0.6rem', borderRadius: 12, fontSize: '0.8rem', fontWeight: 600, backgroundColor: sc.bg, color: sc.color, textTransform: 'capitalize' as const }}>{selectedStatement.status}</span>;
                  })()}
                </div>
                <div><span style={{ fontWeight: 500, color: '#666' }}>Total Amount:</span> <strong>${selectedStatement.total_amount.toFixed(2)}</strong></div>
                <div><span style={{ fontWeight: 500, color: '#666' }}>Created:</span> {new Date(selectedStatement.created_at).toLocaleString()}</div>
                {selectedStatement.approved_by_1 && <div><span style={{ fontWeight: 500, color: '#666' }}>Approved By (Step 1):</span> {selectedStatement.approved_by_1}</div>}
                {selectedStatement.approved_by_2 && <div><span style={{ fontWeight: 500, color: '#666' }}>Approved By (Step 2):</span> {selectedStatement.approved_by_2}</div>}
                {selectedStatement.variance_notes && <div style={{ gridColumn: '1 / -1' }}><span style={{ fontWeight: 500, color: '#666' }}>Variance Notes:</span> {selectedStatement.variance_notes}</div>}
                {selectedStatement.exported_at && <div><span style={{ fontWeight: 500, color: '#666' }}>Exported:</span> {new Date(selectedStatement.exported_at).toLocaleString()}</div>}
              </div>

              {/* Line Items */}
              <h4 style={{ margin: '0 0 0.75rem' }}>Line Items</h4>
              {lineItems.length === 0 ? (
                <div style={{ color: '#999', textAlign: 'center', padding: '1rem' }}>No line items</div>
              ) : (
                <DataTable<ChargeLineItem> columns={lineItemColumns} data={lineItems} />
              )}

              {/* Actions */}
              <div style={{ display: 'flex', gap: '0.5rem', marginTop: '1.5rem', flexWrap: 'wrap', borderTop: '1px solid #eee', paddingTop: '1rem' }}>
                {/* Reconcile */}
                {(selectedStatement.status === 'generated' || selectedStatement.status === 'draft') && (
                  <button onClick={() => setShowReconcile(true)} style={btnWarning} disabled={actionLoading}>Reconcile</button>
                )}
                {/* Approve */}
                {selectedStatement.status === 'reconciled' && (() => {
                  const { alreadyApproved, stepLabel } = getApproveInfo(selectedStatement);
                  return (
                    <button onClick={handleApprove} style={alreadyApproved ? btnDisabled : btnSuccess} disabled={actionLoading || alreadyApproved}
                      title={alreadyApproved ? 'You have already approved this statement' : ''}>
                      {actionLoading ? 'Processing...' : stepLabel}
                    </button>
                  );
                })()}
                {/* Export CSV */}
                {(selectedStatement.status === 'approved' || selectedStatement.status === 'reconciled' || selectedStatement.status === 'exported') && (
                  <button onClick={handleExport} style={btnPrimary} disabled={actionLoading}>
                    {actionLoading ? 'Exporting...' : 'Export CSV'}
                  </button>
                )}
              </div>

              {/* Reconcile Form */}
              {showReconcile && (
                <div style={{ marginTop: '1rem', padding: '1rem', backgroundColor: '#fff', borderRadius: 4, border: '1px solid #e0e0e0' }}>
                  <h4 style={{ margin: '0 0 0.5rem' }}>Reconcile Statement</h4>
                  {reconcileErr && <div style={{ color: '#dc3545', marginBottom: '0.5rem', fontSize: '0.85rem' }}>{reconcileErr}</div>}
                  <p style={{ fontSize: '0.85rem', color: '#666', margin: '0 0 0.75rem' }}>
                    {selectedStatement.total_amount > 25 ? 'Total exceeds $25 -- variance notes are required.' : 'Provide reconciliation notes.'}
                  </p>
                  <textarea value={reconcileNotes} onChange={e => setReconcileNotes(e.target.value)}
                    rows={3} style={{ ...inputStyle, resize: 'vertical', marginBottom: '0.75rem' }}
                    placeholder="Enter variance notes..." />
                  <div style={{ display: 'flex', gap: '0.5rem' }}>
                    <button onClick={handleReconcile} disabled={actionLoading} style={actionLoading ? btnDisabled : btnWarning}>
                      {actionLoading ? 'Reconciling...' : 'Confirm Reconcile'}
                    </button>
                    <button onClick={() => { setShowReconcile(false); setReconcileErr(''); setReconcileNotes(''); }} style={btnSecondary}>Cancel</button>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      )}

      {/* Generate Statement Modal */}
      {showGenerate && (
        <Modal title="Generate Statement" onClose={() => { setShowGenerate(false); setGenErr(''); }} width={420}>
          {genErr && <div style={{ color: '#dc3545', marginBottom: '0.75rem', fontSize: '0.85rem' }}>{genErr}</div>}
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Period Start *</label>
            <input type="date" value={genForm.period_start} onChange={e => setGenForm({ ...genForm, period_start: e.target.value })} style={inputStyle} />
          </div>
          <div style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Period End *</label>
            <input type="date" value={genForm.period_end} onChange={e => setGenForm({ ...genForm, period_end: e.target.value })} style={inputStyle} />
          </div>
          <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
            <button onClick={() => { setShowGenerate(false); setGenErr(''); }} style={btnSecondary}>Cancel</button>
            <button onClick={handleGenerate} disabled={genSubmitting} style={genSubmitting ? btnDisabled : btnPrimary}>
              {genSubmitting ? 'Generating...' : 'Generate'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default StatementsPage;
