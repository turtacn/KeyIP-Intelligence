import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import { Company } from '../../types/domain';
import { Filter } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface GapAnalysisProps {
  companies: Company[];
}

const GapAnalysis: React.FC<GapAnalysisProps> = ({ companies }) => {
  const { t } = useTranslation();
  const [selectedCompetitors, setSelectedCompetitors] = useState<string[]>(['Samsung SDI', 'LG Chem']);
  const [showFilters, setShowFilters] = useState(false);

  // Mock data for domains
  const domains = ['Blue Emitter', 'Green Emitter', 'Red Emitter', 'HTL', 'ETL', 'Encapsulation'];

  // Mock data for counts (Organization vs Competitors)
  const getCount = (domain: string, entity: string) => {
    // Generate pseudo-random count based on name hash for consistency
    const seed = domain.length + entity.length;
    if (entity === 'My Org') {
      // Simulate gaps
      if (domain === 'Red Emitter') return 2;
      if (domain === 'Encapsulation') return 0;
      return (seed * 3) % 20 + 5;
    }
    // Competitors usually have more
    return (seed * 7) % 50 + 10;
  };

  const isGap = (orgCount: number, compCount: number) => {
    return orgCount < 5 && compCount > 20;
  };

  return (
    <Card className="flex flex-col h-full">
      <div className="flex justify-between items-center mb-4">
        <h3 className="text-lg font-semibold text-slate-800">{t('portfolio.gap.title')}</h3>
        <Button
          size="sm"
          variant="outline"
          onClick={() => setShowFilters(!showFilters)}
          leftIcon={<Filter className="w-4 h-4" />}
        >
          {t('portfolio.gap.select_comp')}
        </Button>
      </div>

      {showFilters && (
        <div className="bg-slate-50 p-4 rounded-lg mb-4 grid grid-cols-2 md:grid-cols-4 gap-2 animate-in fade-in slide-in-from-top-2">
          {companies.map(comp => (
            <label key={comp.id} className="flex items-center space-x-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                checked={selectedCompetitors.includes(comp.name)}
                onChange={() => {
                  if (selectedCompetitors.includes(comp.name)) {
                    setSelectedCompetitors(selectedCompetitors.filter(c => c !== comp.name));
                  } else {
                    setSelectedCompetitors([...selectedCompetitors, comp.name]);
                  }
                }}
                className="rounded text-blue-600 focus:ring-blue-500"
              />
              <span>{comp.name}</span>
            </label>
          ))}
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-slate-200 border border-slate-200 rounded-lg">
          <thead className="bg-slate-50">
            <tr>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider sticky left-0 bg-slate-50 z-10">
                {t('portfolio.gap.domain')}
              </th>
              <th scope="col" className="px-6 py-3 text-left text-xs font-medium text-slate-900 uppercase tracking-wider font-bold bg-blue-50/50">
                {t('portfolio.gap.my_org')}
              </th>
              {selectedCompetitors.map(comp => (
                <th key={comp} scope="col" className="px-6 py-3 text-left text-xs font-medium text-slate-500 uppercase tracking-wider">
                  {comp}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-slate-200">
            {domains.map((domain) => {
              const myCount = getCount(domain, 'My Org');
              return (
                <tr key={domain} className="hover:bg-slate-50 transition-colors">
                  <td className="px-6 py-4 whitespace-nowrap text-sm font-medium text-slate-900 sticky left-0 bg-white z-10">
                    {domain}
                  </td>
                  <td className="px-6 py-4 whitespace-nowrap text-sm text-slate-900 font-bold bg-blue-50/20">
                    <div className="flex items-center gap-2">
                      <span>{myCount}</span>
                      <div className="w-16 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                        <div className="h-full bg-blue-600 rounded-full" style={{ width: `${Math.min(100, myCount * 2)}%` }}></div>
                      </div>
                    </div>
                  </td>
                  {selectedCompetitors.map(comp => {
                    const count = getCount(domain, comp);
                    const gap = isGap(myCount, count);
                    return (
                      <td key={comp} className={`px-6 py-4 whitespace-nowrap text-sm text-slate-500 ${gap ? 'bg-red-50' : ''}`}>
                        <div className="flex items-center gap-2">
                          <span className={gap ? 'text-red-600 font-bold' : ''}>{count}</span>
                          <div className="w-16 h-1.5 bg-slate-100 rounded-full overflow-hidden">
                            <div className={`h-full rounded-full ${gap ? 'bg-red-400' : 'bg-slate-400'}`} style={{ width: `${Math.min(100, count * 2)}%` }}></div>
                          </div>
                          {gap && <span className="text-xs text-red-500 font-bold ml-1">GAP</span>}
                        </div>
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </Card>
  );
};

export default GapAnalysis;
