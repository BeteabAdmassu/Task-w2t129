import React, { useState, useEffect } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { workOrdersAPI, filesAPI } from '../../services/api';
import { useAuth } from '../../hooks/useAuth';
import { useDraftAutoSave } from '../../hooks/useDraftAutoSave';
import { DraftRecoveryDialog } from '../common/DraftRecoveryDialog';
import type { WorkOrder, ManagedFile } from '../../types';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';

const btnPrimary: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#1976d2', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const btnDisabled: React.CSSProperties = { ...btnPrimary, opacity: 0.6, cursor: 'not-allowed' };
const btnSecondary: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#6c757d', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const btnDanger: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#dc3545', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const btnSuccess: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#28a745', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontSize: '0.9rem',
};
const inputStyle: React.CSSProperties = {
  width: '100%', padding: '0.5rem', border: '1px solid #ccc', borderRadius: 4, fontSize: '0.9rem', boxSizing: 'border-box',
};
const cardStyle: React.CSSProperties = {
  backgroundColor: '#fff', border: '1px solid #e0e0e0', borderRadius: 8, padding: '1.5rem', marginBottom: '1rem',
};
const successStyle: React.CSSProperties = {
  padding: '0.75rem 1rem', backgroundColor: '#d4edda', border: '1px solid #c3e6cb', borderRadius: 4, color: '#155724', marginBottom: '1rem',
};
const priorityColors: Record<string, string> = { urgent: '#dc3545', high: '#fd7e14', normal: '#28a745' };

const statusSteps = ['submitted', 'dispatched', 'in_progress', 'completed', 'closed'];

const WorkOrderDetailPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { user } = useAuth();

  const [order, setOrder] = useState<WorkOrder | null>(null);
  const [photos, setPhotos] = useState<ManagedFile[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [actionLoading, setActionLoading] = useState(false);
  const [successMsg, setSuccessMsg] = useState('');

  // Close form
  const [showCloseForm, setShowCloseForm] = useState(false);
  const [closeForm, setCloseForm] = useState({ parts_cost: '', labor_cost: '' });
  const [closeErr, setCloseErr] = useState('');
  // Draft auto-save: preserves parts/labor cost entries if the page is closed before submitting
  const { clearDraft: clearCloseDraft } = useDraftAutoSave('wo_close', id ?? null, closeForm);

  // Rating
  const [rating, setRating] = useState(0);
  const [ratingSubmitted, setRatingSubmitted] = useState(false);

  // Attach-after-create photo flow
  const [attachFiles, setAttachFiles] = useState<File[]>([]);
  const [attachLoading, setAttachLoading] = useState(false);
  const [attachErr, setAttachErr] = useState('');

  const fetchOrder = async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const r = await workOrdersAPI.get(id);
      const payload = r.data;
      // Support new envelope {work_order, photos} and legacy plain WorkOrder.
      const wo: WorkOrder = (payload as { work_order?: WorkOrder }).work_order ?? (payload as unknown as WorkOrder);
      const photoList: ManagedFile[] = (payload as { photos?: ManagedFile[] }).photos ?? [];
      setOrder(wo);
      setPhotos(photoList);
      if (wo.rating) { setRating(wo.rating); setRatingSubmitted(true); }
    } catch (e: any) {
      setError(e.response?.data?.error || 'Failed to load work order');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { fetchOrder(); }, [id]);

  const showSuccess = (msg: string) => {
    setSuccessMsg(msg);
    setTimeout(() => setSuccessMsg(''), 3000);
  };

  const handleStatusUpdate = async (newStatus: string) => {
    if (!order) return;
    setActionLoading(true);
    try {
      await workOrdersAPI.update(order.id, { status: newStatus });
      showSuccess(`Status updated to ${newStatus.replace(/_/g, ' ')}`);
      fetchOrder();
    } catch (e: any) {
      alert('Failed: ' + (e.response?.data?.error || e.message));
    } finally {
      setActionLoading(false);
    }
  };

  const handleClose = async () => {
    if (!order) return;
    const parts = parseFloat(closeForm.parts_cost);
    const labor = parseFloat(closeForm.labor_cost);
    if (isNaN(parts) || parts < 0) { setCloseErr('Parts cost must be a non-negative number'); return; }
    if (isNaN(labor) || labor < 0) { setCloseErr('Labor cost must be a non-negative number'); return; }
    setActionLoading(true);
    setCloseErr('');
    try {
      await workOrdersAPI.close(order.id, { parts_cost: parts, labor_cost: labor });
      showSuccess('Work order closed');
      clearCloseDraft();
      setShowCloseForm(false);
      fetchOrder();
    } catch (e: any) {
      setCloseErr(e.response?.data?.error || 'Failed to close order');
    } finally {
      setActionLoading(false);
    }
  };

  const handleRate = async (value: number) => {
    if (!order) return;
    setRating(value);
    setActionLoading(true);
    try {
      await workOrdersAPI.rate(order.id, value);
      setRatingSubmitted(true);
      showSuccess('Rating submitted');
    } catch (e: any) {
      alert('Failed to rate: ' + (e.response?.data?.error || e.message));
    } finally {
      setActionLoading(false);
    }
  };

  const handleAttachPhotos = async () => {
    if (!id || attachFiles.length === 0) return;
    setAttachLoading(true);
    setAttachErr('');
    try {
      for (const file of attachFiles) {
        const fd = new FormData();
        fd.append('file', file);
        const uploadRes = await filesAPI.upload(fd);
        const fileObj = (uploadRes.data as any).file ?? uploadRes.data;
        if (fileObj?.id) {
          await workOrdersAPI.linkPhoto(id, fileObj.id);
        }
      }
      setAttachFiles([]);
      showSuccess(`${attachFiles.length} photo${attachFiles.length > 1 ? 's' : ''} attached`);
      fetchOrder();
    } catch (e: any) {
      setAttachErr(e.response?.data?.error || 'Failed to attach photos');
    } finally {
      setAttachLoading(false);
    }
  };

  if (loading) return <LoadingSpinner message="Loading work order..." />;
  if (error) return <ErrorMessage message={error} onRetry={fetchOrder} />;
  if (!order) return <ErrorMessage message="Work order not found" />;

  const deadline = new Date(order.sla_deadline);
  const now = new Date();
  const isBreached = deadline < now && order.status !== 'completed' && order.status !== 'closed' && order.status !== 'cancelled';
  const remainingMs = deadline.getTime() - now.getTime();
  const remainingHours = Math.floor(remainingMs / (1000 * 60 * 60));
  const remainingMins = Math.floor((remainingMs % (1000 * 60 * 60)) / (1000 * 60));
  const currentStepIndex = statusSteps.indexOf(order.status);
  const isMaintenance = user?.role === 'maintenance_tech' || user?.role === 'system_admin';

  const handleCloseDraftRestore = (state: unknown) => {
    const s = state as typeof closeForm;
    if (s && typeof s === 'object') {
      setCloseForm({
        parts_cost: (s as any).parts_cost || '',
        labor_cost: (s as any).labor_cost || '',
      });
      setShowCloseForm(true);
    }
  };

  return (
    <div style={{ padding: '1.5rem', maxWidth: 900, margin: '0 auto' }}>
      <DraftRecoveryDialog
        formType="wo_close"
        formId={id}
        onRestore={handleCloseDraftRestore}
        onDiscard={clearCloseDraft}
      />
      <button onClick={() => navigate('/work-orders')} style={{ ...btnSecondary, marginBottom: '1rem' }}>
        &larr; Back to Work Orders
      </button>

      {successMsg && <div style={successStyle}>{successMsg}</div>}

      {/* Header */}
      <div style={{ ...cardStyle, display: 'flex', justifyContent: 'space-between', flexWrap: 'wrap', gap: '1rem' }}>
        <div>
          <h2 style={{ margin: '0 0 0.5rem' }}>Work Order #{order.id.slice(0, 8)}</h2>
          <span style={{
            padding: '0.2rem 0.8rem', borderRadius: 12, fontSize: '0.85rem', fontWeight: 600,
            backgroundColor: (priorityColors[order.priority] || '#666') + '20', color: priorityColors[order.priority] || '#666',
          }}>{order.priority}</span>
          <span style={{
            marginLeft: '0.5rem', padding: '0.2rem 0.8rem', borderRadius: 12, fontSize: '0.85rem', fontWeight: 600,
            backgroundColor: '#e3f2fd', color: '#1565c0',
          }}>{order.status.replace(/_/g, ' ')}</span>
          <span style={{
            marginLeft: '0.5rem', padding: '0.2rem 0.8rem', borderRadius: 12, fontSize: '0.85rem',
            backgroundColor: '#f5f5f5', color: '#333', textTransform: 'capitalize' as const,
          }}>{order.trade}</span>
        </div>
        <div style={{ textAlign: 'right' }}>
          <div style={{ fontSize: '0.85rem', color: '#666' }}>SLA Deadline</div>
          <div style={{ fontWeight: 600, color: isBreached ? '#dc3545' : remainingMs < 3600000 ? '#fd7e14' : '#333' }}>
            {deadline.toLocaleString()}
          </div>
          {isBreached ? (
            <div style={{ color: '#dc3545', fontWeight: 700, fontSize: '0.9rem', marginTop: 4 }}>SLA BREACHED</div>
          ) : order.status !== 'completed' && order.status !== 'closed' && order.status !== 'cancelled' ? (
            <div style={{ color: remainingMs < 3600000 ? '#fd7e14' : '#28a745', fontSize: '0.85rem', marginTop: 4 }}>
              {remainingHours > 0 ? `${remainingHours}h ${remainingMins}m remaining` : `${remainingMins}m remaining`}
            </div>
          ) : null}
        </div>
      </div>

      {/* Status Timeline */}
      <div style={cardStyle}>
        <h3 style={{ margin: '0 0 1rem' }}>Status Progress</h3>
        <div style={{ display: 'flex', alignItems: 'center', gap: 0 }}>
          {statusSteps.map((step, i) => {
            const isActive = i <= currentStepIndex;
            const isCurrent = i === currentStepIndex;
            return (
              <React.Fragment key={step}>
                <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', flex: 1 }}>
                  <div style={{
                    width: 32, height: 32, borderRadius: '50%', display: 'flex', alignItems: 'center', justifyContent: 'center',
                    backgroundColor: isActive ? '#1976d2' : '#e0e0e0', color: isActive ? '#fff' : '#999',
                    fontWeight: 600, fontSize: '0.8rem', border: isCurrent ? '3px solid #0d47a1' : 'none',
                  }}>
                    {i + 1}
                  </div>
                  <div style={{ fontSize: '0.75rem', marginTop: 4, color: isActive ? '#1976d2' : '#999', textTransform: 'capitalize' as const, textAlign: 'center' }}>
                    {step.replace(/_/g, ' ')}
                  </div>
                </div>
                {i < statusSteps.length - 1 && (
                  <div style={{ flex: 0.5, height: 3, backgroundColor: i < currentStepIndex ? '#1976d2' : '#e0e0e0', marginBottom: 20 }} />
                )}
              </React.Fragment>
            );
          })}
        </div>
      </div>

      {/* Details */}
      <div style={cardStyle}>
        <h3 style={{ margin: '0 0 1rem' }}>Details</h3>
        <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem' }}>
          <div><span style={{ fontWeight: 500, color: '#666' }}>Location:</span> {order.location}</div>
          <div><span style={{ fontWeight: 500, color: '#666' }}>Trade:</span> <span style={{ textTransform: 'capitalize' as const }}>{order.trade}</span></div>
          <div><span style={{ fontWeight: 500, color: '#666' }}>Submitted By:</span> {order.submitted_by}</div>
          <div><span style={{ fontWeight: 500, color: '#666' }}>Assigned To:</span> {order.assigned_to || 'Unassigned'}</div>
          <div><span style={{ fontWeight: 500, color: '#666' }}>Created:</span> {new Date(order.created_at).toLocaleString()}</div>
          {order.closed_at && <div><span style={{ fontWeight: 500, color: '#666' }}>Closed:</span> {new Date(order.closed_at).toLocaleString()}</div>}
          {(order.parts_cost > 0 || order.labor_cost > 0) && (
            <>
              <div><span style={{ fontWeight: 500, color: '#666' }}>Parts Cost:</span> ${order.parts_cost.toFixed(2)}</div>
              <div><span style={{ fontWeight: 500, color: '#666' }}>Labor Cost:</span> ${order.labor_cost.toFixed(2)}</div>
            </>
          )}
        </div>
        <div style={{ marginTop: '1rem' }}>
          <span style={{ fontWeight: 500, color: '#666' }}>Description:</span>
          <p style={{ margin: '0.5rem 0 0', whiteSpace: 'pre-wrap' }}>{order.description}</p>
        </div>
      </div>

      {/* Photos — view existing + attach new */}
      {(order.status !== 'closed' && order.status !== 'cancelled') || photos.length > 0 ? (
        <div style={cardStyle}>
          <h3 style={{ margin: '0 0 1rem' }}>Photos {photos.length > 0 ? `(${photos.length})` : ''}</h3>

          {/* Existing photos */}
          {photos.length > 0 && (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.75rem', marginBottom: '1rem' }}>
              {photos.map(photo => (
                <div key={photo.id} style={{
                  border: '1px solid #e0e0e0', borderRadius: 4, padding: '0.75rem',
                  minWidth: 150, maxWidth: 220, fontSize: '0.82rem',
                }}>
                  <div style={{ fontWeight: 500, wordBreak: 'break-all', marginBottom: '0.25rem' }}>{photo.original_name}</div>
                  <div style={{ color: '#888', marginBottom: '0.5rem' }}>{(photo.size_bytes / 1024).toFixed(1)} KB</div>
                  <button
                    onClick={async () => {
                      try {
                        const res = await filesAPI.download(photo.id);
                        const url = window.URL.createObjectURL(new Blob([res.data]));
                        const a = document.createElement('a');
                        a.href = url;
                        a.download = photo.original_name;
                        a.click();
                        window.URL.revokeObjectURL(url);
                      } catch {
                        alert('Failed to download file');
                      }
                    }}
                    style={{ ...btnPrimary, fontSize: '0.78rem', padding: '0.25rem 0.6rem' }}>
                    Download
                  </button>
                </div>
              ))}
            </div>
          )}
          {photos.length === 0 && <div style={{ color: '#999', marginBottom: '1rem', fontSize: '0.9rem' }}>No photos attached yet.</div>}

          {/* Attach new photos (disabled when closed or cancelled) */}
          {order.status !== 'closed' && order.status !== 'cancelled' && (
            <div style={{ borderTop: photos.length > 0 ? '1px solid #f0f0f0' : 'none', paddingTop: photos.length > 0 ? '1rem' : 0 }}>
              <div style={{ fontSize: '0.85rem', fontWeight: 500, marginBottom: '0.5rem', color: '#444' }}>Attach photos</div>
              {attachErr && <div style={{ color: '#dc3545', fontSize: '0.82rem', marginBottom: '0.5rem' }}>{attachErr}</div>}
              <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center', flexWrap: 'wrap' }}>
                <input
                  type="file"
                  multiple
                  accept="image/*,application/pdf"
                  onChange={e => { setAttachFiles(Array.from(e.target.files || [])); setAttachErr(''); }}
                  style={{ fontSize: '0.85rem' }}
                />
                {attachFiles.length > 0 && (
                  <button
                    onClick={handleAttachPhotos}
                    disabled={attachLoading}
                    style={attachLoading ? btnDisabled : btnPrimary}>
                    {attachLoading ? 'Uploading...' : `Attach ${attachFiles.length} file${attachFiles.length > 1 ? 's' : ''}`}
                  </button>
                )}
              </div>
            </div>
          )}
        </div>
      ) : null}

      {/* Maintenance Actions */}
      {isMaintenance && order.status !== 'closed' && order.status !== 'cancelled' && (
        <div style={cardStyle}>
          <h3 style={{ margin: '0 0 1rem' }}>Actions</h3>
          <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
            {order.status === 'submitted' && (
              <button onClick={() => handleStatusUpdate('dispatched')} disabled={actionLoading} style={actionLoading ? btnDisabled : btnPrimary}>
                {actionLoading ? 'Updating...' : 'Dispatch'}
              </button>
            )}
            {order.status === 'dispatched' && (
              <button onClick={() => handleStatusUpdate('in_progress')} disabled={actionLoading} style={actionLoading ? btnDisabled : btnPrimary}>
                {actionLoading ? 'Updating...' : 'Start Work'}
              </button>
            )}
            {order.status === 'in_progress' && (
              <button onClick={() => handleStatusUpdate('completed')} disabled={actionLoading} style={actionLoading ? btnDisabled : btnSuccess}>
                {actionLoading ? 'Updating...' : 'Mark Completed'}
              </button>
            )}
            {order.status === 'completed' && !showCloseForm && (
              <button onClick={() => setShowCloseForm(true)} style={btnDanger}>Close Order</button>
            )}
          </div>

          {showCloseForm && (
            <div style={{ marginTop: '1rem', padding: '1rem', backgroundColor: '#f9f9f9', borderRadius: 4 }}>
              <h4 style={{ margin: '0 0 0.75rem' }}>Close Work Order</h4>
              {closeErr && <div style={{ color: '#dc3545', marginBottom: '0.5rem', fontSize: '0.85rem' }}>{closeErr}</div>}
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '0.75rem', marginBottom: '1rem' }}>
                <div>
                  <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Parts Cost ($)</label>
                  <input type="number" min="0" step="0.01" value={closeForm.parts_cost}
                    onChange={e => setCloseForm({ ...closeForm, parts_cost: e.target.value })} style={inputStyle} placeholder="0.00" />
                </div>
                <div>
                  <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Labor Cost ($)</label>
                  <input type="number" min="0" step="0.01" value={closeForm.labor_cost}
                    onChange={e => setCloseForm({ ...closeForm, labor_cost: e.target.value })} style={inputStyle} placeholder="0.00" />
                </div>
              </div>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                <button onClick={handleClose} disabled={actionLoading} style={actionLoading ? btnDisabled : btnDanger}>
                  {actionLoading ? 'Closing...' : 'Confirm Close'}
                </button>
                <button onClick={() => { setShowCloseForm(false); setCloseErr(''); }} style={btnSecondary}>Cancel</button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Rating */}
      {(order.status === 'completed' || order.status === 'closed') && (
        <div style={cardStyle}>
          <h3 style={{ margin: '0 0 1rem' }}>Rating</h3>
          {ratingSubmitted ? (
            <div>
              <div style={{ display: 'flex', gap: '0.25rem' }}>
                {[1, 2, 3, 4, 5].map(v => (
                  <span key={v} style={{ fontSize: '1.5rem', color: v <= rating ? '#ffc107' : '#e0e0e0' }}>&#9733;</span>
                ))}
              </div>
              <div style={{ color: '#666', fontSize: '0.85rem', marginTop: 4 }}>Rating: {rating}/5</div>
            </div>
          ) : (
            <div>
              <p style={{ margin: '0 0 0.5rem', color: '#666' }}>Rate this work order:</p>
              <div style={{ display: 'flex', gap: '0.5rem' }}>
                {[1, 2, 3, 4, 5].map(v => (
                  <button key={v} onClick={() => handleRate(v)} disabled={actionLoading}
                    style={{
                      width: 44, height: 44, borderRadius: '50%', border: '2px solid #ddd', cursor: actionLoading ? 'not-allowed' : 'pointer',
                      backgroundColor: rating === v ? '#ffc107' : '#fff', fontSize: '1rem', fontWeight: 600,
                      color: rating === v ? '#fff' : '#333', transition: 'all 0.2s',
                    }}
                    onMouseEnter={e => { if (!actionLoading) { e.currentTarget.style.backgroundColor = '#fff3cd'; e.currentTarget.style.borderColor = '#ffc107'; } }}
                    onMouseLeave={e => { e.currentTarget.style.backgroundColor = rating === v ? '#ffc107' : '#fff'; e.currentTarget.style.borderColor = '#ddd'; }}>
                    {v}
                  </button>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default WorkOrderDetailPage;
