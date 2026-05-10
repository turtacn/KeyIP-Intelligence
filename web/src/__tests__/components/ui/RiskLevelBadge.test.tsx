import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import RiskLevelBadge from '@/components/ui/RiskLevelBadge';

describe('RiskLevelBadge', () => {
  it('renders HIGH risk level', () => {
    render(<RiskLevelBadge level="HIGH" />);
    expect(screen.getByText('HIGH RISK')).toBeInTheDocument();
  });

  it('renders MEDIUM risk level', () => {
    render(<RiskLevelBadge level="MEDIUM" />);
    expect(screen.getByText('MEDIUM RISK')).toBeInTheDocument();
  });

  it('renders LOW risk level', () => {
    render(<RiskLevelBadge level="LOW" />);
    expect(screen.getByText('LOW RISK')).toBeInTheDocument();
  });

  it('renders NONE risk level', () => {
    render(<RiskLevelBadge level="NONE" />);
    expect(screen.getByText('NONE RISK')).toBeInTheDocument();
  });

  it('applies additional className', () => {
    render(<RiskLevelBadge level="HIGH" className="my-custom-class" />);
    const badge = screen.getByText('HIGH RISK');
    expect(badge.className).toContain('my-custom-class');
  });
});
