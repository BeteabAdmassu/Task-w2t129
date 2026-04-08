import React, { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { membersAPI } from '../../services/api';
import { useFetch } from '../../hooks/useFetch';
import { useDraftAutoSave } from '../../hooks/useDraftAutoSave';
import type { Member, MembershipTier, PaginatedResponse } from '../../types';
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

const statusColors: Record<string, { bg: string; color: string }> = {
  active: { bg: '#d4edda', color: '#155724' },
  frozen: { bg: '#cce5ff', color: '#004085' },
  expired: { bg: '#f8d7da', color: '#721c24' },
};

const MembersPage: React.FC = () => {
  const navigate = useNavigate();
  const [search, setSearch] = useState('');
  const [page, setPage] = useState(1);

  const { data, loading, error, refetch } = useFetch<PaginatedResponse<Member>>(
    () => membersAPI.list({ search: search || undefined, page, page_size: 20 }).then(r => ({ data: r.data })),
    [search, page]
  );

  const { data: tiers } = useFetch<MembershipTier[]>(
    () => membersAPI.listTiers().then(r => ({ data: r.data.data || r.data })),
    []
  );

  // Create modal
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ name: '', id_number: '', phone: '', tier_id: '' });
  const [createErr, setCreateErr] = useState('');
  const [createSubmitting, setCreateSubmitting] = useState(false);
  const [createSuccess, setCreateSuccess] = useState('');

  const { clearDraft: clearMemberDraft } = useDraftAutoSave('member_create', null, createForm);

  // Context menu
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; member: Member } | null>(null);

  // Action state
  const [actionMsg, setActionMsg] = useState('');

  const handleCreate = async () => {
    if (!createForm.name.trim()) { setCreateErr('Name is required'); return; }
    if (!createForm.phone.trim()) { setCreateErr('Phone is required'); return; }
    if (!/^\+?[\d\s-]{7,15}$/.test(createForm.phone.trim())) { setCreateErr('Invalid phone number format'); return; }
    if (!createForm.tier_id) { setCreateErr('Tier is required'); return; }
    setCreateSubmitting(true);
    setCreateErr('');
    try {
      await membersAPI.create({
        name: createForm.name.trim(),
        id_number: createForm.id_number.trim(),
        phone: createForm.phone.trim(),
        tier_id: createForm.tier_id,
      });
      clearMemberDraft();
      setCreateSuccess('Member created successfully');
      setShowCreate(false);
      setCreateForm({ name: '', id_number: '', phone: '', tier_id: '' });
      refetch();
      setTimeout(() => setCreateSuccess(''), 3000);
    } catch (e: any) {
      setCreateErr(e.response?.data?.error || 'Failed to create member');
    } finally {
      setCreateSubmitting(false);
    }
  };

  const handleFreeze = async (member: Member) => {
    try {
      await membersAPI.freeze(member.id);
      setActionMsg(`${member.name} has been frozen`);
      refetch();
      setTimeout(() => setActionMsg(''), 3000);
    } catch (e: any) {
      alert('Failed to freeze: ' + (e.response?.data?.error || e.message));
    }
  };

  const handleUnfreeze = async (member: Member) => {
    try {
      await membersAPI.unfreeze(member.id);
      setActionMsg(`${member.name} has been unfrozen`);
      refetch();
      setTimeout(() => setActionMsg(''), 3000);
    } catch (e: any) {
      alert('Failed to unfreeze: ' + (e.response?.data?.error || e.message));
    }
  };

  const tierName = (tierId: string) => {
    const tier = tiers?.find(t => t.id === tierId);
    return tier ? tier.name : tierId;
  };

  const columns = [
    { key: 'name', header: 'Name', sortable: true },
    { key: 'phone', header: 'Phone', sortable: true },
    { key: 'tier_id', header: 'Tier', sortable: true, render: (m: Member) => <span style={{ textTransform: 'capitalize' as const }}>{tierName(m.tier_id)}</span> },
    { key: 'points_balance', header: 'Points', sortable: true, render: (m: Member) => m.points_balance.toLocaleString() },
    { key: 'stored_value', header: 'Stored Value', sortable: true, render: (m: Member) => `$${m.stored_value.toFixed(2)}` },
    {
      key: 'status', header: 'Status', sortable: true,
      render: (m: Member) => {
        const sc = statusColors[m.status] || { bg: '#eee', color: '#333' };
        return <span style={{ padding: '0.2rem 0.6rem', borderRadius: 12, fontSize: '0.8rem', fontWeight: 600, backgroundColor: sc.bg, color: sc.color }}>{m.status}</span>;
      },
    },
    {
      key: 'expires_at', header: 'Expires', sortable: true,
      render: (m: Member) => {
        const d = new Date(m.expires_at);
        const expired = d < new Date();
        return <span style={{ color: expired ? '#dc3545' : '#333' }}>{d.toLocaleDateString()}{expired ? ' (expired)' : ''}</span>;
      },
    },
  ];

  const members = data?.data || [];

  return (
    <div style={{ padding: '1.5rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h2 style={{ margin: 0 }}>Members</h2>
        <button onClick={() => { setShowCreate(true); if (tiers && tiers.length > 0 && !createForm.tier_id) setCreateForm(f => ({ ...f, tier_id: tiers[0].id })); }} style={btnPrimary}>+ New Member</button>
      </div>

      {createSuccess && <div style={successStyle}>{createSuccess}</div>}
      {actionMsg && <div style={successStyle}>{actionMsg}</div>}

      {/* Search */}
      <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem' }}>
        <input
          type="text" placeholder="Search by name or phone..." value={search}
          onChange={e => { setSearch(e.target.value); setPage(1); }}
          style={{ ...inputStyle, maxWidth: 350 }}
        />
      </div>

      {loading && <LoadingSpinner />}
      {error && <ErrorMessage message={error} onRetry={refetch} />}
      {!loading && !error && members.length === 0 && (
        <EmptyState message="No members found" actionLabel="Create Member" onAction={() => setShowCreate(true)} />
      )}
      {!loading && !error && members.length > 0 && (
        <>
          <DataTable<Member>
            columns={columns}
            data={members}
            onRowClick={(m) => navigate(`/members/${m.id}`)}
            onContextMenu={(m, e) => setCtxMenu({ x: e.clientX, y: e.clientY, member: m })}
          />
          <Pagination page={page} pageSize={20} total={data?.total || 0} onPageChange={setPage} />
        </>
      )}

      {/* Context Menu */}
      {ctxMenu && (
        <ContextMenu x={ctxMenu.x} y={ctxMenu.y} onClose={() => setCtxMenu(null)} items={[
          { label: 'View Details / Edit', onClick: () => navigate(`/members/${ctxMenu.member.id}`) },
          {
            label: 'Add Value / Top-Up',
            onClick: () => navigate(`/members/${ctxMenu.member.id}`),
            disabled: ctxMenu.member.status === 'frozen' || ctxMenu.member.status === 'expired',
          },
          {
            label: 'Redeem Benefit',
            onClick: () => navigate(`/members/${ctxMenu.member.id}`),
            disabled: ctxMenu.member.status !== 'active',
          },
          {
            label: 'Freeze',
            onClick: () => handleFreeze(ctxMenu.member),
            disabled: ctxMenu.member.status === 'frozen',
          },
          {
            label: 'Unfreeze',
            onClick: () => handleUnfreeze(ctxMenu.member),
            disabled: ctxMenu.member.status !== 'frozen',
          },
        ]} />
      )}

      {/* Create Modal */}
      {showCreate && (
        <Modal title="New Member" onClose={() => { setShowCreate(false); setCreateErr(''); }} width={480}>
          {createErr && <div style={{ color: '#dc3545', marginBottom: '0.75rem', fontSize: '0.85rem' }}>{createErr}</div>}
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Name *</label>
            <input value={createForm.name} onChange={e => setCreateForm({ ...createForm, name: e.target.value })} style={inputStyle} placeholder="Full name" />
          </div>
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>ID Number</label>
            <input value={createForm.id_number} onChange={e => setCreateForm({ ...createForm, id_number: e.target.value })} style={inputStyle} placeholder="Government ID" />
          </div>
          <div style={{ marginBottom: '0.75rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Phone *</label>
            <input value={createForm.phone} onChange={e => setCreateForm({ ...createForm, phone: e.target.value })} style={inputStyle} placeholder="+1-234-567-8900" />
          </div>
          <div style={{ marginBottom: '1rem' }}>
            <label style={{ display: 'block', marginBottom: '0.25rem', fontWeight: 500 }}>Tier *</label>
            <select value={createForm.tier_id} onChange={e => setCreateForm({ ...createForm, tier_id: e.target.value })} style={selectStyle}>
              <option value="">Select tier...</option>
              {tiers && tiers.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
            </select>
          </div>
          <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
            <button onClick={() => { setShowCreate(false); setCreateErr(''); }} style={btnSecondary}>Cancel</button>
            <button onClick={handleCreate} disabled={createSubmitting} style={createSubmitting ? btnDisabled : btnPrimary}>
              {createSubmitting ? 'Creating...' : 'Create Member'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default MembersPage;
