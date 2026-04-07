import React, { useState, useEffect } from 'react';
import { systemAPI } from '../../services/api';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';

const cardStyle: React.CSSProperties = {
  backgroundColor: '#fff',
  borderRadius: 8,
  padding: '1.25rem',
  boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
  border: '1px solid #e0e0e0',
  marginBottom: '1.5rem',
};

type ActionStatus = { type: 'success' | 'error'; message: string } | null;

const SystemConfigPage: React.FC = () => {
  const [config, setConfig] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [backupBusy, setBackupBusy] = useState(false);
  const [rollbackBusy, setRollbackBusy] = useState(false);
  const [confirmRollback, setConfirmRollback] = useState(false);
  const [actionStatus, setActionStatus] = useState<ActionStatus>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await systemAPI.getConfig();
      // Backend returns { config: { key: value, ... } }
      setConfig(res.data?.config || {});
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to load configuration');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleSave = async () => {
    setSaving(true);
    setActionStatus(null);
    try {
      // Backend PUT /system/config expects { key, value } — send one request per key
      for (const [key, value] of Object.entries(config)) {
        await systemAPI.updateConfig(key, value);
      }
      setActionStatus({ type: 'success', message: 'Configuration saved.' });
    } catch (err: any) {
      setActionStatus({ type: 'error', message: err.response?.data?.error || 'Save failed.' });
    } finally {
      setSaving(false);
    }
  };

  const handleBackup = async () => {
    setBackupBusy(true);
    setActionStatus(null);
    try {
      await systemAPI.backup();
      setActionStatus({ type: 'success', message: 'Backup initiated successfully.' });
    } catch (err: any) {
      setActionStatus({ type: 'error', message: err.response?.data?.error || 'Backup failed.' });
    } finally {
      setBackupBusy(false);
    }
  };

  const handleRollbackConfirmed = async () => {
    setConfirmRollback(false);
    setRollbackBusy(true);
    setActionStatus(null);
    try {
      await systemAPI.rollback();
      setActionStatus({ type: 'success', message: 'Rollback completed. The system has been restored to the previous state.' });
    } catch (err: any) {
      setActionStatus({ type: 'error', message: err.response?.data?.error || 'Rollback failed.' });
    } finally {
      setRollbackBusy(false);
    }
  };

  if (loading) return <LoadingSpinner message="Loading configuration..." />;
  if (error) return <ErrorMessage message={error} onRetry={load} />;

  return (
    <div>
      <div style={{ marginBottom: '1.5rem' }}>
        <h1 style={{ margin: 0, fontSize: '1.5rem', color: '#333' }}>System Configuration</h1>
        <p style={{ margin: '0.25rem 0 0', color: '#666', fontSize: '0.9rem' }}>
          Manage system settings and administrative operations.
        </p>
      </div>

      {actionStatus && (
        <div style={{
          padding: '0.75rem 1rem',
          borderRadius: 6,
          marginBottom: '1rem',
          backgroundColor: actionStatus.type === 'success' ? '#e8f5e9' : '#fdecea',
          color: actionStatus.type === 'success' ? '#2e7d32' : '#c62828',
          border: `1px solid ${actionStatus.type === 'success' ? '#a5d6a7' : '#ef9a9a'}`,
        }}>
          {actionStatus.message}
        </div>
      )}

      {/* Config key-value editor */}
      <div style={cardStyle}>
        <h2 style={{ margin: '0 0 1rem', fontSize: '1rem', color: '#333' }}>Configuration Values</h2>
        {Object.keys(config).length === 0 ? (
          <p style={{ color: '#999', fontSize: '0.9rem' }}>No configuration keys found.</p>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '0.75rem' }}>
            {Object.entries(config).map(([key, value]) => (
              <div key={key} style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
                <label style={{ minWidth: 200, fontWeight: 500, fontSize: '0.875rem', color: '#555' }}>{key}</label>
                <input
                  type="text"
                  value={value}
                  onChange={(e) => setConfig((prev) => ({ ...prev, [key]: e.target.value }))}
                  style={{ flex: 1, padding: '0.4rem 0.6rem', borderRadius: 4, border: '1px solid #ccc', fontSize: '0.875rem' }}
                />
              </div>
            ))}
          </div>
        )}
        <div style={{ marginTop: '1rem' }}>
          <button
            onClick={handleSave}
            disabled={saving}
            style={{
              padding: '0.6rem 1.4rem',
              backgroundColor: saving ? '#9e9e9e' : '#1a237e',
              color: '#fff',
              border: 'none',
              borderRadius: 4,
              cursor: saving ? 'not-allowed' : 'pointer',
              fontWeight: 500,
              fontSize: '0.875rem',
            }}
          >
            {saving ? 'Saving…' : 'Save Configuration'}
          </button>
        </div>
      </div>

      {/* System operations */}
      <div style={cardStyle}>
        <h2 style={{ margin: '0 0 0.5rem', fontSize: '1rem', color: '#333' }}>System Operations</h2>
        <p style={{ margin: '0 0 1rem', fontSize: '0.85rem', color: '#666' }}>
          These actions affect the entire system. Use with caution.
        </p>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.75rem' }}>
          <button
            onClick={handleBackup}
            disabled={backupBusy}
            style={{
              padding: '0.6rem 1.2rem',
              backgroundColor: backupBusy ? '#9e9e9e' : '#1565c0',
              color: '#fff',
              border: 'none',
              borderRadius: 4,
              cursor: backupBusy ? 'not-allowed' : 'pointer',
              fontWeight: 500,
              fontSize: '0.875rem',
            }}
          >
            {backupBusy ? 'Initiating Backup…' : 'Initiate Backup'}
          </button>

          <button
            onClick={() => setConfirmRollback(true)}
            disabled={rollbackBusy}
            style={{
              padding: '0.6rem 1.2rem',
              backgroundColor: rollbackBusy ? '#9e9e9e' : '#b71c1c',
              color: '#fff',
              border: 'none',
              borderRadius: 4,
              cursor: rollbackBusy ? 'not-allowed' : 'pointer',
              fontWeight: 500,
              fontSize: '0.875rem',
            }}
          >
            {rollbackBusy ? 'Rolling Back…' : 'Rollback System'}
          </button>
        </div>
      </div>

      {/* Rollback confirmation dialog */}
      {confirmRollback && (
        <div style={{
          position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.5)',
          display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000,
        }}>
          <div style={{
            backgroundColor: '#fff', borderRadius: 8, padding: '2rem',
            maxWidth: 440, width: '90%', boxShadow: '0 4px 24px rgba(0,0,0,0.2)',
          }}>
            <h3 style={{ margin: '0 0 0.75rem', color: '#b71c1c', fontSize: '1.1rem' }}>
              Confirm System Rollback
            </h3>
            <p style={{ margin: '0 0 1.5rem', fontSize: '0.9rem', color: '#555', lineHeight: 1.5 }}>
              This will restore the system to its previous state. All changes made since the last
              backup will be lost. This action cannot be undone. Are you sure you want to proceed?
            </p>
            <div style={{ display: 'flex', gap: '0.75rem', justifyContent: 'flex-end' }}>
              <button
                onClick={() => setConfirmRollback(false)}
                style={{
                  padding: '0.6rem 1.2rem', backgroundColor: '#f5f5f5',
                  color: '#333', border: '1px solid #ccc', borderRadius: 4,
                  cursor: 'pointer', fontWeight: 500, fontSize: '0.875rem',
                }}
              >
                Cancel
              </button>
              <button
                onClick={handleRollbackConfirmed}
                style={{
                  padding: '0.6rem 1.2rem', backgroundColor: '#b71c1c',
                  color: '#fff', border: 'none', borderRadius: 4,
                  cursor: 'pointer', fontWeight: 500, fontSize: '0.875rem',
                }}
              >
                Yes, Rollback
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default SystemConfigPage;
