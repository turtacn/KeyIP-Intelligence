import React from 'react';
import Card from '../../components/ui/Card';
import StatusBadge from '../../components/ui/StatusBadge';
import { LifecycleEvent, Jurisdiction } from '../../types/domain';
import { useTranslation } from 'react-i18next';

interface LegalStatusMonitorProps {
  events: LifecycleEvent[];
  loading?: boolean;
}

const LegalStatusMonitor: React.FC<LegalStatusMonitorProps> = ({ events }) => {
  const { t } = useTranslation();
  // Mock grouping by patent (since events are flattened)
  const patents = Array.from(new Set(events.map(e => e.patentId)));

  const getStatusForJurisdiction = (patentId: string, jurisdiction: Jurisdiction) => {
    // In a real app, this would come from a Patent entity, not events
    // Mocking status based on events for now
    const event = events.find(e => e.patentId === patentId && e.jurisdiction === jurisdiction);
    return event ? event.status : 'active'; // Default mock
  };

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
      {patents.map(patentId => (
        <Card key={patentId} padding="none" className="overflow-hidden">
          <div className="px-6 py-4 border-b border-slate-100 bg-slate-50 flex justify-between items-center">
            <h3 className="font-semibold text-slate-800">{patentId}</h3>
            <StatusBadge status="active" label={t('lifecycle.status.active_family')} />
          </div>
          <div className="p-6 space-y-4">
            {['CN', 'US', 'EP', 'JP', 'KR'].map((jurisdiction) => (
              <div key={jurisdiction} className="flex justify-between items-center py-2 border-b border-slate-50 last:border-0">
                <div className="flex items-center space-x-3">
                  <span className="w-8 h-8 rounded-full bg-slate-100 flex items-center justify-center text-xs font-bold text-slate-600">
                    {jurisdiction}
                  </span>
                  <div>
                    <p className="text-sm font-medium text-slate-700">
                      {jurisdiction === 'CN' ? 'CNIPA' :
                       jurisdiction === 'US' ? 'USPTO' :
                       jurisdiction === 'EP' ? 'EPO' :
                       jurisdiction === 'JP' ? 'JPO' : 'KIPO'}
                    </p>
                    <p className="text-xs text-slate-400">{t('lifecycle.status.last_sync')}: {t('lifecycle.status.today')}</p>
                  </div>
                </div>
                <StatusBadge
                  status={getStatusForJurisdiction(patentId, jurisdiction as Jurisdiction) === 'completed' ? 'active' : 'pending'}
                  label={getStatusForJurisdiction(patentId, jurisdiction as Jurisdiction) === 'completed' ? t('portfolio.panorama.granted') : t('portfolio.panorama.pending')}
                  className="scale-90"
                />
              </div>
            ))}
          </div>
        </Card>
      ))}
    </div>
  );
};

export default LegalStatusMonitor;
