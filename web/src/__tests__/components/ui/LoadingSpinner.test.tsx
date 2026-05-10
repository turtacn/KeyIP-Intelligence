import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import LoadingSpinner from '@/components/ui/LoadingSpinner';

describe('LoadingSpinner', () => {
  it('renders with aria-label', () => {
    render(<LoadingSpinner />);
    expect(screen.getByLabelText('Loading')).toBeInTheDocument();
  });

  it('renders with default md size', () => {
    render(<LoadingSpinner />);
    const spinner = screen.getByLabelText('Loading');
    const classStr = spinner.getAttribute('class') || '';
    expect(classStr).toContain('w-6');
    expect(classStr).toContain('h-6');
  });

  it('renders with sm size', () => {
    render(<LoadingSpinner size="sm" />);
    const spinner = screen.getByLabelText('Loading');
    const classStr = spinner.getAttribute('class') || '';
    expect(classStr).toContain('w-4');
    expect(classStr).toContain('h-4');
  });

  it('renders with lg size', () => {
    render(<LoadingSpinner size="lg" />);
    const spinner = screen.getByLabelText('Loading');
    const classStr = spinner.getAttribute('class') || '';
    expect(classStr).toContain('w-10');
    expect(classStr).toContain('h-10');
  });

  it('applies additional className', () => {
    render(<LoadingSpinner className="extra-spinner-class" />);
    const spinner = screen.getByLabelText('Loading');
    const classStr = spinner.getAttribute('class') || '';
    expect(classStr).toContain('extra-spinner-class');
  });
});
