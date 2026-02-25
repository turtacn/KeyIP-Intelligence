import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import DataTable, { Column } from '../../components/ui/DataTable';
import { Search as SearchIcon } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const Search: React.FC = () => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState('all');
  const [results] = useState([]); // Mock results empty for now

  const tabs = ['all', 'patents', 'molecules', 'companies', 'alerts'];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4">
        <h1 className="text-2xl font-bold text-slate-900">Search Results</h1>
        <div className="relative">
          <input
            type="text"
            placeholder={t('search.global_placeholder')}
            className="w-full pl-10 pr-4 py-3 border border-slate-300 rounded-lg shadow-sm focus:ring-2 focus:ring-blue-500 focus:border-transparent text-lg"
          />
          <SearchIcon className="absolute left-3.5 top-1/2 -translate-y-1/2 text-slate-400 w-5 h-5" />
        </div>
      </div>

      <div className="border-b border-slate-200">
        <nav className="-mb-px flex space-x-8">
          {tabs.map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`
                whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm capitalize
                ${activeTab === tab
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300'
                }
              `}
            >
              {tab}
            </button>
          ))}
        </nav>
      </div>

      <Card padding="none" className="min-h-[400px] flex items-center justify-center">
        {results.length === 0 ? (
          <div className="text-center text-slate-500">
            <SearchIcon className="w-12 h-12 mx-auto mb-4 text-slate-300" />
            <p>No results found. Try a different query.</p>
          </div>
        ) : (
          <DataTable columns={[{ header: 'Name', accessor: 'name' } as Column<any>]} data={results} />
        )}
      </Card>
    </div>
  );
};

export default Search;
