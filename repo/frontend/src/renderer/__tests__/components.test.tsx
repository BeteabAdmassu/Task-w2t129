/**
 * components.test.tsx — Component/route-render integration tests (F-010).
 *
 * Uses React Testing Library to render actual components and assert that
 * the correct DOM elements are present — verifying route/page wiring at
 * the render level, not just at the logic level.
 */

import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';

// ─── Mock useAuth so components can render without a real auth server ─────────

const mockUseAuth = {
  isAuthenticated: false,
  user: null as null | { id: string; username: string; role: string },
  loading: false,
  error: null as string | null,
  login: vi.fn(),
  logout: vi.fn(),
};

vi.mock('../hooks/useAuth', () => ({
  useAuth: () => mockUseAuth,
}));

// ─── Mock API service to prevent real HTTP calls ──────────────────────────────

// vi.hoisted ensures mock functions are created before vi.mock hoists its factory.
const mockWorkOrdersGet = vi.hoisted(() => vi.fn());
// Shared list mock that returns a resolved promise so DashboardPage can load.
const mockWorkOrdersList = vi.hoisted(() =>
  vi.fn().mockResolvedValue({ data: { data: [], total: 0, page: 1, page_size: 20 } })
);
const mockUsersList = vi.hoisted(() =>
  vi.fn().mockResolvedValue({ data: [] })
);
const mockSystemHealth = vi.hoisted(() =>
  vi.fn().mockResolvedValue({ data: { status: 'ok' } })
);
const mockLearningExport = vi.hoisted(() => vi.fn());
const mockFilesUpload = vi.hoisted(() => vi.fn());
const mockWorkOrdersLinkPhoto = vi.hoisted(() => vi.fn());
const mockMembersGet = vi.hoisted(() => vi.fn());
const mockMembersTransactions = vi.hoisted(() =>
  vi.fn().mockResolvedValue({ data: { data: [], total: 0 } })
);
const mockMembersCreatePackage = vi.hoisted(() => vi.fn());
const mockStocktakeList = vi.hoisted(() => vi.fn());
const mockStocktakeCreate = vi.hoisted(() => vi.fn());
const mockChangePassword = vi.hoisted(() => vi.fn());

vi.mock('../services/api', () => ({
  // Default export: the raw axios instance used directly by DraftRecoveryDialog.
  // Reject with 404 so the dialog silently does nothing (no draft = no UI).
  default: {
    get: vi.fn().mockRejectedValue({ response: { status: 404 } }),
    put: vi.fn().mockResolvedValue({ data: {} }),
    delete: vi.fn().mockResolvedValue({ data: {} }),
  },
  authAPI: {
    login: vi.fn(),
    logout: vi.fn(),
    me: vi.fn(),
    changePassword: mockChangePassword,
  },
  skusAPI: {
    getLowStock: vi.fn().mockResolvedValue({ data: { data: [] } }),
  },
  inventoryAPI: { listSKUs: vi.fn(), getSKU: vi.fn() },
  membersAPI: {
    listMembers: vi.fn(),
    getMember: vi.fn(),
    list: vi.fn().mockResolvedValue({ data: { data: [], total: 0 } }),
    get: mockMembersGet,
    transactions: mockMembersTransactions,
    createPackage: mockMembersCreatePackage,
    freeze: vi.fn(),
    unfreeze: vi.fn(),
    redeem: vi.fn(),
    addValue: vi.fn(),
    refund: vi.fn(),
  },
  usersAPI: {
    list: mockUsersList,
  },
  systemAPI: {
    health: mockSystemHealth,
  },
  workOrdersAPI: {
    listWorkOrders: vi.fn(),
    get: mockWorkOrdersGet,
    list: mockWorkOrdersList,
    linkPhoto: mockWorkOrdersLinkPhoto,
  },
  filesAPI: {
    upload: mockFilesUpload,
    download: vi.fn(),
  },
  learningAPI: {
    listSubjects: vi.fn().mockResolvedValue({ data: { data: [], total: 0 } }),
    listChapters: vi.fn().mockResolvedValue({ data: { data: [], total: 0 } }),
    listKnowledgePoints: vi.fn().mockResolvedValue({ data: { data: [], total: 0 } }),
    searchKnowledgePoints: vi.fn().mockResolvedValue({ data: { data: [], total: 0 } }),
    exportContent: mockLearningExport,
  },
  stocktakeAPI: {
    list: mockStocktakeList,
    create: mockStocktakeCreate,
    get: vi.fn(),
    updateLines: vi.fn(),
    complete: vi.fn(),
  },
}));

// ─── Import pages AFTER mocks are in place ────────────────────────────────────

import LoginPage from '../components/admin/LoginPage';
import DashboardPage from '../components/admin/DashboardPage';
import StocktakePage from '../components/inventory/StocktakePage';

// ─── Helpers ──────────────────────────────────────────────────────────────────

function renderInRouter(element: React.ReactElement, initialPath = '/') {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="*" element={element} />
      </Routes>
    </MemoryRouter>,
  );
}

