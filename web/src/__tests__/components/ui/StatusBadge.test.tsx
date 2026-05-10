import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import StatusBadge from '@/components/ui/StatusBadge';

describe('StatusBadge', () => {
  it('renders with active status', () => {
    render(<StatusBadge status="active" />);
    const badge = screen.getByText('active');
    expect(badge).toBeInTheDocument();
    expect(badge.className).toContain('capitalize');
  });

  it('renders with pending status', () => {
    render(<StatusBadge status="pending" />);
    expect(screen.getByText('pending')).toBeInTheDocument();
  });

  it('renders with completed status', () => {
    render(<StatusBadge status="completed" />);
    expect(screen.getByText('completed')).toBeInTheDocument();
  });

  it('renders with error status', () => {
    render(<StatusBadge status="error" />);
    expect(screen.getByText('error')).toBeInTheDocument();
  });

  it('renders with inactive status', () => {
    render(<StatusBadge status="inactive" />);
    expect(screen.getByText('inactive')).toBeInTheDocument();
  });

  it('renders custom label instead of status', () => {
    render(<StatusBadge status="active" label="Custom Label" />);
    expect(screen.getByText('Custom Label')).toBeInTheDocument();
    expect(screen.queryByText('active')).not.toBeInTheDocument();
  });
});
