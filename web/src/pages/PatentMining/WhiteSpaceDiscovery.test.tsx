import { render, screen, fireEvent, act } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import WhiteSpaceDiscovery from './WhiteSpaceDiscovery';

// Mock Recharts ResponsiveContainer because it relies on ResizeObserver and DOM dimensions
vi.mock('recharts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('recharts')>();
  return {
    ...actual,
    ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
      <div style={{ width: 800, height: 500 }} data-testid="responsive-container">
        {children}
      </div>
    ),
  };
});

describe('WhiteSpaceDiscovery', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('renders correctly initially', () => {
    render(<WhiteSpaceDiscovery />);
    expect(screen.getByText('Discovery Parameters')).toBeInTheDocument();
    expect(screen.getByText('Molecular Space Map')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Identify White Spaces/i })).toBeInTheDocument();
    // Chart should not be visible initially
    expect(screen.queryByTestId('responsive-container')).not.toBeInTheDocument();
  });

  it('shows chart after analysis', async () => {
    render(<WhiteSpaceDiscovery />);

    const analyzeButton = screen.getByRole('button', { name: /Identify White Spaces/i });
    fireEvent.click(analyzeButton);

    // Fast-forward timers inside act to flush updates
    act(() => {
      vi.advanceTimersByTime(2000);
    });

    // Wait for the chart container to appear
    // Since we used act(), the update should happen immediately.
    expect(screen.getByTestId('responsive-container')).toBeInTheDocument();

    // Check if legend items are present (custom HTML legend)
    expect(screen.getByText('Patented')).toBeInTheDocument();
    expect(screen.getByText('Candidate')).toBeInTheDocument();
    expect(screen.getByText('White Space')).toBeInTheDocument();
  });
});
