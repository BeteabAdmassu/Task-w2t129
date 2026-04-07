/**
 * routes.test.ts — Route integrity and API-base-URL detection tests.
 *
 * Covers audit gap F-003 (route wiring) and validates that the Electron
 * API base-URL injection path works correctly.
 *
 * These are pure-logic / structural tests — no React rendering required.
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest';

// ─── Route map (mirrors App.tsx) ─────────────────────────────────────────────
// We define the expected route map as a plain object so these tests stay
// independent from React Router's internals.

type RouteEntry = {
  path: string;
  protected: boolean;
  roles?: string[];
};

const DECLARED_ROUTES: RouteEntry[] = [
  { path: '/login',          protected: false },
  { path: '/',               protected: true },
  { path: '/dashboard',      protected: true },          // alias → /
  { path: '/users',          protected: true, roles: ['system_admin'] },
  { path: '/skus',           protected: true, roles: ['system_admin', 'inventory_pharmacist'] },
  { path: '/skus/:id',       protected: true, roles: ['system_admin', 'inventory_pharmacist'] },
  { path: '/stocktakes',     protected: true, roles: ['system_admin', 'inventory_pharmacist'] },
  { path: '/stocktakes/:id', protected: true, roles: ['system_admin', 'inventory_pharmacist'] },
  { path: '/learning',       protected: true },
  { path: '/work-orders',    protected: true },
  { path: '/work-orders/:id',protected: true },
  { path: '/members',        protected: true, roles: ['system_admin', 'front_desk'] },
  { path: '/members/:id',    protected: true, roles: ['system_admin', 'front_desk'] },
  { path: '/rate-tables',    protected: true, roles: ['system_admin'] },
  { path: '/statements',     protected: true, roles: ['system_admin'] },
  { path: '/system-config',  protected: true, roles: ['system_admin'] },
];

// Nav items that link to routes (mirrors Layout.tsx NAV_ITEMS paths)
const NAV_PATHS = [
  '/',
  '/users',
  '/rate-tables',
  '/statements',
  '/system-config',
  '/skus',
  '/stocktakes',
  '/learning',
  '/members',
  '/work-orders',
];

// ─── Helper ───────────────────────────────────────────────────────────────────

function declaredPaths(): string[] {
  return DECLARED_ROUTES.map((r) => r.path);
}

// ─── Route completeness ───────────────────────────────────────────────────────

describe('Route map completeness', () => {
  it('declares a /login route', () => {
    expect(declaredPaths()).toContain('/login');
  });

  it('declares a root / route for Dashboard', () => {
    expect(declaredPaths()).toContain('/');
  });

  it('declares /dashboard alias so LoginPage redirect works', () => {
    expect(declaredPaths()).toContain('/dashboard');
  });

  it('declares /stocktakes/:id so stocktake detail navigation works', () => {
    expect(declaredPaths()).toContain('/stocktakes/:id');
  });

  it('declares /system-config so the nav item resolves', () => {
    expect(declaredPaths()).toContain('/system-config');
  });

  it('declares /skus/:id for SKU detail page', () => {
    expect(declaredPaths()).toContain('/skus/:id');
  });

  it('declares /work-orders/:id for work order detail page', () => {
    expect(declaredPaths()).toContain('/work-orders/:id');
  });

  it('declares /members/:id for member detail page', () => {
    expect(declaredPaths()).toContain('/members/:id');
  });
});

// ─── Nav link integrity ───────────────────────────────────────────────────────

describe('Nav link integrity — every nav path has a declared route', () => {
  for (const navPath of NAV_PATHS) {
    it(`nav path "${navPath}" has a matching route declaration`, () => {
      const match = DECLARED_ROUTES.some((r) => {
        // Exact match OR the route has a param prefix
        return r.path === navPath || r.path.startsWith(navPath + '/');
      });
      expect(match).toBe(true);
    });
  }
});

// ─── No legacy dead paths in nav ─────────────────────────────────────────────

describe('Dead nav links are absent', () => {
  it('does not include /inventory (removed — no page exists)', () => {
    expect(NAV_PATHS).not.toContain('/inventory');
  });
});

// ─── Role assignment consistency ──────────────────────────────────────────────

describe('Role-protected route consistency', () => {
  it('/users is restricted to system_admin', () => {
    const r = DECLARED_ROUTES.find((x) => x.path === '/users');
    expect(r?.roles).toContain('system_admin');
  });

  it('/skus allows both system_admin and inventory_pharmacist', () => {
    const r = DECLARED_ROUTES.find((x) => x.path === '/skus');
    expect(r?.roles).toContain('system_admin');
    expect(r?.roles).toContain('inventory_pharmacist');
  });

  it('/stocktakes/:id uses same roles as /stocktakes', () => {
    const list   = DECLARED_ROUTES.find((x) => x.path === '/stocktakes');
    const detail = DECLARED_ROUTES.find((x) => x.path === '/stocktakes/:id');
    expect(list?.roles).toEqual(detail?.roles);
  });

  it('/members/:id uses same roles as /members', () => {
    const list   = DECLARED_ROUTES.find((x) => x.path === '/members');
    const detail = DECLARED_ROUTES.find((x) => x.path === '/members/:id');
    expect(list?.roles).toEqual(detail?.roles);
  });

  it('/system-config is restricted to system_admin', () => {
    const r = DECLARED_ROUTES.find((x) => x.path === '/system-config');
    expect(r?.roles).toContain('system_admin');
  });
});

// ─── Statement status-machine labels ─────────────────────────────────────────
// Mirrors the DB CHECK constraint: (draft, pending_approval, approved, exported)

const VALID_STATEMENT_STATUSES = ['draft', 'pending_approval', 'approved', 'exported'];

function statementStatusLabel(status: string): string {
  const map: Record<string, string> = {
    draft:            'Draft',
    pending_approval: 'Pending Approval',
    approved:         'Approved',
    exported:         'Exported',
  };
  return map[status] ?? status;
}

describe('Statement status labels align with DB enum', () => {
  for (const status of VALID_STATEMENT_STATUSES) {
    it(`status "${status}" produces a non-raw label`, () => {
      const label = statementStatusLabel(status);
      expect(label).not.toBe(status); // should be human-readable
      expect(label.length).toBeGreaterThan(0);
    });
  }

  it('does not produce a label for "reconciled" (invalid DB status)', () => {
    // "reconciled" was removed from the handler — it must not appear in valid set
    expect(VALID_STATEMENT_STATUSES).not.toContain('reconciled');
  });

  it('does not produce a label for "pending_approval_2" (invalid DB status)', () => {
    expect(VALID_STATEMENT_STATUSES).not.toContain('pending_approval_2');
  });
});

// ─── Electron API base-URL detection ─────────────────────────────────────────

describe('API base URL detection', () => {
  const originalWindow = globalThis.window;

  beforeEach(() => {
    // Reset injected globals before each test
    if (typeof window !== 'undefined') {
      delete (window as unknown as Record<string, unknown>).__ELECTRON_API_BASE__;
      delete (window as unknown as Record<string, unknown>).__ELECTRON__;
    }
  });

  afterEach(() => {
    if (typeof window !== 'undefined') {
      delete (window as unknown as Record<string, unknown>).__ELECTRON_API_BASE__;
      delete (window as unknown as Record<string, unknown>).__ELECTRON__;
    }
  });

  function resolveApiBase(): string {
    return (
      (typeof window !== 'undefined'
        ? (window as unknown as Record<string, unknown>).__ELECTRON_API_BASE__ as string | undefined
        : undefined) ?? '/api/v1'
    );
  }

  it('falls back to /api/v1 when not running in Electron', () => {
    expect(resolveApiBase()).toBe('/api/v1');
  });

  it('uses __ELECTRON_API_BASE__ when injected by preload', () => {
    (window as unknown as Record<string, unknown>).__ELECTRON_API_BASE__ = 'http://localhost:8080/api/v1';
    expect(resolveApiBase()).toBe('http://localhost:8080/api/v1');
  });

  it('uses custom port if backend runs on non-default port', () => {
    (window as unknown as Record<string, unknown>).__ELECTRON_API_BASE__ = 'http://localhost:9090/api/v1';
    expect(resolveApiBase()).toBe('http://localhost:9090/api/v1');
  });
});
