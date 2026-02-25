import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import { ScatterChart, Scatter, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ReferenceArea } from 'recharts';
import { Search } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const WhiteSpaceDiscovery: React.FC = () => {
  const { t } = useTranslation();
  const [domain, setDomain] = useState('Blue Emitter');
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<any[]>([]);

  const handleAnalyze = () => {
    setLoading(true);
    setTimeout(() => {
      // Mock data generation
      const mockData = Array.from({ length: 50 }).map((_, i) => ({
        id: `mol_${i}`,
        homo: -5.0 - Math.random() * 1.5,
        eqe: 5 + Math.random() * 20,
        patented: Math.random() > 0.3,
      }));
      setData(mockData);
      setLoading(false);
    }, 1500);
  };

  const domains = ['Blue Emitter', 'Green Emitter', 'Red Emitter', 'HTL', 'ETL', 'Encapsulation'];

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 h-full">
      <Card className="lg:col-span-1 h-full flex flex-col" bodyClassName="flex flex-col">
        <h3 className="text-lg font-semibold text-slate-800 mb-4">Discovery Parameters</h3>
        <div className="space-y-4 flex-1">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Technology Domain</label>
            <select
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              className="w-full border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
            >
              {domains.map((d) => (
                <option key={d} value={d}>{d}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">Core Scaffold (Optional)</label>
            <input
              type="text"
              placeholder={t('mining.whitespace.search_placeholder')}
              className="w-full border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
            />
          </div>
          <div className="bg-blue-50 p-4 rounded-lg text-sm text-blue-800 border border-blue-100 mt-4">
            <h4 className="font-semibold mb-2">Target Properties</h4>
            <ul className="list-disc list-inside space-y-1 opacity-80">
              <li>HOMO: -5.0 to -6.5 eV</li>
              <li>LUMO: -2.0 to -3.0 eV</li>
              <li>EQE: &gt; 20%</li>
            </ul>
          </div>
        </div>
        <div className="mt-6 pt-4 border-t border-slate-100">
          <Button onClick={handleAnalyze} isLoading={loading} className="w-full" leftIcon={<Search className="w-4 h-4" />}>
            Identify White Spaces
          </Button>
        </div>
      </Card>

      <Card className="lg:col-span-2 h-[500px] flex flex-col" bodyClassName="flex flex-col">
        <h3 className="text-lg font-semibold text-slate-800 mb-2">Molecular Space Map</h3>
        <p className="text-sm text-slate-500 mb-4">Visualizing patented vs. potential candidate space based on electronic properties.</p>

        {data.length > 0 ? (
          <div className="flex-1 w-full relative">
            <ResponsiveContainer width="100%" height="100%">
              <ScatterChart margin={{ top: 20, right: 20, bottom: 20, left: 20 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis
                  type="number"
                  dataKey="homo"
                  name="HOMO Level"
                  unit=" eV"
                  domain={[-6.5, -5.0]}
                  label={{ value: 'HOMO Level (eV)', position: 'bottom', offset: 0 }}
                />
                <YAxis
                  type="number"
                  dataKey="eqe"
                  name="EQE"
                  unit="%"
                  domain={[0, 30]}
                  label={{ value: 'External Quantum Efficiency (%)', angle: -90, position: 'insideLeft' }}
                />
                <Tooltip cursor={{ strokeDasharray: '3 3' }} />

                {/* Highlight White Space */}
                <ReferenceArea x1={-6.0} x2={-5.5} y1={20} y2={28} stroke="none" fill="#10b981" fillOpacity={0.1} />

                <Scatter name="Patented Molecules" data={data.filter(d => d.patented)} fill="#94a3b8" shape="circle" />
                <Scatter name="Potential Candidates" data={data.filter(d => !d.patented)} fill="#3b82f6" shape="star" />
              </ScatterChart>
            </ResponsiveContainer>

            <div className="absolute top-4 right-4 bg-white/90 p-2 rounded border border-slate-200 text-xs shadow-sm">
              <div className="flex items-center gap-2 mb-1">
                <span className="w-3 h-3 rounded-full bg-slate-400"></span> Patented
              </div>
              <div className="flex items-center gap-2 mb-1">
                <span className="w-3 h-3 text-blue-500 font-bold">★</span> Candidate
              </div>
              <div className="flex items-center gap-2">
                <span className="w-3 h-3 bg-green-100 border border-green-200 block"></span> White Space
              </div>
            </div>
          </div>
        ) : (
          <div className="flex-1 flex flex-col items-center justify-center text-slate-400 bg-slate-50/50 rounded-lg border-2 border-dashed border-slate-200">
            <p>Select domain and click Analyze to visualize molecular space.</p>
          </div>
        )}
      </Card>
    </div>
  );
};

export default WhiteSpaceDiscovery;
