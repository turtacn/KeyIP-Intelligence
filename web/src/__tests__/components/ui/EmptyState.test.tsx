import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Inbox } from 'lucide-react';
import EmptyState from '@/components/ui/EmptyState';

describe('EmptyState', () => {
  it('renders title and description', () => {
    render(
      <EmptyState
        icon={Inbox}
        title="No items found"
        description="There are no items to display."
      />
    );
    expect(screen.getByText('No items found')).toBeInTheDocument();
    expect(screen.getByText('There are no items to display.')).toBeInTheDocument();
  });

  it('renders the icon', () => {
    render(
      <EmptyState
        icon={Inbox}
        title="Empty"
        description="No data"
      />
    );
    // The icon should render as an SVG element
    const iconContainer = screen.getByText('Empty').closest('div');
    expect(iconContainer?.querySelector('svg')).toBeInTheDocument();
  });

  it('renders action element when provided', () => {
    render(
      <EmptyState
        icon={Inbox}
        title="Empty"
        description="No data"
        action={<button>Add Item</button>}
      />
    );
    expect(screen.getByRole('button', { name: /add item/i })).toBeInTheDocument();
  });

  it('does not render action when not provided', () => {
    render(
      <EmptyState
        icon={Inbox}
        title="Empty"
        description="No data"
      />
    );
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
