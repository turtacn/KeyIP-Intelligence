import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import Button from '../../components/ui/Button';
import { Play, TrendingUp, TrendingDown, RefreshCcw } from 'lucide-react';
import { useTranslation } from 'react-i18next';

const WhatIfSimulator: React.FC = () => {
  const { t } = useTranslation();
  const [scenarioName, setScenarioName] = useState('');
  const [actionType, setActionType] = useState('add_patent');
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<any | null>(null);

  const handleRun = () => {
    if (!scenarioName.trim()) return;
    setLoading(true);
    setTimeout(() => {
      setLoading(false);
      setResult({
        current: { coverage: 78, value: 45000000, risk: 12 },
        simulated: { coverage: 85, value: 52000000, risk: 8 },
        delta: { coverage: 7, value: 7000000, risk: -4 }
      });
    }, 2000);
  };

  return (
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 h-full">
      <Card className="lg:col-span-1 h-full flex flex-col">
        <h3 className="text-lg font-semibold text-slate-800 mb-4">{t('portfolio.simulator.builder_title')}</h3>
        <div className="space-y-4 flex-1">
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">{t('portfolio.simulator.name_label')}</label>
            <input
              type="text"
              value={scenarioName}
              onChange={(e) => setScenarioName(e.target.value)}
              placeholder={t('portfolio.simulator.name_placeholder')}
              className="w-full border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-slate-700 mb-1">{t('portfolio.simulator.action_label')}</label>
            <select
              value={actionType}
              onChange={(e) => setActionType(e.target.value)}
              className="w-full border-slate-300 rounded-lg text-sm focus:ring-blue-500 focus:border-blue-500"
            >
              <option value="add_patent">File New Patent (Blue Host)</option>
              <option value="acquire_competitor">Acquire Competitor Portfolio</option>
              <option value="abandon_low_value">Abandon Bottom 10% Patents</option>
              <option value="expire_key_patent">Key Patent Expiry</option>
            </select>
          </div>
        </div>
        <div className="mt-6 pt-4 border-t border-slate-100">
          <Button
            onClick={handleRun}
            isLoading={loading}
            disabled={!scenarioName.trim()}
            className="w-full"
            leftIcon={<Play className="w-4 h-4" />}
          >
            {t('portfolio.simulator.run_btn')}
          </Button>
        </div>
      </Card>

      <Card className="lg:col-span-2 h-full flex flex-col bg-slate-50/50">
        <h3 className="text-lg font-semibold text-slate-800 mb-4">{t('portfolio.simulator.results_title')}</h3>
        {result ? (
          <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
            <div className="grid grid-cols-3 gap-4 text-center">
              <div className="bg-white p-4 rounded-lg border border-slate-200">
                <div className="text-sm text-slate-500 mb-2">{t('portfolio.simulator.coverage')}</div>
                <div className="text-3xl font-bold text-slate-800">{result.simulated.coverage}%</div>
                <div className="text-sm font-medium text-green-600 flex items-center justify-center mt-1">
                  <TrendingUp className="w-3 h-3 mr-1" /> +{result.delta.coverage}%
                </div>
                <div className="text-xs text-slate-400 mt-2">{t('portfolio.simulator.current')}: {result.current.coverage}%</div>
              </div>

              <div className="bg-white p-4 rounded-lg border border-slate-200">
                <div className="text-sm text-slate-500 mb-2">{t('portfolio.simulator.value')}</div>
                <div className="text-3xl font-bold text-indigo-600">${(result.simulated.value / 1000000).toFixed(1)}M</div>
                <div className="text-sm font-medium text-green-600 flex items-center justify-center mt-1">
                  <TrendingUp className="w-3 h-3 mr-1" /> +${(result.delta.value / 1000000).toFixed(1)}M
                </div>
                <div className="text-xs text-slate-400 mt-2">{t('portfolio.simulator.current')}: ${(result.current.value / 1000000).toFixed(1)}M</div>
              </div>

              <div className="bg-white p-4 rounded-lg border border-slate-200">
                <div className="text-sm text-slate-500 mb-2">{t('portfolio.simulator.risk')}</div>
                <div className="text-3xl font-bold text-red-600">{result.simulated.risk}%</div>
                <div className="text-sm font-medium text-green-600 flex items-center justify-center mt-1">
                  <TrendingDown className="w-3 h-3 mr-1" /> {result.delta.risk}% {t('portfolio.simulator.improved')}
                </div>
                <div className="text-xs text-slate-400 mt-2">{t('portfolio.simulator.current')}: {result.current.risk}%</div>
              </div>
            </div>

            <div className="bg-blue-50 border border-blue-100 p-4 rounded-lg text-sm text-blue-800">
              <h4 className="font-semibold mb-2">{t('portfolio.simulator.ai_analysis_title')}</h4>
              <p>
                Adding a new Blue Host patent significantly improves portfolio coverage in the OLED domain (+7%) and increases overall valuation. Risk exposure decreases due to better defensive positioning against competitors in the blue emitter space.
              </p>
            </div>
          </div>
        ) : (
          <div className="flex-1 flex flex-col items-center justify-center text-slate-400 text-sm">
            <RefreshCcw className="w-12 h-12 mb-4 opacity-20" />
            <p>{t('portfolio.simulator.hint')}</p>
          </div>
        )}
      </Card>
    </div>
  );
};

export default WhatIfSimulator;
