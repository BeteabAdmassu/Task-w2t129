import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '../../hooks/useAuth';
import { skusAPI, workOrdersAPI, membersAPI, usersAPI, systemAPI } from '../../services/api';
import LoadingSpinner from '../common/LoadingSpinner';
import ErrorMessage from '../common/ErrorMessage';
import type { SKU, WorkOrder, Member, User } from '../../types';

interface DashboardData {
  lowStockSkus: SKU[];
  workOrders: WorkOrder[];
  members: Member[];
  users: User[];
  systemHealth: { status: string } | null;
}

const cardStyle: React.CSSProperties = {
  backgroundColor: '#fff',
  borderRadius: 8,
  padding: '1.25rem',
  boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
  border: '1px solid #e0e0e0',
};

const statCardStyle = (color: string): React.CSSProperties => ({
  ...cardStyle,
  borderLeft: `4px solid ${color}`,
  display: 'flex',
  flexDirection: 'column',
  gap: '0.5rem',
});

const quickActionBtnStyle: React.CSSProperties = {
  padding: '0.6rem 1.2rem',
  fontSize: '0.85rem',
  backgroundColor: '#1a237e',
  color: '#fff',
  border: 'none',
  borderRadius: 4,
  cursor: 'pointer',
  fontWeight: 500,
};

