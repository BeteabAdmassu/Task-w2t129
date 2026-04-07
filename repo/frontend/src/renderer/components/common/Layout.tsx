import React, { useEffect } from 'react';
import { useNavigate, useLocation, Link } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';

interface Props {
  children: React.ReactNode;
}

interface NavItem {
  label: string;
  path: string;
  shortcut: string;
  roles: string[] | 'all';
}

const NAV_ITEMS: NavItem[] = [
  { label: 'Dashboard', path: '/dashboard', shortcut: 'Alt+D', roles: 'all' },
  { label: 'Users', path: '/users', shortcut: 'Alt+U', roles: ['system_admin'] },
  { label: 'Rate Tables', path: '/rate-tables', shortcut: 'Alt+R', roles: ['system_admin'] },
  { label: 'Statements', path: '/statements', shortcut: 'Alt+T', roles: ['system_admin'] },
  { label: 'System Config', path: '/system-config', shortcut: 'Alt+Y', roles: ['system_admin'] },
  { label: 'SKUs', path: '/skus', shortcut: 'Alt+S', roles: ['inventory_pharmacist'] },
  { label: 'Inventory', path: '/inventory', shortcut: 'Alt+I', roles: ['inventory_pharmacist'] },
  { label: 'Stocktakes', path: '/stocktakes', shortcut: 'Alt+K', roles: ['inventory_pharmacist'] },
  { label: 'Learning', path: '/learning', shortcut: 'Alt+L', roles: ['learning_coordinator'] },
  { label: 'Members', path: '/members', shortcut: 'Alt+M', roles: ['front_desk'] },
  { label: 'Work Orders', path: '/work-orders', shortcut: 'Alt+W', roles: 'all' },
];

const Layout: React.FC<Props> = ({ children }) => {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();

  const visibleItems = NAV_ITEMS.filter((item) => {
    if (item.roles === 'all') return true;
    return user ? item.roles.includes(user.role) : false;
  });

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (!e.altKey) return;
      const key = e.key.toLowerCase();
      const shortcutMap: Record<string, string> = {};
      visibleItems.forEach((item) => {
        const k = item.shortcut.split('+')[1]?.toLowerCase();
        if (k) shortcutMap[k] = item.path;
      });
      if (shortcutMap[key]) {
        e.preventDefault();
        navigate(shortcutMap[key]);
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [visibleItems, navigate]);

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  return (
    <div style={{ display: 'flex', minHeight: '100vh', fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif" }}>
      {/* Sidebar */}
      <aside style={{
        width: 240,
        backgroundColor: '#1a237e',
        color: '#fff',
        display: 'flex',
        flexDirection: 'column',
        flexShrink: 0,
      }}>
        <div style={{ padding: '1.5rem 1rem', borderBottom: '1px solid rgba(255,255,255,0.1)' }}>
          <h2 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, letterSpacing: '0.05em' }}>MedOps Console</h2>
          <span style={{ fontSize: '0.75rem', opacity: 0.7 }}>Offline Operations</span>
        </div>

        <nav style={{ flex: 1, padding: '0.5rem 0', overflowY: 'auto' }}>
          {visibleItems.map((item) => {
            const active = location.pathname.startsWith(item.path);
            return (
              <Link
                key={item.path}
                to={item.path}
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  padding: '0.65rem 1rem',
                  color: active ? '#fff' : 'rgba(255,255,255,0.75)',
                  textDecoration: 'none',
                  backgroundColor: active ? 'rgba(255,255,255,0.15)' : 'transparent',
                  borderLeft: active ? '3px solid #64b5f6' : '3px solid transparent',
                  fontSize: '0.9rem',
                  transition: 'background-color 0.15s',
                }}
                onMouseEnter={(e) => {
                  if (!active) e.currentTarget.style.backgroundColor = 'rgba(255,255,255,0.08)';
                }}
                onMouseLeave={(e) => {
                  if (!active) e.currentTarget.style.backgroundColor = 'transparent';
                }}
              >
                <span>{item.label}</span>
                <span style={{ fontSize: '0.65rem', opacity: 0.5, fontFamily: 'monospace' }}>{item.shortcut}</span>
              </Link>
            );
          })}
        </nav>

        <div style={{ padding: '1rem', borderTop: '1px solid rgba(255,255,255,0.1)', fontSize: '0.8rem', opacity: 0.6 }}>
          v1.0.0
        </div>
      </aside>

      {/* Main content area */}
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minWidth: 0 }}>
        {/* Header */}
        <header style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          padding: '0.75rem 1.5rem',
          backgroundColor: '#fff',
          borderBottom: '1px solid #e0e0e0',
          boxShadow: '0 1px 3px rgba(0,0,0,0.05)',
        }}>
          <div style={{ fontSize: '0.85rem', color: '#666' }}>
            {location.pathname.split('/').filter(Boolean).map((seg) => seg.replace(/-/g, ' ')).map((s) => s.charAt(0).toUpperCase() + s.slice(1)).join(' / ')}
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
            <span style={{ fontSize: '0.9rem', color: '#333' }}>
              <strong>{user?.username}</strong>
              <span style={{
                marginLeft: '0.5rem',
                fontSize: '0.75rem',
                backgroundColor: '#e3f2fd',
                color: '#1565c0',
                padding: '0.15rem 0.5rem',
                borderRadius: 12,
              }}>
                {user?.role?.replace(/_/g, ' ')}
              </span>
            </span>
            <button
              onClick={handleLogout}
              style={{
                padding: '0.4rem 1rem',
                fontSize: '0.85rem',
                backgroundColor: 'transparent',
                border: '1px solid #ccc',
                borderRadius: 4,
                cursor: 'pointer',
                color: '#666',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = '#f5f5f5';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = 'transparent';
              }}
            >
              Logout
            </button>
          </div>
        </header>

        {/* Page content */}
        <main style={{ flex: 1, padding: '1.5rem', backgroundColor: '#f5f7fa', overflowY: 'auto' }}>
          {children}
        </main>
      </div>
    </div>
  );
};

export default Layout;
