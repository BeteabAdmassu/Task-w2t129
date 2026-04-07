import { useState, useEffect, useCallback } from 'react';
import { authAPI } from '../services/api';
import type { User } from '../types';

export function useAuth() {
  const [user, setUser] = useState<User | null>(() => {
    const stored = localStorage.getItem('medops_user');
    return stored ? JSON.parse(stored) : null;
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const login = useCallback(async (username: string, password: string) => {
    setLoading(true);
    setError(null);
    try {
      const res = await authAPI.login(username, password);
      const { token, user: userData } = res.data;
      localStorage.setItem('medops_token', token);
      localStorage.setItem('medops_user', JSON.stringify(userData));
      setUser(userData);
      return userData;
    } catch (err: any) {
      const msg = err.response?.data?.error || 'Login failed';
      setError(msg);
      throw new Error(msg);
    } finally {
      setLoading(false);
    }
  }, []);

  const logout = useCallback(async () => {
    try {
      await authAPI.logout();
    } catch {
      // ignore
    }
    localStorage.removeItem('medops_token');
    localStorage.removeItem('medops_user');
    setUser(null);
  }, []);

  const isAuthenticated = !!user;
  const hasRole = useCallback(
    (roles: string[]) => user ? roles.includes(user.role) : false,
    [user]
  );

  return { user, loading, error, login, logout, isAuthenticated, hasRole };
}
