import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import PortfolioOptimizer from '@/pages/PortfolioOptimizer';

// Mock Recharts ResponsiveContainer to avoid dimension issues in tests
vi.mock('recharts', async () => {
  const actual = await vi.importActual<typeof import('recharts')>('recharts');
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 800, height: 500 }} data-testid="responsive-container">
        {children}
      </div>
    ),
  };
});

// Create mock functions for each hook
const mockUsePortfolio = vi.fn();
const mockUsePartners = vi.fn();
const mockUsePatents = vi.fn();
const mockUsePortfolioConstellation = vi.fn();

vi.mock('@/hooks/usePortfolio', () => ({
  usePortfolio: () => mockUsePortfolio(),
}));

vi.mock('@/hooks/usePartner', () => ({
  usePartners: () => mockUsePartners(),
}));

vi.mock('@/hooks/usePatents', () => ({
  usePatents: () => mockUsePatents(),
}));

vi.mock('@/hooks/usePortfolioConstellation', () => ({
  usePortfolioConstellation: () => mockUsePortfolioConstellation(),
}));

const mockSummaryData = {
  id: 'pf-1',
  totalPatents: 158,
  granted: 95,
  pending: 53,
  lapsed: 10,
  totalValue: 48500000,
  healthGrade: 'B+',
};

const mockScoresData = {
  coverage: 78,
  concentration: 65,
  aging: 82,
  totalValue: 48500000,
  healthGrade: 'B+',
};

const mockCoverageData = {
  'Blue Emitter': 45,
  'Green Emitter': 32,
  'Red Emitter': 15,
};

const mockCompanies = [
  { id: 'comp_001', name: 'Samsung SDI', country: 'KR', portfolioSize: 12500, type: 'Conglomerate' },
  { id: 'comp_002', name: 'LG Chem', country: 'KR', portfolioSize: 9800, type: 'Conglomerate' },
];

const mockPatents = [
  { id: 'pat_001', patentNumber: 'US12345678B2', title: 'OLED compound', status: 'granted' },
];

const mockConstellation = {
  portfolio_id: 'pf-1',
  points: [{ id: 'pt-1', x: 1, y: 2, point_type: 'own_patent', value_score: 85 }],
  clusters: [{ cluster_id: 'c1', label: 'test', center_x: 0, center_y: 0, point_count: 1, tech_domain: 'test' }],
  white_spaces: [],
  total_points: 1,
};

// scrollIntoView is not implemented in jsdom
Element.prototype.scrollIntoView = vi.fn();

