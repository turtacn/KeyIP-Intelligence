import React, { useState, useCallback, useRef, useEffect, useMemo } from 'react';
import {
  ChevronLeft,
  ChevronRight,
  ChevronsUpDown,
  ChevronUp,
  ChevronDown,
  Download,
  Columns3,
  X,
  Search,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';

// ─── Types ───────────────────────────────────────────────────────────────────

export type SortDirection = 'asc' | 'desc' | null;

export interface SortState {
  column: string;
  direction: SortDirection;
}

export interface Column<T> {
  /** Display text in the header cell */
  header: string;
  /** Key of T or a render function */
  accessor: keyof T | ((row: T) => React.ReactNode);
  /** Optional class name(s) for the column cells */
  className?: string;
  /** Whether this column can be sorted. Default false. */
  sortable?: boolean;
  /** Key used for sorting. Defaults to the string version of `accessor` when accessor is a keyof T. */
  sortKey?: string;
  /** Whether this column is visible. Default true. */
  visible?: boolean;
  /** Optional ID for the column (used for visibility management). Defaults to header string. */
  id?: string;
  /** Optional width for the column */
  width?: string | number;
  /** Whether the column should stick to the left when horizontally scrolling */
  sticky?: 'left' | 'right';
}

interface PaginationConfig {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}

interface SelectionConfig<T> {
  selectedIds: Set<string>;
  onSelectionChange: (ids: Set<string>) => void;
  getRowId: (row: T) => string;
}

interface DataTableProps<T> {
  /** Array of data rows */
  data: T[];
  /** Column definitions */
  columns: Column<T>[];
  /** Loading state */
  isLoading?: boolean;
  /** Pagination configuration */
  pagination?: PaginationConfig;
  /** Click handler for a row */
  onRowClick?: (row: T) => void;
  /** Whether to show the toolbar (search, column toggle, export). Default false. */
  showToolbar?: boolean;
  /** Placeholder text for the search input in toolbar */
  searchPlaceholder?: string;
  /** Called when the search query changes */
  onSearch?: (query: string) => void;
  /** External sort state */
  sortState?: SortState;
  /** Called when sort changes */
  onSort?: (sort: SortState) => void;
  /** Enable column visibility toggle in toolbar */
  enableColumnVisibility?: boolean;
  /** Enable CSV export in toolbar */
  enableExport?: boolean;
  /** Make header sticky. Default false. */
  stickyHeader?: boolean;
  /** Enable row selection with checkboxes */
  selectable?: boolean;
  /** Selection configuration (required when selectable is true) */
  selection?: SelectionConfig<T>;
  /** Text to show when data is empty */
  emptyText?: string;
  /** Text to show when loading */
  loadingText?: string;
  /** Additional class name for the wrapper */
  className?: string;
}

// ─── Utility: CSV export ────────────────────────────────────────────────────

export function exportToCsv<T>(
  data: T[],
  columns: Column<T>[],
  filename = 'export.csv'
): void {
  const visibleColumns = columns.filter((c) => c.visible !== false);
  const headerRow = visibleColumns.map((c) => `"${c.header.replace(/"/g, '""')}"`).join(',');

  const bodyRows = data.map((row) =>
    visibleColumns
      .map((col) => {
        let value: unknown;
        if (typeof col.accessor === 'function') {
          // Render the cell to extract text — this is best-effort for custom renderers
          const rendered = col.accessor(row);
          if (typeof rendered === 'string') return rendered;
          if (rendered === null || rendered === undefined) return '';
          // Use a basic heuristic: extract text from React elements
          if (React.isValidElement(rendered)) {
            const props = rendered.props as Record<string, unknown>;
            // Attempt to get meaningful text from known patterns
            const text =
              typeof props.children === 'string'
                ? props.children
                : typeof props.label === 'string'
                  ? props.label
                  : typeof props.value === 'string' || typeof props.value === 'number'
                    ? String(props.value)
                    : '';
            return text;
          }
          return String(rendered);
        }
        value = row[col.accessor as keyof T];
        if (value === null || value === undefined) return '';
        return String(value);
      })
      .map((v) => `"${String(v).replace(/"/g, '""')}"`)
      .join(',')
  );

  const csv = [headerRow, ...bodyRows].join('\n');
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

// ─── Sort Icon ──────────────────────────────────────────────────────────────

const SortIcon: React.FC<{ direction: SortDirection; active: boolean }> = ({
  direction,
  active,
}) => {
  if (!active) {
    return <ChevronsUpDown className="w-3.5 h-3.5 text-slate-300 group-hover:text-slate-400 flex-shrink-0" aria-hidden="true" />;
  }
  if (direction === 'asc') {
    return <ChevronUp className="w-3.5 h-3.5 text-blue-600 flex-shrink-0" aria-hidden="true" />;
  }
  return <ChevronDown className="w-3.5 h-3.5 text-blue-600 flex-shrink-0" aria-hidden="true" />;
};

// ─── Column Visibility Dropdown ─────────────────────────────────────────────

interface ColumnToggleDropdownProps<T> {
  columns: Column<T>[];
  visibleColumns: Set<string>;
  onToggle: (id: string) => void;
  onClose: () => void;
}

function ColumnToggleDropdown<T>({
  columns,
  visibleColumns,
  onToggle,
  onClose,
}: ColumnToggleDropdownProps<T>) {
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [onClose]);

  // Focus management: focus first checkbox on open
  useEffect(() => {
    const firstInput = dropdownRef.current?.querySelector('input');
    firstInput?.focus();
  }, []);

  return (
    <div
      ref={dropdownRef}
      className="absolute right-0 top-full mt-1 w-56 bg-white border border-slate-200 rounded-lg shadow-lg z-50 py-1"
      role="menu"
      aria-label="Column visibility"
    >
      <div className="px-3 py-2 text-xs font-semibold text-slate-400 uppercase tracking-wider border-b border-slate-100">
        Toggle Columns
      </div>
      {columns.map((col) => {
        const colId = col.id || col.header;
        const isVisible = visibleColumns.has(colId);
        return (
          <label
            key={colId}
            className="flex items-center px-3 py-1.5 hover:bg-slate-50 cursor-pointer text-sm text-slate-700"
            role="menuitemcheckbox"
            aria-checked={isVisible}
          >
            <input
              type="checkbox"
              checked={isVisible}
              onChange={() => onToggle(colId)}
              className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-slate-300 rounded mr-2"
            />
            <span className="truncate">{col.header}</span>
          </label>
        );
      })}
    </div>
  );
}

// ─── Toolbar ─────────────────────────────────────────────────────────────────

interface ToolbarProps<T> {
  columns: Column<T>[];
  visibleColumns: Set<string>;
  onToggleColumn: (id: string) => void;
  enableColumnVisibility: boolean;
  enableExport: boolean;
  data: T[];
  columnsDef: Column<T>[];
  searchQuery: string;
  onSearch?: (query: string) => void;
  searchPlaceholder?: string;
}

function ToolbarInner<T>({
  columns,
  visibleColumns,
  onToggleColumn,
  enableColumnVisibility,
  enableExport,
  data,
  columnsDef,
  searchQuery,
  onSearch,
  searchPlaceholder,
}: ToolbarProps<T>) {
  const { t } = useTranslation();
  const [showColumnToggle, setShowColumnToggle] = useState(false);

  const handleExport = useCallback(() => {
    exportToCsv(data, columnsDef, `table-export-${Date.now()}.csv`);
  }, [data, columnsDef]);

  const visibleCount = columns.filter((c) => visibleColumns.has(c.id || c.header)).length;
  const totalCount = columns.length;

  return (
    <div className="px-4 py-2.5 border-b border-slate-200 bg-slate-50/80 flex flex-wrap items-center gap-2">
      {/* Search */}
      {onSearch && (
        <div className="relative flex-1 min-w-[180px] max-w-xs">
          <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-slate-400 pointer-events-none" aria-hidden="true" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => onSearch(e.target.value)}
            placeholder={searchPlaceholder || t('table.search_placeholder', 'Search...')}
            className="w-full pl-8 pr-7 py-1.5 text-xs border border-slate-300 rounded-md focus:ring-blue-500 focus:border-blue-500 bg-white"
            aria-label={t('table.search_label', 'Search table')}
          />
          {searchQuery && (
            <button
              onClick={() => onSearch('')}
              className="absolute right-1.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600"
              aria-label={t('table.clear_search', 'Clear search')}
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      )}

      <div className="flex-1" />

      {/* Column visibility toggle */}
      {enableColumnVisibility && (
        <div className="relative">
          <button
            onClick={() => setShowColumnToggle((prev) => !prev)}
            className="inline-flex items-center px-2.5 py-1.5 text-xs font-medium text-slate-600 bg-white border border-slate-300 rounded-md hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
            aria-label={t('table.toggle_columns', 'Toggle columns')}
            aria-expanded={showColumnToggle}
          >
            <Columns3 className="w-3.5 h-3.5 mr-1.5" aria-hidden="true" />
            {t('table.columns', 'Columns')}
            <span className="ml-1.5 text-slate-400 text-xs">
              ({visibleCount}/{totalCount})
            </span>
          </button>
          {showColumnToggle && (
            <ColumnToggleDropdown
              columns={columns}
              visibleColumns={visibleColumns}
              onToggle={onToggleColumn}
              onClose={() => setShowColumnToggle(false)}
            />
          )}
        </div>
      )}

      {/* Export button */}
      {enableExport && (
        <button
          onClick={handleExport}
          className="inline-flex items-center px-2.5 py-1.5 text-xs font-medium text-slate-600 bg-white border border-slate-300 rounded-md hover:bg-slate-50 focus:outline-none focus:ring-2 focus:ring-blue-500"
          aria-label={t('table.export_csv', 'Export to CSV')}
        >
          <Download className="w-3.5 h-3.5 mr-1.5" aria-hidden="true" />
          {t('table.export', 'Export')}
        </button>
      )}
    </div>
  );
}

const Toolbar = React.memo(ToolbarInner) as typeof ToolbarInner;

// ─── Main DataTable Component ───────────────────────────────────────────────

const DataTable = <T,>({
  data,
  columns: columnsProp,
  isLoading,
  pagination,
  onRowClick,
  showToolbar = false,
  searchPlaceholder,
  onSearch,
  sortState: externalSortState,
  onSort: externalOnSort,
  enableColumnVisibility = false,
  enableExport = false,
  stickyHeader = false,
  selectable = false,
  selection,
  emptyText,
  loadingText,
  className = '',
}: DataTableProps<T>) => {
  const { t } = useTranslation();

  // ── Internal state for column visibility ──
  const allColumnIds = useMemo(
    () => columnsProp.map((col) => col.id || col.header),
    [columnsProp]
  );
  const [visibleColumnIds, setVisibleColumnIds] = useState<Set<string>>(
    () => new Set(allColumnIds.filter((_, i) => columnsProp[i].visible !== false))
  );

  // Sync visible columns when columnsProp changes
  useEffect(() => {
    setVisibleColumnIds((prev) => {
      const next = new Set<string>();
      for (const col of columnsProp) {
        const id = col.id || col.header;
        // If explicitly hidden, don't show
        if (col.visible === false) continue;
        // If previously visible or not explicitly hidden, keep visible
        if (prev.has(id) || col.visible === true || col.visible === undefined) {
          next.add(id);
        }
      }
      return next;
    });
  }, [columnsProp]);

  // ── Internal state for search ──
  const [internalSearchQuery, setInternalSearchQuery] = useState('');

  const searchQuery = onSearch ? internalSearchQuery : '';
  const handleSearchChange = useCallback(
    (query: string) => {
      setInternalSearchQuery(query);
      onSearch?.(query);
    },
    [onSearch]
  );

  // ── Column visibility handlers ──
  const handleToggleColumn = useCallback((id: string) => {
    setVisibleColumnIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }, []);

  // ── Sort handling ──
  const handleSort = useCallback(
    (column: Column<T>) => {
      const sortKey = column.sortKey || (typeof column.accessor === 'string' ? column.accessor : column.header);
      if (!column.sortable) return;

      if (externalOnSort && externalSortState) {
        // External sort management
        if (externalSortState.column !== sortKey) {
          externalOnSort({ column: sortKey, direction: 'asc' });
        } else if (externalSortState.direction === 'asc') {
          externalOnSort({ column: sortKey, direction: 'desc' });
        } else {
          externalOnSort({ column: '', direction: null });
        }
      }
    },
    [externalSortState, externalOnSort]
  );

  // ── Selection handling ──
  const isAllSelected = useMemo(() => {
    if (!selectable || !selection) return false;
    return data.length > 0 && data.every((row) => selection.selectedIds.has(selection.getRowId(row)));
  }, [selectable, selection, data]);

  const isSomeSelected = useMemo(() => {
    if (!selectable || !selection) return false;
    return data.some((row) => selection.selectedIds.has(selection.getRowId(row))) && !isAllSelected;
  }, [selectable, selection, data, isAllSelected]);

  const handleSelectAll = useCallback(() => {
    if (!selection) return;
    if (isAllSelected) {
      selection.onSelectionChange(new Set());
    } else {
      const allIds = data.map((row) => selection.getRowId(row));
      selection.onSelectionChange(new Set(allIds));
    }
  }, [selection, data, isAllSelected]);

  const handleSelectRow = useCallback(
    (id: string) => {
      if (!selection) return;
      const next = new Set(selection.selectedIds);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      selection.onSelectionChange(next);
    },
    [selection]
  );

  // ── Filter visible columns ──
  const visibleColumns = useMemo(
    () => columnsProp.filter((col) => visibleColumnIds.has(col.id || col.header)),
    [columnsProp, visibleColumnIds]
  );

  // ── Sticky header classes ──
  const headerClass = stickyHeader
    ? 'sticky top-0 z-10'
    : '';

  // ── Render ──
  return (
    <div className={`overflow-hidden bg-white border border-slate-200 rounded-lg shadow-sm ${className}`}>
      {/* Toolbar */}
      {showToolbar && (
        <Toolbar
          columns={columnsProp}
          visibleColumns={visibleColumnIds}
          onToggleColumn={handleToggleColumn}
          enableColumnVisibility={enableColumnVisibility}
          enableExport={enableExport}
          data={data}
          columnsDef={columnsProp}
          searchQuery={searchQuery}
          onSearch={onSearch ? handleSearchChange : undefined}
          searchPlaceholder={searchPlaceholder}
        />
      )}

      {/* Table */}
      <div className="overflow-x-auto overflow-y-auto max-h-full">
        <table className="min-w-full divide-y divide-slate-200">
          <thead className={`bg-slate-50 ${headerClass}`}>
            <tr>
              {/* Select all checkbox */}
              {selectable && selection && (
                <th scope="col" className="w-10 px-3 py-3 text-left">
                  <input
                    type="checkbox"
                    checked={isAllSelected}
                    ref={(el) => {
                      if (el) el.indeterminate = isSomeSelected;
                    }}
                    onChange={handleSelectAll}
                    className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-slate-300 rounded cursor-pointer"
                    aria-label={t('table.select_all', 'Select all rows')}
                  />
                </th>
              )}

              {visibleColumns.map((col) => {
                const sortKey = col.sortKey || (typeof col.accessor === 'string' ? col.accessor : col.header);
                const isSorted = externalSortState
                  ? externalSortState.column === sortKey
                  : false;
                const isSortable = col.sortable === true;

                return (
                  <th
                    key={col.id || col.header}
                    scope="col"
                    className={`px-6 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider ${
                      isSortable ? 'cursor-pointer select-none group' : ''
                    } ${col.className || ''}`}
                    style={{
                      ...(col.width ? { width: col.width } : {}),
                      ...(col.sticky === 'left' ? { position: 'sticky', left: 0, zIndex: 11, backgroundColor: 'inherit' } : {}),
                      ...(col.sticky === 'right' ? { position: 'sticky', right: 0, zIndex: 11, backgroundColor: 'inherit' } : {}),
                    }}
                    onClick={() => isSortable && handleSort(col)}
                    onKeyDown={(e) => {
                      if (isSortable && (e.key === 'Enter' || e.key === ' ')) {
                        e.preventDefault();
                        handleSort(col);
                      }
                    }}
                    tabIndex={isSortable ? 0 : undefined}
                    aria-sort={
                      isSorted && externalSortState?.direction === 'asc'
                        ? 'ascending'
                        : isSorted && externalSortState?.direction === 'desc'
                          ? 'descending'
                          : undefined
                    }
                    role={isSortable ? 'columnheader button' : 'columnheader'}
                  >
                    <div className="flex items-center gap-1.5">
                      <span>{col.header}</span>
                      {isSortable && (
                        <SortIcon
                          direction={isSorted ? externalSortState!.direction : null}
                          active={isSorted}
                        />
                      )}
                    </div>
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-slate-200">
            {isLoading ? (
              <tr>
                <td colSpan={visibleColumns.length + (selectable ? 1 : 0)} className="px-6 py-4 text-center">
                  <div className="flex justify-center items-center h-24">
                    <div className="flex flex-col items-center gap-2">
                      <div className="w-6 h-6 border-2 border-blue-600 border-t-transparent rounded-full animate-spin" />
                      <span className="text-slate-400 text-sm">
                        {loadingText || t('table.loading_data', 'Loading data...')}
                      </span>
                    </div>
                  </div>
                </td>
              </tr>
            ) : data.length === 0 ? (
              <tr>
                <td colSpan={visibleColumns.length + (selectable ? 1 : 0)} className="px-6 py-8 text-center text-slate-500">
                  <div className="flex flex-col items-center gap-1">
                    <span className="text-sm">
                      {emptyText || t('table.no_data', 'No data available')}
                    </span>
                  </div>
                </td>
              </tr>
            ) : (
              data.map((row, rowIndex) => {
                const rowId = selection ? selection.getRowId(row) : String(rowIndex);
                const isSelected = selection ? selection.selectedIds.has(rowId) : false;

                return (
                  <tr
                    key={rowId}
                    className={[
                      'transition-colors',
                      onRowClick ? 'cursor-pointer hover:bg-slate-50' : '',
                      isSelected ? 'bg-blue-50 hover:bg-blue-100' : '',
                    ].join(' ')}
                    onClick={() => onRowClick && onRowClick(row)}
                    aria-selected={selectable ? isSelected : undefined}
                  >
                    {/* Selection checkbox */}
                    {selectable && selection && (
                      <td className="px-3 py-4 whitespace-nowrap text-sm text-slate-700 w-10">
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => handleSelectRow(rowId)}
                          onClick={(e) => e.stopPropagation()}
                          className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-slate-300 rounded cursor-pointer"
                          aria-label={t('table.select_row', 'Select row')}
                        />
                      </td>
                    )}

                    {visibleColumns.map((col) => (
                      <td
                        key={col.id || col.header}
                        className="px-6 py-4 whitespace-nowrap text-sm text-slate-700"
                        style={{
                          ...(col.sticky === 'left' ? { position: 'sticky', left: 0, zIndex: 1, backgroundColor: 'inherit' } : {}),
                          ...(col.sticky === 'right' ? { position: 'sticky', right: 0, zIndex: 1, backgroundColor: 'inherit' } : {}),
                        }}
                      >
                        {typeof col.accessor === 'function'
                          ? col.accessor(row)
                          : (row[col.accessor as keyof T] as React.ReactNode)}
                      </td>
                    ))}
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {pagination && (
        <div className="bg-white px-4 py-3 flex items-center justify-between border-t border-slate-200 sm:px-6">
          <div className="flex-1 flex justify-between sm:hidden">
            <button
              onClick={() => pagination.onPageChange(pagination.currentPage - 1)}
              disabled={pagination.currentPage === 1}
              className="relative inline-flex items-center px-4 py-2 border border-slate-300 text-sm font-medium rounded-md text-slate-700 bg-white hover:bg-slate-50 disabled:opacity-50"
              aria-label={t('pagination.previous', 'Previous page')}
            >
              {t('pagination.previous_short', 'Previous')}
            </button>
            <button
              onClick={() => pagination.onPageChange(pagination.currentPage + 1)}
              disabled={pagination.currentPage === pagination.totalPages}
              className="ml-3 relative inline-flex items-center px-4 py-2 border border-slate-300 text-sm font-medium rounded-md text-slate-700 bg-white hover:bg-slate-50 disabled:opacity-50"
              aria-label={t('pagination.next', 'Next page')}
            >
              {t('pagination.next_short', 'Next')}
            </button>
          </div>
          <div className="hidden sm:flex-1 sm:flex sm:items-center sm:justify-between">
            <div>
              <p className="text-sm text-slate-700">
                {t('pagination.showing', 'Page')}{' '}
                <span className="font-medium">{pagination.currentPage}</span>{' '}
                {t('pagination.of', 'of')}{' '}
                <span className="font-medium">{pagination.totalPages}</span>
              </p>
            </div>
            <div>
              <nav className="relative z-0 inline-flex rounded-md shadow-sm -space-x-px" aria-label={t('pagination.nav_label', 'Pagination')}>
                <button
                  onClick={() => pagination.onPageChange(pagination.currentPage - 1)}
                  disabled={pagination.currentPage === 1}
                  className="relative inline-flex items-center px-2 py-2 rounded-l-md border border-slate-300 bg-white text-sm font-medium text-slate-500 hover:bg-slate-50 disabled:opacity-50"
                  aria-label={t('pagination.go_previous', 'Go to previous page')}
                >
                  <span className="sr-only">{t('pagination.previous', 'Previous')}</span>
                  <ChevronLeft className="h-5 w-5" aria-hidden="true" />
                </button>
                <button
                  onClick={() => pagination.onPageChange(pagination.currentPage + 1)}
                  disabled={pagination.currentPage === pagination.totalPages}
                  className="relative inline-flex items-center px-2 py-2 rounded-r-md border border-slate-300 bg-white text-sm font-medium text-slate-500 hover:bg-slate-50 disabled:opacity-50"
                  aria-label={t('pagination.go_next', 'Go to next page')}
                >
                  <span className="sr-only">{t('pagination.next', 'Next')}</span>
                  <ChevronRight className="h-5 w-5" aria-hidden="true" />
                </button>
              </nav>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default DataTable;
