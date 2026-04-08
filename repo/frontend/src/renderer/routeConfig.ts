/**
 * routeConfig.ts — Canonical route metadata shared between App.tsx routing
 * and routes.test.ts integrity checks.
 *
 * Keeping route metadata in one place prevents test drift: tests import from
 * here instead of duplicating the route list as mirrored constants.
 */

export type RouteConfig = {
  path: string;
  protected: boolean;
  /** If present, only these roles may access the route. */
  roles?: string[];
};

export const ROUTE_CONFIG: RouteConfig[] = [
  { path: '/login',           protected: false },
  { path: '/',                protected: true },
  { path: '/dashboard',       protected: true },           // alias → /
  { path: '/users',           protected: true, roles: ['system_admin'] },
  { path: '/skus',            protected: true, roles: ['system_admin', 'inventory_pharmacist'] },
  { path: '/skus/:id',        protected: true, roles: ['system_admin', 'inventory_pharmacist'] },
  { path: '/stocktakes',      protected: true, roles: ['system_admin', 'inventory_pharmacist'] },
  { path: '/stocktakes/:id',  protected: true, roles: ['system_admin', 'inventory_pharmacist'] },
  { path: '/learning',        protected: true },
  { path: '/work-orders',     protected: true },
  { path: '/work-orders/:id', protected: true },
  { path: '/members',         protected: true, roles: ['system_admin', 'front_desk'] },
  { path: '/members/:id',     protected: true, roles: ['system_admin', 'front_desk'] },
  { path: '/rate-tables',     protected: true, roles: ['system_admin'] },
  { path: '/statements',      protected: true, roles: ['system_admin'] },
  { path: '/system-config',   protected: true, roles: ['system_admin'] },
];