describe('PortfolioOptimizer', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('shows loading state initially', () => {
    mockUsePortfolio.mockReturnValue({
      summary: { data: null, loading: true, error: null, refetch: vi.fn() },
      scores: { data: null, loading: true, error: null, refetch: vi.fn() },
      coverage: { data: null, loading: true, error: null, refetch: vi.fn() },
    });
    mockUsePartners.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() });
    mockUsePatents.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() });
    mockUsePortfolioConstellation.mockReturnValue({ data: null, loading: true, error: null, refetch: vi.fn() });

    const { container } = render(<MemoryRouter><PortfolioOptimizer /></MemoryRouter>);

    // Should show PortfolioOptimizerSkeleton (animate-pulse elements)
    const skeletons = container.querySelectorAll('.animate-pulse');
    expect(skeletons.length).toBeGreaterThan(0);
    // Should NOT show nav items
    expect(screen.queryByText('portfolio.nav.panorama')).not.toBeInTheDocument();
  });

  it('renders full portfolio view with all data', () => {
    mockUsePortfolio.mockReturnValue({
      summary: { data: mockSummaryData, loading: false, error: null, refetch: vi.fn() },
      scores: { data: mockScoresData, loading: false, error: null, refetch: vi.fn() },
      coverage: { data: mockCoverageData, loading: false, error: null, refetch: vi.fn() },
    });
    mockUsePartners.mockReturnValue({ data: mockCompanies, loading: false, error: null, refetch: vi.fn() });
    mockUsePatents.mockReturnValue({ data: mockPatents, loading: false, error: null, refetch: vi.fn() });
    mockUsePortfolioConstellation.mockReturnValue({ data: mockConstellation, loading: false, error: null, refetch: vi.fn() });

    render(<MemoryRouter><PortfolioOptimizer /></MemoryRouter>);

    // All nav items should be rendered
    expect(screen.getByText('portfolio.nav.panorama')).toBeInTheDocument();
    expect(screen.getByText('Constellation')).toBeInTheDocument();
    expect(screen.getByText('portfolio.nav.gap')).toBeInTheDocument();
    expect(screen.getByText('portfolio.nav.scoring')).toBeInTheDocument();
    expect(screen.getByText('portfolio.nav.budget')).toBeInTheDocument();
    expect(screen.getByText('portfolio.nav.simulator')).toBeInTheDocument();

    // Section titles should be visible
    expect(screen.getByText('portfolio.panorama.title')).toBeInTheDocument();
    expect(screen.getByText('portfolio.panorama.desc')).toBeInTheDocument();
    expect(screen.getAllByText('Patent Constellation Map').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('portfolio.gap.title').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('portfolio.scoring.title').length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText('portfolio.budget.title')).toBeInTheDocument();
    expect(screen.getByText('portfolio.simulator.title')).toBeInTheDocument();

    // Summary stat values should be visible (from PanoramaView)
    expect(screen.getByText('158')).toBeInTheDocument();
    expect(screen.getByText('95')).toBeInTheDocument();
    expect(screen.getByText('53')).toBeInTheDocument();
    expect(screen.getByText('$48.5M')).toBeInTheDocument();
    expect(screen.getByText('B+')).toBeInTheDocument();
  });

  it('navigates sections via nav click', () => {
    mockUsePortfolio.mockReturnValue({
      summary: { data: mockSummaryData, loading: false, error: null, refetch: vi.fn() },
      scores: { data: mockScoresData, loading: false, error: null, refetch: vi.fn() },
      coverage: { data: mockCoverageData, loading: false, error: null, refetch: vi.fn() },
    });
    mockUsePartners.mockReturnValue({ data: mockCompanies, loading: false, error: null, refetch: vi.fn() });
    mockUsePatents.mockReturnValue({ data: mockPatents, loading: false, error: null, refetch: vi.fn() });
    mockUsePortfolioConstellation.mockReturnValue({ data: mockConstellation, loading: false, error: null, refetch: vi.fn() });

    render(<MemoryRouter><PortfolioOptimizer /></MemoryRouter>);

    // Click on Gap Analysis nav
    fireEvent.click(screen.getByText('portfolio.nav.gap'));
    // Section content for gap should still be present
    expect(screen.getAllByText('portfolio.gap.title').length).toBeGreaterThanOrEqual(1);
  });

  it('shows error state when portfolio fetch fails', () => {
    mockUsePortfolio.mockReturnValue({
      summary: { data: null, loading: false, error: 'Portfolio API error', refetch: vi.fn() },
      scores: { data: null, loading: false, error: null, refetch: vi.fn() },
      coverage: { data: null, loading: false, error: null, refetch: vi.fn() },
    });
    mockUsePartners.mockReturnValue({ data: null, loading: false, error: null, refetch: vi.fn() });
    mockUsePatents.mockReturnValue({ data: null, loading: false, error: null, refetch: vi.fn() });
    mockUsePortfolioConstellation.mockReturnValue({ data: null, loading: false, error: null, refetch: vi.fn() });

    render(<MemoryRouter><PortfolioOptimizer /></MemoryRouter>);

    // PageError should render
    expect(
      screen.getByText('Failed to load portfolio data')
    ).toBeInTheDocument();
    expect(
      screen.getByText('There was a problem fetching your portfolio. Please try again.')
    ).toBeInTheDocument();
    expect(screen.getByText('Portfolio API error')).toBeInTheDocument();
    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows empty state when summary data is null', () => {
    mockUsePortfolio.mockReturnValue({
      summary: { data: null, loading: false, error: null, refetch: vi.fn() },
      scores: { data: null, loading: false, error: null, refetch: vi.fn() },
      coverage: { data: null, loading: false, error: null, refetch: vi.fn() },
    });
    mockUsePartners.mockReturnValue({ data: null, loading: false, error: null, refetch: vi.fn() });
    mockUsePatents.mockReturnValue({ data: null, loading: false, error: null, refetch: vi.fn() });
    mockUsePortfolioConstellation.mockReturnValue({ data: null, loading: false, error: null, refetch: vi.fn() });

    render(<MemoryRouter><PortfolioOptimizer /></MemoryRouter>);

    // EmptyState should render
    expect(
      screen.getByText('No portfolio data')
    ).toBeInTheDocument();
    expect(
      screen.getByText('No portfolio metrics are available.')
    ).toBeInTheDocument();
    // Refresh button should be shown
    expect(screen.getByText('Refresh')).toBeInTheDocument();
  });
});
