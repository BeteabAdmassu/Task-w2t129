import React, { useState, FormEvent, useEffect } from 'react';
import { usersAPI } from '../../services/api';
import { useFetch } from '../../hooks/useFetch';
import { useDraftAutoSave } from '../../hooks/useDraftAutoSave';
import { DraftRecoveryDialog } from '../common/DraftRecoveryDialog';
import type { User } from '../../types';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';
import EmptyState from '../common/EmptyState';
import DataTable from '../common/DataTable';
import Modal from '../common/Modal';
import ContextMenu from '../common/ContextMenu';

const ROLES = [
  'system_admin',
  'inventory_pharmacist',
  'learning_coordinator',
  'front_desk',
  'maintenance_tech',
];

const UsersPage: React.FC = () => {
  const { data: usersData, loading, error, refetch } = useFetch<{ data: User[] } | User[]>(
    () => usersAPI.list(),
    []
  );
  const users: User[] = Array.isArray(usersData) ? usersData : (usersData as any)?.data || [];

  // Ctrl+N shortcut: open create modal
  useEffect(() => {
    const handler = () => setShowCreate(true);
    window.addEventListener('medops:create-new', handler);
    return () => window.removeEventListener('medops:create-new', handler);
  }, []);

  // F2 shortcut: open edit-role modal for the first user in the list
  useEffect(() => {
    const handler = () => {
      if (users.length > 0) {
        setEditUser(users[0]);
        setEditRole(users[0].role);
      }
    };
    window.addEventListener('medops:edit-row', handler);
    return () => window.removeEventListener('medops:edit-row', handler);
  }, [users]);

  // Create modal
  const [showCreate, setShowCreate] = useState(false);
  const [createForm, setCreateForm] = useState({ username: '', password: '', role: ROLES[0] });
  const [createErrors, setCreateErrors] = useState<Record<string, string>>({});
  const [createLoading, setCreateLoading] = useState(false);
  const [createSuccess, setCreateSuccess] = useState('');

  // Draft auto-save: persists the create-user form every 30 s
  // Note: password field is intentionally included so the form can be recovered,
  // but draft storage is server-side (scoped to userID) — not localStorage.
  const { clearDraft: clearUserDraft } = useDraftAutoSave('user_create', null, createForm);

  // Edit modal
  const [editUser, setEditUser] = useState<User | null>(null);
  const [editRole, setEditRole] = useState('');
  const [editLoading, setEditLoading] = useState(false);
  const [editSuccess, setEditSuccess] = useState('');

  // Context menu
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; user: User } | null>(null);

  // General operation feedback
  const [opError, setOpError] = useState<string | null>(null);
  const [opSuccess, setOpSuccess] = useState<string | null>(null);

  const validateCreate = (): boolean => {
    const errs: Record<string, string> = {};
    if (!createForm.username.trim()) errs.username = 'Username is required';
    if (!createForm.password) errs.password = 'Password is required';
    else if (createForm.password.length < 12) errs.password = 'Password must be at least 12 characters';
    if (!createForm.role) errs.role = 'Role is required';
    setCreateErrors(errs);
    return Object.keys(errs).length === 0;
  };

  const handleCreate = async (e: FormEvent) => {
    e.preventDefault();
    if (!validateCreate()) return;
    setCreateLoading(true);
    setCreateSuccess('');
    try {
      await usersAPI.create({
        username: createForm.username.trim(),
        password: createForm.password,
        role: createForm.role,
      });
      setCreateSuccess('User created successfully');
      clearUserDraft();
      setCreateForm({ username: '', password: '', role: ROLES[0] });
      setCreateErrors({});
      refetch();
      setTimeout(() => {
        setShowCreate(false);
        setCreateSuccess('');
      }, 1500);
    } catch (err: any) {
      setCreateErrors({ _form: err.response?.data?.error || 'Failed to create user' });
    } finally {
      setCreateLoading(false);
    }
  };

  const handleEditRole = async () => {
    if (!editUser) return;
    setEditLoading(true);
    setEditSuccess('');
    try {
      await usersAPI.update(editUser.id, { role: editRole });
      setEditSuccess('Role updated successfully');
      refetch();
      setTimeout(() => {
        setEditUser(null);
        setEditSuccess('');
      }, 1500);
    } catch (err: any) {
      setOpError(err.response?.data?.error || 'Failed to update role');
    } finally {
      setEditLoading(false);
    }
  };

  const handleDeactivate = async (u: User) => {
    setOpError(null);
    setOpSuccess(null);
    try {
      await usersAPI.update(u.id, { is_active: !u.is_active });
      setOpSuccess(`User ${u.is_active ? 'deactivated' : 'activated'} successfully`);
      refetch();
      setTimeout(() => setOpSuccess(null), 3000);
    } catch (err: any) {
      setOpError(err.response?.data?.error || 'Operation failed');
    }
  };

  const handleUnlock = async (u: User) => {
    setOpError(null);
    setOpSuccess(null);
    try {
      await usersAPI.unlock(u.id);
      setOpSuccess('User unlocked successfully');
      refetch();
      setTimeout(() => setOpSuccess(null), 3000);
    } catch (err: any) {
      setOpError(err.response?.data?.error || 'Failed to unlock user');
    }
  };

  const columns = [
    { key: 'username', header: 'Username', sortable: true },
    {
      key: 'role', header: 'Role', sortable: true,
      render: (u: User) => (
        <span style={{
          padding: '0.15rem 0.5rem',
          borderRadius: 12,
          fontSize: '0.8rem',
          backgroundColor: '#e3f2fd',
          color: '#1565c0',
        }}>
          {u.role.replace(/_/g, ' ')}
        </span>
      ),
    },
    {
      key: 'is_active', header: 'Active', sortable: true,
      render: (u: User) => (
        <span style={{
          padding: '0.15rem 0.5rem',
          borderRadius: 12,
          fontSize: '0.8rem',
          backgroundColor: u.is_active ? '#e8f5e9' : '#fdecea',
          color: u.is_active ? '#2e7d32' : '#c62828',
        }}>
          {u.is_active ? 'Active' : 'Inactive'}
        </span>
      ),
    },
    {
      key: 'created_at', header: 'Created', sortable: true,
      render: (u: User) => new Date(u.created_at).toLocaleDateString(),
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

  const handleUserDraftRestore = (state: unknown) => {
    const s = state as typeof createForm;
    if (s && typeof s === 'object') {
      setCreateForm({
        username: (s as any).username || '',
        password: (s as any).password || '',
        role: (s as any).role || ROLES[0],
      });
      setShowCreate(true);
    }
  };

  if (loading) return <LoadingSpinner message="Loading users..." />;
  if (error) return <ErrorMessage message={error} onRetry={refetch} />;

  return (
    <div>
      <DraftRecoveryDialog
        formType="user_create"
        onRestore={handleUserDraftRestore}
        onDiscard={clearUserDraft}
      />
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
        <h1 style={{ margin: 0, fontSize: '1.5rem', color: '#333' }}>User Management</h1>
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
          + Create User
        </button>
      </div>

      {opError && <ErrorMessage message={opError} />}
      {opSuccess && (
        <div style={{ padding: '0.75rem', backgroundColor: '#e8f5e9', border: '1px solid #c8e6c9', borderRadius: 4, color: '#2e7d32', fontSize: '0.9rem', marginBottom: '1rem' }}>
          {opSuccess}
        </div>
      )}

      {users.length === 0 ? (
        <EmptyState message="No users found" actionLabel="Create User" onAction={() => setShowCreate(true)} />
      ) : (
        <DataTable
          columns={columns}
          data={users}
          onContextMenu={(u, e) => setCtxMenu({ x: e.clientX, y: e.clientY, user: u })}
        />
      )}

      {/* Context menu */}
      {ctxMenu && (
        <ContextMenu
          x={ctxMenu.x}
          y={ctxMenu.y}
          onClose={() => setCtxMenu(null)}
          items={[
            { label: 'Edit Role', onClick: () => { setEditUser(ctxMenu.user); setEditRole(ctxMenu.user.role); } },
            { label: ctxMenu.user.is_active ? 'Deactivate' : 'Activate', onClick: () => handleDeactivate(ctxMenu.user) },
            { label: 'Unlock Account', onClick: () => handleUnlock(ctxMenu.user) },
          ]}
        />
      )}

      {/* Create user modal */}
      {showCreate && (
        <Modal title="Create User" onClose={() => { setShowCreate(false); setCreateErrors({}); setCreateSuccess(''); }}>
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
              <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Username *</label>
              <input
                type="text"
                value={createForm.username}
                onChange={(e) => { setCreateForm((p) => ({ ...p, username: e.target.value })); setCreateErrors((p) => ({ ...p, username: '' })); }}
                style={inputStyle(!!createErrors.username)}
                placeholder="Enter username"
              />
              {createErrors.username && <span style={{ fontSize: '0.8rem', color: '#dc3545' }}>{createErrors.username}</span>}
            </div>

            <div style={{ marginBottom: '1rem' }}>
              <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Password *</label>
              <input
                type="password"
                value={createForm.password}
                onChange={(e) => { setCreateForm((p) => ({ ...p, password: e.target.value })); setCreateErrors((p) => ({ ...p, password: '' })); }}
                style={inputStyle(!!createErrors.password)}
                placeholder="Minimum 12 characters"
              />
              {createErrors.password && <span style={{ fontSize: '0.8rem', color: '#dc3545' }}>{createErrors.password}</span>}
            </div>

            <div style={{ marginBottom: '1.5rem' }}>
              <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Role *</label>
              <select
                value={createForm.role}
                onChange={(e) => setCreateForm((p) => ({ ...p, role: e.target.value }))}
                style={{ ...inputStyle(false), cursor: 'pointer' }}
              >
                {ROLES.map((r) => (
                  <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>
                ))}
              </select>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
              <button
                type="button"
                onClick={() => { setShowCreate(false); setCreateErrors({}); setCreateSuccess(''); }}
                style={{ padding: '0.5rem 1rem', border: '1px solid #ccc', borderRadius: 4, backgroundColor: '#fff', cursor: 'pointer' }}
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={createLoading}
                style={{
                  padding: '0.5rem 1.25rem',
                  backgroundColor: createLoading ? '#6c757d' : '#1a237e',
                  color: '#fff',
                  border: 'none',
                  borderRadius: 4,
                  cursor: createLoading ? 'not-allowed' : 'pointer',
                  fontWeight: 500,
                }}
              >
                {createLoading ? 'Creating...' : 'Create User'}
              </button>
            </div>
          </form>
        </Modal>
      )}

      {/* Edit role modal */}
      {editUser && (
        <Modal title={`Edit Role: ${editUser.username}`} onClose={() => { setEditUser(null); setEditSuccess(''); }}>
          {editSuccess && (
            <div style={{ padding: '0.5rem', backgroundColor: '#e8f5e9', borderRadius: 4, color: '#2e7d32', fontSize: '0.85rem', marginBottom: '1rem' }}>
              {editSuccess}
            </div>
          )}
          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.3rem', fontSize: '0.85rem', fontWeight: 600 }}>Role</label>
            <select
              value={editRole}
              onChange={(e) => setEditRole(e.target.value)}
              style={{ ...inputStyle(false), cursor: 'pointer' }}
            >
              {ROLES.map((r) => (
                <option key={r} value={r}>{r.replace(/_/g, ' ')}</option>
              ))}
            </select>
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
            <button
              type="button"
              onClick={() => { setEditUser(null); setEditSuccess(''); }}
              style={{ padding: '0.5rem 1rem', border: '1px solid #ccc', borderRadius: 4, backgroundColor: '#fff', cursor: 'pointer' }}
            >
              Cancel
            </button>
            <button
              onClick={handleEditRole}
              disabled={editLoading}
              style={{
                padding: '0.5rem 1.25rem',
                backgroundColor: editLoading ? '#6c757d' : '#1a237e',
                color: '#fff',
                border: 'none',
                borderRadius: 4,
                cursor: editLoading ? 'not-allowed' : 'pointer',
                fontWeight: 500,
              }}
            >
              {editLoading ? 'Saving...' : 'Save Changes'}
            </button>
          </div>
        </Modal>
      )}
    </div>
  );
};

export default UsersPage;
