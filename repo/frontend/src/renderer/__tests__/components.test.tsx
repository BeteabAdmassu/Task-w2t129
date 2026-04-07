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

vi.mock('../services/api', () => ({
  authAPI: {
    login: vi.fn(),
    logout: vi.fn(),
    me: vi.fn(),
  },
  inventoryAPI: { listSKUs: vi.fn(), getSKU: vi.fn() },
  membersAPI: { listMembers: vi.fn(), getMember: vi.fn() },
  workOrdersAPI: { listWorkOrders: vi.fn() },
  learningAPI: { listSubjects: vi.fn(), listChapters: vi.fn() },
}));

// ─── Import pages AFTER mocks are in place ────────────────────────────────────

import LoginPage from '../components/admin/LoginPage';
import DashboardPage from '../components/admin/DashboardPage';

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

  it('renders the dashboard heading or welcome element', () => {
    renderInRouter(<DashboardPage />);
    // Dashboard should contain some identifying heading
    const headings = screen.getAllByRole('heading');
    expect(headings.length).toBeGreaterThan(0);
  });
});

// ─── ProtectedRoute redirect behavior ────────────────────────────────────────

import App from '../App';

describe('App routing — unauthenticated redirect', () => {
  beforeEach(() => {
    mockUseAuth.isAuthenticated = false;
    mockUseAuth.user = null;
  });

  it('shows login page when navigating to / unauthenticated', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    );
    // The login page heading should be visible
    expect(screen.getByText('MedOps Console')).toBeTruthy();
  });

  it('shows login page when navigating to /members unauthenticated', () => {
    render(
      <MemoryRouter initialEntries={['/members']}>
        <App />
      </MemoryRouter>,
    );
    expect(screen.getByText('MedOps Console')).toBeTruthy();
  });

  it('shows login page when navigating to /skus unauthenticated', () => {
    render(
      <MemoryRouter initialEntries={['/skus']}>
        <App />
      </MemoryRouter>,
    );
    expect(screen.getByText('MedOps Console')).toBeTruthy();
  });
});

describe('App routing — authenticated access', () => {
  beforeEach(() => {
    mockUseAuth.isAuthenticated = true;
    mockUseAuth.user = { id: '1', username: 'admin', role: 'system_admin' };
  });

  it('does not show login page when authenticated user visits /', () => {
    render(
      <MemoryRouter initialEntries={['/']}>
        <App />
      </MemoryRouter>,
    );
    // Login heading should NOT be visible
    expect(screen.queryByRole('heading', { name: /medops console/i })).toBeNull();
  });

  it('redirects /login to / when already authenticated', () => {
    render(
      <MemoryRouter initialEntries={['/login']}>
        <App />
      </MemoryRouter>,
    );
    expect(screen.queryByRole('heading', { name: /medops console/i })).toBeNull();
  });
});
