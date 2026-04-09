import React, { useState, useEffect } from 'react';
import { chargesAPI } from '../../services/api';
import { useDraftAutoSave } from '../../hooks/useDraftAutoSave';
import { DraftRecoveryDialog } from '../common/DraftRecoveryDialog';
import type { ChargeStatement, ChargeLineItem, RateTable, User } from '../../types';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';
import EmptyState from '../common/EmptyState';
import DataTable from '../common/DataTable';
import Modal from '../common/Modal';
import ContextMenu from '../common/ContextMenu';

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
const btnWarning: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#ffc107', color: '#333', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const successStyle: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#d4edda', border: '1px solid #c3e6cb', borderRadius: 4, color: '#155724', marginBottom: '1rem',
};

/** Canonical statement lifecycle: pending → reconciled → approved → paid */
const statusColors: Record<string, { bg: string; color: string }> = {
  pending:    { bg: '#cce5ff', color: '#004085' },
  reconciled: { bg: '#fff3cd', color: '#856404' },
  approved:   { bg: '#d4edda', color: '#155724' },
  paid:       { bg: '#d1ecf1', color: '#0c5460' },
};

const StatementsPage: React.FC = () => {
  // Current user (for approver distinctness check)
  const currentUser: User | null = (() => {
    try { return JSON.parse(localStorage.getItem('medops_user') || 'null'); } catch { return null; }
  })();

  // List state
  const [statements, setStatements] = useState<ChargeStatement[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; stmt: ChargeStatement } | null>(null);

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
  const [genForm, setGenForm] = useState({ period_start: '', period_end: '', rate_table_id: '' });
  const [genLineItems, setGenLineItems] = useState<Array<{ description: string; quantity: string }>>(
    [{ description: '', quantity: '' }]
  );
  const [genErr, setGenErr] = useState('');
  const [genSubmitting, setGenSubmitting] = useState(false);
  const [successMsg, setSuccessMsg] = useState('');

  // Rate tables for the generate-statement dropdown
  const [rateTables, setRateTables] = useState<RateTable[]>([]);
  const [rateTablesLoading, setRateTablesLoading] = useState(false);

  useEffect(() => {
    setRateTablesLoading(true);
    chargesAPI.listRateTables()
      .then(res => setRateTables(res.data.data || res.data || []))
      .catch(() => setRateTables([]))
      .finally(() => setRateTablesLoading(false));
  }, []);

  const { clearDraft: clearGenDraft } = useDraftAutoSave('statement_generate', null, genForm);

  // Detail / expanded view
  const [selectedStatement, setSelectedStatement] = useState<ChargeStatement | null>(null);
  const [lineItems, setLineItems] = useState<ChargeLineItem[]>([]);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');

  // Reconcile form state
  const [showReconcile, setShowReconcile] = useState(false);
  const [reconcileExpected, setReconcileExpected] = useState('');
  const [reconcileNotes, setReconcileNotes] = useState('');
  const [reconcileErr, setReconcileErr] = useState('');

  // Action loading
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
    if (!genForm.rate_table_id) { setGenErr('Rate table is required'); return; }
    const validItems = genLineItems.filter(li => li.description.trim() || li.quantity.trim());
    if (validItems.length === 0) { setGenErr('At least one line item is required'); return; }
    for (const li of validItems) {
      if (!li.description.trim()) { setGenErr('Each line item must have a description'); return; }
      const qty = parseFloat(li.quantity);
      if (isNaN(qty) || qty < 0) { setGenErr('Each line item must have a non-negative quantity'); return; }
    }
    setGenSubmitting(true);
    setGenErr('');
    try {
      await chargesAPI.generateStatement({
        period_start: genForm.period_start,
        period_end: genForm.period_end,
        rate_table_id: genForm.rate_table_id,
        line_items: validItems.map(li => ({ description: li.description.trim(), quantity: parseFloat(li.quantity) })),
      });
      clearGenDraft();
      showSuccess('Statement generated successfully');
      setShowGenerate(false);
      setGenForm({ period_start: '', period_end: '', rate_table_id: '' });
      setGenLineItems([{ description: '', quantity: '' }]);
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

  // Reconcile (pending → reconciled)
  const handleReconcile = async () => {
    if (!selectedStatement) return;
    const expectedNum = parseFloat(reconcileExpected);
    if (!reconcileExpected || isNaN(expectedNum) || expectedNum < 0) {
      setReconcileErr('A valid expected total is required');
      return;
    }
    const variance = Math.abs(selectedStatement.total_amount - expectedNum);
    if (variance > 25 && reconcileNotes.trim().length === 0) {
      setReconcileErr('Variance notes are required when variance exceeds $25');
      return;
    }
    setActionLoading(true);
    setReconcileErr('');
    try {
      await chargesAPI.reconcile(selectedStatement.id, {
        expected_total: expectedNum,
        variance_notes: reconcileNotes.trim() || undefined,
      });
      showSuccess('Statement reconciled — awaiting second-step approval');
      setShowReconcile(false);
      setReconcileExpected('');
      setReconcileNotes('');
      const r = await chargesAPI.getStatement(selectedStatement.id);
      const d = r.data;
      setSelectedStatement(d.statement || d);
      fetchStatements();
    } catch (e: any) {
      setReconcileErr(e.response?.data?.error || 'Reconciliation failed');
    } finally {
      setActionLoading(false);
    }
  };

  // Approve (reconciled → approved), only if different user from reconciler
  const handleApprove = async (overrideId?: string) => {
    const id = overrideId ?? selectedStatement?.id;
    if (!id) return;
    setActionLoading(true);
    try {
      await chargesAPI.approve(id);
      showSuccess('Statement approved');
      const r = await chargesAPI.getStatement(id);
      const d = r.data;
      setSelectedStatement(d.statement || d);
      fetchStatements();
    } catch (e: any) {
      setError(e.response?.data?.error || 'Approval failed');
    } finally {
      setActionLoading(false);
    }
  };

  // Export / Print Record — lightweight CSV export with no status change.
  // Available for any statement regardless of status (context menu entry).
  const handleExportRecord = (stmt: ChargeStatement) => {
    const rows = [
      ['ID', 'Period Start', 'Period End', 'Total Amount', 'Expected Total', 'Status',
       'Approved By 1', 'Approved By 2', 'Reconciled At', 'Variance Notes', 'Paid At', 'Created At'],
      [
        stmt.id,
        new Date(stmt.period_start).toLocaleDateString(),
        new Date(stmt.period_end).toLocaleDateString(),
        stmt.total_amount.toFixed(2),
        stmt.expected_total != null ? String(stmt.expected_total) : '',
        stmt.status,
        stmt.approved_by_1 || '',
        stmt.approved_by_2 || '',
        stmt.reconciled_at ? new Date(stmt.reconciled_at).toLocaleDateString() : '',
        stmt.variance_notes || '',
        stmt.paid_at ? new Date(stmt.paid_at).toLocaleDateString() : '',
        new Date(stmt.created_at).toLocaleDateString(),
      ],
    ];
    const csv = rows.map(r => r.map(c => `"${c}"`).join(',')).join('\n');
    const url = window.URL.createObjectURL(new Blob([csv], { type: 'text/csv' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `statement-record-${stmt.id.slice(0, 8)}.csv`;
    a.click();
    window.URL.revokeObjectURL(url);
  };

  // Export CSV (approved → paid)
  const handleExport = async (overrideId?: string) => {
    const id = overrideId ?? selectedStatement?.id;
    if (!id) return;
    setActionLoading(true);
    try {
      const r = await chargesAPI.exportStatement(id, 'csv');
      const url = window.URL.createObjectURL(new Blob([r.data]));
      const a = document.createElement('a');
      a.href = url;
      a.download = `statement-${id.slice(0, 8)}.csv`;
      a.click();
      window.URL.revokeObjectURL(url);
      showSuccess('Statement exported — status updated to paid');
      const r2 = await chargesAPI.getStatement(id);
      const d = r2.data;
      setSelectedStatement(d.statement || d);
      fetchStatements();
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

  const handleGenDraftRestore = (state: unknown) => {
    const s = state as typeof genForm;
    if (s && typeof s === 'object') {
      setGenForm({
        period_start: (s as any).period_start || '',
        period_end: (s as any).period_end || '',
        rate_table_id: (s as any).rate_table_id || '',
      });
      setShowGenerate(true);
    }
  };

  return (
    <div style={{ padding: '1.5rem' }}>
      <DraftRecoveryDialog
        formType="statement_generate"
        onRestore={handleGenDraftRestore}
        onDiscard={clearGenDraft}
      />
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
          onContextMenu={(stmt, e) => setCtxMenu({ x: e.clientX, y: e.clientY, stmt })}
        />
      )}

      {/* Context menu */}
      {ctxMenu && (
        <ContextMenu
          x={ctxMenu.x}
          y={ctxMenu.y}
          onClose={() => setCtxMenu(null)}
          items={[
            { label: 'View Details', onClick: () => { viewDetail(ctxMenu.stmt); setCtxMenu(null); } },
            ...(ctxMenu.stmt.status === 'pending'
              ? [{ label: 'Reconcile', onClick: () => { viewDetail(ctxMenu.stmt); setShowReconcile(true); setCtxMenu(null); } }]
              : []),
            ...(ctxMenu.stmt.status === 'reconciled'
              ? [{ label: 'Approve', onClick: () => { handleApprove(ctxMenu.stmt.id); setCtxMenu(null); } }]
              : []),
            ...(ctxMenu.stmt.status === 'approved'
              ? [{ label: 'Export / Mark Paid', onClick: () => { handleExport(ctxMenu.stmt.id); setCtxMenu(null); } }]
              : []),
            { label: 'Export / Print Record', onClick: () => { handleExportRecord(ctxMenu.stmt); setCtxMenu(null); } },
          ]}
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
                <div><span style={{ fontWeight: 500, color: '#666' }}>Period:</span> {new Date(selectedStatement.period_start).toLocaleDateString()} – {new Date(selectedStatement.period_end).toLocaleDateString()}</div>
                <div><span style={{ fontWeight: 500, color: '#666' }}>Status:</span>{' '}
                  {(() => {
                    const sc = statusColors[selectedStatement.status] || { bg: '#eee', color: '#333' };
                    return <span style={{ padding: '0.2rem 0.6rem', borderRadius: 12, fontSize: '0.8rem', fontWeight: 600, backgroundColor: sc.bg, color: sc.color, textTransform: 'capitalize' as const }}>{selectedStatement.status}</span>;
                  })()}
                </div>
                <div><span style={{ fontWeight: 500, color: '#666' }}>Total Amount:</span> <strong>${selectedStatement.total_amount.toFixed(2)}</strong></div>
                {selectedStatement.expected_total > 0 && (
                  <div><span style={{ fontWeight: 500, color: '#666' }}>Expected Total:</span> ${selectedStatement.expected_total.toFixed(2)}</div>
                )}
                <div><span style={{ fontWeight: 500, color: '#666' }}>Created:</span> {new Date(selectedStatement.created_at).toLocaleString()}</div>
                {selectedStatement.reconciled_at && <div><span style={{ fontWeight: 500, color: '#666' }}>Reconciled At:</span> {new Date(selectedStatement.reconciled_at).toLocaleString()}</div>}
                {selectedStatement.approved_by_1 && <div><span style={{ fontWeight: 500, color: '#666' }}>Reconciled By:</span> {selectedStatement.approved_by_1}</div>}
                {selectedStatement.approved_by_2 && <div><span style={{ fontWeight: 500, color: '#666' }}>Approved By:</span> {selectedStatement.approved_by_2}</div>}
                {selectedStatement.variance_notes && <div style={{ gridColumn: '1 / -1' }}><span style={{ fontWeight: 500, color: '#666' }}>Variance Notes:</span> {selectedStatement.variance_notes}</div>}
                {selectedStatement.paid_at && <div><span style={{ fontWeight: 500, color: '#666' }}>Paid At:</span> {new Date(selectedStatement.paid_at).toLocaleString()}</div>}
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
                {/* Reconcile: only available from pending */}
                {selectedStatement.status === 'pending' && (
                  <button onClick={() => setShowReconcile(true)} style={btnWarning} disabled={actionLoading}>
                    Reconcile
                  </button>
                )}
                {/* Approve: only available from reconciled, and only if the current user is not the reconciler */}
                {selectedStatement.status === 'reconciled' && currentUser && selectedStatement.approved_by_1 !== currentUser.id && (
                  <button onClick={() => handleApprove()} style={btnPrimary} disabled={actionLoading}>
                    {actionLoading ? 'Approving...' : 'Approve'}
                  </button>
                )}
                {selectedStatement.status === 'reconciled' && currentUser && selectedStatement.approved_by_1 === currentUser.id && (
                  <span style={{ color: '#856404', fontSize: '0.9rem', alignSelf: 'center' }}>Awaiting approval by a different user</span>
                )}
                {/* Export CSV: only available from approved */}
                {selectedStatement.status === 'approved' && (
                  <button onClick={() => handleExport()} style={btnPrimary} disabled={actionLoading}>
                    {actionLoading ? 'Exporting...' : 'Export CSV'}
                  </button>
                )}
              </div>

              {/* Reconcile Form */}
              {showReconcile && selectedStatement.status === 'pending' && (
                <div style={{ marginTop: '1rem', padding: '1rem', backgroundColor: '#fff', borderRadius: 4, border: '1px solid #e0e0e0' }}>
                  <h4 style={{ margin: '0 0 0.5rem' }}>Reconcile Statement</h4>
                  {reconcileErr && <div style={{ color: '#dc3545', marginBottom: '0.5rem', fontSize: '0.85rem' }}>{reconcileErr}</div>}
                  <p style={{ fontSize: '0.85rem', color: '#666', margin: '0 0 0.75rem' }}>
                    System total: <strong>${selectedStatement.total_amount.toFixed(2)}</strong>. Enter the expected total to compute variance. Notes are required if variance exceeds $25.
                  </p>
                  <div style={{ marginBottom: '0.75rem' }}>
                    <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Expected Total *</label>
                    <input
                      type="number"
                      min="0"
                      step="0.01"
                      value={reconcileExpected}
                      onChange={e => setReconcileExpected(e.target.value)}
                      style={inputStyle}
                      placeholder="0.00"
                    />
                  </div>
                  <div style={{ marginBottom: '0.75rem' }}>
                    <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Variance Notes {reconcileExpected && Math.abs(selectedStatement.total_amount - parseFloat(reconcileExpected || '0')) > 25 ? '*' : ''}</label>
                    <textarea
                      value={reconcileNotes}
                      onChange={e => setReconcileNotes(e.target.value)}
                      rows={3}
                      style={{ ...inputStyle, resize: 'vertical' }}
                      placeholder="Enter variance notes..."
                    />
                  </div>
                  <div style={{ display: 'flex', gap: '0.5rem' }}>
                    <button onClick={handleReconcile} disabled={actionLoading} style={actionLoading ? btnDisabled : btnWarning}>
                      {actionLoading ? 'Processing...' : 'Confirm Reconcile'}
                    </button>
                    <button onClick={() => { setShowReconcile(false); setReconcileErr(''); setReconcileExpected(''); setReconcileNotes(''); }} style={btnSecondary}>Cancel</button>
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      )}

      {/* Generate Statement Modal */}
      {showGenerate && (
        <Modal title="Generate Statement" onClose={() => { setShowGenerate(false); setGenErr(''); setGenLineItems([{ description: '', quantity: '' }]); }} width={560}>
          {genErr && <div style={{ color: '#dc3545', marginBottom: '0.75rem', fontSize: '0.85rem' }}>{genErr}</div>}
          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '0.75rem' }}>
            <div>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Period Start *</label>
              <input type="date" value={genForm.period_start} onChange={e => setGenForm({ ...genForm, period_start: e.target.value })} style={inputStyle} />
            </div>
            <div>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Period End *</label>
              <input type="date" value={genForm.period_end} onChange={e => setGenForm({ ...genForm, period_end: e.target.value })} style={inputStyle} />
            </div>
          </div>
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Rate Table *</label>
            {rateTablesLoading ? (
              <div style={{ fontSize: '0.85rem', color: '#666' }}>Loading rate tables…</div>
            ) : (
              <select value={genForm.rate_table_id} onChange={e => setGenForm({ ...genForm, rate_table_id: e.target.value })} style={{ ...inputStyle, appearance: 'auto' as const }}>
                <option value="">— Select a rate table —</option>
                {rateTables.map(rt => (
                  <option key={rt.id} value={rt.id}>{rt.name} ({rt.type})</option>
                ))}
              </select>
            )}
          </div>
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.5rem', fontWeight: 500 }}>Line Items *</label>
            {genLineItems.map((li, idx) => (
              <div key={idx} style={{ display: 'grid', gridTemplateColumns: '1fr 120px 32px', gap: '0.4rem', marginBottom: '0.4rem', alignItems: 'center' }}>
                <input
                  placeholder="Description"
                  value={li.description}
                  onChange={e => {
                    const updated = [...genLineItems];
                    updated[idx] = { ...updated[idx], description: e.target.value };
                    setGenLineItems(updated);
                  }}
                  style={inputStyle}
                />
                <input
                  type="number"
                  min="0"
                  step="any"
                  placeholder="Quantity"
                  value={li.quantity}
                  onChange={e => {
                    const updated = [...genLineItems];
                    updated[idx] = { ...updated[idx], quantity: e.target.value };
                    setGenLineItems(updated);
                  }}
                  style={inputStyle}
                />
                <button
                  onClick={() => setGenLineItems(genLineItems.filter((_, i) => i !== idx))}
                  disabled={genLineItems.length === 1}
                  style={{ padding: '0.4rem', background: '#dc3545', color: '#fff', border: 'none', borderRadius: 4, cursor: genLineItems.length === 1 ? 'not-allowed' : 'pointer', opacity: genLineItems.length === 1 ? 0.4 : 1 }}
                  title="Remove"
                >×</button>
              </div>
            ))}
            <button
              onClick={() => setGenLineItems([...genLineItems, { description: '', quantity: '' }])}
              style={{ ...btnSecondary, fontSize: '0.8rem', padding: '0.3rem 0.75rem', marginTop: '0.25rem' }}
            >+ Add Line Item</button>
          </div>
          <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
            <button onClick={() => { setShowGenerate(false); setGenErr(''); setGenLineItems([{ description: '', quantity: '' }]); }} style={btnSecondary}>Cancel</button>
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
