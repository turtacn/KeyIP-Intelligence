import React, { useState } from 'react';
import { usePortfolio } from '../../hooks/usePortfolio';
import { usePartners } from '../../hooks/usePartner';
import { usePatents } from '../../hooks/usePatents';
import PanoramaView from './PanoramaView';
import CoverageTreemap from './CoverageTreemap';
import GapAnalysis from './GapAnalysis';
import ValueScoring from './ValueScoring';
import BudgetOptimizer from './BudgetOptimizer';
import WhatIfSimulator from './WhatIfSimulator';
import LoadingSpinner from '../../components/ui/LoadingSpinner';

const PortfolioOptimizer: React.FC = () => {
  const { summary, scores, coverage } = usePortfolio();
  const { data: companies } = usePartners();
  const { data: patents } = usePatents();

  const [activeSection, setActiveSection] = useState('panorama');

  const isLoading = summary.loading || scores.loading || coverage.loading;

  const scrollToSection = (id: string) => {
    setActiveSection(id);
    const element = document.getElementById(id);
    if (element) {
      element.scrollIntoView({ behavior: 'smooth', block: 'start' });
    }
  };

  if (isLoading) {
    return (
      <div className="flex justify-center items-center h-full">
        <LoadingSpinner size="lg" />
      </div>
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
            { id: 'panorama', label: 'Portfolio Panorama' },
            { id: 'gap', label: 'Competitive Gap' },
            { id: 'scoring', label: 'Value Scoring' },
            { id: 'budget', label: 'Budget Optimizer' },
            { id: 'simulator', label: 'What-If Simulator' },
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
              <h2 className="text-xl font-bold text-slate-900 mb-2">Portfolio Panorama</h2>
              <p className="text-slate-500">Overview of portfolio health, value, and domain coverage.</p>
            </div>
            <PanoramaView summary={summary.data} loading={summary.loading} />
            <div className="mt-6">
              <CoverageTreemap data={coverageData} scores={scoresData} />
            </div>
          </div>

          {/* Section 2: Gap Analysis */}
          <div id="gap" className="scroll-mt-6">
            <div className="mb-6">
              <h2 className="text-xl font-bold text-slate-900 mb-2">Competitive Gap Analysis</h2>
              <p className="text-slate-500">Compare patent coverage against key competitors across technology domains.</p>
            </div>
            <div className="h-[500px]">
              <GapAnalysis companies={companies || []} />
            </div>
          </div>

          {/* Section 3: Value Scoring */}
          <div id="scoring" className="scroll-mt-6">
            <div className="mb-6">
              <h2 className="text-xl font-bold text-slate-900 mb-2">Patent Value Scoring</h2>
              <p className="text-slate-500">AI-driven valuation of individual patent assets based on technical, legal, and commercial factors.</p>
            </div>
            <ValueScoring patents={patents || []} />
          </div>

          {/* Section 4: Budget */}
          <div id="budget" className="scroll-mt-6">
            <div className="mb-6">
              <h2 className="text-xl font-bold text-slate-900 mb-2">Budget Optimization</h2>
              <p className="text-slate-500">Analyze maintenance costs and identify savings opportunities.</p>
            </div>
            <div className="h-[500px]">
              <BudgetOptimizer />
            </div>
          </div>

          {/* Section 5: Simulator */}
          <div id="simulator" className="scroll-mt-6">
            <div className="mb-6">
              <h2 className="text-xl font-bold text-slate-900 mb-2">What-If Simulator</h2>
              <p className="text-slate-500">Simulate strategic moves and their impact on portfolio metrics.</p>
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
