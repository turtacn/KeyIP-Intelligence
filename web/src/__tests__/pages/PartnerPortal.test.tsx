import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import PartnerPortal from '@/pages/PartnerPortal';

// Mock the usePartners hook to control data/loading/error states
const mockUsePartners = vi.fn();
vi.mock('@/hooks/usePartner', () => ({
  usePartners: () => mockUsePartners(),
}));

const mockPartners = [
  { id: 'comp_001', name: 'Samsung SDI', country: 'KR', portfolioSize: 12500, type: 'Conglomerate' },
  { id: 'comp_002', name: 'LG Chem', country: 'KR', portfolioSize: 9800, type: 'Conglomerate' },
  { id: 'comp_003', name: 'Universal Display Corp (UDC)', country: 'US', portfolioSize: 4500, type: 'IP Licensing' },
];

describe('PartnerPortal', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('shows loading state initially', () => {
    mockUsePartners.mockReturnValue({
      data: null,
      loading: true,
      error: null,
      refetch: vi.fn(),
    });

    const { container } = render(<PartnerPortal />);

    // Should show skeleton cards (animate-pulse with bg-slate-200)
    const skeletons = container.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
    // Should NOT show partner data
    expect(screen.queryByText('partners.nav.admin')).not.toBeInTheDocument();
    expect(screen.queryByText('Samsung SDI')).not.toBeInTheDocument();
  });

  it('renders partner portal with partner data', () => {
    mockUsePartners.mockReturnValue({
      data: mockPartners,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<PartnerPortal />);

    // Navigation items should be rendered (admin is active by default)
    expect(screen.getByText('partners.nav.admin')).toBeInTheDocument();
    expect(screen.getByText('partners.nav.agency')).toBeInTheDocument();
    expect(screen.getByText('partners.nav.counsel')).toBeInTheDocument();
    expect(screen.getByText('partners.nav.api')).toBeInTheDocument();

    // Admin view should display partner names via DataTable
    expect(screen.getByText('Samsung SDI')).toBeInTheDocument();
    expect(screen.getByText('LG Chem')).toBeInTheDocument();
    expect(screen.getByText('Universal Display Corp (UDC)')).toBeInTheDocument();

    // Table column headers should be visible
    expect(screen.getByText('partners.admin.table.name')).toBeInTheDocument();
    expect(screen.getByText('partners.admin.table.type')).toBeInTheDocument();
    expect(screen.getByText('partners.admin.table.country')).toBeInTheDocument();

    // Add partner button should be present
    expect(screen.getByText('partners.admin.add_btn')).toBeInTheDocument();
  });

  it('switches between portal views', () => {
    mockUsePartners.mockReturnValue({
      data: mockPartners,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<PartnerPortal />);

    // Click on Agency view
    fireEvent.click(screen.getByText('partners.nav.agency'));
    // Agency view should show its title
    expect(screen.getByText('partners.agency.title')).toBeInTheDocument();

    // Click on Counsel view
    fireEvent.click(screen.getByText('partners.nav.counsel'));
    expect(screen.getByText('partners.counsel.title')).toBeInTheDocument();

    // Click on API Portal view
    fireEvent.click(screen.getByText('partners.nav.api'));
    expect(screen.getByText('partners.api.title')).toBeInTheDocument();

    // Click back to Admin — partner data should still render
    fireEvent.click(screen.getByText('partners.nav.admin'));
    expect(screen.getByText('Samsung SDI')).toBeInTheDocument();
  });

  it('shows error state when data fetch fails', () => {
    mockUsePartners.mockReturnValue({
      data: null,
      loading: false,
      error: 'Network request failed',
      refetch: vi.fn(),
    });

    render(<PartnerPortal />);

    // PageError should render with error details
    expect(
      screen.getByText('Failed to load partner data')
    ).toBeInTheDocument();
    expect(
      screen.getByText('There was a problem fetching partner information.')
    ).toBeInTheDocument();
    expect(screen.getByText('Network request failed')).toBeInTheDocument();
    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows empty state when no partners exist', () => {
    mockUsePartners.mockReturnValue({
      data: [],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<PartnerPortal />);

    // Nav should still render
    expect(screen.getByText('partners.nav.admin')).toBeInTheDocument();
    // Admin view header should render
    expect(screen.getByText('partners.admin.title')).toBeInTheDocument();
    // No partner names should appear
    expect(screen.queryByText('Samsung SDI')).not.toBeInTheDocument();
    // Add button should still appear
    expect(screen.getByText('partners.admin.add_btn')).toBeInTheDocument();
  });
});