const DashboardPage: React.FC = () => {
  const { user } = useAuth();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [data, setData] = useState<DashboardData>({
    lowStockSkus: [],
    workOrders: [],
    members: [],
    users: [],
    systemHealth: null,
  });

  const fetchData = async () => {
    setLoading(true);
    setError(null);
    try {
      const promises: Promise<void>[] = [];
      const result: DashboardData = {
        lowStockSkus: [],
        workOrders: [],
        members: [],
        users: [],
        systemHealth: null,
      };

      // Everyone gets work orders
      promises.push(
        workOrdersAPI.list({ page: 1, page_size: 100 }).then((res) => {
          result.workOrders = res.data.data || res.data || [];
        }).catch(() => { /* non-critical */ })
      );

      if (user?.role === 'inventory_pharmacist') {
        promises.push(
          skusAPI.getLowStock().then((res) => {
            result.lowStockSkus = res.data.data || res.data || [];
          }).catch(() => {})
        );
      }

      if (user?.role === 'front_desk') {
        promises.push(
          membersAPI.list({ page: 1, page_size: 500 }).then((res) => {
            result.members = res.data.data || res.data || [];
          }).catch(() => {})
        );
      }

      if (user?.role === 'system_admin') {
        promises.push(
          usersAPI.list().then((res) => {
            result.users = res.data.data || res.data || [];
          }).catch(() => {})
        );
        promises.push(
          systemAPI.health().then((res) => {
            result.systemHealth = res.data;
          }).catch(() => {})
        );
      }

      await Promise.all(promises);
      setData(result);
    } catch (err: any) {
      setError(err.response?.data?.error || err.message || 'Failed to load dashboard');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, [user?.role]);

  if (loading) return <LoadingSpinner message="Loading dashboard..." />;
  if (error) return <ErrorMessage message={error} onRetry={fetchData} />;

  const openWorkOrders = data.workOrders.filter((wo) => wo.status !== 'closed' && wo.status !== 'completed');
  const slaBreaches = data.workOrders.filter((wo) => {
    if (wo.status === 'closed' || wo.status === 'completed') return false;
    return new Date(wo.sla_deadline) < new Date();
  });

  const expiringMembers = data.members.filter((m) => {
    const expiry = new Date(m.expires_at);
    const thirtyDays = new Date();
    thirtyDays.setDate(thirtyDays.getDate() + 30);
    return m.status === 'active' && expiry <= thirtyDays;
  });

  return (
    <div>
      <div style={{ marginBottom: '1.5rem' }}>
        <h1 style={{ margin: 0, fontSize: '1.5rem', color: '#333' }}>Dashboard</h1>
        <p style={{ margin: '0.25rem 0 0', color: '#666', fontSize: '0.9rem' }}>
          Welcome back, {user?.username}
        </p>
      </div>

      {/* Stat cards grid */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '1rem', marginBottom: '1.5rem' }}>

        {/* Inventory Pharmacist stats */}
        {user?.role === 'inventory_pharmacist' && (
          <>
            <div style={statCardStyle('#f44336')}>
              <span style={{ fontSize: '0.8rem', color: '#666', textTransform: 'uppercase', fontWeight: 600 }}>Low Stock Alerts</span>
              <span style={{ fontSize: '2rem', fontWeight: 700, color: data.lowStockSkus.length > 0 ? '#f44336' : '#333' }}>
                {data.lowStockSkus.length}
              </span>
              <span style={{ fontSize: '0.8rem', color: '#999' }}>SKUs below threshold</span>
            </div>
            <div style={statCardStyle('#ff9800')}>
              <span style={{ fontSize: '0.8rem', color: '#666', textTransform: 'uppercase', fontWeight: 600 }}>Open Work Orders</span>
              <span style={{ fontSize: '2rem', fontWeight: 700, color: '#333' }}>{openWorkOrders.length}</span>
              <span style={{ fontSize: '0.8rem', color: '#999' }}>Pending resolution</span>
            </div>
          </>
        )}

        {/* Front desk stats */}
        {user?.role === 'front_desk' && (
          <>
            <div style={statCardStyle('#ff9800')}>
              <span style={{ fontSize: '0.8rem', color: '#666', textTransform: 'uppercase', fontWeight: 600 }}>Expiring Memberships</span>
              <span style={{ fontSize: '2rem', fontWeight: 700, color: expiringMembers.length > 0 ? '#ff9800' : '#333' }}>
                {expiringMembers.length}
              </span>
              <span style={{ fontSize: '0.8rem', color: '#999' }}>Within 30 days</span>
            </div>
            <div style={statCardStyle('#4caf50')}>
              <span style={{ fontSize: '0.8rem', color: '#666', textTransform: 'uppercase', fontWeight: 600 }}>Total Members</span>
              <span style={{ fontSize: '2rem', fontWeight: 700, color: '#333' }}>{data.members.length}</span>
              <span style={{ fontSize: '0.8rem', color: '#999' }}>All members</span>
            </div>
          </>
        )}

        {/* Maintenance tech stats */}
        {user?.role === 'maintenance_tech' && (
          <>
            <div style={statCardStyle('#2196f3')}>
              <span style={{ fontSize: '0.8rem', color: '#666', textTransform: 'uppercase', fontWeight: 600 }}>Open Work Orders</span>
              <span style={{ fontSize: '2rem', fontWeight: 700, color: '#333' }}>{openWorkOrders.length}</span>
              <span style={{ fontSize: '0.8rem', color: '#999' }}>Pending resolution</span>
            </div>
            <div style={statCardStyle('#f44336')}>
              <span style={{ fontSize: '0.8rem', color: '#666', textTransform: 'uppercase', fontWeight: 600 }}>SLA Breaches</span>
              <span style={{ fontSize: '2rem', fontWeight: 700, color: slaBreaches.length > 0 ? '#f44336' : '#333' }}>
                {slaBreaches.length}
              </span>
              <span style={{ fontSize: '0.8rem', color: '#999' }}>Past deadline</span>
            </div>
          </>
        )}

        {/* System admin stats */}
        {user?.role === 'system_admin' && (
          <>
            <div style={statCardStyle('#1a237e')}>
              <span style={{ fontSize: '0.8rem', color: '#666', textTransform: 'uppercase', fontWeight: 600 }}>Total Users</span>
              <span style={{ fontSize: '2rem', fontWeight: 700, color: '#333' }}>{data.users.length}</span>
              <span style={{ fontSize: '0.8rem', color: '#999' }}>Registered accounts</span>
            </div>
            <div style={statCardStyle(data.systemHealth?.status === 'ok' ? '#4caf50' : '#f44336')}>
              <span style={{ fontSize: '0.8rem', color: '#666', textTransform: 'uppercase', fontWeight: 600 }}>System Health</span>
              <span style={{ fontSize: '2rem', fontWeight: 700, color: data.systemHealth?.status === 'ok' ? '#4caf50' : '#f44336' }}>
                {data.systemHealth?.status?.toUpperCase() || 'UNKNOWN'}
              </span>
              <span style={{ fontSize: '0.8rem', color: '#999' }}>Current status</span>
            </div>
          </>
        )}

        {/* Learning coordinator */}
        {user?.role === 'learning_coordinator' && (
          <div style={statCardStyle('#9c27b0')}>
            <span style={{ fontSize: '0.8rem', color: '#666', textTransform: 'uppercase', fontWeight: 600 }}>Open Work Orders</span>
            <span style={{ fontSize: '2rem', fontWeight: 700, color: '#333' }}>{openWorkOrders.length}</span>
            <span style={{ fontSize: '0.8rem', color: '#999' }}>Facility requests</span>
          </div>
        )}
      </div>

      {/* Low stock alerts for inventory pharmacist */}
      {user?.role === 'inventory_pharmacist' && data.lowStockSkus.length > 0 && (
        <div style={{ ...cardStyle, marginBottom: '1.5rem' }}>
          <h3 style={{ margin: '0 0 1rem', fontSize: '1rem', color: '#f44336' }}>Low Stock Alerts</h3>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
            <thead>
              <tr style={{ borderBottom: '2px solid #eee' }}>
                <th style={{ textAlign: 'left', padding: '0.5rem' }}>SKU Name</th>
                <th style={{ textAlign: 'left', padding: '0.5rem' }}>NDC</th>
                <th style={{ textAlign: 'left', padding: '0.5rem' }}>Location</th>
                <th style={{ textAlign: 'right', padding: '0.5rem' }}>Threshold</th>
              </tr>
            </thead>
            <tbody>
              {data.lowStockSkus.slice(0, 10).map((sku) => (
                <tr key={sku.id} style={{ borderBottom: '1px solid #f0f0f0' }}>
                  <td style={{ padding: '0.5rem' }}>{sku.name}</td>
                  <td style={{ padding: '0.5rem' }}>{sku.ndc || '-'}</td>
                  <td style={{ padding: '0.5rem' }}>{sku.storage_location}</td>
                  <td style={{ padding: '0.5rem', textAlign: 'right' }}>{sku.low_stock_threshold}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Recent work orders for maintenance tech */}
      {user?.role === 'maintenance_tech' && openWorkOrders.length > 0 && (
        <div style={{ ...cardStyle, marginBottom: '1.5rem' }}>
          <h3 style={{ margin: '0 0 1rem', fontSize: '1rem', color: '#333' }}>Open Work Orders</h3>
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
            <thead>
              <tr style={{ borderBottom: '2px solid #eee' }}>
                <th style={{ textAlign: 'left', padding: '0.5rem' }}>Description</th>
                <th style={{ textAlign: 'left', padding: '0.5rem' }}>Priority</th>
                <th style={{ textAlign: 'left', padding: '0.5rem' }}>SLA Deadline</th>
                <th style={{ textAlign: 'left', padding: '0.5rem' }}>Status</th>
              </tr>
            </thead>
            <tbody>
              {openWorkOrders.slice(0, 10).map((wo) => (
                <tr key={wo.id} style={{ borderBottom: '1px solid #f0f0f0' }}>
                  <td style={{ padding: '0.5rem' }}>{wo.description.substring(0, 60)}</td>
                  <td style={{ padding: '0.5rem' }}>
                    <span style={{
                      padding: '0.15rem 0.5rem',
                      borderRadius: 12,
                      fontSize: '0.75rem',
                      fontWeight: 600,
                      backgroundColor: wo.priority === 'urgent' ? '#fdecea' : wo.priority === 'high' ? '#fff3e0' : '#e8f5e9',
                      color: wo.priority === 'urgent' ? '#c62828' : wo.priority === 'high' ? '#e65100' : '#2e7d32',
                    }}>
                      {wo.priority}
                    </span>
                  </td>
                  <td style={{ padding: '0.5rem' }}>{new Date(wo.sla_deadline).toLocaleDateString()}</td>
                  <td style={{ padding: '0.5rem' }}>{wo.status}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Quick actions */}
      <div style={cardStyle}>
        <h3 style={{ margin: '0 0 1rem', fontSize: '1rem', color: '#333' }}>Quick Actions</h3>
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.75rem' }}>
          {user?.role === 'inventory_pharmacist' && (
            <>
              <button style={quickActionBtnStyle} onClick={() => navigate('/skus')}>Manage SKUs</button>
              <button style={quickActionBtnStyle} onClick={() => navigate('/stocktakes')}>New Stocktake</button>
            </>
          )}
          {user?.role === 'front_desk' && (
            <>
              <button style={quickActionBtnStyle} onClick={() => navigate('/members')}>Manage Members</button>
            </>
          )}
          {user?.role === 'maintenance_tech' && (
            <>
              <button style={quickActionBtnStyle} onClick={() => navigate('/work-orders')}>View Work Orders</button>
            </>
          )}
          {user?.role === 'system_admin' && (
            <>
              <button style={quickActionBtnStyle} onClick={() => navigate('/users')}>Manage Users</button>
              <button style={quickActionBtnStyle} onClick={() => navigate('/system-config')}>System Config</button>
              <button style={quickActionBtnStyle} onClick={() => navigate('/rate-tables')}>Rate Tables</button>
            </>
          )}
          {user?.role === 'learning_coordinator' && (
            <>
              <button style={quickActionBtnStyle} onClick={() => navigate('/learning')}>Manage Learning</button>
            </>
          )}
          {/* All roles can submit work orders */}
          <button
            style={{ ...quickActionBtnStyle, backgroundColor: '#ff9800' }}
            onClick={() => navigate('/work-orders')}
          >
            Submit Repair Request
          </button>
        </div>
      </div>
    </div>
  );
};

export default DashboardPage;
