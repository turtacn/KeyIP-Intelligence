import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortfolio } from '../../hooks/usePortfolio';
import { usePartners } from '../../hooks/usePartner';
import { usePatents } from '../../hooks/usePatents';
import { usePortfolioConstellation } from '../../hooks/usePortfolioConstellation';
import PanoramaView from './PanoramaView';
import GapAnalysis from './GapAnalysis';
import ValueScoring from './ValueScoring';
import BudgetOptimizer from './BudgetOptimizer';
import WhatIfSimulator from './WhatIfSimulator';
import ConstellationMap from '../../components/visualization/ConstellationMap';
import PageError from '../../components/ui/PageError';
import EmptyState from '../../components/ui/EmptyState';
import { SkeletonCard } from '../../components/ui/Skeleton';
import { BarChart3 } from 'lucide-react';

const PortfolioOptimizer: React.FC = () => {
  const { t } = useTranslation();
  const { summary, scores, coverage } = usePortfolio();
  const { data: companies, loading: partnersLoading, error: partnersError } = usePartners();
  const { data: patents, loading: patentsLoading, error: patentsError } = usePatents();

  const portfolioId = (summary.data as { id?: string } | null)?.id;
  const { data: constellation, loading: constellationLoading, error: constellationError, refetch: refetchConstellation } = usePortfolioConstellation(portfolioId);

  const [activeSection, setActiveSection] = useState('panorama');

  const isLoading = summary.loading || scores.loading || coverage.loading || partnersLoading || patentsLoading || constellationLoading;
  const error = summary.error || scores.error || coverage.error || partnersError || patentsError || constellationError;

  const scrollToSection = (id: string) => {
    setActiveSection(id);
    const element = document.getElementById(id);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };

  if (isLoading) {
    return (
      <div className="space-y-6 pb-12">
        {/* Sub-nav skeleton */}
        <div className="bg-white border-b border-slate-200 rounded-lg p-4 animate-pulse">
          <div className="flex gap-8">
            {[1, 2, 3, 4, 5].map((i) => (
              <div key={i} className="h-4 w-20 bg-slate-200 rounded" />
            ))}
          </div>
        </div>

        {/* Section skeletons */}
        {[1, 2, 3].map((section) => (
          <div key={section} className="space-y-4">
            <div className="animate-pulse space-y-2">
              <div className="h-6 w-48 bg-slate-200 rounded" />
              <div className="h-4 w-72 bg-slate-200 rounded" />
            </div>
            <SkeletonCard rows={4} className="h-64" />
          </div>
        ))}
      </div>
    );
  }

  if (error) {
    return (
      <PageError
        error={error}
        onRetry={summary.refetch}
        title={t('portfolio.error_title', 'Failed to load portfolio data')}
        description={t('portfolio.error_desc', 'There was a problem fetching your portfolio. Please try again.')}
      />
    );
  }

  if (!summary.data) {
    return (
      <EmptyState
        icon={BarChart3}
        title={t('portfolio.empty_title', 'No portfolio data')}
        description={t('portfolio.empty_desc', 'No portfolio metrics are available.')}
        action={
          <button
            onClick={summary.refetch}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
          >
            {t('portfolio.refetch', 'Refresh')}
          </button>
        }
      />
    );
  }

  // Type assertion for mock data structure compatibility
  const coverageData = (coverage.data || {}) as { [key: string]: number };
  const scoresData = (scores.data || {}) as unknown as { [key: string]: number };

  return (
    <div className="flex flex-col h-[calc(100vh-8rem)]">
      {/* Sticky Sub-nav */}
      <div className="bg-white border-b border-slate-200 sticky top-0 z-10 mb-6">
        <nav className="flex space-x-8 px-1 overflow-x-auto" aria-label="Portfolio Sections">
          {[
            { id: 'panorama', label: t('portfolio.nav.panorama') },
            { id: 'constellation', label: 'Constellation' },
            { id: 'gap', label: t('portfolio.nav.gap') },
            { id: 'scoring', label: t('portfolio.nav.scoring') },
            { id: 'budget', label: t('portfolio.nav.budget') },
            { id: 'simulator', label: t('portfolio.nav.simulator') },
          ].map((item) => (
            <button
              key={item.id}
              onClick={() => scrollToSection(item.id)}
              className={`
                whitespace-nowrap py-4 px-1 border-b-2 font-medium text-sm transition-colors
                ${activeSection === item.id
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-slate-500 hover:text-slate-700 hover:border-slate-300'
                }
              `}
            >
              {item.label}
            </button>
          ))}
        </nav>
      </div>

      <div className="flex-1 overflow-y-auto pb-12 scroll-smooth">
        <div className="space-y-12">
          {/* Section 1: Panorama */}
          <div id="panorama" className="scroll-mt-6">
            <div className="mb-6">
              <h2 className="text-xl font-bold text-slate-900 mb-2">{t('portfolio.panorama.title')}</h2>
              <p className="text-slate-500">{t('portfolio.panorama.desc')}</p>
            </div>
            <PanoramaView
              summary={summary.data}
              loading={false}
              coverageData={coverageData}
              scoresData={scoresData}
            />
          </div>

          {/* Section 2: Constellation Map */}
          <div id="constellation" className="scroll-mt-6">
            <div className="mb-6">
              <h2 className="text-xl font-bold text-slate-900 mb-2">Patent Constellation Map</h2>
              <p className="text-slate-500">
                Explore patent positioning in molecular space. Self-owned patents (green) are shown alongside competitor patents (red). Bubble size reflects patent value score.
              </p>
            </div>
            <ConstellationMap
              data={constellation}
              loading={constellationLoading}
              error={constellationError}
              onRetry={refetchConstellation}
            />
          </div>

          {/* Section 3: Gap Analysis */}
          <div id="gap" className="scroll-mt-6">
            <div className="mb-6">
              <h2 className="text-xl font-bold text-slate-900 mb-2">{t('portfolio.gap.title')}</h2>
              <p className="text-slate-500">Compare patent coverage against key competitors across technology domains.</p>
            </div>
            <div className="h-[500px]">
              <GapAnalysis companies={companies || []} />
            </div>
          </div>

          {/* Section 4: Value Scoring */}
          <div id="scoring" className="scroll-mt-6">
            <div className="mb-6">
              <h2 className="text-xl font-bold text-slate-900 mb-2">{t('portfolio.scoring.title')}</h2>
              <p className="text-slate-500">AI-driven valuation of individual patent assets based on technical, legal, and commercial factors.</p>
            </div>
            <ValueScoring patents={patents || []} />
          </div>

          {/* Section 5: Budget */}
          <div id="budget" className="scroll-mt-6">
            <div className="mb-6">
              <h2 className="text-xl font-bold text-slate-900 mb-2">{t('portfolio.budget.title')}</h2>
              <p className="text-slate-500">{t('portfolio.budget.desc')}</p>
            </div>
            <div className="h-[500px]">
              <BudgetOptimizer />
            </div>
          </div>

          {/* Section 6: Simulator */}
          <div id="simulator" className="scroll-mt-6">
            <div className="mb-6">
              <h2 className="text-xl font-bold text-slate-900 mb-2">{t('portfolio.simulator.title')}</h2>
              <p className="text-slate-500">{t('portfolio.simulator.desc')}</p>
            </div>
            <div className="h-[500px]">
              <WhatIfSimulator />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default PortfolioOptimizer;