// ─── LoginPage render tests ───────────────────────────────────────────────────

describe('LoginPage component render', () => {
  beforeEach(() => {
    mockUseAuth.isAuthenticated = false;
    mockUseAuth.user = null;
    mockUseAuth.error = null;
    mockUseAuth.loading = false;
    mockUseAuth.login.mockReset();
  });

  it('renders the application title', () => {
    renderInRouter(<LoginPage />);
    expect(screen.getByText('MedOps Console')).toBeTruthy();
  });

  it('renders a username input field', () => {
    renderInRouter(<LoginPage />);
    const usernameInput = screen.getByPlaceholderText(/username/i);
    expect(usernameInput).toBeTruthy();
  });

  it('renders a password input field', () => {
    renderInRouter(<LoginPage />);
    const passwordInput = screen.getByPlaceholderText(/password/i);
    expect(passwordInput).toBeTruthy();
  });

  it('renders a submit button', () => {
    renderInRouter(<LoginPage />);
    const submitBtn = screen.getByRole('button', { name: /sign in/i });
    expect(submitBtn).toBeTruthy();
  });

  it('shows validation error when submitting empty form', async () => {
    renderInRouter(<LoginPage />);
    const btn = screen.getByRole('button', { name: /sign in/i });
    fireEvent.click(btn);
    await waitFor(() => {
      expect(screen.getByText(/username is required/i)).toBeTruthy();
    });
  });

  it('shows password validation error when only username is filled', async () => {
    renderInRouter(<LoginPage />);
    const usernameInput = screen.getByPlaceholderText(/username/i);
    fireEvent.change(usernameInput, { target: { value: 'admin' } });
    const btn = screen.getByRole('button', { name: /sign in/i });
    fireEvent.click(btn);
    await waitFor(() => {
      expect(screen.getByText(/password is required/i)).toBeTruthy();
    });
  });

  it('calls login with credentials when form is submitted with valid input', async () => {
    mockUseAuth.login.mockResolvedValueOnce({ id: '1', username: 'admin', role: 'system_admin' });
    renderInRouter(<LoginPage />);

    fireEvent.change(screen.getByPlaceholderText(/username/i), {
      target: { value: 'admin' },
    });
    fireEvent.change(screen.getByPlaceholderText(/password/i), {
      target: { value: 'AdminPass1234' },
    });
    fireEvent.click(screen.getByRole('button', { name: /sign in/i }));

    await waitFor(() => {
      expect(mockUseAuth.login).toHaveBeenCalledWith('admin', 'AdminPass1234');
    });
  });

  it('renders error message from useAuth when login fails', () => {
    mockUseAuth.error = 'Invalid credentials';
    renderInRouter(<LoginPage />);
    expect(screen.getByText(/invalid credentials/i)).toBeTruthy();
  });

  it('disables the submit button while loading', () => {
    mockUseAuth.loading = true;
    renderInRouter(<LoginPage />);
    const btn = screen.getByRole('button', { name: /signing in/i });
    expect(btn).toBeTruthy();
    expect((btn as HTMLButtonElement).disabled).toBe(true);
  });
});

// ─── DashboardPage render tests ───────────────────────────────────────────────

describe('DashboardPage component render', () => {
  beforeEach(() => {
    mockUseAuth.isAuthenticated = true;
    mockUseAuth.user = { id: '1', username: 'admin', role: 'system_admin' };
  });

  it('renders without crashing when user is authenticated', () => {
    renderInRouter(<DashboardPage />);
    // If it renders without throwing, the route/component wiring works
    expect(document.body).toBeTruthy();
  });

  it('renders the dashboard heading or welcome element', async () => {
    renderInRouter(<DashboardPage />);
    // Wait for loading to complete, then check for the dashboard heading.
    await waitFor(() => {
      const headings = screen.getAllByRole('heading');
      expect(headings.length).toBeGreaterThan(0);
    });
  });
});

// ─── ProtectedRoute redirect behavior ────────────────────────────────────────

import App from '../App';

// App already contains BrowserRouter internally — do NOT wrap in another router.
// Use window.history.pushState to control the initial path before rendering.

describe('App routing — unauthenticated redirect', () => {
  beforeEach(() => {
    mockUseAuth.isAuthenticated = false;
    mockUseAuth.user = null;
  });

  it('shows login page when navigating to / unauthenticated', () => {
    window.history.pushState({}, '', '/');
    render(<App />);
    expect(screen.getByText('MedOps Console')).toBeTruthy();
  });

  it('shows login page when navigating to /members unauthenticated', () => {
    window.history.pushState({}, '', '/members');
    render(<App />);
    expect(screen.getByText('MedOps Console')).toBeTruthy();
  });

  it('shows login page when navigating to /skus unauthenticated', () => {
    window.history.pushState({}, '', '/skus');
    render(<App />);
    expect(screen.getByText('MedOps Console')).toBeTruthy();
  });
});

