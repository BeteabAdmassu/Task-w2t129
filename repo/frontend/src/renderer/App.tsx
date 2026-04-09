import React from 'react';
// HashRouter is required for Electron packaged mode where the renderer runs
// from file:// origins.  BrowserRouter's absolute-path navigation (e.g.
// window.location.href = '/login') resolves to file:///login which does not
// exist.  HashRouter maps all routes under the fragment (#/login, #/, …) so
// navigation works identically in both web/Docker and packaged desktop modes.
import { HashRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
import { ROUTE_CONFIG } from './routeConfig';
import Layout from './components/common/Layout';
import LoginPage from './components/admin/LoginPage';
import DashboardPage from './components/admin/DashboardPage';
import UsersPage from './components/admin/UsersPage';
import SKUListPage from './components/inventory/SKUListPage';
import SKUDetailPage from './components/inventory/SKUDetailPage';
import StocktakePage from './components/inventory/StocktakePage';
import LearningPage from './components/learning/LearningPage';
import WorkOrdersPage from './components/workorders/WorkOrdersPage';
import WorkOrderDetailPage from './components/workorders/WorkOrderDetailPage';
import MembersPage from './components/members/MembersPage';
import MemberDetailPage from './components/members/MemberDetailPage';
import RateTablesPage from './components/charges/RateTablesPage';
import StatementsPage from './components/charges/StatementsPage';
import SystemConfigPage from './components/admin/SystemConfigPage';
import ForcePasswordChangePage from './components/admin/ForcePasswordChangePage';

/** Returns the role list for a given path from the canonical ROUTE_CONFIG. */
function routeRoles(path: string): string[] {
  return ROUTE_CONFIG.find(r => r.path === path)?.roles ?? [];
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated, user, bootstrapping } = useAuth();
  if (bootstrapping) return null;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (user?.must_change_password) return <ForcePasswordChangePage />;
  return <Layout>{children}</Layout>;
}

function RoleRoute({ children, roles }: { children: React.ReactNode; roles: string[] }) {
  const { user, isAuthenticated, bootstrapping } = useAuth();
  if (bootstrapping) return null;
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (user?.must_change_password) return <ForcePasswordChangePage />;
  if (user && !roles.includes(user.role)) return <Navigate to="/" replace />;
  return <Layout>{children}</Layout>;
}

export default function App() {
  const { isAuthenticated } = useAuth();

  return (
    <HashRouter>
      <Routes>
        <Route path="/login" element={isAuthenticated ? <Navigate to="/" replace /> : <LoginPage />} />
        <Route path="/" element={<ProtectedRoute><DashboardPage /></ProtectedRoute>} />
        {/* /dashboard is an alias for / — referenced by nav shortcuts and post-login redirect */}
        <Route path="/dashboard" element={<Navigate to="/" replace />} />
        <Route path="/users" element={<RoleRoute roles={routeRoles('/users')}><UsersPage /></RoleRoute>} />
        <Route path="/skus" element={<RoleRoute roles={routeRoles('/skus')}><SKUListPage /></RoleRoute>} />
        <Route path="/skus/:id" element={<RoleRoute roles={routeRoles('/skus/:id')}><SKUDetailPage /></RoleRoute>} />
        <Route path="/stocktakes" element={<RoleRoute roles={routeRoles('/stocktakes')}><StocktakePage /></RoleRoute>} />
        <Route path="/stocktakes/:id" element={<RoleRoute roles={routeRoles('/stocktakes/:id')}><StocktakePage /></RoleRoute>} />
        <Route path="/learning" element={<ProtectedRoute><LearningPage /></ProtectedRoute>} />
        <Route path="/work-orders" element={<ProtectedRoute><WorkOrdersPage /></ProtectedRoute>} />
        <Route path="/work-orders/:id" element={<ProtectedRoute><WorkOrderDetailPage /></ProtectedRoute>} />
        <Route path="/members" element={<RoleRoute roles={routeRoles('/members')}><MembersPage /></RoleRoute>} />
        <Route path="/members/:id" element={<RoleRoute roles={routeRoles('/members/:id')}><MemberDetailPage /></RoleRoute>} />
        <Route path="/rate-tables" element={<RoleRoute roles={routeRoles('/rate-tables')}><RateTablesPage /></RoleRoute>} />
        <Route path="/statements" element={<RoleRoute roles={routeRoles('/statements')}><StatementsPage /></RoleRoute>} />
        <Route path="/system-config" element={<RoleRoute roles={routeRoles('/system-config')}><SystemConfigPage /></RoleRoute>} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </HashRouter>
  );
}
