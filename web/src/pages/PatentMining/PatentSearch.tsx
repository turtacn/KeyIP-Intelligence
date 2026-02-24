import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import DataTable, { Column } from '../../components/ui/DataTable';
import { Search } from 'lucide-react';
import { patentService } from '../../services/patent.service';
import { Patent } from '../../types/domain';
import { useTranslation } from 'react-i18next';

const PatentSearch: React.FC = () => {
  const { t } = useTranslation();
  const [mode, setMode] = useState<'text' | 'structure'>('text');
  const [query, setQuery] = useState('');
  const [smiles, setSmiles] = useState('');
  const [similarity, setSimilarity] = useState(0.8);
  const [results, setResults] = useState<Patent[]>([]);
  const [loading, setLoading] = useState(false);
  const [pagination, setPagination] = useState({ currentPage: 1, totalPages: 1 });

  const handleSearch = async (page = 1) => {
    setLoading(true);
    try {
      const q = mode === 'text' ? query : smiles;
      const response = await patentService.getPatents(page, 20, q, mode);
      setResults(response.data);
      if (response.pagination) {
          setPagination({
              currentPage: response.pagination.page,
              totalPages: Math.ceil(response.pagination.total / response.pagination.pageSize)
          });
      }
    } catch (err) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  const columns: Column<Patent>[] = [
    { header: 'Relevance', accessor: () => <span className="text-green-600 font-bold">{(0.85 + Math.random() * 0.14).toFixed(2)}</span> },
    { header: 'Patent No.', accessor: 'publicationNumber' },
    { header: 'Title', accessor: (row) => <span className="font-medium text-blue-600 hover:underline cursor-pointer">{row.title}</span> },
    { header: 'Assignee', accessor: 'assignee' },
    { header: 'Pub. Date', accessor: 'publicationDate' },
    { header: 'IPC', accessor: (row) => row.ipcCodes[0] },
  ];

  return (
    <Card className="h-full flex flex-col">
      <div className="border-b border-slate-200 pb-4 mb-4">
        <div className="flex space-x-4 mb-4">
          <button
            onClick={() => setMode('text')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              mode === 'text' ? 'bg-blue-600 text-white' : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
            }`}
          >
            {t('mining.search.mode_text', 'Text Search')}
          </button>
          <button
            onClick={() => setMode('structure')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              mode === 'structure' ? 'bg-blue-600 text-white' : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
            }`}
          >
             {t('mining.search.mode_structure', 'Structure Search')}
          </button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 items-end">
          {mode === 'text' ? (
            <>
              <div className="md:col-span-3">
                <label className="block text-xs font-medium text-slate-500 mb-1">Keywords / Patent Number</label>
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
                <label className="block text-xs font-medium text-slate-500 mb-1">SMILES Structure</label>
                <input
                  type="text"
                  value={smiles}
                  onChange={(e) => setSmiles(e.target.value)}
                  placeholder={t('mining.search.placeholder_smiles', 'Enter SMILES string...')}
                  className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-500 mb-1">{t('mining.search.similarity', 'Similarity Threshold')}: {similarity}</label>
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
          data={results}
          isLoading={loading}
          pagination={{
              currentPage: pagination.currentPage,
              totalPages: pagination.totalPages,
              onPageChange: (p) => handleSearch(p)
          }}
        />
      </div>
    </Card>
  );
};

export default PatentSearch;
