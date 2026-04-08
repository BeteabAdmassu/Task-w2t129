import React, { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { workOrdersAPI, filesAPI } from '../../services/api';
import { useAuth } from '../../hooks/useAuth';
import { useFetch } from '../../hooks/useFetch';
import { useDraftAutoSave } from '../../hooks/useDraftAutoSave';
import { DraftRecoveryDialog } from '../common/DraftRecoveryDialog';
import type { WorkOrder, PaginatedResponse } from '../../types';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';
import EmptyState from '../common/EmptyState';
import DataTable from '../common/DataTable';
import Pagination from '../common/Pagination';
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
const selectStyle: React.CSSProperties = { ...inputStyle, appearance: 'auto' as const };
const successStyle: React.CSSProperties = {
  padding: '0.5rem 1rem', backgroundColor: '#d4edda', border: '1px solid #c3e6cb', borderRadius: 4, color: '#155724', marginBottom: '1rem',
};

const priorityColors: Record<string, string> = { urgent: '#dc3545', high: '#fd7e14', normal: '#28a745' };
const statusOptions = ['', 'submitted', 'dispatched', 'in_progress', 'completed', 'closed'];
const priorityOptions = ['', 'urgent', 'high', 'normal'];
const tradeOptions = ['electrical', 'plumbing', 'hvac', 'general'];

const WorkOrdersPage: React.FC = () => {
  const navigate = useNavigate();
  const { user } = useAuth();
  const isMaintenance = user?.role === 'maintenance_tech' || user?.role === 'system_admin';
  const [page, setPage] = useState(1);
  const [statusFilter, setStatusFilter] = useState('');
  const [priorityFilter, setPriorityFilter] = useState('');

  const { data, loading, error, refetch } = useFetch<PaginatedResponse<WorkOrder>>(
    () => workOrdersAPI.list({
      status: statusFilter || undefined,
      page,
      page_size: 20,
    }).then(r => ({ data: r.data })),
    [page, statusFilter]
  );

  // Filter by priority client-side (API may not support it)
  const filteredData = data?.data?.filter(wo => !priorityFilter || wo.priority === priorityFilter) || [];

  // Create modal
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ trade: 'general', priority: 'normal' as string, description: '', location: '' });
  const [createErr, setCreateErr] = useState('');
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [createSuccess, setCreateSuccess] = useState('');
  const [selectedFiles, setSelectedFiles] = useState<File[]>([]);

  // Draft auto-save: saves create form state every 30s
  const { clearDraft } = useDraftAutoSave('work_order', 'default', createForm);

  // Context menu
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; order: WorkOrder } | null>(null);

  const handleCreate = async () => {
    if (!createForm.description.trim()) { setCreateErr('Description is required'); return; }
    if (!createForm.location.trim()) { setCreateErr('Location is required'); return; }
    setCreateSubmitting(true);
    setCreateErr('');
    try {
      // Upload any attached photos first, collect their IDs.
      const photoIds: string[] = [];
      for (const file of selectedFiles) {
        const fd = new FormData();
        fd.append('file', file);
        const res = await filesAPI.upload(fd);
        const fileObj = (res.data as any).file ?? res.data;
        if (fileObj?.id) photoIds.push(fileObj.id);
      }

      await workOrdersAPI.create({
        trade: createForm.trade,
        priority: createForm.priority,
        description: createForm.description.trim(),
        location: createForm.location.trim(),
        photo_ids: photoIds,
      });
      setCreateSuccess('Work order created successfully');
      setShowCreate(false);
      setCreateForm({ trade: 'general', priority: 'normal', description: '', location: '' });
      setSelectedFiles([]);
      clearDraft();
      refetch();
      setTimeout(() => setCreateSuccess(''), 3000);
    } catch (e: any) {
      setCreateErr(e.response?.data?.error || 'Failed to create work order');
    } finally {
      setCreateSubmitting(false);
    }
  };

  const handleStatusUpdate = async (order: WorkOrder, newStatus: string) => {
    try {
      await workOrdersAPI.update(order.id, { status: newStatus });
      refetch();
    } catch (e: any) {
      alert('Failed to update status: ' + (e.response?.data?.error || e.message));
    }
  };

  const handleClose = async (order: WorkOrder) => {
    const partsCost = prompt('Parts cost:', '0');
    if (partsCost === null) return;
    const laborCost = prompt('Labor cost:', '0');
    if (laborCost === null) return;
    try {
      await workOrdersAPI.close(order.id, { parts_cost: parseFloat(partsCost) || 0, labor_cost: parseFloat(laborCost) || 0 });
      refetch();
    } catch (e: any) {
      alert('Failed to close order: ' + (e.response?.data?.error || e.message));
    }
  };

  const handleCancel = async (order: WorkOrder) => {
    if (!window.confirm(`Cancel work order ${order.id.slice(0, 8)}? This cannot be undone.`)) return;
    try {
      await workOrdersAPI.update(order.id, { status: 'cancelled' });
      refetch();
    } catch (e: any) {
      alert('Failed to cancel: ' + (e.response?.data?.error || e.message));
    }
  };

  const columns = [
    { key: 'id', header: 'ID', sortable: true, render: (wo: WorkOrder) => <span style={{ fontFamily: 'monospace', fontSize: '0.8rem' }}>{wo.id.slice(0, 8)}</span> },
    { key: 'trade', header: 'Trade', sortable: true, render: (wo: WorkOrder) => <span style={{ textTransform: 'capitalize' as const }}>{wo.trade}</span> },
    {
      key: 'priority', header: 'Priority', sortable: true,
      render: (wo: WorkOrder) => (
        <span style={{
          padding: '0.2rem 0.6rem', borderRadius: 12, fontSize: '0.8rem', fontWeight: 600,
          backgroundColor: priorityColors[wo.priority] + '20', color: priorityColors[wo.priority],
        }}>{wo.priority}</span>
      ),
    },
    { key: 'status', header: 'Status', sortable: true, render: (wo: WorkOrder) => <span style={{ textTransform: 'capitalize' as const }}>{wo.status.replace(/_/g, ' ')}</span> },
    { key: 'location', header: 'Location', sortable: true },
    {
      key: 'sla_deadline', header: 'SLA Deadline', sortable: true,
      render: (wo: WorkOrder) => {
        const deadline = new Date(wo.sla_deadline);
        const isBreached = deadline < new Date() && wo.status !== 'completed' && wo.status !== 'closed';
        return <span style={{ color: isBreached ? '#dc3545' : '#333', fontWeight: isBreached ? 600 : 400 }}>
          {deadline.toLocaleDateString()} {deadline.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
          {isBreached && ' (BREACHED)'}
        </span>;
      },
    },
    { key: 'assigned_to', header: 'Assigned To', render: (wo: WorkOrder) => wo.assigned_to || <span style={{ color: '#999' }}>Unassigned</span> },
  ];

  const handleDraftRestore = (state: unknown) => {
    const s = state as typeof createForm;
    if (s && typeof s === 'object') {
      setCreateForm({
        trade: (s as any).trade || 'general',
        priority: (s as any).priority || 'normal',
        description: (s as any).description || '',
        location: (s as any).location || '',
      });
      setShowCreate(true);
    }
  };

  return (
    <div style={{ padding: '1.5rem' }}>
      <DraftRecoveryDialog
        formType="work_order"
        formId="default"
        onRestore={handleDraftRestore}
        onDiscard={clearDraft}
      />

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h2 style={{ margin: 0 }}>Work Orders</h2>
        <button onClick={() => setShowCreate(true)} style={btnPrimary}>+ New Work Order</button>
      </div>

      {createSuccess && <div style={successStyle}>{createSuccess}</div>}

      {/* Filters */}
      <div style={{ display: 'flex', gap: '1rem', marginBottom: '1rem', flexWrap: 'wrap' }}>
        <div>
          <label style={{ display: 'block', fontSize: '0.8rem', marginBottom: 2, color: '#666' }}>Status</label>
          <select value={statusFilter} onChange={e => { setStatusFilter(e.target.value); setPage(1); }} style={{ ...selectStyle, width: 160 }}>
            <option value="">All Statuses</option>
            {statusOptions.filter(Boolean).map(s => <option key={s} value={s}>{s.replace(/_/g, ' ')}</option>)}
          </select>
        </div>
        <div>
          <label style={{ display: 'block', fontSize: '0.8rem', marginBottom: 2, color: '#666' }}>Priority</label>
          <select value={priorityFilter} onChange={e => { setPriorityFilter(e.target.value); setPage(1); }} style={{ ...selectStyle, width: 160 }}>
            <option value="">All Priorities</option>
            {priorityOptions.filter(Boolean).map(p => <option key={p} value={p}>{p}</option>)}
          </select>
        </div>
      </div>

      {loading && <LoadingSpinner />}
      {error && <ErrorMessage message={error} onRetry={refetch} />}
      {!loading && !error && filteredData.length === 0 && (
        <EmptyState message="No work orders found" actionLabel="Create Work Order" onAction={() => setShowCreate(true)} />
      )}
      {!loading && !error && filteredData.length > 0 && (
        <>
          <DataTable<WorkOrder>
            columns={columns}
            data={filteredData}
            onRowClick={(wo) => navigate(`/work-orders/${wo.id}`)}
            onContextMenu={(wo, e) => setCtxMenu({ x: e.clientX, y: e.clientY, order: wo })}
          />
          <Pagination page={page} pageSize={20} total={data?.total || 0} onPageChange={setPage} />
        </>
      )}

      {/* Context Menu */}
      {ctxMenu && (
        <ContextMenu x={ctxMenu.x} y={ctxMenu.y} onClose={() => setCtxMenu(null)} items={[
          { label: 'View Details', onClick: () => navigate(`/work-orders/${ctxMenu.order.id}`) },
          {
            label: 'Update Status',
            onClick: () => {
              const nextStatus: Record<string, string> = { submitted: 'dispatched', dispatched: 'in_progress', in_progress: 'completed' };
              const next = nextStatus[ctxMenu.order.status];
              if (next) handleStatusUpdate(ctxMenu.order, next);
              else alert('Cannot advance status from ' + ctxMenu.order.status);
            },
            disabled: !isMaintenance || ctxMenu.order.status === 'completed' || ctxMenu.order.status === 'closed' || ctxMenu.order.status === 'cancelled',
          },
          {
            label: 'Close Order',
            onClick: () => handleClose(ctxMenu.order),
            disabled: !isMaintenance || ctxMenu.order.status !== 'completed',
          },
          {
            label: 'Cancel / Void',
            onClick: () => handleCancel(ctxMenu.order),
            disabled: !isMaintenance || !['submitted', 'dispatched'].includes(ctxMenu.order.status),
          },
          {
            label: 'Export / Print',
            onClick: () => navigate(`/work-orders/${ctxMenu.order.id}`),
          },
        ]} />
      )}

      {/* Create Modal */}
      {showCreate && (
        <Modal title="New Work Order" onClose={() => { setShowCreate(false); setCreateErr(''); setSelectedFiles([]); }} width={500}>
          {createErr && <div style={{ color: '#dc3545', marginBottom: '0.75rem', fontSize: '0.85rem' }}>{createErr}</div>}
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Trade *</label>
            <select value={createForm.trade} onChange={e => setCreateForm({ ...createForm, trade: e.target.value })} style={selectStyle}>
              {tradeOptions.map(t => <option key={t} value={t}>{t.charAt(0).toUpperCase() + t.slice(1)}</option>)}
            </select>
          </div>
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Priority *</label>
            <select value={createForm.priority} onChange={e => setCreateForm({ ...createForm, priority: e.target.value })} style={selectStyle}>
              <option value="urgent">Urgent</option>
              <option value="high">High</option>
              <option value="normal">Normal</option>
            </select>
          </div>
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Description *</label>
            <textarea value={createForm.description} onChange={e => setCreateForm({ ...createForm, description: e.target.value })}
              rows={4} style={{ ...inputStyle, resize: 'vertical' }} placeholder="Describe the issue..." />
          </div>
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Location *</label>
            <input value={createForm.location} onChange={e => setCreateForm({ ...createForm, location: e.target.value })}
              style={inputStyle} placeholder="Building, floor, room..." />
          </div>
          <div style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Photos (optional)</label>
            <input
              type="file"
              multiple
              accept="image/*,application/pdf"
              onChange={e => setSelectedFiles(Array.from(e.target.files || []))}
              style={{ width: '100%', fontSize: '0.85rem' }}
            />
            {selectedFiles.length > 0 && (
              <div style={{ marginTop: '0.3rem', fontSize: '0.8rem', color: '#666' }}>
                {selectedFiles.length} file{selectedFiles.length > 1 ? 's' : ''} selected
              </div>
            )}
          </div>
          <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
            <button onClick={() => { setShowCreate(false); setCreateErr(''); setSelectedFiles([]); }} style={btnSecondary}>Cancel</button>
            <button onClick={handleCreate} disabled={createSubmitting} style={createSubmitting ? btnDisabled : btnPrimary}>
              {createSubmitting ? 'Creating...' : 'Create Work Order'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default WorkOrdersPage;
