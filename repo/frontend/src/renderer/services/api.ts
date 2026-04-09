import axios, { AxiosError } from 'axios';
import type { ErrorResponse, WorkOrderDetailResponse } from '../types';

// In Electron (desktop) mode the preload script injects __ELECTRON_API_BASE__
// with an absolute URL so API calls work from file:// origins.
// In web/Docker mode we fall back to the relative path proxied by nginx.
const API_BASE: string =
  (typeof window !== 'undefined'
    ? (window as unknown as Record<string, unknown>).__ELECTRON_API_BASE__ as string | undefined
    : undefined) ?? '/api/v1';

const api = axios.create({
  baseURL: API_BASE,
  headers: { 'Content-Type': 'application/json' },
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('medops_token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

api.interceptors.response.use(
  (response) => response,
  (error: AxiosError<ErrorResponse>) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('medops_token');
      localStorage.removeItem('medops_user');
      window.location.hash = '/login';
    }
    return Promise.reject(error);
  }
);

// Auth
export const authAPI = {
  login: (username: string, password: string) =>
    api.post('/auth/login', { username, password }),
  logout: () => api.post('/auth/logout'),
  getMe: () => api.get('/auth/me'),
  changePassword: (old_password: string, new_password: string) =>
    api.put('/auth/password', { old_password, new_password }),
};

// Users
export const usersAPI = {
  list: () => api.get('/users'),
  create: (data: { username: string; password: string; role: string }) =>
    api.post('/users', data),
  update: (id: string, data: { role?: string; is_active?: boolean }) =>
    api.put(`/users/${id}`, data),
  delete: (id: string) => api.delete(`/users/${id}`),
  unlock: (id: string) => api.post(`/users/${id}/unlock`),
};

// Inventory - SKUs
export const skusAPI = {
  list: (params?: { search?: string; page?: number; page_size?: number }) =>
    api.get('/skus', { params }),
  get: (id: string) => api.get(`/skus/${id}`),
  create: (data: Record<string, unknown>) => api.post('/skus', data),
  update: (id: string, data: Record<string, unknown>) => api.put(`/skus/${id}`, data),
  getBatches: (id: string) => api.get(`/skus/${id}/batches`),
  getLowStock: () => api.get('/skus/low-stock'),
};

// Inventory - Transactions
export const inventoryAPI = {
  receive: (data: Record<string, unknown>) => api.post('/inventory/receive', data),
  dispense: (data: Record<string, unknown>) => api.post('/inventory/dispense', data),
  transactions: (params?: { sku_id?: string; page?: number; page_size?: number }) =>
    api.get('/inventory/transactions', { params }),
  adjust: (data: Record<string, unknown>) => api.post('/inventory/adjust', data),
};

// Stocktakes
export const stocktakeAPI = {
  list: () => api.get('/stocktakes'),
  create: (data: { period_start: string; period_end: string }) =>
    api.post('/stocktakes', data),
  get: (id: string) => api.get(`/stocktakes/${id}`),
  updateLines: (id: string, lines: Array<Record<string, unknown>>) =>
    api.put(`/stocktakes/${id}/lines`, { lines }),
  complete: (id: string) => api.post(`/stocktakes/${id}/complete`),
};

// Learning
export const learningAPI = {
  listSubjects: () => api.get('/learning/subjects'),
  createSubject: (data: { name: string; description: string; sort_order: number }) =>
    api.post('/learning/subjects', data),
  updateSubject: (id: string, data: Record<string, unknown>) =>
    api.put(`/learning/subjects/${id}`, data),
  listChapters: (params?: { subject_id?: string }) =>
    api.get('/learning/chapters', { params }),
  createChapter: (data: { subject_id: string; name: string; sort_order: number }) =>
    api.post('/learning/chapters', data),
  listKnowledgePoints: (params?: { chapter_id?: string; page?: number; page_size?: number }) =>
    api.get('/learning/knowledge-points', { params }),
  createKnowledgePoint: (data: Record<string, unknown>) =>
    api.post('/learning/knowledge-points', data),
  updateKnowledgePoint: (id: string, data: Record<string, unknown>) =>
    api.put(`/learning/knowledge-points/${id}`, data),
  search: (params: { q: string; page?: number; page_size?: number }) =>
    api.get('/learning/search', { params }),
  importContent: (formData: FormData) =>
    api.post('/learning/import', formData, { headers: { 'Content-Type': 'multipart/form-data' } }),
  exportContent: (id: string, format: 'md' | 'html' = 'md') =>
    api.get(`/learning/export/${id}`, { params: { format }, responseType: 'blob' }),
};

// Work Orders
export const workOrdersAPI = {
  list: (params?: { status?: string; page?: number; page_size?: number }) =>
    api.get('/work-orders', { params }),
  get: (id: string) => api.get<WorkOrderDetailResponse>(`/work-orders/${id}`),
  create: (data: Record<string, unknown>) => api.post('/work-orders', data),
  update: (id: string, data: Record<string, unknown>) => api.put(`/work-orders/${id}`, data),
  close: (id: string, data: { parts_cost: number; labor_cost: number }) =>
    api.post(`/work-orders/${id}/close`, data),
  rate: (id: string, rating: number) =>
    api.post(`/work-orders/${id}/rate`, { rating }),
  analytics: () => api.get('/work-orders/analytics'),
  linkPhoto: (id: string, fileId: string) =>
    api.post(`/work-orders/${id}/photos`, { file_id: fileId }),
  getPhotos: (id: string) => api.get(`/work-orders/${id}/photos`),
};

// Members
export const membersAPI = {
  list: (params?: { search?: string; page?: number; page_size?: number }) =>
    api.get('/members', { params }),
  get: (id: string) => api.get(`/members/${id}`),
  create: (data: Record<string, unknown>) => api.post('/members', data),
  update: (id: string, data: Record<string, unknown>) => api.put(`/members/${id}`, data),
  freeze: (id: string) => api.post(`/members/${id}/freeze`),
  unfreeze: (id: string) => api.post(`/members/${id}/unfreeze`),
  redeem: (id: string, data: { type: string; amount?: number; package_id?: string }) =>
    api.post(`/members/${id}/redeem`, data),
  addValue: (id: string, data: { type: string; amount: number }) =>
    api.post(`/members/${id}/add-value`, data),
  refund: (id: string, data: { amount: number }) =>
    api.post(`/members/${id}/refund`, data),
  transactions: (id: string, params?: { page?: number; page_size?: number }) =>
    api.get(`/members/${id}/transactions`, { params }),
  listPackages: (id: string) =>
    api.get(`/members/${id}/packages`),
  createPackage: (id: string, data: { package_name: string; total_sessions: number; expires_at: string }) =>
    api.post(`/members/${id}/packages`, data),
  listTiers: () => api.get('/membership-tiers'),
};

// Charges
export const chargesAPI = {
  listRateTables: () => api.get('/rate-tables'),
  createRateTable: (data: Record<string, unknown>) => api.post('/rate-tables', data),
  updateRateTable: (id: string, data: Record<string, unknown>) => api.put(`/rate-tables/${id}`, data),
  importCSV: (formData: FormData) =>
    api.post('/rate-tables/import-csv', formData, { headers: { 'Content-Type': 'multipart/form-data' } }),
  listStatements: (params?: { page?: number; page_size?: number }) =>
    api.get('/statements', { params }),
  getStatement: (id: string) => api.get(`/statements/${id}`),
  generateStatement: (data: {
    period_start: string;
    period_end: string;
    rate_table_id: string;
    line_items: Array<{ description: string; quantity: number }>;
  }) => api.post('/statements/generate', data),
  // expected_total is required so the backend can compute ABS(total-expected)>25 variance check.
  reconcile: (id: string, data: { expected_total: number; variance_notes?: string }) =>
    api.post(`/statements/${id}/reconcile`, data),
  approve: (id: string) => api.post(`/statements/${id}/approve`),
  exportStatement: (id: string, format: 'csv' | 'json' = 'csv') =>
    api.post(`/statements/${id}/export`, {}, { params: { format }, responseType: 'blob' }),
};

// Files
export const filesAPI = {
  upload: (formData: FormData) =>
    api.post('/files/upload', formData, { headers: { 'Content-Type': 'multipart/form-data' } }),
  download: (id: string) => api.get(`/files/${id}`, { responseType: 'blob' }),
  exportZip: (fileIds: string[]) =>
    api.post('/files/export-zip', { file_ids: fileIds }, { responseType: 'blob' }),
};

// System
export const systemAPI = {
  health: () => api.get('/health'),
  backup: () => api.post('/system/backup'),
  backupStatus: () => api.get('/system/backup/status'),
  getConfig: () => api.get('/system/config'),
  updateConfig: (key: string, value: string) => api.put('/system/config', { key, value }),
  applyUpdate: (file?: File) => {
    if (file) {
      const fd = new FormData();
      fd.append('file', file);
      return api.post('/system/update', fd, { headers: { 'Content-Type': 'multipart/form-data' } });
    }
    return api.post('/system/update');
  },
  rollback: () => api.post('/system/rollback'),
};

// Drafts
export const draftsAPI = {
  save: (formType: string, data: Record<string, unknown>) =>
    api.put(`/drafts/${formType}`, data),
  list: () => api.get('/drafts'),
  get: (formType: string, formId: string) => api.get(`/drafts/${formType}/${formId}`),
  delete: (formType: string, formId: string) => api.delete(`/drafts/${formType}/${formId}`),
};

export default api;