describe('App routing — authenticated access', () => {
  beforeEach(() => {
    mockUseAuth.isAuthenticated = true;
    mockUseAuth.user = { id: '1', username: 'admin', role: 'system_admin' };
  });

  it('does not show the login form when authenticated user visits /', () => {
    window.history.pushState({}, '', '/');
    render(<App />);
    // Login page is identified by its username/password inputs, not just the heading
    // (Layout also has a "MedOps Console" heading, so we check for the form instead).
    expect(screen.queryByPlaceholderText(/username/i)).toBeNull();
  });

  it('redirects /login to dashboard when already authenticated', () => {
    window.history.pushState({}, '', '/login');
    render(<App />);
    // When authenticated, /login should redirect — the login form should not be visible.
    expect(screen.queryByPlaceholderText(/username/i)).toBeNull();
  });
});

// ─── WorkOrderDetailPage envelope parsing tests ───────────────────────────────

import WorkOrderDetailPage from '../components/workorders/WorkOrderDetailPage';
import LearningPage from '../components/learning/LearningPage';

const baseWorkOrder = {
  id: 'wo-abc-123',
  submitted_by: 'uid-submitter',
  assigned_to: 'uid-tech',
  trade: 'electrical',
  priority: 'high' as const,
  sla_deadline: new Date(Date.now() + 86400000).toISOString(),
  status: 'submitted',
  description: 'Broken outlet in room 3',
  location: 'Building A, Room 303',
  parts_cost: 0,
  labor_cost: 0,
  created_at: new Date().toISOString(),
};

