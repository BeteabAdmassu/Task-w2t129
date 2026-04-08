import React, { useState, useEffect, FormEvent } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { skusAPI, inventoryAPI } from '../../services/api';
import { useFetch } from '../../hooks/useFetch';
import { useDraftAutoSave } from '../../hooks/useDraftAutoSave';
import { DraftRecoveryDialog } from '../common/DraftRecoveryDialog';
import type { SKU, InventoryBatch, StockTransaction, PaginatedResponse } from '../../types';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';
import EmptyState from '../common/EmptyState';
import DataTable from '../common/DataTable';
import Pagination from '../common/Pagination';
import ContextMenu from '../common/ContextMenu';

const REASON_CODES_IN = ['purchase_order', 'return', 'adjustment', 'donation', 'transfer_in'];
const REASON_CODES_OUT = ['dispensed', 'expired', 'damaged', 'adjustment', 'transfer_out'];

const cardStyle: React.CSSProperties = {
  backgroundColor: '#fff',
  borderRadius: 8,
  padding: '1.25rem',
  boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
  border: '1px solid #e0e0e0',
  marginBottom: '1.5rem',
};

const inputStyle = (hasError: boolean): React.CSSProperties => ({
  width: '100%',
  padding: '0.6rem',
  fontSize: '0.9rem',
  border: `1px solid ${hasError ? '#dc3545' : '#ccc'}`,
  borderRadius: 4,
  boxSizing: 'border-box',
});

const btnPrimary: React.CSSProperties = {
  padding: '0.5rem 1.25rem',
  backgroundColor: '#1a237e',
  color: '#fff',
  border: 'none',
  borderRadius: 4,
  cursor: 'pointer',
  fontWeight: 500,
  fontSize: '0.9rem',
};

const SKUDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  // Fetch SKU
  const { data: sku, loading: skuLoading, error: skuError, refetch: refetchSku } = useFetch<SKU>(
    () => skusAPI.get(id!),
    [id]
  );

  // Fetch batches
  const { data: batchesData, loading: batchesLoading, error: batchesError, refetch: refetchBatches } = useFetch<InventoryBatch[] | { data: InventoryBatch[] }>(
    () => skusAPI.getBatches(id!),
    [id]
  );
  const batches: InventoryBatch[] = Array.isArray(batchesData) ? batchesData : (batchesData as any)?.data || [];

  // Fetch transactions
  const [batchCtxMenu, setBatchCtxMenu] = useState<{ x: number; y: number; batch: InventoryBatch } | null>(null);
  const [txPage, setTxPage] = useState(1);
  const txPageSize = 15;
  const { data: txData, loading: txLoading, error: txError, refetch: refetchTx } = useFetch<PaginatedResponse<StockTransaction>>(
    () => inventoryAPI.transactions({ sku_id: id, page: txPage, page_size: txPageSize }),
    [id, txPage]
  );
  const transactions = txData?.data || [];
  const txTotal = txData?.total || 0;

  // Receive form
  const [receiveForm, setReceiveForm] = useState({
    lot_number: '', expiration_date: '', quantity: 1, reason_code: REASON_CODES_IN[0],
  });
  const [receiveErrors, setReceiveErrors] = useState<Record<string, string>>({});
  const [receiveLoading, setReceiveLoading] = useState(false);
  const [receiveSuccess, setReceiveSuccess] = useState('');
  // Draft auto-save: preserves in-progress receive entry if the page is closed unexpectedly
  const { clearDraft: clearReceiveDraft } = useDraftAutoSave('sku_receive', id ?? null, receiveForm);

  // Dispense form
  const [dispenseForm, setDispenseForm] = useState({
    batch_id: '', quantity: 1, reason_code: REASON_CODES_OUT[0], prescription_id: '',
  });
  const [dispenseErrors, setDispenseErrors] = useState<Record<string, string>>({});
  const [dispenseLoading, setDispenseLoading] = useState(false);
  const [dispenseSuccess, setDispenseSuccess] = useState('');
  // Draft auto-save: preserves in-progress dispense entry if the page is closed unexpectedly
  const { clearDraft: clearDispenseDraft } = useDraftAutoSave('sku_dispense', id ?? null, dispenseForm);

  // Set default batch when batches load
  useEffect(() => {
    if (batches.length > 0 && !dispenseForm.batch_id) {
      setDispenseForm((p) => ({ ...p, batch_id: batches[0].id }));
    }
  }, [batches]);

  const validateReceive = (): boolean => {
    const errs: Record<string, string> = {};
    if (!receiveForm.lot_number.trim()) errs.lot_number = 'Lot number is required';
    if (!receiveForm.expiration_date) errs.expiration_date = 'Expiration date is required';
    if (receiveForm.quantity < 1) errs.quantity = 'Quantity must be at least 1';
    if (!receiveForm.reason_code) errs.reason_code = 'Reason code is required';
    setReceiveErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleReceive = async (e: FormEvent) => {
    e.preventDefault();
    if (!validateReceive()) return;
    setReceiveLoading(true);
    setReceiveSuccess('');
    try {
      await inventoryAPI.receive({
        sku_id: id,
        lot_number: receiveForm.lot_number.trim(),
        expiration_date: receiveForm.expiration_date,
        quantity: receiveForm.quantity,
        reason_code: receiveForm.reason_code,
      });
      setReceiveSuccess(`Received ${receiveForm.quantity} units successfully`);
      clearReceiveDraft();
      setReceiveForm({ lot_number: '', expiration_date: '', quantity: 1, reason_code: REASON_CODES_IN[0] });
      setReceiveErrors({});
      refetchBatches();
      refetchTx();
      setTimeout(() => setReceiveSuccess(''), 4000);
    } catch (err: any) {
      setReceiveErrors({ _form: err.response?.data?.error || 'Failed to receive inventory' });
    } finally {
      setReceiveLoading(false);
    }
  };

  const validateDispense = (): boolean => {
    const errs: Record<string, string> = {};
    if (!dispenseForm.batch_id) errs.batch_id = 'Select a batch';
    if (dispenseForm.quantity < 1) errs.quantity = 'Quantity must be at least 1';
    const selectedBatch = batches.find((b) => b.id === dispenseForm.batch_id);
    if (selectedBatch && dispenseForm.quantity > selectedBatch.quantity_on_hand) {
      errs.quantity = `Cannot dispense more than ${selectedBatch.quantity_on_hand} on hand`;
    }
    if (!dispenseForm.reason_code) errs.reason_code = 'Reason code is required';
    setDispenseErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleDispense = async (e: FormEvent) => {
    e.preventDefault();
    if (!validateDispense()) return;
    setDispenseLoading(true);
    setDispenseSuccess('');
    try {
      await inventoryAPI.dispense({
        sku_id: id,
        batch_id: dispenseForm.batch_id,
        quantity: dispenseForm.quantity,
        reason_code: dispenseForm.reason_code,
        prescription_id: dispenseForm.prescription_id.trim() || undefined,
      });
      setDispenseSuccess(`Dispensed ${dispenseForm.quantity} units successfully`);
      clearDispenseDraft();
      setDispenseForm((p) => ({ ...p, quantity: 1, prescription_id: '' }));
      setDispenseErrors({});
      refetchBatches();
      refetchTx();
      setTimeout(() => setDispenseSuccess(''), 4000);
    } catch (err: any) {
      setDispenseErrors({ _form: err.response?.data?.error || 'Failed to dispense inventory' });
    } finally {
      setDispenseLoading(false);
    }
  };

  if (skuLoading) return <LoadingSpinner message="Loading SKU details..." />;
  if (skuError) return <ErrorMessage message={skuError} onRetry={refetchSku} />;
  if (!sku) return <ErrorMessage message="SKU not found" />;

  const batchColumns = [
    { key: 'lot_number', header: 'Lot Number', sortable: true },
    {
      key: 'expiration_date', header: 'Expiration', sortable: true,
      render: (b: InventoryBatch) => {
        const exp = new Date(b.expiration_date);
        const isExpired = exp < new Date();
        const isExpiringSoon = !isExpired && exp < new Date(Date.now() + 90 * 86400000);
        return (
          <span style={{ color: isExpired ? '#c62828' : isExpiringSoon ? '#e65100' : '#333', fontWeight: isExpired || isExpiringSoon ? 600 : 400 }}>
            {exp.toLocaleDateString()}
            {isExpired && ' (EXPIRED)'}
            {isExpiringSoon && ' (Expiring soon)'}
          </span>
        );
      },
    },
    { key: 'quantity_on_hand', header: 'Qty on Hand', sortable: true },
  ];

  const txColumns = [
    {
      key: 'type', header: 'Type',
      render: (t: StockTransaction) => (
        <span style={{
          padding: '0.15rem 0.5rem',
          borderRadius: 12,
          fontSize: '0.75rem',
          fontWeight: 600,
          backgroundColor: t.type === 'in' ? '#e8f5e9' : '#fce4ec',
          color: t.type === 'in' ? '#2e7d32' : '#c62828',
        }}>
          {t.type === 'in' ? 'IN' : 'OUT'}
        </span>
      ),
    },
    { key: 'quantity', header: 'Qty', sortable: true },
    { key: 'reason_code', header: 'Reason', render: (t: StockTransaction) => t.reason_code.replace(/_/g, ' ') },
    { key: 'prescription_id', header: 'Rx ID', render: (t: StockTransaction) => t.prescription_id || '-' },
    { key: 'performed_by', header: 'By' },
    { key: 'created_at', header: 'Date', sortable: true, render: (t: StockTransaction) => new Date(t.created_at).toLocaleString() },
  ];

  const handleReceiveDraftRestore = (state: unknown) => {
    const s = state as typeof receiveForm;
    if (s && typeof s === 'object') {
      setReceiveForm({
        lot_number: (s as any).lot_number || '',
        expiration_date: (s as any).expiration_date || '',
        quantity: (s as any).quantity ?? 1,
        reason_code: (s as any).reason_code || REASON_CODES_IN[0],
      });
    }
  };

  const handleDispenseDraftRestore = (state: unknown) => {
    const s = state as typeof dispenseForm;
    if (s && typeof s === 'object') {
      setDispenseForm((p) => ({
        ...p,
        quantity: (s as any).quantity ?? 1,
        reason_code: (s as any).reason_code || REASON_CODES_OUT[0],
        prescription_id: (s as any).prescription_id || '',
        // batch_id comes from live batch data; keep existing selection
      }));
    }
  };

  return (
    <div>
      {/* Draft recovery dialogs for inline receive/dispense forms */}
      <DraftRecoveryDialog
        formType="sku_receive"
        formId={id}
        onRestore={handleReceiveDraftRestore}
        onDiscard={clearReceiveDraft}
      />
      <DraftRecoveryDialog
        formType="sku_dispense"
        formId={id}
        onRestore={handleDispenseDraftRestore}
        onDiscard={clearDispenseDraft}
      />
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginBottom: '1.5rem' }}>
        <button
          onClick={() => navigate('/skus')}
          style={{ padding: '0.4rem 0.8rem', border: '1px solid #ccc', borderRadius: 4, backgroundColor: '#fff', cursor: 'pointer', fontSize: '0.85rem' }}
        >
          &larr; Back to SKUs
        </button>
        <div>
          <h1 style={{ margin: 0, fontSize: '1.5rem', color: '#333' }}>{sku.name}</h1>
          <span style={{ fontSize: '0.85rem', color: '#666' }}>
            {sku.ndc && `NDC: ${sku.ndc}`}{sku.ndc && sku.upc && ' | '}{sku.upc && `UPC: ${sku.upc}`}
          </span>
        </div>
        <span style={{
          marginLeft: 'auto',
          padding: '0.2rem 0.75rem',
          borderRadius: 12,
          fontSize: '0.85rem',
          backgroundColor: sku.is_active ? '#e8f5e9' : '#fdecea',
          color: sku.is_active ? '#2e7d32' : '#c62828',
        }}>
          {sku.is_active ? 'Active' : 'Inactive'}
        </span>
      </div>

      {/* SKU Info */}
      <div style={cardStyle}>
        <h3 style={{ margin: '0 0 0.75rem', fontSize: '1rem' }}>SKU Information</h3>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '0.75rem', fontSize: '0.9rem' }}>
          <div><span style={{ color: '#666' }}>Description:</span> <span>{sku.description || '-'}</span></div>
          <div><span style={{ color: '#666' }}>Unit:</span> <span>{sku.unit_of_measure}</span></div>
          <div><span style={{ color: '#666' }}>Storage:</span> <span>{sku.storage_location}</span></div>
          <div><span style={{ color: '#666' }}>Low Stock Threshold:</span> <span>{sku.low_stock_threshold}</span></div>
          <div><span style={{ color: '#666' }}>Created:</span> <span>{new Date(sku.created_at).toLocaleDateString()}</span></div>
        </div>
      </div>

      {/* Batches */}
      <div style={cardStyle}>
        <h3 style={{ margin: '0 0 0.75rem', fontSize: '1rem' }}>Inventory Batches</h3>
        {batchesLoading && <LoadingSpinner message="Loading batches..." />}
        {batchesError && <ErrorMessage message={batchesError} onRetry={refetchBatches} />}
        {!batchesLoading && !batchesError && batches.length === 0 && (
          <EmptyState message="No batches found for this SKU" />
        )}
        {!batchesLoading && !batchesError && batches.length > 0 && (
          <DataTable columns={batchColumns} data={batches}
            onContextMenu={(batch, e) => setBatchCtxMenu({ x: e.clientX, y: e.clientY, batch })}
          />
        )}
      </div>

      {/* Batch context menu */}
      {batchCtxMenu && (
        <ContextMenu
          x={batchCtxMenu.x}
          y={batchCtxMenu.y}
          onClose={() => setBatchCtxMenu(null)}
          items={[
            {
              label: 'Dispense from this Batch',
              onClick: () => {
                setDispenseForm((f) => ({ ...f, batch_id: batchCtxMenu.batch.id }));
                setBatchCtxMenu(null);
              },
            },
            {
              label: `Expires: ${batchCtxMenu.batch.expiration_date ? new Date(batchCtxMenu.batch.expiration_date).toLocaleDateString() : 'N/A'}`,
              onClick: () => setBatchCtxMenu(null),
            },
          ]}
        />
      )}

      {/* Forms side by side */}
      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1.5rem', marginBottom: '1.5rem' }}>
        {/* Receive form */}
        <div style={cardStyle}>
          <h3 style={{ margin: '0 0 1rem', fontSize: '1rem', color: '#2e7d32' }}>Receive Inventory</h3>
          <form onSubmit={handleReceive}>
            {receiveErrors._form && (
              <div style={{ padding: '0.5rem', backgroundColor: '#fdecea', borderRadius: 4, color: '#721c24', fontSize: '0.85rem', marginBottom: '0.75rem' }}>
                {receiveErrors._form}
              </div>
            )}
            {receiveSuccess && (
              <div style={{ padding: '0.5rem', backgroundColor: '#e8f5e9', borderRadius: 4, color: '#2e7d32', fontSize: '0.85rem', marginBottom: '0.75rem' }}>
                {receiveSuccess}
              </div>
            )}

            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', fontWeight: 600 }}>Lot Number *</label>
              <input type="text" value={receiveForm.lot_number}
                onChange={(e) => { setReceiveForm((p) => ({ ...p, lot_number: e.target.value })); setReceiveErrors((p) => ({ ...p, lot_number: '' })); }}
                style={inputStyle(!!receiveErrors.lot_number)} placeholder="e.g. LOT-2026-001" />
              {receiveErrors.lot_number && <span style={{ fontSize: '0.75rem', color: '#dc3545' }}>{receiveErrors.lot_number}</span>}
            </div>

            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', fontWeight: 600 }}>Expiration Date *</label>
              <input type="date" value={receiveForm.expiration_date}
                onChange={(e) => { setReceiveForm((p) => ({ ...p, expiration_date: e.target.value })); setReceiveErrors((p) => ({ ...p, expiration_date: '' })); }}
                style={inputStyle(!!receiveErrors.expiration_date)} />
              {receiveErrors.expiration_date && <span style={{ fontSize: '0.75rem', color: '#dc3545' }}>{receiveErrors.expiration_date}</span>}
            </div>

            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', fontWeight: 600 }}>Quantity *</label>
              <input type="number" value={receiveForm.quantity} min="1"
                onChange={(e) => { setReceiveForm((p) => ({ ...p, quantity: parseInt(e.target.value) || 0 })); setReceiveErrors((p) => ({ ...p, quantity: '' })); }}
                style={inputStyle(!!receiveErrors.quantity)} />
              {receiveErrors.quantity && <span style={{ fontSize: '0.75rem', color: '#dc3545' }}>{receiveErrors.quantity}</span>}
            </div>

            <div style={{ marginBottom: '1rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', fontWeight: 600 }}>Reason Code *</label>
              <select value={receiveForm.reason_code}
                onChange={(e) => setReceiveForm((p) => ({ ...p, reason_code: e.target.value }))}
                style={{ ...inputStyle(false), cursor: 'pointer' }}>
                {REASON_CODES_IN.map((r) => (
                  <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>
                ))}
              </select>
            </div>

            <button type="submit" disabled={receiveLoading}
              style={{ ...btnPrimary, backgroundColor: receiveLoading ? '#6c757d' : '#2e7d32', cursor: receiveLoading ? 'not-allowed' : 'pointer', width: '100%' }}>
              {receiveLoading ? 'Receiving...' : 'Receive Stock'}
            </button>
          </form>
        </div>

        {/* Dispense form */}
        <div style={cardStyle}>
          <h3 style={{ margin: '0 0 1rem', fontSize: '1rem', color: '#c62828' }}>Dispense Inventory</h3>
          <form onSubmit={handleDispense}>
            {dispenseErrors._form && (
              <div style={{ padding: '0.5rem', backgroundColor: '#fdecea', borderRadius: 4, color: '#721c24', fontSize: '0.85rem', marginBottom: '0.75rem' }}>
                {dispenseErrors._form}
              </div>
            )}
            {dispenseSuccess && (
              <div style={{ padding: '0.5rem', backgroundColor: '#e8f5e9', borderRadius: 4, color: '#2e7d32', fontSize: '0.85rem', marginBottom: '0.75rem' }}>
                {dispenseSuccess}
              </div>
            )}

            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', fontWeight: 600 }}>Select Batch *</label>
              {batches.length === 0 ? (
                <p style={{ fontSize: '0.85rem', color: '#999' }}>No batches available to dispense from</p>
              ) : (
                <select value={dispenseForm.batch_id}
                  onChange={(e) => { setDispenseForm((p) => ({ ...p, batch_id: e.target.value })); setDispenseErrors((p) => ({ ...p, batch_id: '' })); }}
                  style={{ ...inputStyle(!!dispenseErrors.batch_id), cursor: 'pointer' }}>
                  {batches.map((b) => (
                    <option key={b.id} value={b.id}>
                      Lot {b.lot_number} - Exp: {new Date(b.expiration_date).toLocaleDateString()} - Qty: {b.quantity_on_hand}
                    </option>
                  ))}
                </select>
              )}
              {dispenseErrors.batch_id && <span style={{ fontSize: '0.75rem', color: '#dc3545' }}>{dispenseErrors.batch_id}</span>}
            </div>

            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', fontWeight: 600 }}>Quantity *</label>
              <input type="number" value={dispenseForm.quantity} min="1"
                onChange={(e) => { setDispenseForm((p) => ({ ...p, quantity: parseInt(e.target.value) || 0 })); setDispenseErrors((p) => ({ ...p, quantity: '' })); }}
                style={inputStyle(!!dispenseErrors.quantity)} />
              {dispenseErrors.quantity && <span style={{ fontSize: '0.75rem', color: '#dc3545' }}>{dispenseErrors.quantity}</span>}
            </div>

            <div style={{ marginBottom: '0.75rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', fontWeight: 600 }}>Reason Code *</label>
              <select value={dispenseForm.reason_code}
                onChange={(e) => setDispenseForm((p) => ({ ...p, reason_code: e.target.value }))}
                style={{ ...inputStyle(false), cursor: 'pointer' }}>
                {REASON_CODES_OUT.map((r) => (
                  <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>
                ))}
              </select>
            </div>

            <div style={{ marginBottom: '1rem' }}>
              <label style={{ display: 'block', marginBottom: '0.25rem', fontSize: '0.85rem', fontWeight: 600 }}>Prescription ID</label>
              <input type="text" value={dispenseForm.prescription_id}
                onChange={(e) => setDispenseForm((p) => ({ ...p, prescription_id: e.target.value }))}
                style={inputStyle(false)} placeholder="Optional" />
            </div>

            <button type="submit" disabled={dispenseLoading || batches.length === 0}
              style={{ ...btnPrimary, backgroundColor: dispenseLoading || batches.length === 0 ? '#6c757d' : '#c62828', cursor: dispenseLoading || batches.length === 0 ? 'not-allowed' : 'pointer', width: '100%' }}>
              {dispenseLoading ? 'Dispensing...' : 'Dispense Stock'}
            </button>
          </form>
        </div>
      </div>

      {/* Transaction history */}
      <div style={cardStyle}>
        <h3 style={{ margin: '0 0 0.75rem', fontSize: '1rem' }}>Transaction History</h3>
        {txLoading && <LoadingSpinner message="Loading transactions..." />}
        {txError && <ErrorMessage message={txError} onRetry={refetchTx} />}
        {!txLoading && !txError && transactions.length === 0 && (
          <EmptyState message="No transactions recorded for this SKU" />
        )}
        {!txLoading && !txError && transactions.length > 0 && (
          <>
            <DataTable columns={txColumns} data={transactions} />
            <Pagination page={txPage} pageSize={txPageSize} total={txTotal} onPageChange={setTxPage} />
          </>
        )}
      </div>
    </div>
  );
};

export default SKUDetailPage;
