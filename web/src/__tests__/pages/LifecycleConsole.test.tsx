import React from 'react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import LifecycleConsole from '@/pages/LifecycleConsole';

// Mock the useLifecycle hook to control data/loading/error states
const mockUseLifecycle = vi.fn();
vi.mock('@/hooks/useLifecycle', () => ({
  useLifecycle: () => mockUseLifecycle(),
}));

const mockEvents = [
  {
    id: 'le_001',
    patentId: 'pat_001',
    jurisdiction: 'CN',
    eventType: 'annuity_due',
    dueDate: '2024-09-20',
    feeAmount: 1200,
    currency: 'CNY',
    status: 'pending',
  },
  {
    id: 'le_002',
    patentId: 'pat_002',
    jurisdiction: 'US',
    eventType: 'response_deadline',
    dueDate: '2024-07-15',
    feeAmount: 0,
    status: 'pending',
  },
  {
    id: 'le_005',
    patentId: 'pat_005',
    jurisdiction: 'KR',
    eventType: 'examination',
    dueDate: '2024-06-30',
    status: 'completed',
  },
];

describe('LifecycleConsole', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it('shows loading state initially', () => {
    mockUseLifecycle.mockReturnValue({
      data: null,
      loading: true,
      error: null,
      refetch: vi.fn(),
    });

    const { container } = render(<LifecycleConsole />);

    // Should show SkeletonTable (animate-pulse elements with aria-hidden)
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
    // Should NOT show the title nor tabs when loading
    expect(screen.queryByText('lifecycle.title')).not.toBeInTheDocument();
    expect(screen.queryByText('lifecycle.tabs.calendar')).not.toBeInTheDocument();
  });

  it('renders lifecycle console with event data', () => {
    mockUseLifecycle.mockReturnValue({
      data: mockEvents,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<LifecycleConsole />);

    // Title and export button should be visible
    expect(screen.getByText('lifecycle.title')).toBeInTheDocument();
    expect(screen.getByText('lifecycle.calendar.export')).toBeInTheDocument();

    // Tabs should be rendered
    expect(screen.getByText('lifecycle.tabs.calendar')).toBeInTheDocument();
    expect(screen.getByText('lifecycle.tabs.annuity')).toBeInTheDocument();
    expect(screen.getByText('lifecycle.tabs.status')).toBeInTheDocument();

    // Calendar tab is active by default — DeadlineTable should show patent IDs
    expect(screen.getByText('pat_001')).toBeInTheDocument();
    expect(screen.getByText('pat_002')).toBeInTheDocument();
    expect(screen.getByText('pat_005')).toBeInTheDocument();

    // Jurisdictions should appear
    expect(screen.getByText('CN')).toBeInTheDocument();
    expect(screen.getByText('US')).toBeInTheDocument();
    expect(screen.getByText('KR')).toBeInTheDocument();
  });

  it('switches between tabs', () => {
    mockUseLifecycle.mockReturnValue({
      data: mockEvents,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<LifecycleConsole />);

    // Click on annuity tab
    fireEvent.click(screen.getByText('lifecycle.tabs.annuity'));

    // Annuity events should be shown (only annuity_due filtered)
    // The AnnuityManager filters for eventType === 'annuity_due'
    // Only le_001 is annuity_due
    expect(screen.getByText('pat_001')).toBeInTheDocument();
    // Table header for fee amount
    expect(screen.getByText('lifecycle.annuity.fee_amount')).toBeInTheDocument();

    // Click on status tab
    fireEvent.click(screen.getByText('lifecycle.tabs.status'));
    // LegalStatusMonitor should show status-related content
    expect(screen.getByText('lifecycle.table.patent_id')).toBeInTheDocument();
  });

  it('shows error state when data fetch fails', () => {
    mockUseLifecycle.mockReturnValue({
      data: null,
      loading: false,
      error: 'Failed to fetch lifecycle events',
      refetch: vi.fn(),
    });

    render(<LifecycleConsole />);

    // PageError should be rendered with error text and retry button
    expect(
      screen.getByText('Failed to load lifecycle data')
    ).toBeInTheDocument();
    expect(
      screen.getByText('There was a problem fetching lifecycle events.')
    ).toBeInTheDocument();
    expect(screen.getByText('Failed to fetch lifecycle events')).toBeInTheDocument();
    expect(screen.getByText('Retry')).toBeInTheDocument();
  });

  it('shows empty state when no events match', () => {
    mockUseLifecycle.mockReturnValue({
      data: [],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<LifecycleConsole />);

    // Title and tabs should render
    expect(screen.getByText('lifecycle.title')).toBeInTheDocument();
    // The DeadlineTable should render with no rows, showing only column headers
    expect(screen.getByText('lifecycle.table.patent_id')).toBeInTheDocument();
    expect(screen.getByText('lifecycle.table.jurisdiction')).toBeInTheDocument();
  });

  it('triggers CSV export when export button is clicked', () => {
    // Mock createElement and click for CSV download
    const createElementSpy = vi.spyOn(document, 'createElement');
    const clickSpy = vi.fn();
    createElementSpy.mockReturnValue({
      setAttribute: vi.fn(),
      click: clickSpy,
      href: '',
      download: '',
    } as unknown as HTMLAnchorElement);

    mockUseLifecycle.mockReturnValue({
      data: mockEvents,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<LifecycleConsole />);

    const exportButton = screen.getByText('lifecycle.calendar.export');
    fireEvent.click(exportButton);

    // Should have created a download link
    expect(createElementSpy).toHaveBeenCalledWith('a');
    expect(clickSpy).toHaveBeenCalled();

    createElementSpy.mockRestore();
  });
});
