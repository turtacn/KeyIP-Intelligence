import React from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Routes, Route } from 'react-router-dom';
import AppShell from '@/components/layout/AppShell';

// Mock the layout sub-components to isolate AppShell testing
vi.mock('@/components/layout/Sidebar', () => ({
  default: () => <div data-testid="sidebar">Sidebar Mock</div>,
}));

vi.mock('@/components/layout/TopBar', () => ({
  default: () => <div data-testid="topbar">TopBar Mock</div>,
}));

vi.mock('@/components/layout/Breadcrumb', () => ({
  default: () => <div data-testid="breadcrumb">Breadcrumb Mock</div>,
}));

describe('AppShell', () => {
  it('renders sidebar, topbar, and breadcrumb', () => {
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Routes>
          <Route path="/" element={<AppShell />}>
            <Route path="dashboard" element={<div>Dashboard Page</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByTestId('sidebar')).toBeInTheDocument();
    expect(screen.getByTestId('topbar')).toBeInTheDocument();
    expect(screen.getByTestId('breadcrumb')).toBeInTheDocument();
  });

  it('renders child route content via Outlet', () => {
    render(
      <MemoryRouter initialEntries={['/patents']}>
        <Routes>
          <Route path="/" element={<AppShell />}>
            <Route path="patents" element={<div>Patents Content</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    expect(screen.getByText('Patents Content')).toBeInTheDocument();
  });

  it('renders loading spinner fallback for lazy routes via Suspense', () => {
    // Create a lazy component that never resolves so Suspense fallback shows
    const LazyComponent = React.lazy(
      () =>
        new Promise<{ default: React.ComponentType }>(() => {
          // Intentionally never resolve to keep Suspense in fallback state
        })
    );

    render(
      <MemoryRouter initialEntries={['/lazy']}>
        <Routes>
          <Route path="/" element={<AppShell />}>
            <Route path="lazy" element={<LazyComponent />} />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    // AppShell wraps Outlet in Suspense with LoadingSpinner fallback
    expect(screen.getByLabelText('Loading')).toBeInTheDocument();
  });

  it('has correct layout structure with sidebar offset', () => {
    render(
      <MemoryRouter initialEntries={['/dashboard']}>
        <Routes>
          <Route path="/" element={<AppShell />}>
            <Route path="dashboard" element={<div>Content</div>} />
          </Route>
        </Routes>
      </MemoryRouter>
    );

    // Main content should have ml-64 class for sidebar offset
    const main = document.querySelector('main');
    expect(main).toBeInTheDocument();
    expect(main?.className).toContain('ml-64');
  });
});
