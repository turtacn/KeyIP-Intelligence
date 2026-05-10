import React, { useState, useMemo } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import DataTable, { Column } from '../../components/ui/DataTable';
import { useTableState } from '../../hooks/useTableState';
import { Search } from 'lucide-react';
import { patentService } from '../../services/patent.service';
import { Patent } from '../../types/domain';
import { useTranslation } from 'react-i18next';
import MoleculeViewer from '../../components/ui/MoleculeViewer';

const PatentSearch: React.FC = () => {
  const { t } = useTranslation();
  const [mode, setMode] = useState<'text' | 'structure'>('text');
  const [query, setQuery] = useState('');
  const [smiles, setSmiles] = useState('');
  const [similarity, setSimilarity] = useState(0.8);
  const [results, setResults] = useState<Patent[]>([]);
  const [loading, setLoading] = useState(false);
  const [totalPages, setTotalPages] = useState(1);

  const tableState = useTableState({ pageSize: 20 });

  const handleSearch = async (page = 1) => {
    setLoading(true);
    try {
      const q = mode === 'text' ? query : smiles;
      const response = await patentService.getPatents(page, 20, q);
      setResults(response.data);
      if (response.pagination) {
        setTotalPages(Math.ceil(response.pagination.total / response.pagination.pageSize));
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  // Client-side sorting for already loaded results
  const sortedResults = useMemo(() => {
    if (!tableState.sort.column || !tableState.sort.direction) return results;

    return [...results].sort((a, b) => {
      let comparison = 0;
      const col = tableState.sort.column;

      switch (col) {
        case 'publicationNumber':
          comparison = a.publicationNumber.localeCompare(b.publicationNumber);
          break;
        case 'title':
          comparison = a.title.localeCompare(b.title);
          break;
        case 'assignee':
          comparison = a.assignee.localeCompare(b.assignee);
          break;
        case 'publicationDate':
          comparison = new Date(a.publicationDate).getTime() - new Date(b.publicationDate).getTime();
          break;
        case 'ipcCodes':
          comparison = (a.ipcCodes[0] || '').localeCompare(b.ipcCodes[0] || '');
          break;
        default:
          return 0;
      }

      return tableState.sort.direction === 'asc' ? comparison : -comparison;
    });
  }, [results, tableState.sort]);

  // Client-side search/filter
  const filteredResults = useMemo(() => {
    if (!tableState.searchQuery) return sortedResults;
    const q = tableState.searchQuery.toLowerCase();
    return sortedResults.filter(
      (p) =>
        p.title.toLowerCase().includes(q) ||
        p.publicationNumber.toLowerCase().includes(q) ||
        p.assignee.toLowerCase().includes(q) ||
        p.id.toLowerCase().includes(q)
    );
  }, [sortedResults, tableState.searchQuery]);

  const columns: Column<Patent>[] = useMemo(
    () => [
      {
        header: t('table.structure'),
        accessor: () => {
          const demoSmiles = 'C1=CC=CC=C1';
          return (
            <div className="w-24 h-16">
              <MoleculeViewer smiles={demoSmiles} width={96} height={64} />
            </div>
          );
        },
        id: 'structure',
      },
      {
        header: t('table.relevance'),
        accessor: () => (
          <span className="text-green-600 font-bold">
            {(0.85 + Math.random() * 0.14).toFixed(2)}
          </span>
        ),
        id: 'relevance',
        sortable: true,
        sortKey: 'relevance',
      },
      {
        header: t('table.patent_no'),
        accessor: 'publicationNumber',
        sortable: true,
        sortKey: 'publicationNumber',
      },
      {
        header: t('table.title'),
        accessor: (row) => (
          <span className="font-medium text-blue-600 hover:underline cursor-pointer">
            {row.title}
          </span>
        ),
        sortable: true,
        sortKey: 'title',
      },
      {
        header: t('table.assignee'),
        accessor: 'assignee',
        sortable: true,
        sortKey: 'assignee',
      },
      {
        header: t('table.pub_date'),
        accessor: 'publicationDate',
        sortable: true,
        sortKey: 'publicationDate',
      },
      {
        header: 'IPC',
        accessor: (row) => row.ipcCodes[0],
        sortable: true,
        sortKey: 'ipcCodes',
      },
    ],
    [t]
  );

  return (
    <Card className="h-full flex flex-col">
      <div className="border-b border-slate-200 pb-4 mb-4">
        <div className="flex space-x-4 mb-4">
          <button
            onClick={() => setMode('text')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              mode === 'text'
                ? 'bg-blue-600 text-white'
                : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
            }`}
          >
            {t('mining.search.mode_text', 'Text Search')}
          </button>
          <button
            onClick={() => setMode('structure')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              mode === 'structure'
                ? 'bg-blue-600 text-white'
                : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
            }`}
          >
            {t('mining.search.mode_structure', 'Structure Search')}
          </button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 items-end">
          {mode === 'text' ? (
            <>
              <div className="md:col-span-3">
                <label className="block text-xs font-medium text-slate-500 mb-1">
                  {t('mining.search.label_keywords')}
                </label>
                <div className="relative">
                  <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-4 h-4" />
                  <input
                    type="text"
                    value={query}
                    onChange={(e) => setQuery(e.target.value)}
                    onKeyDown={(e) => e.key === 'Enter' && handleSearch(1)}
                    placeholder={t('mining.search.placeholder_text', 'e.g., Blue OLED Host Material')}
                    className="w-full pl-9 pr-3 py-2 border border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
                  />
                </div>
              </div>
            </>
          ) : (
            <>
              <div className="md:col-span-2">
                <label className="block text-xs font-medium text-slate-500 mb-1">
                  {t('mining.search.label_smiles')}
                </label>
                <div className="relative">
                  <input
                    type="text"
                    value={smiles}
                    onChange={(e) => setSmiles(e.target.value)}
                    placeholder={t('mining.search.placeholder_smiles', 'Enter SMILES string...')}
                    className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500 mb-2"
                  />
                  {smiles && (
                    <div className="absolute top-full left-0 z-10 bg-white border border-slate-200 shadow-lg rounded-lg p-2 mt-1">
                      <MoleculeViewer smiles={smiles} width={200} height={100} />
                    </div>
                  )}
                </div>
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-500 mb-1">
                  {t('mining.search.similarity', 'Similarity Threshold')}: {similarity}
                </label>
                <input
                  type="range"
                  min="0.5"
                  max="1.0"
                  step="0.05"
                  value={similarity}
                  onChange={(e) => setSimilarity(parseFloat(e.target.value))}
                  className="w-full h-2 bg-slate-200 rounded-lg appearance-none cursor-pointer accent-blue-600"
                />
              </div>
            </>
          )}

          <Button onClick={() => handleSearch(1)} isLoading={loading} leftIcon={<Search className="w-4 h-4" />}>
            {t('mining.search.btn_search', 'Search')}
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-hidden">
        <DataTable
          columns={columns}
          data={filteredResults}
          isLoading={loading}
          sortState={tableState.sort}
          onSort={tableState.setSortState}
          showToolbar
          enableExport
          enableColumnVisibility
          onSearch={tableState.setSearchQuery}
          searchPlaceholder={t('mining.search.filter_results', 'Filter results...')}
          pagination={
            totalPages > 1
              ? {
                  currentPage: tableState.currentPage,
                  totalPages,
                  onPageChange: (p) => {
                    tableState.setCurrentPage(p);
                    handleSearch(p);
                  },
                }
              : undefined
          }
        />
      </div>
    </Card>
  );
};

export default PatentSearch;