function renderDetailPage(woId = 'wo-abc-123') {
  return render(
    <MemoryRouter initialEntries={[`/work-orders/${woId}`]}>
      <Routes>
        <Route path="/work-orders/:id" element={<WorkOrderDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('WorkOrderDetailPage — API envelope parsing', () => {
  beforeEach(() => {
    mockUseAuth.isAuthenticated = true;
    mockUseAuth.user = { id: 'uid-submitter', username: 'submitter', role: 'front_desk' };
    mockWorkOrdersGet.mockReset();
  });

  it('renders order fields correctly when API returns new envelope {work_order, photos}', async () => {
    mockWorkOrdersGet.mockResolvedValueOnce({
      data: {
        work_order: baseWorkOrder,
        photos: [],
      },
    });

    renderDetailPage();

    await waitFor(() => {
      // Work order ID prefix should appear in the heading
      expect(screen.getByText(/wo-abc-1/i)).toBeTruthy();
    });
    // Priority badge
    expect(screen.getByText('high')).toBeTruthy();
    // Description
    expect(screen.getByText('Broken outlet in room 3')).toBeTruthy();
  });

  it('renders order fields correctly when API returns legacy plain WorkOrder', async () => {
    // Backward compatibility: bare WorkOrder object (no envelope)
    mockWorkOrdersGet.mockResolvedValueOnce({
      data: baseWorkOrder,
    });

    renderDetailPage();

    await waitFor(() => {
      expect(screen.getByText(/wo-abc-1/i)).toBeTruthy();
    });
    expect(screen.getByText('high')).toBeTruthy();
    expect(screen.getByText('Broken outlet in room 3')).toBeTruthy();
  });

  it('initializes rating from wo.rating in envelope response', async () => {
    const ratedWo = { ...baseWorkOrder, status: 'completed', rating: 4 };
    mockWorkOrdersGet.mockResolvedValueOnce({
      data: {
        work_order: ratedWo,
        photos: [],
      },
    });

    renderDetailPage();

    await waitFor(() => {
      // The "Rating: 4/5" text should appear since ratingSubmitted is true
      expect(screen.getByText(/rating.*4.*5/i)).toBeTruthy();
    });
  });

  it('shows error message when API call fails', async () => {
    mockWorkOrdersGet.mockRejectedValueOnce({
      response: { data: { error: 'Work order not found' } },
    });

    renderDetailPage();

    await waitFor(() => {
      expect(screen.getByText(/work order not found/i)).toBeTruthy();
    });
  });
});

// ─── WorkOrderDetailPage — attach-after-create photo flow (B-003) ─────────────

describe('WorkOrderDetailPage — attach-after-create photo flow', () => {
  beforeEach(() => {
    mockUseAuth.isAuthenticated = true;
    mockUseAuth.user = { id: 'uid-tech', username: 'tech', role: 'maintenance_tech' };
    mockWorkOrdersGet.mockReset();
    mockFilesUpload.mockReset();
    mockWorkOrdersLinkPhoto.mockReset();
  });

  it('renders a file input for attaching photos when order is not closed', async () => {
    mockWorkOrdersGet.mockResolvedValueOnce({
      data: { work_order: { ...baseWorkOrder, status: 'submitted' }, photos: [] },
    });

    renderDetailPage();

    await waitFor(() => {
      expect(screen.getByText(/wo-abc-1/i)).toBeTruthy();
    });

    // File input should be present since order is not closed
    const fileInput = document.querySelector('input[type="file"]');
    expect(fileInput).toBeTruthy();
  });

  it('shows no file input when order status is closed', async () => {
    mockWorkOrdersGet.mockResolvedValueOnce({
      data: { work_order: { ...baseWorkOrder, status: 'closed' }, photos: [] },
    });

    renderDetailPage();

    await waitFor(() => {
      expect(screen.getByText(/wo-abc-1/i)).toBeTruthy();
    });

    // File input hidden for closed orders
    const fileInput = document.querySelector('input[type="file"]');
    expect(fileInput).toBeNull();
  });

  it('calls filesAPI.upload and workOrdersAPI.linkPhoto when attach button is clicked', async () => {
    mockWorkOrdersGet
      .mockResolvedValueOnce({ data: { work_order: { ...baseWorkOrder, status: 'submitted' }, photos: [] } })
      .mockResolvedValueOnce({ data: { work_order: { ...baseWorkOrder, status: 'submitted' }, photos: [] } }); // refetch after attach

    const fakeFileId = 'file-uuid-999';
    mockFilesUpload.mockResolvedValueOnce({ data: { file: { id: fakeFileId } } });
    mockWorkOrdersLinkPhoto.mockResolvedValueOnce({ data: {} });

    renderDetailPage();

    await waitFor(() => {
      expect(screen.getByText(/wo-abc-1/i)).toBeTruthy();
    });

    // Simulate selecting a file
    const fileInput = document.querySelector('input[type="file"]') as HTMLInputElement;
    expect(fileInput).toBeTruthy();

    const mockFile = new File(['photo data'], 'photo.jpg', { type: 'image/jpeg' });
    Object.defineProperty(fileInput, 'files', { value: [mockFile], writable: false, configurable: true });
    fireEvent.change(fileInput);

    // Attach button should appear
    const attachBtn = await waitFor(() => screen.getByRole('button', { name: /attach 1 file/i }));
    expect(attachBtn).toBeTruthy();

    fireEvent.click(attachBtn);

    await waitFor(() => {
      expect(mockFilesUpload).toHaveBeenCalledTimes(1);
      expect(mockWorkOrdersLinkPhoto).toHaveBeenCalledWith('wo-abc-123', fakeFileId);
    });
  });
});

// ─── LearningPage — export format selector (B-002) ───────────────────────────

import { learningAPI as learningAPIMock } from '../services/api';

describe('LearningPage — export format selector passes correct format', () => {
  // Helpers to drive the three-panel learning UI to show a KP
  const subject = { id: 'subj-1', name: 'Pharmacology', description: '', sort_order: 0 };
  const chapter = { id: 'chap-1', name: 'Chapter 1', subject_id: 'subj-1', sort_order: 0 };
  const kp = { id: 'kp-1', title: 'Aspirin', content: 'Aspirin content', chapter_id: 'chap-1', tags: [], classifications: {} };

  beforeEach(() => {
    mockUseAuth.isAuthenticated = true;
    mockUseAuth.user = { id: '1', username: 'admin', role: 'system_admin' };
    mockLearningExport.mockReset();

    (learningAPIMock.listSubjects as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { data: [subject] },
    });
    (learningAPIMock.listChapters as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { data: [chapter] },
    });
    (learningAPIMock.listKnowledgePoints as ReturnType<typeof vi.fn>).mockResolvedValue({
      data: { data: [kp], total: 1 },
    });
    mockLearningExport.mockResolvedValue({ data: new Blob(['# Aspirin'], { type: 'text/markdown' }) });

    // Stub URL methods used by download logic
    (window.URL.createObjectURL as unknown) = vi.fn(() => 'blob:mock');
    (window.URL.revokeObjectURL as unknown) = vi.fn();
  });

  function renderLearningPage() {
    return render(
      <MemoryRouter initialEntries={['/learning']}>
        <Routes>
          <Route path="*" element={<LearningPage />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it('calls exportContent with "md" by default when Export is clicked', async () => {
    renderLearningPage();

    // Wait for subjects to load and click the first one
    const subjectItem = await waitFor(() => screen.getByText('Pharmacology'));
    fireEvent.click(subjectItem);

    // Wait for chapter to appear and click it
    const chapterItem = await waitFor(() => screen.getByText('Chapter 1'));
    fireEvent.click(chapterItem);

    // Wait for KP to appear
    await waitFor(() => screen.getByText('Aspirin'));

    // Click Export without changing format (default is md)
    const exportBtn = screen.getByRole('button', { name: /^export$/i });
    fireEvent.click(exportBtn);

    await waitFor(() => {
      expect(mockLearningExport).toHaveBeenCalledWith('kp-1', 'md');
    });
  });

  it('calls exportContent with "html" when format selector is changed to HTML', async () => {
    renderLearningPage();

    const subjectItem = await waitFor(() => screen.getByText('Pharmacology'));
    fireEvent.click(subjectItem);

    const chapterItem = await waitFor(() => screen.getByText('Chapter 1'));
    fireEvent.click(chapterItem);

    await waitFor(() => screen.getByText('Aspirin'));

    // Change the format selector to html
    const formatSelect = screen.getByTitle('Export format') as HTMLSelectElement;
    fireEvent.change(formatSelect, { target: { value: 'html' } });
    expect(formatSelect.value).toBe('html');

    // Now click Export
    const exportBtn = screen.getByRole('button', { name: /^export$/i });
    fireEvent.click(exportBtn);

    await waitFor(() => {
      expect(mockLearningExport).toHaveBeenCalledWith('kp-1', 'html');
    });
  });
});

// ─── MemberDetailPage — session package create flow ──────────────────────────

import MemberDetailPage from '../components/members/MemberDetailPage';

const baseMember = {
  id: 'mem-001',
  name: 'Alice Smith',
  phone: '+1-555-0100',
  tier_id: 'tier-gold',
  points_balance: 500,
  stored_value: 25.0,
  status: 'active' as const,
  expires_at: new Date(Date.now() + 86400000 * 365).toISOString(),
  created_at: new Date().toISOString(),
};

const futureDate = new Date(Date.now() + 86400000 * 60).toISOString().slice(0, 10); // 60 days from now

function renderMemberDetail(memberId = 'mem-001') {
  return render(
    <MemoryRouter initialEntries={[`/members/${memberId}`]}>
      <Routes>
        <Route path="/members/:id" element={<MemberDetailPage />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('MemberDetailPage — session package create flow', () => {
  beforeEach(() => {
    mockUseAuth.isAuthenticated = true;
    mockUseAuth.user = { id: 'u-1', username: 'frontdesk', role: 'front_desk' };
    mockMembersGet.mockReset();
    mockMembersTransactions.mockReset();
    mockMembersCreatePackage.mockReset();

    // Default: member with no packages
    mockMembersGet.mockResolvedValue({
      data: { ...baseMember, packages: [] },
    });
    mockMembersTransactions.mockResolvedValue({ data: { data: [], total: 0 } });
  });

  it('renders "Add Package" button and shows create form when clicked', async () => {
    renderMemberDetail();

    await waitFor(() => screen.getByText('Alice Smith'));

    const addBtn = screen.getByRole('button', { name: /add package/i });
    expect(addBtn).toBeTruthy();
    fireEvent.click(addBtn);

    expect(screen.getByTestId('pkg-name-input')).toBeTruthy();
    expect(screen.getByTestId('pkg-sessions-input')).toBeTruthy();
    expect(screen.getByTestId('pkg-expires-input')).toBeTruthy();
    expect(screen.getByTestId('pkg-create-submit')).toBeTruthy();
  });

  it('shows validation error when package name is empty', async () => {
    renderMemberDetail();
    await waitFor(() => screen.getByText('Alice Smith'));

    fireEvent.click(screen.getByRole('button', { name: /add package/i }));

    // Leave name empty, fill sessions and date
    fireEvent.change(screen.getByTestId('pkg-sessions-input'), { target: { value: '5' } });
    fireEvent.change(screen.getByTestId('pkg-expires-input'), { target: { value: futureDate } });
    fireEvent.click(screen.getByTestId('pkg-create-submit'));

    await waitFor(() => {
      expect(screen.getByTestId('pkg-create-error').textContent).toMatch(/name is required/i);
    });
    expect(mockMembersCreatePackage).not.toHaveBeenCalled();
  });

  it('shows validation error when total_sessions is not a positive integer', async () => {
    renderMemberDetail();
    await waitFor(() => screen.getByText('Alice Smith'));

    fireEvent.click(screen.getByRole('button', { name: /add package/i }));

    fireEvent.change(screen.getByTestId('pkg-name-input'), { target: { value: 'Test Pack' } });
    fireEvent.change(screen.getByTestId('pkg-sessions-input'), { target: { value: '0' } });
    fireEvent.change(screen.getByTestId('pkg-expires-input'), { target: { value: futureDate } });
    fireEvent.click(screen.getByTestId('pkg-create-submit'));

    await waitFor(() => {
      expect(screen.getByTestId('pkg-create-error').textContent).toMatch(/positive integer/i);
    });
    expect(mockMembersCreatePackage).not.toHaveBeenCalled();
  });

  it('shows validation error when expires_at is in the past', async () => {
    renderMemberDetail();
    await waitFor(() => screen.getByText('Alice Smith'));

    fireEvent.click(screen.getByRole('button', { name: /add package/i }));

    fireEvent.change(screen.getByTestId('pkg-name-input'), { target: { value: 'Test Pack' } });
    fireEvent.change(screen.getByTestId('pkg-sessions-input'), { target: { value: '5' } });
    fireEvent.change(screen.getByTestId('pkg-expires-input'), { target: { value: '2020-01-01' } });
    fireEvent.click(screen.getByTestId('pkg-create-submit'));

    await waitFor(() => {
      expect(screen.getByTestId('pkg-create-error').textContent).toMatch(/future/i);
    });
    expect(mockMembersCreatePackage).not.toHaveBeenCalled();
  });

  it('calls createPackage with correct payload and appends package on success', async () => {
    const createdPkg = {
      id: 'pkg-new-1',
      member_id: 'mem-001',
      package_name: 'Gold Pack',
      total_sessions: 10,
      remaining_sessions: 10,
      expires_at: new Date(Date.now() + 86400000 * 60).toISOString(),
    };
    mockMembersCreatePackage.mockResolvedValueOnce({ data: createdPkg });

    renderMemberDetail();
    await waitFor(() => screen.getByText('Alice Smith'));

    fireEvent.click(screen.getByRole('button', { name: /add package/i }));

    fireEvent.change(screen.getByTestId('pkg-name-input'), { target: { value: 'Gold Pack' } });
    fireEvent.change(screen.getByTestId('pkg-sessions-input'), { target: { value: '10' } });
    fireEvent.change(screen.getByTestId('pkg-expires-input'), { target: { value: futureDate } });
    fireEvent.click(screen.getByTestId('pkg-create-submit'));

    await waitFor(() => {
      expect(mockMembersCreatePackage).toHaveBeenCalledWith('mem-001', {
        package_name: 'Gold Pack',
        total_sessions: 10,
        expires_at: futureDate,
      });
    });

    // Package appears in the list
    await waitFor(() => {
      expect(screen.getByText('Gold Pack')).toBeTruthy();
    });

    // Form is hidden after success
    expect(screen.queryByTestId('pkg-name-input')).toBeNull();
  });

  it('disables submit button while create request is in flight', async () => {
    let resolveCreate!: (v: unknown) => void;
    mockMembersCreatePackage.mockReturnValueOnce(
      new Promise(res => { resolveCreate = res; })
    );

    renderMemberDetail();
    await waitFor(() => screen.getByText('Alice Smith'));

    fireEvent.click(screen.getByRole('button', { name: /add package/i }));

    fireEvent.change(screen.getByTestId('pkg-name-input'), { target: { value: 'Pack A' } });
    fireEvent.change(screen.getByTestId('pkg-sessions-input'), { target: { value: '5' } });
    fireEvent.change(screen.getByTestId('pkg-expires-input'), { target: { value: futureDate } });
    fireEvent.click(screen.getByTestId('pkg-create-submit'));

    // While request is pending the button is disabled
    await waitFor(() => {
      const btn = screen.getByTestId('pkg-create-submit') as HTMLButtonElement;
      expect(btn.disabled).toBe(true);
      expect(btn.textContent).toMatch(/creating/i);
    });

    // Resolve the request
    resolveCreate({ data: { id: 'p1', member_id: 'mem-001', package_name: 'Pack A', total_sessions: 5, remaining_sessions: 5, expires_at: new Date(Date.now() + 86400000 * 60).toISOString() } });
  });

  it('shows API error message when createPackage fails', async () => {
    mockMembersCreatePackage.mockRejectedValueOnce({
      response: { data: { error: 'Member not found in tenant' } },
    });

    renderMemberDetail();
    await waitFor(() => screen.getByText('Alice Smith'));

    fireEvent.click(screen.getByRole('button', { name: /add package/i }));

    fireEvent.change(screen.getByTestId('pkg-name-input'), { target: { value: 'Bad Pack' } });
    fireEvent.change(screen.getByTestId('pkg-sessions-input'), { target: { value: '3' } });
    fireEvent.change(screen.getByTestId('pkg-expires-input'), { target: { value: futureDate } });
    fireEvent.click(screen.getByTestId('pkg-create-submit'));

    await waitFor(() => {
      expect(screen.getByTestId('pkg-create-error').textContent).toMatch(/member not found in tenant/i);
    });
  });

  it('shows existing packages from member detail response with status badges', async () => {
    const activePkg = {
      id: 'pkg-a',
      member_id: 'mem-001',
      package_name: 'Active Pack',
      total_sessions: 5,
      remaining_sessions: 3,
      expires_at: new Date(Date.now() + 86400000 * 30).toISOString(),
    };
    const depletedPkg = {
      id: 'pkg-b',
      member_id: 'mem-001',
      package_name: 'Used Pack',
      total_sessions: 5,
      remaining_sessions: 0,
      expires_at: new Date(Date.now() + 86400000 * 30).toISOString(),
    };

    mockMembersGet.mockResolvedValueOnce({
      data: { ...baseMember, packages: [activePkg, depletedPkg] },
    });

    renderMemberDetail();

    await waitFor(() => {
      expect(screen.getByText('Active Pack')).toBeTruthy();
      expect(screen.getByText('Used Pack')).toBeTruthy();
    });

    // Status badges — 'active' appears on both member status and package badge
    const activeBadges = screen.getAllByText('active');
    expect(activeBadges.length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('depleted')).toBeTruthy();
  });
});

// ─── F-005 regression: stocktake list/history UI ─────────────────────────────

function renderStocktakePage(initialPath = '/stocktakes') {
  return render(
    <MemoryRouter initialEntries={[initialPath]}>
      <Routes>
        <Route path="/stocktakes" element={<StocktakePage />} />
        <Route path="/stocktakes/:id" element={<div data-testid="stocktake-detail">Detail</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('StocktakePage — history list (F-005)', () => {
  beforeEach(() => {
    mockStocktakeList.mockReset();
    mockStocktakeCreate.mockReset();
  });

  it('calls stocktakeAPI.list on mount to load history', async () => {
    mockStocktakeList.mockResolvedValueOnce({ data: { data: [] } });
    renderStocktakePage();
    await waitFor(() => {
      expect(mockStocktakeList).toHaveBeenCalledTimes(1);
    });
  });

  it('renders history rows for each returned stocktake', async () => {
    mockStocktakeList.mockResolvedValueOnce({
      data: {
        data: [
          { id: 'st-001', period_start: '2026-01-01', period_end: '2026-01-31', status: 'completed', created_by: 'u1', created_at: '2026-01-31T10:00:00Z' },
          { id: 'st-002', period_start: '2026-02-01', period_end: '2026-02-28', status: 'open', created_by: 'u1', created_at: '2026-02-28T10:00:00Z' },
        ],
      },
    });
    renderStocktakePage();
    await waitFor(() => {
      expect(screen.getByText('completed')).toBeTruthy();
      expect(screen.getByText('open')).toBeTruthy();
    });
    // Period range text
    expect(screen.getByText(/2026-01-01.*2026-01-31/)).toBeTruthy();
    expect(screen.getByText(/2026-02-01.*2026-02-28/)).toBeTruthy();
  });

  it('renders empty state when no stocktakes exist', async () => {
    mockStocktakeList.mockResolvedValueOnce({ data: { data: [] } });
    renderStocktakePage();
    await waitFor(() => {
      expect(screen.getByText(/no stocktakes yet/i)).toBeTruthy();
    });
  });

  it('renders error state when list API fails', async () => {
    mockStocktakeList.mockRejectedValueOnce(new Error('network error'));
    renderStocktakePage();
    await waitFor(() => {
      expect(screen.getByText(/failed to load stocktake history/i)).toBeTruthy();
    });
  });

  it('clicking Open navigates to /stocktakes/:id', async () => {
    mockStocktakeList.mockResolvedValueOnce({
      data: {
        data: [
          { id: 'st-nav-01', period_start: '2026-03-01', period_end: '2026-03-31', status: 'open', created_by: 'u1', created_at: '2026-03-31T10:00:00Z' },
        ],
      },
    });
    renderStocktakePage();
    await waitFor(() => screen.getByText('open'));
    // Click the Open button
    const openBtn = screen.getByRole('button', { name: /open/i });
    fireEvent.click(openBtn);
    // Should navigate to the detail route
    await waitFor(() => {
      expect(screen.getByTestId('stocktake-detail')).toBeTruthy();
    });
  });

  it('shows the Stocktake History heading', async () => {
    mockStocktakeList.mockResolvedValueOnce({ data: { data: [] } });
    renderStocktakePage();
    await waitFor(() => {
      expect(screen.getByText(/stocktake history/i)).toBeTruthy();
    });
  });
});

// ─── ForcePasswordChangePage — packaged-routing / reload-safe navigation ──────
//
// After a successful password change, the page must call window.location.reload()
// rather than window.location.href = '/' so the navigation is safe for both
// packaged Electron (file:// origin) and dev/web (http://localhost) modes.
// window.location.href = '/' resolves to file:///  in packaged mode and can
// break routing; window.location.reload() is always safe.

import ForcePasswordChangePage from '../components/admin/ForcePasswordChangePage';

describe('ForcePasswordChangePage', () => {
  let reloadSpy: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    localStorage.clear();
    mockUseAuth.user = { id: 'u-force', username: 'nurse1', role: 'front_desk' } as any;
    mockUseAuth.isAuthenticated = true;
    mockChangePassword.mockReset();
    // Spy on window.location.reload — jsdom does not implement it by default.
    reloadSpy = vi.fn();
    Object.defineProperty(window, 'location', {
      value: { ...window.location, reload: reloadSpy },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    localStorage.clear();
    vi.restoreAllMocks();
  });

  function renderForcePage() {
    return render(
      <MemoryRouter>
        <ForcePasswordChangePage />
      </MemoryRouter>,
    );
  }

  it('renders the force-password-change form', () => {
    renderForcePage();
    expect(screen.getByText(/you must change your password/i)).toBeTruthy();
    expect(screen.getByRole('button', { name: /change password/i })).toBeTruthy();
  });

  it('shows error when current password is empty', async () => {
    renderForcePage();
    fireEvent.click(screen.getByRole('button', { name: /change password/i }));
    await waitFor(() => {
      expect(screen.getByText(/current password is required/i)).toBeTruthy();
    });
    expect(mockChangePassword).not.toHaveBeenCalled();
  });

  it('shows error when new password is shorter than 12 characters', async () => {
    renderForcePage();
    const inputs = document.querySelectorAll('input[type="password"]');
    fireEvent.change(inputs[0], { target: { value: 'OldPass1234!' } });
    fireEvent.change(inputs[1], { target: { value: 'Short1!' } });
    fireEvent.change(inputs[2], { target: { value: 'Short1!' } });
    fireEvent.click(screen.getByRole('button', { name: /change password/i }));
    await waitFor(() => {
      expect(screen.getByText(/at least 12 characters/i)).toBeTruthy();
    });
    expect(mockChangePassword).not.toHaveBeenCalled();
  });

  it('shows error when new passwords do not match', async () => {
    renderForcePage();
    const inputs = document.querySelectorAll('input[type="password"]');
    fireEvent.change(inputs[0], { target: { value: 'OldPass1234!' } });
    fireEvent.change(inputs[1], { target: { value: 'NewPassword123!' } });
    fireEvent.change(inputs[2], { target: { value: 'Different123!' } });
    fireEvent.click(screen.getByRole('button', { name: /change password/i }));
    await waitFor(() => {
      expect(screen.getByText(/passwords do not match/i)).toBeTruthy();
    });
    expect(mockChangePassword).not.toHaveBeenCalled();
  });

  it('shows error when new password equals current password', async () => {
    renderForcePage();
    const inputs = document.querySelectorAll('input[type="password"]');
    fireEvent.change(inputs[0], { target: { value: 'SamePassword123!' } });
    fireEvent.change(inputs[1], { target: { value: 'SamePassword123!' } });
    fireEvent.change(inputs[2], { target: { value: 'SamePassword123!' } });
    fireEvent.click(screen.getByRole('button', { name: /change password/i }));
    await waitFor(() => {
      expect(screen.getByText(/must differ/i)).toBeTruthy();
    });
    expect(mockChangePassword).not.toHaveBeenCalled();
  });

  it('on success: calls changePassword, updates localStorage, and calls window.location.reload()', async () => {
    mockChangePassword.mockResolvedValueOnce({ data: {} });

    renderForcePage();
    const inputs = document.querySelectorAll('input[type="password"]');
    fireEvent.change(inputs[0], { target: { value: 'OldPass1234!' } });
    fireEvent.change(inputs[1], { target: { value: 'NewPassword123!!' } });
    fireEvent.change(inputs[2], { target: { value: 'NewPassword123!!' } });
    fireEvent.click(screen.getByRole('button', { name: /change password/i }));

    await waitFor(() => {
      expect(mockChangePassword).toHaveBeenCalledWith('OldPass1234!', 'NewPassword123!!');
    });

    // localStorage must be updated with must_change_password: false
    const stored = JSON.parse(localStorage.getItem('medops_user') || '{}');
    expect(stored.must_change_password).toBe(false);

    // Must use reload() — NOT window.location.href = '/' (file:// unsafe)
    expect(reloadSpy).toHaveBeenCalledTimes(1);
  });

  it('on success: does NOT set window.location.href (packaged file:// safety)', async () => {
    mockChangePassword.mockResolvedValueOnce({ data: {} });

    // Track any href assignment attempt
    let hrefSet = false;
    const locationDescriptor = Object.getOwnPropertyDescriptor(window, 'location')!;
    Object.defineProperty(window, 'location', {
      value: {
        ...window.location,
        reload: reloadSpy,
        set href(_: string) { hrefSet = true; },
      },
      writable: true,
      configurable: true,
    });

    renderForcePage();
    const inputs = document.querySelectorAll('input[type="password"]');
    fireEvent.change(inputs[0], { target: { value: 'OldPass1234!' } });
    fireEvent.change(inputs[1], { target: { value: 'NewPassword123!!' } });
    fireEvent.change(inputs[2], { target: { value: 'NewPassword123!!' } });
    fireEvent.click(screen.getByRole('button', { name: /change password/i }));

    await waitFor(() => {
      expect(reloadSpy).toHaveBeenCalled();
    });

    expect(hrefSet).toBe(false);

    // Restore
    Object.defineProperty(window, 'location', locationDescriptor);
  });

  it('shows API error message when changePassword call fails', async () => {
    mockChangePassword.mockRejectedValueOnce({
      response: { data: { error: 'Incorrect current password' } },
    });

    renderForcePage();
    const inputs = document.querySelectorAll('input[type="password"]');
    fireEvent.change(inputs[0], { target: { value: 'WrongOld1234!' } });
    fireEvent.change(inputs[1], { target: { value: 'NewPassword123!!' } });
    fireEvent.change(inputs[2], { target: { value: 'NewPassword123!!' } });
    fireEvent.click(screen.getByRole('button', { name: /change password/i }));

    await waitFor(() => {
      expect(screen.getByText(/incorrect current password/i)).toBeTruthy();
    });
    expect(reloadSpy).not.toHaveBeenCalled();
  });
});
