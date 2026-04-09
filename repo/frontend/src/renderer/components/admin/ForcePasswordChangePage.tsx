/**
 * ForcePasswordChangePage — shown when the logged-in user has must_change_password=true.
 * Blocks access to all other routes until a new password is set.
 */
import React, { useState, FormEvent } from 'react';
import { authAPI } from '../../services/api';
import { useAuth } from '../../hooks/useAuth';

const cardStyle: React.CSSProperties = {
  maxWidth: 440,
  margin: '10vh auto',
  backgroundColor: '#fff',
  borderRadius: 8,
  padding: '2rem',
  boxShadow: '0 2px 16px rgba(0,0,0,0.12)',
  border: '1px solid #e0e0e0',
};

const inputStyle: React.CSSProperties = {
  width: '100%',
  padding: '0.6rem 0.75rem',
  border: '1px solid #ccc',
  borderRadius: 4,
  fontSize: '0.95rem',
  boxSizing: 'border-box',
  marginTop: '0.25rem',
};

const btnPrimary: React.CSSProperties = {
  width: '100%',
  padding: '0.7rem',
  backgroundColor: '#1976d2',
  color: '#fff',
  border: 'none',
  borderRadius: 4,
  cursor: 'pointer',
  fontWeight: 600,
  fontSize: '1rem',
  marginTop: '1.25rem',
};

const ForcePasswordChangePage: React.FC = () => {
  const { user, logout } = useAuth();
  const [oldPassword, setOldPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirm, setConfirm] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');

    if (!oldPassword) { setError('Current password is required'); return; }
    if (newPassword.length < 12) { setError('New password must be at least 12 characters'); return; }
    if (newPassword !== confirm) { setError('Passwords do not match'); return; }
    if (newPassword === oldPassword) { setError('New password must differ from current password'); return; }

    setLoading(true);
    try {
      await authAPI.changePassword(oldPassword, newPassword);
      // Update the stored user to clear must_change_password so the gate lifts
      const updated = { ...user!, must_change_password: false };
      localStorage.setItem('medops_user', JSON.stringify(updated));
      // Reload so useAuth reinitialises from the updated localStorage entry.
      // window.location.reload() is safe for both packaged file:// and dev http://
      // origins — unlike window.location.href = '/' which resolves to file:///
      // in packaged Electron and breaks navigation.
      window.location.reload();
    } catch (err: any) {
      setError(err.response?.data?.error || err.response?.data?.details || 'Password change failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ minHeight: '100vh', backgroundColor: '#f5f7fa' }}>
      <div style={cardStyle}>
        <h1 style={{ margin: '0 0 0.25rem', fontSize: '1.4rem', color: '#1976d2' }}>
          MedOps Console
        </h1>
        <p style={{ margin: '0 0 1.5rem', color: '#d32f2f', fontWeight: 500, fontSize: '0.95rem' }}>
          You must change your password before continuing.
        </p>
        <form onSubmit={handleSubmit} noValidate>
          <div style={{ marginBottom: '1rem' }}>
            <label style={{ fontSize: '0.875rem', fontWeight: 500, color: '#444' }}>
              Current password
              <input
                type="password"
                value={oldPassword}
                onChange={e => setOldPassword(e.target.value)}
                style={inputStyle}
                autoFocus
                autoComplete="current-password"
              />
            </label>
          </div>
          <div style={{ marginBottom: '1rem' }}>
            <label style={{ fontSize: '0.875rem', fontWeight: 500, color: '#444' }}>
              New password <span style={{ fontWeight: 400, color: '#777' }}>(min 12 characters)</span>
              <input
                type="password"
                value={newPassword}
                onChange={e => setNewPassword(e.target.value)}
                style={inputStyle}
                autoComplete="new-password"
              />
            </label>
          </div>
          <div style={{ marginBottom: '0.5rem' }}>
            <label style={{ fontSize: '0.875rem', fontWeight: 500, color: '#444' }}>
              Confirm new password
              <input
                type="password"
                value={confirm}
                onChange={e => setConfirm(e.target.value)}
                style={inputStyle}
                autoComplete="new-password"
              />
            </label>
          </div>
          {error && (
            <p style={{ color: '#d32f2f', fontSize: '0.875rem', margin: '0.5rem 0 0' }}>{error}</p>
          )}
          <button type="submit" disabled={loading} style={{ ...btnPrimary, opacity: loading ? 0.6 : 1, cursor: loading ? 'not-allowed' : 'pointer' }}>
            {loading ? 'Saving…' : 'Change Password'}
          </button>
        </form>
        <button
          onClick={logout}
          style={{ marginTop: '1rem', background: 'none', border: 'none', color: '#666', cursor: 'pointer', fontSize: '0.875rem', textDecoration: 'underline', padding: 0 }}
        >
          Sign out
        </button>
      </div>
    </div>
  );
};

export default ForcePasswordChangePage;
