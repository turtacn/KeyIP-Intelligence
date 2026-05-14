import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import DataTable, { Column } from '@/components/ui/DataTable';

interface TestItem {
  id: string;
  name: string;
}

describe('DataTable', () => {
  const columns: Column<TestItem>[] = [
    { header: 'ID', accessor: 'id' },
    { header: 'Name', accessor: 'name' },
  ];

  const data: TestItem[] = [
    { id: '1', name: 'Alpha' },
    { id: '2', name: 'Beta' },
    { id: '3', name: 'Gamma' },
  ];

  it('renders column headers', () => {
    render(<DataTable data={data} columns={columns} />);
    expect(screen.getByText('ID')).toBeInTheDocument();
    expect(screen.getByText('Name')).toBeInTheDocument();
  });

  it('renders data rows', () => {
    render(<DataTable data={data} columns={columns} />);
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(screen.getByText('Beta')).toBeInTheDocument();
    expect(screen.getByText('Gamma')).toBeInTheDocument();
  });

  it('shows loading state when isLoading is true', () => {
    render(<DataTable data={data} columns={columns} isLoading={true} />);
    expect(screen.getByText('Loading data...')).toBeInTheDocument();
    expect(screen.queryByText('Alpha')).not.toBeInTheDocument();
  });

  it('shows empty state when no data', () => {
    render(<DataTable data={[]} columns={columns} />);
    // DataTable uses t('table.no_data', 'No data available') — fallback to default
    expect(screen.getByText('No data available')).toBeInTheDocument();
  });

  it('handles function accessor for columns', () => {
    const actionCol: Column<TestItem> = {
      header: 'Actions',
      accessor: (row) => <button>Edit {row.id}</button>,
    };
    render(<DataTable data={data} columns={[...columns, actionCol]} />);
    expect(screen.getByText('Edit 1')).toBeInTheDocument();
    expect(screen.getByText('Edit 2')).toBeInTheDocument();
    expect(screen.getByText('Edit 3')).toBeInTheDocument();
  });

  it('renders pagination when provided', () => {
    const onPageChange = vi.fn();
    render(
      <DataTable
        data={data}
        columns={columns}
        pagination={{ currentPage: 1, totalPages: 5, onPageChange }}
      />
    );
    // Shows current page and total pages (t() fallback to 'Page 1 of 5')
    expect(screen.getByText(/^Page/)).toBeInTheDocument();
    // Both Next buttons exist (mobile + desktop views)
    const nextButtons = screen.getAllByText('Next');
    expect(nextButtons.length).toBeGreaterThanOrEqual(1);
  });

  it('calls onPageChange when next is clicked', () => {
    const onPageChange = vi.fn();
    render(
      <DataTable
        data={data}
        columns={columns}
        pagination={{ currentPage: 1, totalPages: 5, onPageChange }}
      />
    );
    const nextButtons = screen.getAllByText('Next');
    fireEvent.click(nextButtons[0]);
    expect(onPageChange).toHaveBeenCalledWith(2);
  });

  it('calls onPageChange when previous is clicked', () => {
    const onPageChange = vi.fn();
    render(
      <DataTable
        data={data}
        columns={columns}
        pagination={{ currentPage: 3, totalPages: 5, onPageChange }}
      />
    );
    const prevButtons = screen.getAllByText('Previous');
    fireEvent.click(prevButtons[0]);
    expect(onPageChange).toHaveBeenCalledWith(2);
  });

  it('disables previous button on first page', () => {
    const onPageChange = vi.fn();
    render(
      <DataTable
        data={data}
        columns={columns}
        pagination={{ currentPage: 1, totalPages: 5, onPageChange }}
      />
    );
    const prevButtons = screen.getAllByText('Previous');
    expect(prevButtons[0]).toBeDisabled();
  });

  it('disables next button on last page', () => {
    const onPageChange = vi.fn();
    render(
      <DataTable
        data={data}
        columns={columns}
        pagination={{ currentPage: 5, totalPages: 5, onPageChange }}
      />
    );
    const nextButtons = screen.getAllByText('Next');
    expect(nextButtons[0]).toBeDisabled();
  });
});
