import { render, screen, fireEvent, act } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import WhiteSpaceDiscovery from './WhiteSpaceDiscovery';
import React from 'react';

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
    expect(screen.getByText('mining.whitespace.params_title')).toBeInTheDocument();
    expect(screen.getByText('mining.whitespace.map_title')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /mining.whitespace.btn_identify/i })).toBeInTheDocument();
    // Chart should not be visible initially
    expect(screen.queryByTestId('responsive-container')).not.toBeInTheDocument();
  });

  it('shows chart after analysis', async () => {
    render(<WhiteSpaceDiscovery />);

    const analyzeButton = screen.getByRole('button', { name: /mining.whitespace.btn_identify/i });
    fireEvent.click(analyzeButton);

    // Fast-forward timers inside act to flush updates
    act(() => {
      vi.advanceTimersByTime(2000);
    });

    // Wait for the chart container to appear
    // Since we used act(), the update should happen immediately.
    expect(screen.getByTestId('responsive-container')).toBeInTheDocument();

    // Check if legend items are present (custom HTML legend)
    expect(screen.getByText('mining.whitespace.legend_patented')).toBeInTheDocument();
    expect(screen.getByText('mining.whitespace.legend_candidate')).toBeInTheDocument();
    expect(screen.getByText('mining.whitespace.legend_whitespace')).toBeInTheDocument();
  });
});
