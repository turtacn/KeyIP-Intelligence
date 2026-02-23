import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import DataTable, { Column } from '../../components/ui/DataTable';
import { Search, Filter, Calendar, Type } from 'lucide-react';
import { patentService } from '../../services/patent.service';
import { Patent } from '../../types/domain';

const PatentSearch: React.FC = () => {
  const [mode, setMode] = useState<'text' | 'structure'>('text');
  const [query, setQuery] = useState('');
  const [smiles, setSmiles] = useState('');
  const [similarity, setSimilarity] = useState(0.8);
  const [results, setResults] = useState<Patent[]>([]);
  const [loading, setLoading] = useState(false);

  const handleSearch = async () => {
    setLoading(true);
    try {
      // Mock search logic using service (which returns mock data)
      const response = await patentService.getPatents();
      // Filter mock data based on query for demo purposes
      const filtered = response.data.filter(p =>
        p.title.toLowerCase().includes(query.toLowerCase()) ||
        p.publicationNumber.toLowerCase().includes(query.toLowerCase())
      );
      setResults(filtered);
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
            Text Search
          </button>
          <button
            onClick={() => setMode('structure')}
            className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
              mode === 'structure' ? 'bg-blue-600 text-white' : 'bg-slate-100 text-slate-600 hover:bg-slate-200'
            }`}
          >
            Structure Search
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
                    placeholder="e.g., Blue OLED Host Material"
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
                  placeholder="Enter SMILES string..."
                  className="w-full px-3 py-2 border border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-500 mb-1">Similarity Threshold: {similarity}</label>
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

          <Button onClick={handleSearch} isLoading={loading} leftIcon={<Search className="w-4 h-4" />}>
            Search
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-hidden">
        <DataTable
          columns={columns}
          data={results}
          isLoading={loading}
          pagination={{ currentPage: 1, totalPages: 5, onPageChange: () => {} }}
        />
      </div>
    </Card>
  );
};

export default PatentSearch;
