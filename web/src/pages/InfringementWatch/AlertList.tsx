import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import RiskLevelBadge from '../../components/ui/RiskLevelBadge';
import { InfringementAlert, RiskLevel } from '../../types/domain';
import { Search, RotateCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';

interface AlertListProps {
  alerts: InfringementAlert[];
  loading: boolean;
  selectedAlertId: string | null;
  onSelectAlert: (id: string) => void;
  onRefresh: () => void;
  onFilterRisk: (risk: RiskLevel | 'All') => void;
  currentRiskFilter: RiskLevel | 'All';
}

const AlertList: React.FC<AlertListProps> = ({
  alerts,
  loading,
  selectedAlertId,
  onSelectAlert,
  onRefresh,
  onFilterRisk,
  currentRiskFilter
}) => {
  const { t } = useTranslation();
  const [searchTerm, setSearchTerm] = useState('');

  const filteredAlerts = alerts.filter(alert =>
    alert.targetPatentId.toLowerCase().includes(searchTerm.toLowerCase()) ||
    alert.triggerMoleculeId.toLowerCase().includes(searchTerm.toLowerCase())
  );

  return (
    <Card padding="none" className="h-[calc(100vh-10rem)] flex flex-col">
      <div className="p-4 border-b border-slate-100 bg-slate-50 space-y-3">
        <div className="flex justify-between items-center">
          <h2 className="font-semibold text-slate-800 flex items-center">
            {t('infringement.alerts_label')}
            <span className="ml-2 bg-slate-200 text-slate-600 px-2 py-0.5 rounded-full text-xs">
              {alerts.length}
            </span>
          </h2>
          <button
            onClick={onRefresh}
            className="p-1.5 text-slate-500 hover:text-blue-600 hover:bg-white rounded-full transition-colors"
            title="Refresh alerts"
          >
            <RotateCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>

        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400 w-4 h-4" />
          <input
            type="text"
            placeholder="Search patent or molecule..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-9 pr-3 py-2 text-sm border border-slate-300 rounded-lg focus:ring-1 focus:ring-blue-500 focus:border-blue-500"
          />
        </div>

        <div className="flex gap-1 overflow-x-auto pb-1 scrollbar-hide">
          {['All', 'HIGH', 'MEDIUM', 'LOW'].map((risk) => (
            <button
              key={risk}
              onClick={() => onFilterRisk(risk as RiskLevel | 'All')}
              className={`
                px-3 py-1 text-xs font-medium rounded-full border whitespace-nowrap transition-colors
                ${currentRiskFilter === risk
                  ? 'bg-slate-800 text-white border-slate-800'
                  : 'bg-white text-slate-600 border-slate-200 hover:bg-slate-50'
                }
              `}
            >
              {risk === 'All' ? 'All Risks' : risk}
            </button>
          ))}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        {loading && alerts.length === 0 ? (
          <div className="flex justify-center items-center h-32">
            <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-600"></div>
          </div>
        ) : filteredAlerts.length === 0 ? (
          <div className="p-8 text-center text-slate-500 text-sm">
            No alerts found matching your criteria.
          </div>
        ) : (
          <div className="divide-y divide-slate-100">
            {filteredAlerts.map((alert) => (
              <div
                key={alert.id}
                onClick={() => onSelectAlert(alert.id)}
                className={`
                  p-4 cursor-pointer hover:bg-slate-50 transition-colors border-l-4
                  ${selectedAlertId === alert.id
                    ? 'bg-blue-50/50 border-blue-500'
                    : 'border-transparent'
                  }
                `}
              >
                <div className="flex justify-between items-start mb-2">
                  <RiskLevelBadge level={alert.riskLevel} />
                  <span className="text-xs text-slate-400">
                    {new Date(alert.detectedAt).toLocaleDateString()}
                  </span>
                </div>
                <h4 className="font-medium text-slate-900 text-sm mb-1">
                  {alert.targetPatentId}
                </h4>
                <div className="flex justify-between items-center text-xs text-slate-500">
                  <span>{t('infringement.comparison.trigger')}: {alert.triggerMoleculeId}</span>
                  <div className="flex items-center gap-1">
                     <span className="font-medium">{(alert.literalScore * 100).toFixed(0)}%</span> {t('infringement.claims.match')}
                  </div>
                </div>
                {/* Score Bar */}
                <div className="mt-2 h-1.5 w-full bg-slate-100 rounded-full overflow-hidden">
                  <div
                    className={`h-full rounded-full ${
                      alert.riskLevel === 'HIGH' ? 'bg-red-500' :
                      alert.riskLevel === 'MEDIUM' ? 'bg-amber-500' : 'bg-blue-500'
                    }`}
                    style={{ width: `${alert.literalScore * 100}%` }}
                  ></div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </Card>
  );
};

export default AlertList;
