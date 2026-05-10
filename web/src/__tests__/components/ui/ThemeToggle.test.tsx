import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { ThemeProvider } from '@/hooks/useTheme';
import ThemeToggle from '@/components/ui/ThemeToggle';

describe('ThemeToggle', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('renders in light mode by default', () => {
    render(
      <ThemeProvider>
        <ThemeToggle />
      </ThemeProvider>
    );
    const button = screen.getByRole('button', { name: /toggle theme/i });
    expect(button).toBeInTheDocument();
    // Light mode shows "Switch to dark mode" title
    expect(button).toHaveAttribute('title', 'Switch to dark mode');
  });

  it('toggles to dark mode on click', () => {
    render(
      <ThemeProvider>
        <ThemeToggle />
      </ThemeProvider>
    );
    const button = screen.getByRole('button', { name: /toggle theme/i });

    fireEvent.click(button);

    // After clicking, title should change to "Switch to light mode"
    expect(button).toHaveAttribute('title', 'Switch to light mode');
  });

  it('persists theme preference to localStorage', () => {
    render(
      <ThemeProvider>
        <ThemeToggle />
      </ThemeProvider>
    );

    // Initial theme is light
    expect(localStorage.getItem('keyip-theme')).toBe('light');

    const button = screen.getByRole('button', { name: /toggle theme/i });
    fireEvent.click(button);

    // After toggle, theme should be dark
    expect(localStorage.getItem('keyip-theme')).toBe('dark');
  });

  it('toggles back to light mode on double click', () => {
    render(
      <ThemeProvider>
        <ThemeToggle />
      </ThemeProvider>
    );
    const button = screen.getByRole('button', { name: /toggle theme/i });

    fireEvent.click(button);
    expect(button).toHaveAttribute('title', 'Switch to light mode');

    fireEvent.click(button);
    expect(button).toHaveAttribute('title', 'Switch to dark mode');
  });
});
