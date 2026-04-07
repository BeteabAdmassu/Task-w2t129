import React from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useAuth } from './hooks/useAuth';
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

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isAuthenticated } = useAuth();
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  return <Layout>{children}</Layout>;
}

function RoleRoute({ children, roles }: { children: React.ReactNode; roles: string[] }) {
  const { user, isAuthenticated } = useAuth();
  if (!isAuthenticated) return <Navigate to="/login" replace />;
  if (user && !roles.includes(user.role)) return <Navigate to="/" replace />;
  return <Layout>{children}</Layout>;
}

export default function App() {
  const { isAuthenticated } = useAuth();

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={isAuthenticated ? <Navigate to="/" replace /> : <LoginPage />} />
        <Route path="/" element={<ProtectedRoute><DashboardPage /></ProtectedRoute>} />
        {/* /dashboard is an alias for / — referenced by nav shortcuts and post-login redirect */}
        <Route path="/dashboard" element={<Navigate to="/" replace />} />
        <Route path="/users" element={<RoleRoute roles={['system_admin']}><UsersPage /></RoleRoute>} />
        <Route path="/skus" element={<RoleRoute roles={['system_admin', 'inventory_pharmacist']}><SKUListPage /></RoleRoute>} />
        <Route path="/skus/:id" element={<RoleRoute roles={['system_admin', 'inventory_pharmacist']}><SKUDetailPage /></RoleRoute>} />
        <Route path="/stocktakes" element={<RoleRoute roles={['system_admin', 'inventory_pharmacist']}><StocktakePage /></RoleRoute>} />
        <Route path="/stocktakes/:id" element={<RoleRoute roles={['system_admin', 'inventory_pharmacist']}><StocktakePage /></RoleRoute>} />
        <Route path="/learning" element={<ProtectedRoute><LearningPage /></ProtectedRoute>} />
        <Route path="/work-orders" element={<ProtectedRoute><WorkOrdersPage /></ProtectedRoute>} />
        <Route path="/work-orders/:id" element={<ProtectedRoute><WorkOrderDetailPage /></ProtectedRoute>} />
        <Route path="/members" element={<RoleRoute roles={['system_admin', 'front_desk']}><MembersPage /></RoleRoute>} />
        <Route path="/members/:id" element={<RoleRoute roles={['system_admin', 'front_desk']}><MemberDetailPage /></RoleRoute>} />
        <Route path="/rate-tables" element={<RoleRoute roles={['system_admin']}><RateTablesPage /></RoleRoute>} />
        <Route path="/statements" element={<RoleRoute roles={['system_admin']}><StatementsPage /></RoleRoute>} />
        <Route path="/system-config" element={<RoleRoute roles={['system_admin']}><SystemConfigPage /></RoleRoute>} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}
