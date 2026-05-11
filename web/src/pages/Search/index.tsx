import React, { useState, useCallback, useRef, useEffect, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import Card from '../../components/ui/Card';
import DataTable, { Column } from '../../components/ui/DataTable';
import LoadingSpinner from '../../components/ui/LoadingSpinner';
import Badge from '../../components/ui/Badge';
import Button from '../../components/ui/Button';
import { Search as SearchIcon, FileText, Beaker, AlertCircle, Clock, X } from 'lucide-react';
import { patentService } from '../../services/patent.service';
import { moleculeService } from '../../services/molecule.service';
import { Patent, Molecule, Jurisdiction } from '../../types/domain';

// ─── Search History Helpers ──────────────────────────────────────────────────
const SEARCH_HISTORY_KEY = 'keyip_search_history';
const MAX_HISTORY = 10;

function getSearchHistory(): string[] {
  try {
    const stored = localStorage.getItem(SEARCH_HISTORY_KEY);
    if (stored) {
      const parsed = JSON.parse(stored);
      if (Array.isArray(parsed)) return parsed;
    }
    return [];
  } catch {
    return [];
  }
}

function addToSearchHistory(query: string): string[] {
  try {
    const history = getSearchHistory();
    const filtered = history.filter((h) => h !== query);
    const updated = [query, ...filtered].slice(0, MAX_HISTORY);
    localStorage.setItem(SEARCH_HISTORY_KEY, JSON.stringify(updated));
    return updated;
  } catch {
    return getSearchHistory();
  }
}

function clearSearchHistory(): void {
  try {
    localStorage.removeItem(SEARCH_HISTORY_KEY);
  } catch {
    // noop
  }
}

// ─── Highlight Utility ──────────────────────────────────────────────────────
function highlightText(text: string, query: string): React.ReactNode {
  if (!query.trim() || !text) return text;
  try {
    const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const regex = new RegExp(`(${escaped})`, 'gi');
    const parts = text.split(regex);
    if (parts.length <= 1) return text;
    return parts.map((part, i) =>
      regex.test(part)
        ? <mark key={i} className="bg-yellow-200/70 text-inherit rounded px-0.5">{part}</mark>
        : part,
    );
  } catch {
    return text;
  }
}

// ─── Types ──────────────────────────────────────────────────────────────────
type SearchScope = 'title' | 'abstract' | 'fulltext' | 'all';

interface PatentResult {
  type: 'patent';
  id: string;
  publicationNumber: string;
  title: string;
  assignee: string;
  legalStatus: string;
  filingDate: string;
}

interface MoleculeResult {
  type: 'molecule';
  id: string;
  name: string;
  smiles: string;
  molecularWeight?: number;
}

type SearchResult = PatentResult | MoleculeResult;

// ─── Page Component ─────────────────────────────────────────────────────────
const Search: React.FC = () => {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [query, setQuery] = useState('');
  const [activeTab, setActiveTab] = useState('all');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [patentResults, setPatentResults] = useState<Patent[]>([]);
  const [moleculeResults, setMoleculeResults] = useState<Molecule[]>([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchHistory, setSearchHistoryState] = useState<string[]>(() => getSearchHistory());
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [searchDuration, setSearchDuration] = useState<number | null>(null);
  // Track the query used for the last search (used for result highlighting)
  const [searchQuery, setSearchQuery] = useState('');

  // ── Filter state ──
  const [searchScope, setSearchScope] = useState<SearchScope>('all');
  const [filingDateFrom, setFilingDateFrom] = useState('');
  const [filingDateTo, setFilingDateTo] = useState('');
  const [pubDateFrom, setPubDateFrom] = useState('');
  const [pubDateTo, setPubDateTo] = useState('');
  const [selectedJurisdictions, setSelectedJurisdictions] = useState<Jurisdiction[]>([]);

  const inputRef = useRef<HTMLInputElement>(null);
  const suggestionsRef = useRef<HTMLDivElement>(null);
  const debounceTimer = useRef<ReturnType<typeof setTimeout>>();
  const lastSearchedQuery = useRef('');
  const lastSearchKey = useRef('');

  // Keep filter values accessible in stable callbacks without re-creating them
  const filtersRef = useRef({ searchScope: 'all' as SearchScope, filingDateFrom: '', filingDateTo: '', pubDateFrom: '', pubDateTo: '', jurisdictions: [] as Jurisdiction[] });
  filtersRef.current = { searchScope, filingDateFrom, filingDateTo, pubDateFrom, pubDateTo, jurisdictions: selectedJurisdictions };

  // ── Tabs ──
  const tabs = useMemo(
    () => [
      { id: 'all', label: t('search.tabs_all') },
      { id: 'patents', label: t('search.tabs_patents') },
      { id: 'molecules', label: t('search.tabs_molecules') },
    ],
    [t],
  );

  // ── Search execution ──
  const performSearch = useCallback(
    async (searchTerm: string) => {
      const trimmed = searchTerm.trim();
      if (!trimmed) return;

      const f = filtersRef.current;
      lastSearchedQuery.current = trimmed;
      lastSearchKey.current = JSON.stringify({ query: trimmed, scope: f.searchScope, filingDateFrom: f.filingDateFrom, filingDateTo: f.filingDateTo, pubDateFrom: f.pubDateFrom, pubDateTo: f.pubDateTo, jurisdictions: f.jurisdictions });
      setLoading(true);
      setError(null);
      setSearched(true);
      setSearchQuery(trimmed);
      setShowSuggestions(false);

      const startTime = performance.now();

      try {
        const [patentResp, moleculeResp] = await Promise.all([
          patentService.getPatents(1, 20, trimmed, {
            scope: f.searchScope,
            filingDateFrom: f.filingDateFrom,
            filingDateTo: f.filingDateTo,
            publicationDateFrom: f.pubDateFrom,
            publicationDateTo: f.pubDateTo,
            jurisdictions: f.jurisdictions,
          }),
          moleculeService.getMolecules(1, 20),
        ]);

        const patents = patentResp.data || [];
        const molecules = moleculeResp.data || [];

        setPatentResults(patents);
        setMoleculeResults(molecules);

        const combined: SearchResult[] = [
          ...patents.map(
            (p: Patent): PatentResult => ({
              type: 'patent' as const,
              id: p.id,
              publicationNumber: p.publicationNumber,
              title: p.title,
              assignee: p.assignee,
              legalStatus: p.legalStatus,
              filingDate: p.filingDate,
            }),
          ),
          ...molecules.map(
            (m: Molecule): MoleculeResult => ({
              type: 'molecule' as const,
              id: m.id,
              name: m.name || m.id,
              smiles: m.smiles,
              molecularWeight: m.molecularWeight,
            }),
          ),
        ];

        setResults(combined);

        // Persist to history
        const updated = addToSearchHistory(trimmed);
        setSearchHistoryState(updated);
      } catch (err) {
        setError(err instanceof Error ? err.message : t('search.error_generic'));
        setResults([]);
        setPatentResults([]);
        setMoleculeResults([]);
      } finally {
        setSearchDuration(performance.now() - startTime);
        setLoading(false);
      }
    },
    [t],
  );

  // ── Consolidated debounced auto-search (query + filter changes) ──
  // Triggers 300ms after the user stops typing or changes a filter,
  // but skips if this exact combination was already searched.
  useEffect(() => {
    const trimmed = query.trim();
    if (!trimmed) return;

    const key = JSON.stringify({ query: trimmed, searchScope, filingDateFrom, filingDateTo, pubDateFrom, pubDateTo, jurisdictions: selectedJurisdictions });
    if (key === lastSearchKey.current) return;

    debounceTimer.current = setTimeout(() => {
      performSearch(query);
    }, 300);

    return () => {
      if (debounceTimer.current) {
        clearTimeout(debounceTimer.current);
      }
    };
  }, [query, searchScope, filingDateFrom, filingDateTo, pubDateFrom, pubDateTo, selectedJurisdictions, performSearch]);

  // ── Form submit handler (Enter key) ──
  const handleSearch = useCallback(
    (e?: { preventDefault?: () => void }) => {
      e?.preventDefault?.();
      if (debounceTimer.current) {
        clearTimeout(debounceTimer.current);
      }
      performSearch(query);
    },
    [query, performSearch],
  );

  // ── Input change ──
  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setQuery(val);
    if (val.trim()) {
      setShowSuggestions(true);
    }
  }, []);

  // ── Clear all filters ──
  const clearAllFilters = useCallback(() => {
    setSearchScope('all');
    setFilingDateFrom('');
    setFilingDateTo('');
    setPubDateFrom('');
    setPubDateTo('');
    setSelectedJurisdictions([]);
  }, []);

  // ── Toggle jurisdiction selection ──
  const toggleJurisdiction = useCallback((j: Jurisdiction) => {
    setSelectedJurisdictions(prev =>
      prev.includes(j) ? prev.filter(x => x !== j) : [...prev, j],
    );
  }, []);

  // ── Active filter chips ──
  const activeFilters = useMemo(() => {
    const chips: { id: string; label: string; onRemove: () => void }[] = [];

    if (searchScope !== 'all') {
      const scopeLabels: Record<SearchScope, string> = {
        title: t('search.scope_title'),
        abstract: t('search.scope_abstract'),
        fulltext: t('search.scope_fulltext'),
        all: t('search.scope_all'),
      };
      chips.push({
        id: 'scope',
        label: scopeLabels[searchScope],
        onRemove: () => setSearchScope('all'),
      });
    }

    if (filingDateFrom || filingDateTo) {
      chips.push({
        id: 'filingDate',
        label: `${t('search.filter_date_filing')}: ${filingDateFrom || '...'} ~ ${filingDateTo || '...'}`,
        onRemove: () => { setFilingDateFrom(''); setFilingDateTo(''); },
      });
    }

    if (pubDateFrom || pubDateTo) {
      chips.push({
        id: 'pubDate',
        label: `${t('search.filter_date_publication')}: ${pubDateFrom || '...'} ~ ${pubDateTo || '...'}`,
        onRemove: () => { setPubDateFrom(''); setPubDateTo(''); },
      });
    }

    selectedJurisdictions.forEach((j) => {
      chips.push({
        id: `jurisdiction-${j}`,
        label: j,
        onRemove: () => setSelectedJurisdictions(prev => prev.filter(x => x !== j)),
      });
    });

    return chips;
  }, [searchScope, filingDateFrom, filingDateTo, pubDateFrom, pubDateTo, selectedJurisdictions, t]);

  // ── Suggestion click ──
  const handleSuggestionClick = useCallback(
    (suggestion: string) => {
      if (debounceTimer.current) {
        clearTimeout(debounceTimer.current);
      }
      setQuery(suggestion);
      setShowSuggestions(false);
      performSearch(suggestion);
    },
    [performSearch],
  );

  // ── Clear input ──
  const handleClear = useCallback(() => {
    setQuery('');
    setShowSuggestions(false);
    inputRef.current?.blur();
  }, []);

  // ── Keyboard shortcut: Ctrl+K / Cmd+K ──
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && (e.key === 'k' || e.key === 'K')) {
        e.preventDefault();
        inputRef.current?.focus();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, []);

  // ── Close suggestions on outside click ──
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        suggestionsRef.current &&
        !suggestionsRef.current.contains(e.target as Node) &&
        inputRef.current &&
        !inputRef.current.contains(e.target as Node)
      ) {
        setShowSuggestions(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // ── Filtered suggestions (matching history) ──
  const filteredSuggestions = useMemo(() => {
    if (!query.trim()) return searchHistory;
    return searchHistory.filter((h) => h.toLowerCase().includes(query.toLowerCase()));
  }, [query, searchHistory]);

  const showSuggestionsDropdown = showSuggestions && searchHistory.length > 0;

  // ── Column definitions with keyword highlighting ──
  // Use the query from the last completed search so highlights don't flicker
  // while the user is still typing.
  const hlQuery = searchQuery;

  const patentColumns: Column<Patent>[] = useMemo(
    () => [
      {
        header: t('search.publication_no'),
        accessor: (row): React.ReactNode => highlightText(row.publicationNumber, hlQuery),
      },
      {
        header: t('search.title_column'),
        accessor: (row) => (
          <button
            onClick={() => navigate(`/patents/${row.id}`)}
            className="font-medium text-blue-600 hover:underline text-left"
          >
            {highlightText(row.title, hlQuery)}
          </button>
        ),
      },
      {
        header: t('search.assignee'),
        accessor: (row): React.ReactNode => highlightText(row.assignee, hlQuery),
      },
      {
        header: t('search.status'),
        accessor: (row) => (
          <Badge
            variant={
              row.legalStatus === 'granted'
                ? 'success'
                : row.legalStatus === 'pending'
                  ? 'warning'
                  : 'default'
            }
            size="sm"
          >
            {row.legalStatus}
          </Badge>
        ),
      },
      { header: t('search.filing_date'), accessor: 'filingDate' },
    ],
    [navigate, t, hlQuery],
  );

  const moleculeColumns: Column<Molecule>[] = useMemo(
    () => [
      {
        header: t('search.id'),
        accessor: (row): React.ReactNode => highlightText(row.id, hlQuery),
      },
      {
        header: t('search.name'),
        accessor: (row) => (
          <button
            onClick={() => navigate(`/molecules/${row.id}`)}
            className="font-medium text-blue-600 hover:underline text-left"
          >
            {highlightText(row.name || row.id, hlQuery)}
          </button>
        ),
      },
      {
        header: t('search.smiles'),
        accessor: (row) => (
          <span className="font-mono text-xs">
            {row.smiles?.substring(0, 40)}
            {row.smiles && row.smiles.length > 40 ? '...' : ''}
          </span>
        ),
      },
      {
        header: t('search.mol_weight'),
        accessor: (row) => (row.molecularWeight ? `${row.molecularWeight.toFixed(1)}` : '-'),
      },
    ],
    [navigate, t, hlQuery],
  );

  const allColumns: Column<SearchResult>[] = useMemo(
    () => [
      {
        header: t('search.type'),
        accessor: (row) => (
          <div className="flex items-center gap-2">
            {row.type === 'patent' ? (
              <FileText className="w-4 h-4 text-blue-500" />
            ) : (
              <Beaker className="w-4 h-4 text-green-500" />
            )}
            <span className="text-xs font-medium uppercase text-slate-500">{row.type}</span>
          </div>
        ),
      },
      {
        header: t('search.title_column'),
        accessor: (row) =>
          row.type === 'patent' ? (
            <button
              onClick={() => navigate(`/patents/${row.id}`)}
              className="font-medium text-blue-600 hover:underline text-left"
            >
              {highlightText((row as PatentResult).title, hlQuery)}
            </button>
          ) : (
            <button
              onClick={() => navigate(`/molecules/${row.id}`)}
              className="font-medium text-blue-600 hover:underline text-left"
            >
              {highlightText((row as MoleculeResult).name, hlQuery)}
            </button>
          ),
      },
      {
        header: t('search.identifier'),
        accessor: (row): React.ReactNode => (
          <span className="text-sm text-slate-500">
            {highlightText(
              row.type === 'patent'
                ? (row as PatentResult).publicationNumber
                : (row as MoleculeResult).id,
              hlQuery,
            )}
          </span>
        ),
      },
      {
        header: t('search.detail'),
        accessor: (row): React.ReactNode =>
          row.type === 'patent'
            ? highlightText((row as PatentResult).assignee, hlQuery)
            : `${(row as MoleculeResult).molecularWeight?.toFixed(1) ?? '-'} g/mol`,
      },
    ],
    [navigate, t, hlQuery],
  );

  // ── Duration formatter ──
  const formatDuration = useCallback((ms: number): string => {
    if (ms < 1000) return `${ms.toFixed(0)}ms`;
    return `${(ms / 1000).toFixed(2)}s`;
  }, []);

  // ── Result count for active tab ──
  const resultCount =
    activeTab === 'all'
      ? results.length
      : activeTab === 'patents'
        ? patentResults.length
        : moleculeResults.length;

  // ── Render: All Results tab ──
  const renderAllResults = () => {
    if (results.length === 0) {
      return (
        <div className="flex flex-col items-center justify-center py-16 text-slate-500">
          <SearchIcon className="w-12 h-12 mb-4 text-slate-300" />
          <p className="text-lg font-medium text-slate-600 mb-1">{t('search.no_results')}</p>
          <p className="text-sm">{t('search.adjust_query')}</p>
        </div>
      );
    }

    return (
      <Card padding="none">
        <DataTable columns={allColumns} data={results} />
      </Card>
    );
  };

  // ── Render: Patents tab ──
  const renderPatentsTab = () => {
    if (patentResults.length === 0) {
      return (
        <div className="flex flex-col items-center justify-center py-16 text-slate-500">
          <FileText className="w-12 h-12 mb-4 text-slate-300" />
          <p className="text-lg font-medium text-slate-600 mb-1">{t('search.no_patents')}</p>
          <p className="text-sm">{t('search.try_different_keywords')}</p>
        </div>
      );
    }

    return (
      <Card padding="none">
        <DataTable
          columns={patentColumns}
          data={patentResults}
          onRowClick={(row) => navigate(`/patents/${row.id}`)}
        />
      </Card>
    );
  };

  // ── Render: Molecules tab ──
  const renderMoleculesTab = () => {
    if (moleculeResults.length === 0) {
      return (
        <div className="flex flex-col items-center justify-center py-16 text-slate-500">
          <Beaker className="w-12 h-12 mb-4 text-slate-300" />
          <p className="text-lg font-medium text-slate-600 mb-1">{t('search.no_molecules')}</p>
          <p className="text-sm">{t('search.try_different_query')}</p>
        </div>
      );
    }

    return (
      <Card padding="none">
        <DataTable
          columns={moleculeColumns}
          data={moleculeResults}
          onRowClick={(row) => navigate(`/molecules/${row.id}`)}
        />
      </Card>
    );
  };

  // ── Render ──
  return (
    <div className="space-y-6">
      {/* ── Header ── */}
      <div className="flex flex-col gap-4">
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-bold text-slate-900">{t('search.title')}</h1>
          <span className="text-xs text-slate-400 hidden sm:inline">
            {t('search.keyboard_hint')}
          </span>
        </div>

        {/* ── Search Input with Scope ── */}
        <form onSubmit={handleSearch}>
          <div className="flex items-stretch">
            {/* Search scope dropdown */}
            <select
              value={searchScope}
              onChange={(e) => setSearchScope(e.target.value as SearchScope)}
              className="px-3 py-3 border border-r-0 border-slate-300 rounded-l-lg bg-white text-sm text-slate-600 focus:ring-2 focus:ring-blue-500 focus:border-transparent focus:z-10 relative"
              aria-label={t('search.scope_label')}
            >
              <option value="all">{t('search.scope_all')}</option>
              <option value="title">{t('search.scope_title')}</option>
              <option value="abstract">{t('search.scope_abstract')}</option>
              <option value="fulltext">{t('search.scope_fulltext')}</option>
            </select>

            <div className="relative flex-1" ref={suggestionsRef}>
              <input
                ref={inputRef}
                type="text"
                value={query}
                onChange={handleInputChange}
                onFocus={() => setShowSuggestions(searchHistory.length > 0)}
                onKeyDown={(e) => {
                  if (e.key === 'Escape' && query) {
                    handleClear();
                  }
                }}
                placeholder={t('search.placeholder')}
                className="w-full pl-10 pr-10 py-3 border border-slate-300 rounded-none rounded-r-lg shadow-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent text-lg"
              />
              <SearchIcon
                className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400 w-5 h-5 pointer-events-none"
                aria-hidden="true"
              />

            {/* Clear button */}
            {query && (
              <button
                type="button"
                onClick={handleClear}
                className="absolute right-3.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-600 transition-colors"
                aria-label="Clear search"
              >
                <X className="w-5 h-5" />
              </button>
            )}

            {/* ── Suggestions dropdown ── */}
            {showSuggestionsDropdown && (
              <div className="absolute top-full left-0 right-0 mt-1 bg-white border border-slate-200 rounded-lg shadow-lg z-50 max-h-80 overflow-y-auto">
                <div className="px-4 py-2 text-xs font-semibold text-slate-400 uppercase tracking-wider flex items-center justify-between border-b border-slate-100">
                  <span className="flex items-center gap-1.5">
                    <Clock className="w-3.5 h-3.5" />
                    {t('search.history_title')}
                  </span>
                  <button
                    type="button"
                    onClick={() => {
                      clearSearchHistory();
                      setSearchHistoryState([]);
                    }}
                    className="text-blue-500 hover:text-blue-700 font-normal normal-case transition-colors"
                  >
                    {t('search.clear_history')}
                  </button>
                </div>

                {filteredSuggestions.length === 0 ? (
                  <div className="px-4 py-6 text-center text-sm text-slate-400">
                    {t('search.no_history')}
                  </div>
                ) : (
                  filteredSuggestions.map((item, index) => (
                    <button
                      key={`${item}-${index}`}
                      type="button"
                      onMouseDown={(e) => {
                        e.preventDefault();
                        handleSuggestionClick(item);
                      }}
                      className="w-full text-left px-4 py-2.5 text-sm text-slate-700 hover:bg-slate-50 flex items-center gap-2 transition-colors"
                    >
                      <Clock className="w-3.5 h-3.5 text-slate-400 flex-shrink-0" />
                      <span className="truncate">{item}</span>
                    </button>
                  ))
                )}
              </div>
            )}
          </div>
        </div>
        </form>

        {/* ── Filters bar ── */}
        <div className="bg-slate-50 border border-slate-200 rounded-lg p-3 space-y-2.5">
          {/* Date range filters */}
          <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
            {/* Filing date */}
            <div className="flex items-center gap-2">
              <label className="text-xs font-medium text-slate-500 whitespace-nowrap uppercase tracking-wider">
                {t('search.filter_date_filing')}
              </label>
              <input
                type="date"
                value={filingDateFrom}
                onChange={(e) => setFilingDateFrom(e.target.value)}
                className="px-2 py-1.5 text-xs border border-slate-300 rounded-md focus:ring-1 focus:ring-blue-500 focus:border-blue-500 bg-white"
                aria-label={t('search.filter_from')}
              />
              <span className="text-slate-400 text-xs">~</span>
              <input
                type="date"
                value={filingDateTo}
                onChange={(e) => setFilingDateTo(e.target.value)}
                className="px-2 py-1.5 text-xs border border-slate-300 rounded-md focus:ring-1 focus:ring-blue-500 focus:border-blue-500 bg-white"
                aria-label={t('search.filter_to')}
              />
            </div>

            {/* Publication date */}
            <div className="flex items-center gap-2">
              <label className="text-xs font-medium text-slate-500 whitespace-nowrap uppercase tracking-wider">
                {t('search.filter_date_publication')}
              </label>
              <input
                type="date"
                value={pubDateFrom}
                onChange={(e) => setPubDateFrom(e.target.value)}
                className="px-2 py-1.5 text-xs border border-slate-300 rounded-md focus:ring-1 focus:ring-blue-500 focus:border-blue-500 bg-white"
                aria-label={t('search.filter_from')}
              />
              <span className="text-slate-400 text-xs">~</span>
              <input
                type="date"
                value={pubDateTo}
                onChange={(e) => setPubDateTo(e.target.value)}
                className="px-2 py-1.5 text-xs border border-slate-300 rounded-md focus:ring-1 focus:ring-blue-500 focus:border-blue-500 bg-white"
                aria-label={t('search.filter_to')}
              />
            </div>
          </div>

          {/* Jurisdiction filter */}
          <div className="flex items-center gap-2">
            <span className="text-xs font-medium text-slate-500 uppercase tracking-wider">
              {t('search.filter_jurisdiction')}
            </span>
            <div className="flex gap-1">
              {(['CN', 'US', 'EP', 'JP', 'KR'] as Jurisdiction[]).map((j) => (
                <button
                  key={j}
                  type="button"
                  onClick={() => toggleJurisdiction(j)}
                  className={`px-2.5 py-1 text-xs font-medium rounded-full border transition-colors ${
                    selectedJurisdictions.includes(j)
                      ? 'bg-blue-100 border-blue-300 text-blue-700'
                      : 'bg-white border-slate-300 text-slate-500 hover:bg-slate-100'
                  }`}
                >
                  {j}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* ── Active filter chips ── */}
        {activeFilters.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-xs text-slate-400 mr-0.5">{t('search.filter_active')}:</span>
            {activeFilters.map((chip) => (
              <span
                key={chip.id}
                className="inline-flex items-center gap-1 px-2 py-0.5 bg-blue-50 text-blue-700 text-xs rounded-full border border-blue-200"
              >
                {chip.label}
                <button
                  type="button"
                  onClick={chip.onRemove}
                  className="text-blue-400 hover:text-blue-600 transition-colors"
                  aria-label={`Remove ${chip.label}`}
                >
                  <X className="w-3 h-3" />
                </button>
              </span>
            ))}
            <button
              type="button"
              onClick={clearAllFilters}
              className="text-xs text-slate-400 hover:text-slate-600 transition-colors ml-1"
            >
              {t('search.filter_clear_all')}
            </button>
          </div>
        )}
      </div>

      {/* ── Search info bar (duration + count) ── */}
      {searched && !loading && !error && (
        <div className="flex items-center gap-4 text-sm">
          <span className="flex items-center gap-1.5 text-slate-500">
            <Clock className="w-4 h-4" />
            {t('search.search_duration', { time: formatDuration(searchDuration ?? 0) })}
          </span>
          <span className="text-slate-400">
            {resultCount} {resultCount === 1 ? t('search.result') : t('search.results')}
          </span>
        </div>
      )}

      {/* ── Tabs ── */}
      <div className="border-b border-slate-200">
        <nav className="-mb-px flex space-x-8">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`
                whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm
                ${
                  activeTab === tab.id
                    ? 'border-blue-500 text-blue-600'
                    : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300'
                }
              `}
            >
              {tab.label}
              {tab.id !== 'all' && (
                <span className="ml-2 text-xs text-slate-400">
                  ({tab.id === 'patents' ? patentResults.length : moleculeResults.length})
                </span>
              )}
            </button>
          ))}
        </nav>
      </div>

      {/* ── Error ── */}
      {error && (
        <div className="flex items-center gap-3 p-4 text-red-700 bg-red-50 rounded-lg border border-red-200">
          <AlertCircle className="w-5 h-5 flex-shrink-0" />
          <div>
            <p className="font-medium">{t('search.error_title')}</p>
            <p className="text-sm">{error}</p>
          </div>
          <Button variant="ghost" size="sm" className="ml-auto" onClick={() => handleSearch()}>
            {t('search.retry')}
          </Button>
        </div>
      )}

      {/* ── Loading ── */}
      {loading && (
        <div className="flex justify-center py-16">
          <LoadingSpinner size="lg" />
        </div>
      )}

      {/* ── Results ── */}
      {!loading && !error && searched && activeTab === 'all' && renderAllResults()}
      {!loading && !error && searched && activeTab === 'patents' && renderPatentsTab()}
      {!loading && !error && searched && activeTab === 'molecules' && renderMoleculesTab()}

      {/* ── Pre-search empty state ── */}
      {!loading && !searched && (
        <Card className="min-h-[400px] flex items-center justify-center">
          <div className="text-center text-slate-500">
            <SearchIcon className="w-16 h-16 mx-auto mb-4 text-slate-300" />
            <p className="text-lg font-medium text-slate-600 mb-1">{t('search.empty_title')}</p>
            <p className="text-sm">{t('search.empty_desc')}</p>
          </div>
        </Card>
      )}
    </div>
  );
};

export default Search;
