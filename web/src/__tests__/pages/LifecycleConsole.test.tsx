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

  it('shows skeleton loading in content area while loading', () => {
    mockUseLifecycle.mockReturnValue({
      data: null,
      loading: true,
      error: null,
      refetch: vi.fn(),
    });

    const { container } = render(<LifecycleConsole />);

    // Title and tabs ARE rendered even during loading (only tab content shows skeleton)
    expect(screen.getByText('lifecycle.title')).toBeInTheDocument();
    expect(screen.getByText('lifecycle.tabs.calendar')).toBeInTheDocument();
    // Tab content area should show SkeletonTable with animate-pulse
    expect(container.querySelector('.animate-pulse')).toBeInTheDocument();
  });

  it('renders lifecycle console with event data', () => {
    mockUseLifecycle.mockReturnValue({
      data: mockEvents,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<LifecycleConsole />);

    // Title should be visible
    expect(screen.getByText('lifecycle.title')).toBeInTheDocument();
    // Export button appears in header (and possibly elsewhere), so use getAllByText
    expect(screen.getAllByText('lifecycle.calendar.export').length).toBeGreaterThanOrEqual(1);

    // Tabs should be rendered
    expect(screen.getByText('lifecycle.tabs.calendar')).toBeInTheDocument();
    expect(screen.getByText('lifecycle.tabs.annuity')).toBeInTheDocument();
    expect(screen.getByText('lifecycle.tabs.status')).toBeInTheDocument();

    // Calendar tab is active by default — DeadlineTable should show patent IDs
    expect(screen.getByText('pat_001')).toBeInTheDocument();
    expect(screen.getByText('pat_002')).toBeInTheDocument();
    expect(screen.getByText('pat_005')).toBeInTheDocument();

    // Jurisdictions should appear
    // Jurisdictions appear in both filter panel and data table
    expect(screen.getAllByText('CN').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('US').length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText('KR').length).toBeGreaterThanOrEqual(1);
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

    // AnnuityManager filters for eventType === 'annuity_due'
    // Only le_001 is annuity_due, so only pat_001 should appear
    expect(screen.getByText('pat_001')).toBeInTheDocument();
    // Table header for fee amount
    expect(screen.getByText('lifecycle.annuity.fee_amount')).toBeInTheDocument();

    // Click on status tab
    fireEvent.click(screen.getByText('lifecycle.tabs.status'));
    // LegalStatusMonitor renders patent IDs as card headings
    expect(screen.getByText('pat_001')).toBeInTheDocument();
    expect(screen.getByText('pat_002')).toBeInTheDocument();
    // Should show jurisdiction office labels
    // CNIPA appears in all 3 patent cards, USPTO also appears 3 times
    expect(screen.getAllByText('CNIPA').length).toBe(3);
    expect(screen.getAllByText('USPTO').length).toBe(3);
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

  it('shows empty table when no events match', () => {
    mockUseLifecycle.mockReturnValue({
      data: [],
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<LifecycleConsole />);

    // Title and tabs should render
    expect(screen.getByText('lifecycle.title')).toBeInTheDocument();
    // The DeadlineTable should render with column headers
    expect(screen.getByText('lifecycle.table.patent_id')).toBeInTheDocument();
    expect(screen.getByText('lifecycle.table.jurisdiction')).toBeInTheDocument();
    // No patent IDs should appear (empty data)
    expect(screen.queryByText('pat_001')).not.toBeInTheDocument();
  });

  it('triggers CSV export when export button is clicked', () => {
    // Use a real anchor element for createElement to avoid appendChild errors
    const originalCreateElement = document.createElement.bind(document);
    const clickSpy = vi.fn();
    const createElementSpy = vi.spyOn(document, 'createElement').mockImplementation((tagName, options) => {
      const el = originalCreateElement(tagName, options);
      if (tagName === 'a') {
        // Wrap the click to spy on it
        const originalClick = el.click.bind(el);
        el.click = () => {
          clickSpy();
          originalClick();
        };
      }
      return el;
    });

    mockUseLifecycle.mockReturnValue({
      data: mockEvents,
      loading: false,
      error: null,
      refetch: vi.fn(),
    });

    render(<LifecycleConsole />);

    const exportButton = screen.getAllByText('lifecycle.calendar.export')[0];
    fireEvent.click(exportButton);

    // Should have created a download link and triggered click
    expect(createElementSpy).toHaveBeenCalledWith('a');
    expect(clickSpy).toHaveBeenCalled();

    createElementSpy.mockRestore();
  });
});
