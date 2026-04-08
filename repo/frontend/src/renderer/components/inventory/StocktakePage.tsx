import React, { useState, useEffect, FormEvent } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { stocktakeAPI } from '../../services/api';
import { useFetch } from '../../hooks/useFetch';
import { useDraftAutoSave } from '../../hooks/useDraftAutoSave';
import { DraftRecoveryDialog } from '../common/DraftRecoveryDialog';
import type { Stocktake, StocktakeLine } from '../../types';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';
import EmptyState from '../common/EmptyState';

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

const StocktakePage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();

  // If we have an ID, show the stocktake detail view, otherwise show create form
  if (id) {
    return <StocktakeDetail id={id} />;
  }
  return <StocktakeCreateAndList />;
};

// ---- Create / List view ----
const StocktakeCreateAndList: React.FC = () => {
  const navigate = useNavigate();

  // Create form
  const [periodStart, setPeriodStart] = useState('');
  const [periodEnd, setPeriodEnd] = useState('');
  const [createErrors, setCreateErrors] = useState<Record<string, string>>({});
  const [createLoading, setCreateLoading] = useState(false);
  const [createSuccess, setCreateSuccess] = useState('');
  // Draft auto-save: preserves period dates if the page is closed before submit
  const { clearDraft: clearStocktakeDraft } = useDraftAutoSave(
    'stocktake_create', null, { period_start: periodStart, period_end: periodEnd },
  );

  const [recentStocktakes, setRecentStocktakes] = useState<Stocktake[]>([]);
  const [listLoading, setListLoading] = useState(true);
  const [listError, setListError] = useState<string | null>(null);

  useEffect(() => {
    stocktakeAPI.list()
      .then((res) => {
        setRecentStocktakes(res.data?.data ?? []);
      })
      .catch(() => setListError('Failed to load stocktake history'))
      .finally(() => setListLoading(false));
  }, []);

  const validateCreate = (): boolean => {
    const errs: Record<string, string> = {};
    if (!periodStart) errs.period_start = 'Start date is required';
    if (!periodEnd) errs.period_end = 'End date is required';
    if (periodStart && periodEnd && new Date(periodStart) > new Date(periodEnd)) {
      errs.period_end = 'End date must be after start date';
    }
    setCreateErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    if (!validateCreate()) return;
    setCreateLoading(true);
    setCreateSuccess('');
    try {
      const res = await stocktakeAPI.create({ period_start: periodStart, period_end: periodEnd });
      const newStocktake = res.data?.stocktake ?? res.data;
      setCreateSuccess('Stocktake created — redirecting…');
      clearStocktakeDraft();
      setPeriodStart('');
      setPeriodEnd('');
      setCreateErrors({});
      // Prepend to history list
      if (newStocktake?.id) {
        setRecentStocktakes((prev) => [newStocktake, ...prev]);
      }
      // Navigate to the new stocktake
      setTimeout(() => {
        navigate(`/stocktakes/${newStocktake?.id}`);
      }, 800);
    } catch (err: any) {
      setCreateErrors({ _form: err.response?.data?.error || 'Failed to create stocktake' });
    } finally {
      setCreateLoading(false);
    }
  };

  const handleStocktakeDraftRestore = (state: unknown) => {
    const s = state as { period_start?: string; period_end?: string };
    if (s && typeof s === 'object') {
      if ((s as any).period_start) setPeriodStart((s as any).period_start);
      if ((s as any).period_end) setPeriodEnd((s as any).period_end);
    }
  };

  return (
    <div>
      <DraftRecoveryDialog
        formType="stocktake_create"
        onRestore={handleStocktakeDraftRestore}
        onDiscard={clearStocktakeDraft}
      />
      <h1 style={{ margin: '0 0 1.5rem', fontSize: '1.5rem', color: '#333' }}>Stocktakes</h1>

      {/* Create form */}
      <div style={cardStyle}>
        <h3 style={{ margin: '0 0 1rem', fontSize: '1rem' }}>Create New Stocktake</h3>
        <form onSubmit={handleCreate}>
          {createErrors._form && (
            <div style={{ padding: '0.5rem', backgroundColor: '#fdecea', borderRadius: 4, color: '#721c24', fontSize: '0.85rem', marginBottom: '0.75rem' }}>
              {createErrors._form}
            </div>
          )}
          {createSuccess && (
            <div style={{ padding: '0.5rem', backgroundColor: '#e8f5e9', borderRadius: 4, color: '#2e7d32', fontSize: '0.85rem', marginBottom: '0.75rem' }}>
              {createSuccess}
            </div>
          )}

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
            <div>
              <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Period Start *</label>
              <input
                type="date"
                value={periodStart}
                onChange={(e) => { setPeriodStart(e.target.value); setCreateErrors((p) => ({ ...p, period_start: '' })); }}
                style={inputStyle(!!createErrors.period_start)}
              />
              {createErrors.period_start && <span style={{ fontSize: '0.75rem', color: '#dc3545' }}>{createErrors.period_start}</span>}
            </div>
            <div>
              <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Period End *</label>
              <input
                type="date"
                value={periodEnd}
                onChange={(e) => { setPeriodEnd(e.target.value); setCreateErrors((p) => ({ ...p, period_end: '' })); }}
                style={inputStyle(!!createErrors.period_end)}
              />
              {createErrors.period_end && <span style={{ fontSize: '0.75rem', color: '#dc3545' }}>{createErrors.period_end}</span>}
            </div>
          </div>

          <button type="submit" disabled={createLoading}
            style={{ ...btnPrimary, cursor: createLoading ? 'not-allowed' : 'pointer', backgroundColor: createLoading ? '#6c757d' : '#1a237e' }}>
            {createLoading ? 'Creating...' : 'Create Stocktake'}
          </button>
        </form>
      </div>

      {/* Instructions */}
      <div style={cardStyle}>
        <h3 style={{ margin: '0 0 0.75rem', fontSize: '1rem' }}>How Stocktakes Work</h3>
        <ol style={{ margin: 0, paddingLeft: '1.25rem', fontSize: '0.9rem', color: '#555', lineHeight: 1.7 }}>
          <li>Create a stocktake by selecting a period date range</li>
          <li>The system will generate lines for each SKU/batch with the current system quantity</li>
          <li>Enter the physically counted quantity for each line</li>
          <li>Review the auto-computed variances (counted - system)</li>
          <li>Complete the stocktake to finalize the count</li>
        </ol>
      </div>

      {/* History / list */}
      <div style={cardStyle}>
        <h3 style={{ margin: '0 0 1rem', fontSize: '1rem' }}>Stocktake History</h3>
        {listLoading && <LoadingSpinner />}
        {listError && <ErrorMessage message={listError} />}
        {!listLoading && !listError && recentStocktakes.length === 0 && (
          <EmptyState message="No stocktakes yet. Create one above." />
        )}
        {!listLoading && !listError && recentStocktakes.length > 0 && (
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
            <thead>
              <tr style={{ borderBottom: '2px solid #e0e0e0', textAlign: 'left' }}>
                <th style={{ padding: '0.5rem 0.75rem' }}>Period</th>
                <th style={{ padding: '0.5rem 0.75rem' }}>Status</th>
                <th style={{ padding: '0.5rem 0.75rem' }}>Created</th>
                <th style={{ padding: '0.5rem 0.75rem' }}></th>
              </tr>
            </thead>
            <tbody>
              {recentStocktakes.map((st) => (
                <tr key={st.id} style={{ borderBottom: '1px solid #f0f0f0' }}>
                  <td style={{ padding: '0.5rem 0.75rem' }}>{st.period_start} – {st.period_end}</td>
                  <td style={{ padding: '0.5rem 0.75rem' }}>
                    <span style={{
                      padding: '0.2rem 0.5rem',
                      borderRadius: 4,
                      fontSize: '0.75rem',
                      fontWeight: 600,
                      backgroundColor: st.status === 'completed' ? '#e8f5e9' : st.status === 'open' ? '#e3f2fd' : '#fff3e0',
                      color: st.status === 'completed' ? '#2e7d32' : st.status === 'open' ? '#1565c0' : '#e65100',
                    }}>{st.status}</span>
                  </td>
                  <td style={{ padding: '0.5rem 0.75rem' }}>{new Date(st.created_at).toLocaleDateString()}</td>
                  <td style={{ padding: '0.5rem 0.75rem' }}>
                    <button
                      onClick={() => navigate(`/stocktakes/${st.id}`)}
                      style={{ ...btnPrimary, padding: '0.25rem 0.75rem', fontSize: '0.8rem' }}
                    >
                      Open
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
};

// ---- Detail view ----
interface StocktakeDetailProps {
  id: string;
}

const StocktakeDetail: React.FC<StocktakeDetailProps> = ({ id }) => {
  const navigate = useNavigate();

  const { data: stocktake, loading, error, refetch } = useFetch<Stocktake>(
    () => stocktakeAPI.get(id),
    [id]
  );

  const [lines, setLines] = useState<StocktakeLine[]>([]);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saveSuccess, setSaveSuccess] = useState('');
  const [completing, setCompleting] = useState(false);
  const [completeError, setCompleteError] = useState<string | null>(null);
  const [completeSuccess, setCompleteSuccess] = useState('');

  useEffect(() => {
    if (stocktake?.lines) {
      setLines(stocktake.lines.map((l) => ({ ...l })));
    }
  }, [stocktake]);

  const handleCountedChange = (lineId: string, value: number) => {
    setLines((prev) =>
      prev.map((l) => {
        if (l.id === lineId) {
          const counted = Math.max(0, value);
          return { ...l, counted_qty: counted, variance: counted - l.system_qty };
        }
        return l;
      })
    );
  };

  const handleSaveLines = async () => {
    setSaving(true);
    setSaveError(null);
    setSaveSuccess('');
    try {
      await stocktakeAPI.updateLines(
        id,
        lines.map((l) => ({ id: l.id, counted_qty: l.counted_qty }))
      );
      setSaveSuccess('Lines saved successfully');
      setTimeout(() => setSaveSuccess(''), 3000);
    } catch (err: any) {
      setSaveError(err.response?.data?.error || 'Failed to save lines');
    } finally {
      setSaving(false);
    }
  };

  const handleComplete = async () => {
    if (!window.confirm('Are you sure you want to complete this stocktake? This action cannot be undone.')) return;
    setCompleting(true);
    setCompleteError(null);
    setCompleteSuccess('');
    try {
      await stocktakeAPI.complete(id);
      setCompleteSuccess('Stocktake completed successfully');
      refetch();
    } catch (err: any) {
      setCompleteError(err.response?.data?.error || 'Failed to complete stocktake');
    } finally {
      setCompleting(false);
    }
  };

  if (loading) return <LoadingSpinner message="Loading stocktake..." />;
  if (error) return <ErrorMessage message={error} onRetry={refetch} />;
  if (!stocktake) return <ErrorMessage message="Stocktake not found" />;

  const isCompleted = stocktake.status === 'completed';
  const totalVariance = lines.reduce((sum, l) => sum + (l.counted_qty - l.system_qty), 0);

  const statusColor = stocktake.status === 'completed' ? '#2e7d32' : '#1565c0';

  return (
    <div>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', marginBottom: '1.5rem' }}>
        <button
          onClick={() => navigate('/stocktakes')}
          style={{ padding: '0.4rem 0.8rem', border: '1px solid #ccc', borderRadius: 4, backgroundColor: '#fff', cursor: 'pointer', fontSize: '0.85rem' }}
        >
          &larr; Back to Stocktakes
        </button>
        <div>
          <h1 style={{ margin: 0, fontSize: '1.5rem', color: '#333' }}>Stocktake</h1>
          <span style={{ fontSize: '0.85rem', color: '#666' }}>
            Period: {new Date(stocktake.period_start).toLocaleDateString()} - {new Date(stocktake.period_end).toLocaleDateString()}
          </span>
        </div>
        <span style={{
          marginLeft: 'auto',
          padding: '0.2rem 0.75rem',
          borderRadius: 12,
          fontSize: '0.85rem',
          fontWeight: 600,
          backgroundColor: statusColor + '22',
          color: statusColor,
        }}>
          {stocktake.status.replace(/_/g, ' ').toUpperCase()}
        </span>
      </div>

      {/* Info card */}
      <div style={cardStyle}>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: '0.75rem', fontSize: '0.9rem' }}>
          <div><span style={{ color: '#666' }}>Created by:</span> <span>{stocktake.created_by}</span></div>
          <div><span style={{ color: '#666' }}>Created at:</span> <span>{new Date(stocktake.created_at).toLocaleString()}</span></div>
          <div><span style={{ color: '#666' }}>Total lines:</span> <span>{lines.length}</span></div>
          <div>
            <span style={{ color: '#666' }}>Net variance:</span>{' '}
            <span style={{ fontWeight: 600, color: totalVariance === 0 ? '#2e7d32' : '#c62828' }}>
              {totalVariance > 0 ? '+' : ''}{totalVariance}
            </span>
          </div>
        </div>
      </div>

      {/* Feedback */}
      {saveError && <ErrorMessage message={saveError} />}
      {saveSuccess && (
        <div style={{ padding: '0.75rem', backgroundColor: '#e8f5e9', border: '1px solid #c8e6c9', borderRadius: 4, color: '#2e7d32', fontSize: '0.9rem', marginBottom: '1rem' }}>
          {saveSuccess}
        </div>
      )}
      {completeError && <ErrorMessage message={completeError} />}
      {completeSuccess && (
        <div style={{ padding: '0.75rem', backgroundColor: '#e8f5e9', border: '1px solid #c8e6c9', borderRadius: 4, color: '#2e7d32', fontSize: '0.9rem', marginBottom: '1rem' }}>
          {completeSuccess}
        </div>
      )}

      {/* Lines table */}
      <div style={cardStyle}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
          <h3 style={{ margin: 0, fontSize: '1rem' }}>Stocktake Lines</h3>
          {!isCompleted && lines.length > 0 && (
            <div style={{ display: 'flex', gap: '0.5rem' }}>
              <button onClick={handleSaveLines} disabled={saving}
                style={{ ...btnPrimary, backgroundColor: saving ? '#6c757d' : '#1a237e', cursor: saving ? 'not-allowed' : 'pointer' }}>
                {saving ? 'Saving...' : 'Save Counts'}
              </button>
              <button onClick={handleComplete} disabled={completing}
                style={{ ...btnPrimary, backgroundColor: completing ? '#6c757d' : '#2e7d32', cursor: completing ? 'not-allowed' : 'pointer' }}>
                {completing ? 'Completing...' : 'Complete Stocktake'}
              </button>
            </div>
          )}
        </div>

        {lines.length === 0 ? (
          <EmptyState message="No stocktake lines generated. The system will create lines based on current inventory." />
        ) : (
          <div style={{ overflowX: 'auto' }}>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.9rem' }}>
              <thead>
                <tr style={{ backgroundColor: '#f5f5f5', borderBottom: '2px solid #ddd' }}>
                  <th style={{ padding: '0.75rem', textAlign: 'left' }}>SKU ID</th>
                  <th style={{ padding: '0.75rem', textAlign: 'left' }}>Batch ID</th>
                  <th style={{ padding: '0.75rem', textAlign: 'right' }}>System Qty</th>
                  <th style={{ padding: '0.75rem', textAlign: 'center', minWidth: 120 }}>Counted Qty</th>
                  <th style={{ padding: '0.75rem', textAlign: 'right' }}>Variance</th>
                  <th style={{ padding: '0.75rem', textAlign: 'left' }}>Loss Reason</th>
                </tr>
              </thead>
              <tbody>
                {lines.map((line) => {
                  const variance = line.counted_qty - line.system_qty;
                  return (
                    <tr key={line.id} style={{ borderBottom: '1px solid #eee' }}>
                      <td style={{ padding: '0.75rem' }}>
                        <span style={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>{line.sku_id.substring(0, 8)}...</span>
                      </td>
                      <td style={{ padding: '0.75rem' }}>
                        <span style={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>{line.batch_id.substring(0, 8)}...</span>
                      </td>
                      <td style={{ padding: '0.75rem', textAlign: 'right', fontWeight: 500 }}>{line.system_qty}</td>
                      <td style={{ padding: '0.75rem', textAlign: 'center' }}>
                        {isCompleted ? (
                          <span style={{ fontWeight: 500 }}>{line.counted_qty}</span>
                        ) : (
                          <input
                            type="number"
                            value={line.counted_qty}
                            min="0"
                            onChange={(e) => handleCountedChange(line.id, parseInt(e.target.value) || 0)}
                            style={{
                              width: 80,
                              padding: '0.35rem 0.5rem',
                              fontSize: '0.9rem',
                              border: '1px solid #ccc',
                              borderRadius: 4,
                              textAlign: 'center',
                            }}
                          />
                        )}
                      </td>
                      <td style={{
                        padding: '0.75rem',
                        textAlign: 'right',
                        fontWeight: 600,
                        color: variance === 0 ? '#666' : variance > 0 ? '#2e7d32' : '#c62828',
                      }}>
                        {variance > 0 ? '+' : ''}{variance}
                      </td>
                      <td style={{ padding: '0.75rem', color: '#666', fontSize: '0.85rem' }}>
                        {line.loss_reason || '-'}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
              <tfoot>
                <tr style={{ backgroundColor: '#f5f5f5', borderTop: '2px solid #ddd' }}>
                  <td colSpan={2} style={{ padding: '0.75rem', fontWeight: 600 }}>Totals</td>
                  <td style={{ padding: '0.75rem', textAlign: 'right', fontWeight: 600 }}>
                    {lines.reduce((s, l) => s + l.system_qty, 0)}
                  </td>
                  <td style={{ padding: '0.75rem', textAlign: 'center', fontWeight: 600 }}>
                    {lines.reduce((s, l) => s + l.counted_qty, 0)}
                  </td>
                  <td style={{
                    padding: '0.75rem',
                    textAlign: 'right',
                    fontWeight: 700,
                    color: totalVariance === 0 ? '#666' : totalVariance > 0 ? '#2e7d32' : '#c62828',
                  }}>
                    {totalVariance > 0 ? '+' : ''}{totalVariance}
                  </td>
                  <td style={{ padding: '0.75rem' }} />
                </tr>
              </tfoot>
            </table>
          </div>
        )}
      </div>
    </div>
  );
};

export default StocktakePage;
