import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { useInfringement } from '../../hooks/useInfringement';
import AlertList from './AlertList';
import AlertDetail from './AlertDetail';
import MoleculeComparison from './MoleculeComparison';
import ClaimAnalysis from './ClaimAnalysis';
import RiskActions from './RiskActions';
import LiveFeed from './LiveFeed';
import PageError from '../../components/ui/PageError';
import EmptyState from '../../components/ui/EmptyState';
import { SkeletonTable, SkeletonCard } from '../../components/ui/Skeleton';
import { ShieldAlert, AlertTriangle } from 'lucide-react';
import { RiskLevel } from '../../types/domain';

const InfringementWatch: React.FC = () => {
  const { t } = useTranslation();
  const [selectedAlertId, setSelectedAlertId] = useState<string | null>(null);
  const [riskFilter, setRiskFilter] = useState<RiskLevel | 'All'>('All');

  const { data: alerts, loading, error, refetch } = useInfringement(
    riskFilter === 'All' ? undefined : riskFilter
  );

  useEffect(() => {
    // Select first alert by default if loaded and none selected
    if (!loading && alerts && alerts.length > 0 && !selectedAlertId) {
      setSelectedAlertId(alerts[0].id);
    }
  }, [loading, alerts, selectedAlertId]);

  const selectedAlert = alerts?.find(a => a.id === selectedAlertId);

  const handleMarkReviewed = (id: string) => {
    // Optimistic update logic (mock)
    console.log('Marked reviewed:', id);
    refetch();
  };

  if (error) {
    return (
      <PageError
        error={error}
        onRetry={refetch}
        title={t('infringement.error_title', 'Failed to load alerts')}
        description={t('infringement.error_desc', 'There was a problem fetching infringement alerts.')}
      />
    );
  }

  return (
    <div className="flex h-[calc(100vh-8rem)] overflow-hidden bg-white border border-slate-200 rounded-lg shadow-sm">
      {/* Left Panel: Alert List */}
      <div className="w-2/5 border-r border-slate-200 bg-slate-50 flex flex-col min-w-[320px]">
        {loading && !alerts ? (
          <div className="p-4 space-y-3">
            <div className="animate-pulse h-8 w-full bg-slate-200 rounded-lg mb-4" />
            <SkeletonTable columns={2} rows={6} />
          </div>
        ) : (
          <AlertList
            alerts={alerts || []}
            loading={loading}
            selectedAlertId={selectedAlertId}
            onSelectAlert={setSelectedAlertId}
            onRefresh={refetch}
            onFilterRisk={setRiskFilter}
            currentRiskFilter={riskFilter}
          />
        )}
      </div>

      {/* Right Panel: Detail View */}
      <div className="w-3/5 flex flex-col bg-white overflow-y-auto relative">
        {loading && !selectedAlert ? (
          <div className="p-6 space-y-6">
            <SkeletonCard rows={3} />
            <SkeletonCard rows={4} />
            <SkeletonCard rows={3} />
          </div>
        ) : selectedAlert ? (
          <div className="p-6 pb-24 space-y-6"> {/* Padding bottom for LiveFeed */}
            <AlertDetail alert={selectedAlert} />

            <MoleculeComparison
              triggerMoleculeId={selectedAlert.triggerMoleculeId}
              similarityScore={selectedAlert.literalScore}
            />

            <ClaimAnalysis alert={selectedAlert} />

            <RiskActions
              alert={selectedAlert}
              onMarkReviewed={handleMarkReviewed}
            />
          </div>
        ) : (
          <div className="flex items-center justify-center h-full">
            {!error && !loading ? (
              <EmptyState
                icon={ShieldAlert}
                title={t('infringement.no_alerts_title', 'No Alerts')}
                description={t('infringement.no_alerts_desc', 'No infringement alerts match your current filter.')}
                action={
                  <button
                    onClick={() => setRiskFilter('All')}
                    className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
                  >
                    {t('infringement.clear_filter', 'Clear Filter')}
                  </button>
                }
              />
            ) : (
              <EmptyState
                icon={AlertTriangle}
                title={t('infringement.title')}
                description={t('infringement.list_title')}
              />
            )}
          </div>
        )}
      </div>

      {/* Live Feed Overlay */}
      <div className="fixed bottom-0 left-64 right-0 z-20 pointer-events-none">
        {/* Pointer events none so it doesn't block clicks unless interacting with the feed itself which should have pointer-events-auto */}
        <div className="pointer-events-auto">
          <LiveFeed />
        </div>
      </div>
    </div>
  );
};

export default InfringementWatch;
