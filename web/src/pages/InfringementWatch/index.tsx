import React, { useState, useEffect } from 'react';
import { useInfringement } from '../../hooks/useInfringement';
import AlertList from './AlertList';
import AlertDetail from './AlertDetail';
import MoleculeComparison from './MoleculeComparison';
import ClaimAnalysis from './ClaimAnalysis';
import RiskActions from './RiskActions';
import LiveFeed from './LiveFeed';
import EmptyState from '../../components/ui/EmptyState';
import { ShieldAlert } from 'lucide-react';
import { RiskLevel } from '../../types/domain';

const InfringementWatch: React.FC = () => {
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
    return <div className="text-red-600 p-8">Error: {error}</div>;
  }

  return (
    <div className="flex h-[calc(100vh-8rem)] overflow-hidden bg-white border border-slate-200 rounded-lg shadow-sm">
      {/* Left Panel: Alert List */}
      <div className="w-2/5 border-r border-slate-200 bg-slate-50 flex flex-col min-w-[320px]">
        <AlertList
          alerts={alerts || []}
          loading={loading}
          selectedAlertId={selectedAlertId}
          onSelectAlert={setSelectedAlertId}
          onRefresh={refetch}
          onFilterRisk={setRiskFilter}
          currentRiskFilter={riskFilter}
        />
      </div>

      {/* Right Panel: Detail View */}
      <div className="w-3/5 flex flex-col bg-white overflow-y-auto relative">
        {loading && !selectedAlert ? (
          <div className="flex items-center justify-center h-full">
            <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
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
            <EmptyState
              icon={ShieldAlert}
              title="No Alert Selected"
              description="Select an infringement alert from the list to view detailed analysis."
            />
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
