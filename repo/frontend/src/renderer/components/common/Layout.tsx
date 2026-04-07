import React, { useEffect, useState, useRef } from 'react';
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
  { label: 'Dashboard', path: '/', shortcut: 'Alt+D', roles: 'all' },
  { label: 'Users', path: '/users', shortcut: 'Alt+U', roles: ['system_admin'] },
  { label: 'Rate Tables', path: '/rate-tables', shortcut: 'Alt+R', roles: ['system_admin'] },
  { label: 'Statements', path: '/statements', shortcut: 'Alt+T', roles: ['system_admin'] },
  { label: 'System Config', path: '/system-config', shortcut: 'Alt+Y', roles: ['system_admin'] },
  { label: 'SKUs', path: '/skus', shortcut: 'Alt+S', roles: ['system_admin', 'inventory_pharmacist'] },
  { label: 'Stocktakes', path: '/stocktakes', shortcut: 'Alt+K', roles: ['system_admin', 'inventory_pharmacist'] },
  { label: 'Learning', path: '/learning', shortcut: 'Alt+L', roles: ['learning_coordinator'] },
  { label: 'Members', path: '/members', shortcut: 'Alt+M', roles: ['front_desk', 'system_admin'] },
  { label: 'Work Orders', path: '/work-orders', shortcut: 'Alt+W', roles: 'all' },
];

const Layout: React.FC<Props> = ({ children }) => {
  const { user, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [paletteQuery, setPaletteQuery] = useState('');
  const paletteInputRef = useRef<HTMLInputElement>(null);

  const visibleItems = NAV_ITEMS.filter((item) => {
    if (item.roles === 'all') return true;
    return user ? item.roles.includes(user.role) : false;
  });

  const filteredPaletteItems = paletteQuery.trim()
    ? visibleItems.filter((item) =>
        item.label.toLowerCase().includes(paletteQuery.toLowerCase())
      )
    : visibleItems;

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      // Alt+<letter> navigation shortcuts
      if (e.altKey && !e.ctrlKey && !e.metaKey) {
        const key = e.key.toLowerCase();
        const shortcutMap: Record<string, string> = {};
        visibleItems.forEach((item) => {
          const k = item.shortcut.split('+')[1]?.toLowerCase();
          if (k) shortcutMap[k] = item.path;
        });
        if (shortcutMap[key]) {
          e.preventDefault();
          navigate(shortcutMap[key]);
          return;
        }
      }

      // Ctrl+K — open command palette
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        setPaletteOpen((prev) => !prev);
        setPaletteQuery('');
        return;
      }

      // Ctrl+N — navigate to a create/new action (goes to current section list)
      if ((e.ctrlKey || e.metaKey) && e.key === 'n') {
        e.preventDefault();
        // Strip any trailing /:id to get the list route
        const listPath = location.pathname.replace(/\/[^/]+$/, '') || location.pathname;
        navigate(listPath);
        return;
      }

      // Ctrl+Enter — submit the focused form
      if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
        const activeForm = document.activeElement?.closest('form');
        if (activeForm) {
          e.preventDefault();
          activeForm.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
        }
        return;
      }

      // F2 — focus the first input on the page
      if (e.key === 'F2') {
        const firstInput = document.querySelector<HTMLInputElement>('main input:not([type=hidden]), main textarea, main select');
        if (firstInput) {
          e.preventDefault();
          firstInput.focus();
        }
        return;
      }

      // Escape — close palette
      if (e.key === 'Escape' && paletteOpen) {
        setPaletteOpen(false);
        return;
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [visibleItems, navigate, location.pathname, paletteOpen]);

  useEffect(() => {
    if (paletteOpen) {
      setTimeout(() => paletteInputRef.current?.focus(), 50);
    }
  }, [paletteOpen]);

  const handleLogout = async () => {
    await logout();
    navigate('/login');
  };

  return (
    <div style={{ display: 'flex', minHeight: '100vh', fontFamily: "'Segoe UI', Tahoma, Geneva, Verdana, sans-serif" }}>
      {/* Command palette (Ctrl+K) */}
      {paletteOpen && (
        <div
          style={{ position: 'fixed', inset: 0, backgroundColor: 'rgba(0,0,0,0.4)', zIndex: 1000, display: 'flex', alignItems: 'flex-start', justifyContent: 'center', paddingTop: '15vh' }}
          onClick={() => setPaletteOpen(false)}
        >
          <div
            style={{ backgroundColor: '#fff', borderRadius: 8, boxShadow: '0 8px 32px rgba(0,0,0,0.2)', width: 480, maxWidth: '90vw', overflow: 'hidden' }}
            onClick={(e) => e.stopPropagation()}
          >
            <input
              ref={paletteInputRef}
              value={paletteQuery}
              onChange={(e) => setPaletteQuery(e.target.value)}
              placeholder="Go to… (type to filter)"
              style={{ width: '100%', padding: '1rem', fontSize: '1rem', border: 'none', borderBottom: '1px solid #eee', outline: 'none', boxSizing: 'border-box' }}
            />
            <div style={{ maxHeight: 320, overflowY: 'auto' }}>
              {filteredPaletteItems.map((item) => (
                <button
                  key={item.path}
                  onClick={() => { navigate(item.path); setPaletteOpen(false); }}
                  style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', width: '100%', padding: '0.75rem 1rem', border: 'none', backgroundColor: 'transparent', cursor: 'pointer', fontSize: '0.9rem', textAlign: 'left' }}
                  onMouseEnter={(e) => { e.currentTarget.style.backgroundColor = '#f5f5f5'; }}
                  onMouseLeave={(e) => { e.currentTarget.style.backgroundColor = 'transparent'; }}
                >
                  <span>{item.label}</span>
                  <span style={{ fontSize: '0.75rem', color: '#999', fontFamily: 'monospace' }}>{item.shortcut}</span>
                </button>
              ))}
              {filteredPaletteItems.length === 0 && (
                <div style={{ padding: '1rem', color: '#999', fontSize: '0.9rem' }}>No matching pages</div>
              )}
            </div>
          </div>
        </div>
      )}
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
            const active = item.path === '/'
              ? location.pathname === '/'
              : location.pathname.startsWith(item.path);
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
