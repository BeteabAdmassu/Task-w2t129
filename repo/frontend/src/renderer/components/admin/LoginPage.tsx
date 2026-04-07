import React, { useState, FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';

const LoginPage: React.FC = () => {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<{ username?: string; password?: string }>({});
  const { login, loading, error } = useAuth();
  const navigate = useNavigate();

  const validate = (): boolean => {
    const errors: { username?: string; password?: string } = {};
    if (!username.trim()) errors.username = 'Username is required';
    if (!password) errors.password = 'Password is required';
    setFieldErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    if (!validate()) return;
    try {
      await login(username.trim(), password);
      navigate('/dashboard');
    } catch {
      // error is set in useAuth
    }
  };

  const inputStyle = (hasError: boolean): React.CSSProperties => ({
    width: '100%',
    padding: '0.75rem',
    fontSize: '0.95rem',
    border: `1px solid ${hasError ? '#dc3545' : '#ccc'}`,
    borderRadius: 4,
    boxSizing: 'border-box',
    outline: 'none',
    transition: 'border-color 0.15s',
  });

  return (
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      backgroundColor: '#f0f2f5',
      fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif",
    }}>
      <div style={{
        width: 400,
        maxWidth: '90vw',
        backgroundColor: '#fff',
        borderRadius: 8,
        boxShadow: '0 4px 24px rgba(0,0,0,0.1)',
        overflow: 'hidden',
      }}>
        {/* Header */}
        <div style={{
          backgroundColor: '#1a237e',
          color: '#fff',
          padding: '2rem 2rem 1.5rem',
          textAlign: 'center',
        }}>
          <h1 style={{ margin: 0, fontSize: '1.4rem', fontWeight: 700 }}>MedOps Console</h1>
          <p style={{ margin: '0.5rem 0 0', opacity: 0.8, fontSize: '0.85rem' }}>Offline Operations Management</p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} style={{ padding: '2rem' }}>
          {error && (
            <div style={{
              padding: '0.75rem',
              backgroundColor: '#fdecea',
              border: '1px solid #f5c6cb',
              borderRadius: 4,
              color: '#721c24',
              fontSize: '0.9rem',
              marginBottom: '1rem',
            }}>
              {error}
            </div>
          )}

          <div style={{ marginBottom: '1.25rem' }}>
            <label style={{ display: 'block', marginBottom: '0.4rem', fontSize: '0.9rem', fontWeight: 600, color: '#333' }}>
              Username
            </label>
            <input
              type="text"
              value={username}
              onChange={(e) => { setUsername(e.target.value); setFieldErrors((prev) => ({ ...prev, username: undefined })); }}
              placeholder="Enter username"
              style={inputStyle(!!fieldErrors.username)}
              autoFocus
              autoComplete="username"
            />
            {fieldErrors.username && (
              <span style={{ fontSize: '0.8rem', color: '#dc3545', marginTop: '0.25rem', display: 'block' }}>{fieldErrors.username}</span>
            )}
          </div>

          <div style={{ marginBottom: '1.5rem' }}>
            <label style={{ display: 'block', marginBottom: '0.4rem', fontSize: '0.9rem', fontWeight: 600, color: '#333' }}>
              Password
            </label>
            <input
              type="password"
              value={password}
              onChange={(e) => { setPassword(e.target.value); setFieldErrors((prev) => ({ ...prev, password: undefined })); }}
              placeholder="Enter password"
              style={inputStyle(!!fieldErrors.password)}
              autoComplete="current-password"
            />
            {fieldErrors.password && (
              <span style={{ fontSize: '0.8rem', color: '#dc3545', marginTop: '0.25rem', display: 'block' }}>{fieldErrors.password}</span>
            )}
          </div>

          <button
            type="submit"
            disabled={loading}
            style={{
              width: '100%',
              padding: '0.75rem',
              fontSize: '1rem',
              fontWeight: 600,
              backgroundColor: loading ? '#6c757d' : '#1a237e',
              color: '#fff',
              border: 'none',
              borderRadius: 4,
              cursor: loading ? 'not-allowed' : 'pointer',
              transition: 'background-color 0.15s',
            }}
          >
            {loading ? 'Signing in...' : 'Sign In'}
          </button>
        </form>
      </div>
    </div>
  );
};

export default LoginPage;
