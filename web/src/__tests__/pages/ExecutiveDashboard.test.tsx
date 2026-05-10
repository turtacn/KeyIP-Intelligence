import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent, act } from '@testing-library/react';
import ExecutiveDashboard from '@/pages/ExecutiveDashboard';

// Mock the useDashboard hook to control data/loading/error states
const mockUseDashboard = vi.fn();
vi.mock('@/hooks/useDashboard', () => ({
  useDashboard: () => mockUseDashboard(),
}));

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

const mockDashboardData = {
  totalPatents: 158,
  activePatents: 142,
  pendingPatents: 10,
  highRiskAlerts: 7,
  dueThisMonth: 3,
  portfolioHealthScore: 76,
  monthlyApplicationTrend: [
    { month: 'Jan', filed: 5, granted: 2 },
    { month: 'Feb', filed: 3, granted: 1 },
  ],
  jurisdictionBreakdown: [
    { jurisdiction: 'CN', count: 65 },
    { jurisdiction: 'US', count: 38 },
  ],
  competitorComparison: [
    { name: 'Organization', portfolioSize: 158 },
    { name: 'Samsung SDI', portfolioSize: 12500 },
  ],
  upcomingDeadlines: [
    {
      id: 'le_001',
      patentId: 'pat_001',
      jurisdiction: 'CN' as const,
      eventType: 'annuity_due',
      dueDate: '2024-09-20',
      feeAmount: 1200,
      currency: 'CNY',
      status: 'pending' as const,
    },
  ],
  recentAlerts: [
    {
      id: 'alert_001',
      targetPatentId: 'pat_001',
      triggerMoleculeId: 'mol_001',
      riskLevel: 'HIGH' as const,
      literalScore: 0.88,
      docScore: 0.76,
      detectedAt: '2024-06-15T10:30:00Z',
      status: 'new' as const,
    },
  ],
};

describe('ExecutiveDashboard', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('shows loading state initially', () => {
    mockUseDashboard.mockReturnValue({
      data: null,
      loading: true,
      error: null,
      refetch: vi.fn(),
    });

    render(<ExecutiveDashboard />);

    // Should show loading spinner
    expect(screen.getByLabelText('Loading')).toBeInTheDocument();
  });

  it('renders dashboard with mock data', () => {
    mockUseDashboard.mockReturnValue({
      data: mockDashboardData,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<ExecutiveDashboard />);

    // Check for dashboard title (translation key since i18n not initialized in test)
    expect(screen.getByText('dashboard.title')).toBeInTheDocument();
    expect(screen.getByText('dashboard.subtitle')).toBeInTheDocument();

    // Check for action buttons
    expect(screen.getByText('dashboard.export')).toBeInTheDocument();
    expect(screen.getByText('dashboard.add_patent')).toBeInTheDocument();

    // Check for NLQueryWidget
    expect(
      screen.getByPlaceholderText('dashboard.nl_query_placeholder')
    ).toBeInTheDocument();

    // Check for KPICards section - should render with metric values
    expect(screen.getByText('dashboard.kpi.total_patents')).toBeInTheDocument();
    expect(screen.getByText('dashboard.kpi.active_patents')).toBeInTheDocument();

    // KPI values should be visible
    expect(screen.getByText('158')).toBeInTheDocument();
    expect(screen.getByText('142')).toBeInTheDocument();
    expect(screen.getByText('7')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();

    // Chart sections should be rendered
    expect(screen.getByText('dashboard.trend.title')).toBeInTheDocument();
    expect(screen.getByText('dashboard.jurisdiction.title')).toBeInTheDocument();
    expect(screen.getByText('dashboard.radar.title')).toBeInTheDocument();

    // Deadlines and alerts tables should render
    expect(screen.getByText('dashboard.deadlines.title')).toBeInTheDocument();
    expect(screen.getByText('dashboard.alerts.title')).toBeInTheDocument();
  });

  it('shows error state when data fetch fails', () => {
    mockUseDashboard.mockReturnValue({
      data: null,
      loading: false,
      error: 'Network error',
      refetch: vi.fn(),
    });

    render(<ExecutiveDashboard />);

    expect(
      screen.getByText(/error loading dashboard data/i)
    ).toBeInTheDocument();
    expect(screen.getByText(/Network error/)).toBeInTheDocument();
  });

  it('generates report when export button is clicked', () => {
    const alertSpy = vi.spyOn(window, 'alert').mockImplementation(() => {});

    mockUseDashboard.mockReturnValue({
      data: mockDashboardData,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<ExecutiveDashboard />);

    const exportButton = screen.getByText('dashboard.export');
    fireEvent.click(exportButton);

    // Fast-forward past the 2-second simulated generation
    act(() => {
      vi.advanceTimersByTime(2000);
    });

    expect(alertSpy).toHaveBeenCalledWith('Report generated successfully!');
    alertSpy.mockRestore();
  });
});
