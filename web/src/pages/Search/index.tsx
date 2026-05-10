import React, { useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import Card from '../../components/ui/Card';
import DataTable, { Column } from '../../components/ui/DataTable';
import LoadingSpinner from '../../components/ui/LoadingSpinner';
import Badge from '../../components/ui/Badge';
import Button from '../../components/ui/Button';
import { Search as SearchIcon, FileText, Beaker, AlertCircle } from 'lucide-react';
import { patentService } from '../../services/patent.service';
import { moleculeService } from '../../services/molecule.service';
import { Patent, Molecule } from '../../types/domain';

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

  const tabs = [
    { id: 'all', label: t('search.tabs_all') },
    { id: 'patents', label: t('search.tabs_patents') },
    { id: 'molecules', label: t('search.tabs_molecules') },
  ];

  const handleSearch = useCallback(async (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    const trimmed = query.trim();
    if (!trimmed) return;

    setLoading(true);
    setError(null);
    setSearched(true);

    try {
      const [patentResp, moleculeResp] = await Promise.all([
        patentService.getPatents(1, 20, trimmed),
        moleculeService.getMolecules(1, 20),
      ]);

      const patents = patentResp.data || [];
      const molecules = moleculeResp.data || [];

      setPatentResults(patents);
      setMoleculeResults(molecules);

      const combined: SearchResult[] = [
        ...patents.map((p: Patent): PatentResult => ({
          type: 'patent' as const,
          id: p.id,
          publicationNumber: p.publicationNumber,
          title: p.title,
          assignee: p.assignee,
          legalStatus: p.legalStatus,
          filingDate: p.filingDate,
        })),
        ...molecules.map((m: Molecule): MoleculeResult => ({
          type: 'molecule' as const,
          id: m.id,
          name: m.name || m.id,
          smiles: m.smiles,
          molecularWeight: m.molecularWeight,
        })),
      ];

      setResults(combined);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed');
      setResults([]);
      setPatentResults([]);
      setMoleculeResults([]);
    } finally {
      setLoading(false);
    }
  }, [query]);

  const patentColumns: Column<Patent>[] = [
    { header: t('search.publication_no'), accessor: 'publicationNumber' },
    { header: t('search.title_column'), accessor: (row) => (
      <button
        onClick={() => navigate(`/patents/${row.id}`)}
        className="font-medium text-blue-600 hover:underline text-left"
      >
        {row.title}
      </button>
    )},
    { header: t('search.assignee'), accessor: 'assignee' },
    { header: t('search.status'), accessor: (row) => (
      <Badge variant={row.legalStatus === 'granted' ? 'success' : row.legalStatus === 'pending' ? 'warning' : 'default'} size="sm">
        {row.legalStatus}
      </Badge>
    )},
    { header: t('search.filing_date'), accessor: 'filingDate' },
  ];

  const moleculeColumns: Column<Molecule>[] = [
    { header: t('search.id'), accessor: 'id' },
    { header: t('search.name'), accessor: (row) => (
      <button
        onClick={() => navigate(`/molecules/${row.id}`)}
        className="font-medium text-blue-600 hover:underline text-left"
      >
        {row.name || row.id}
      </button>
    )},
    { header: t('search.smiles'), accessor: (row) => (
      <span className="font-mono text-xs">{row.smiles?.substring(0, 40)}{row.smiles && row.smiles.length > 40 ? '...' : ''}</span>
    )},
    { header: t('search.mol_weight'), accessor: (row) => row.molecularWeight ? `${row.molecularWeight.toFixed(1)}` : '-' },
  ];

  const allColumns: Column<SearchResult>[] = [
    { header: t('search.type'), accessor: (row) => (
      <div className="flex items-center gap-2">
        {row.type === 'patent' ? (
          <FileText className="w-4 h-4 text-blue-500" />
        ) : (
          <Beaker className="w-4 h-4 text-green-500" />
        )}
        <span className="text-xs font-medium uppercase text-slate-500">{row.type}</span>
      </div>
    )},
    { header: t('search.title_column'), accessor: (row) => (
      row.type === 'patent' ? (
        <button
          onClick={() => navigate(`/patents/${row.id}`)}
          className="font-medium text-blue-600 hover:underline text-left"
        >
          {(row as PatentResult).title}
        </button>
      ) : (
        <button
          onClick={() => navigate(`/molecules/${row.id}`)}
          className="font-medium text-blue-600 hover:underline text-left"
        >
          {(row as MoleculeResult).name}
        </button>
      )
    )},
    { header: t('search.identifier'), accessor: (row) => (
      <span className="text-sm text-slate-500">
        {row.type === 'patent' ? (row as PatentResult).publicationNumber : (row as MoleculeResult).id}
      </span>
    )},
    { header: t('search.detail'), accessor: (row) => (
      row.type === 'patent' ? (row as PatentResult).assignee : `${(row as MoleculeResult).molecularWeight?.toFixed(1) ?? '-'} g/mol`
    )},
  ];

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
        <DataTable
          columns={allColumns}
          data={results}
        />
      </Card>
    );
  };

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

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4">
        <h1 className="text-2xl font-bold text-slate-900">{t('search.title')}</h1>
        <form onSubmit={handleSearch}>
          <div className="relative">
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder={t('search.placeholder')}
              className="w-full pl-10 pr-4 py-3 border border-slate-300 rounded-lg shadow-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent text-lg"
            />
            <SearchIcon className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400 w-5 h-5" />
          </div>
        </form>
      </div>

      {/* Tabs */}
      <div className="border-b border-slate-200">
        <nav className="-mb-px flex space-x-8">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`
                whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm
                ${activeTab === tab.id
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

      {/* Error */}
      {error && (
        <div className="flex items-center gap-3 p-4 text-red-700 bg-red-50 rounded-lg border border-red-200">
          <AlertCircle className="w-5 h-5 flex-shrink-0" />
          <div>
            <p className="font-medium">{t('search.error_title')}</p>
            <p className="text-sm">{error}</p>
          </div>
          <Button variant="ghost" size="sm" className="ml-auto" onClick={handleSearch}>
            {t('search.retry')}
          </Button>
        </div>
      )}

      {/* Loading */}
      {loading && (
        <div className="flex justify-center py-16">
          <LoadingSpinner size="lg" />
        </div>
      )}

      {/* Results */}
      {!loading && !error && searched && activeTab === 'all' && renderAllResults()}
      {!loading && !error && searched && activeTab === 'patents' && renderPatentsTab()}
      {!loading && !error && searched && activeTab === 'molecules' && renderMoleculesTab()}

      {/* Pre-search state */}
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
