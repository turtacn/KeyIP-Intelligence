import { useState, useCallback, useMemo } from 'react';

export type SortDirection = 'asc' | 'desc' | null;

export interface SortState {
  column: string;
  direction: SortDirection;
}

export interface FilterState {
  [key: string]: string;
}

export interface TableState {
  sort: SortState;
  filters: FilterState;
  currentPage: number;
  pageSize: number;
  searchQuery: string;
}

export interface UseTableStateReturn {
  sort: SortState;
  filters: FilterState;
  currentPage: number;
  pageSize: number;
  searchQuery: string;
  setSort: (column: string) => void;
  setSortState: (sort: SortState) => void;
  setFilter: (key: string, value: string) => void;
  clearFilters: () => void;
  setCurrentPage: (page: number) => void;
  setPageSize: (size: number) => void;
  setSearchQuery: (query: string) => void;
  resetState: () => void;
}

const DEFAULT_PAGE_SIZE = 20;

export function useTableState(initial?: Partial<TableState>): UseTableStateReturn {
  const [sort, setSortState] = useState<SortState>(
    initial?.sort ?? { column: '', direction: null }
  );
  const [filters, setFilters] = useState<FilterState>(initial?.filters ?? {});
  const [currentPage, setCurrentPage] = useState(initial?.currentPage ?? 1);
  const [pageSize, setPageSize] = useState(initial?.pageSize ?? DEFAULT_PAGE_SIZE);
  const [searchQuery, setSearchQuery] = useState(initial?.searchQuery ?? '');

  const setSort = useCallback((column: string) => {
    setSortState((prev) => {
      if (prev.column !== column) {
        return { column, direction: 'asc' };
      }
      if (prev.direction === 'asc') {
        return { column, direction: 'desc' };
      }
      if (prev.direction === 'desc') {
        return { column: '', direction: null };
      }
      return { column, direction: 'asc' };
    });
    setCurrentPage(1);
  }, []);

  const setFilter = useCallback((key: string, value: string) => {
    setFilters((prev) => {
      if (value === '' || value === undefined || value === null) {
        const next = { ...prev };
        delete next[key];
        return next;
      }
      return { ...prev, [key]: value };
    });
    setCurrentPage(1);
  }, []);

  const setSortStateExplicit = useCallback((sort: SortState) => {
    setSortState(sort);
    setCurrentPage(1);
  }, []);

  const clearFilters = useCallback(() => {
    setFilters({});
    setCurrentPage(1);
  }, []);

  const resetState = useCallback(() => {
    setSortState({ column: '', direction: null });
    setFilters({});
    setCurrentPage(1);
    setPageSize(DEFAULT_PAGE_SIZE);
    setSearchQuery('');
  }, []);

  return useMemo(
    () => ({
      sort,
      filters,
      currentPage,
      pageSize,
      searchQuery,
      setSort,
      setSortState: setSortStateExplicit,
      setFilter,
      clearFilters,
      setCurrentPage,
      setPageSize,
      setSearchQuery,
      resetState,
    }),
    [
      sort,
      filters,
      currentPage,
      pageSize,
      searchQuery,
      setSort,
      setSortStateExplicit,
      setFilter,
      clearFilters,
      setCurrentPage,
      setPageSize,
      setSearchQuery,
      resetState,
    ]
  );
}
