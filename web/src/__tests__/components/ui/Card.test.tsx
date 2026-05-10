import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import Card from '@/components/ui/Card';

describe('Card', () => {
  it('renders children', () => {
    render(<Card>Content</Card>);
    expect(screen.getByText('Content')).toBeInTheDocument();
  });

  it('renders header when provided', () => {
    render(<Card header="Card Header">Content</Card>);
    expect(screen.getByText('Card Header')).toBeInTheDocument();
  });

  it('does not render header when not provided', () => {
    render(<Card>Content</Card>);
    expect(screen.queryByText('Card Header')).not.toBeInTheDocument();
  });

  it('renders footer when provided', () => {
    render(<Card footer="Card Footer">Content</Card>);
    expect(screen.getByText('Card Footer')).toBeInTheDocument();
  });

  it('does not render footer when not provided', () => {
    render(<Card>Content</Card>);
    expect(screen.queryByText('Card Footer')).not.toBeInTheDocument();
  });

  it('applies padding styles based on padding prop', () => {
    const { rerender } = render(<Card padding="none">Content</Card>);
    // With padding="none", the inner div should have p-0
    expect(screen.getByText('Content').className).toContain('p-0');

    rerender(<Card padding="sm">Content</Card>);
    expect(screen.getByText('Content').className).toContain('p-4');

    rerender(<Card padding="md">Content</Card>);
    expect(screen.getByText('Content').className).toContain('p-6');

    rerender(<Card padding="lg">Content</Card>);
    expect(screen.getByText('Content').className).toContain('p-8');
  });

  it('applies additional className', () => {
    render(<Card className="extra-card-class">Content</Card>);
    const container = screen.getByText('Content').closest('div.bg-white');
    expect(container?.className).toContain('extra-card-class');
  });
});
