import React, { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import PatentabilityAssessor from './PatentabilityAssessor';
import WhiteSpaceDiscovery from './WhiteSpaceDiscovery';
import PatentSearch from './PatentSearch';
import PriorArtAnalysis from './PriorArtAnalysis';
import ClaimDraftAssistant from './ClaimDraftAssistant';
import ErrorBoundary from '../../components/ui/ErrorBoundary';
import EmptyState from '../../components/ui/EmptyState';
import { Search, Map, FileSearch, Scale, PenTool, Wrench } from 'lucide-react';

const PatentMining: React.FC = () => {
  const { t } = useTranslation();
  const [activeTool, setActiveTool] = useState('assessment');

  const tools = useMemo(() => [
    { id: 'assessment', label: t('mining.tool_assessment'), icon: Scale, component: <PatentabilityAssessor /> },
    { id: 'whitespace', label: t('mining.tool_whitespace'), icon: Map, component: <WhiteSpaceDiscovery /> },
    { id: 'search', label: t('mining.tool_search'), icon: Search, component: <PatentSearch /> },
    { id: 'priorart', label: t('mining.tool_priorart'), icon: FileSearch, component: <PriorArtAnalysis /> },
    { id: 'drafting', label: t('mining.tool_drafting'), icon: PenTool, component: <ClaimDraftAssistant /> },
  ], [t]);

  const activeToolConfig = tools.find(t => t.id === activeTool);

  return (
    <div className="flex flex-col lg:flex-row gap-6 h-[calc(100vh-8rem)]">
      {/* Tool Selector */}
      <div className="w-full lg:w-64 flex-shrink-0 bg-white border border-slate-200 rounded-lg shadow-sm overflow-hidden h-fit">
        <div className="p-4 bg-slate-50 border-b border-slate-200 font-semibold text-slate-800">
          {t('mining.tools_nav')}
        </div>
        <nav className="p-2 space-y-1">
          {tools.map((tool) => (
            <button
              key={tool.id}
              onClick={() => setActiveTool(tool.id)}
              className={`
                w-full flex items-center px-4 py-3 text-sm font-medium rounded-md transition-colors
                ${activeTool === tool.id
                  ? 'bg-blue-50 text-blue-700'
                  : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'
                }
              `}
            >
              <tool.icon className={`w-5 h-5 mr-3 ${activeTool === tool.id ? 'text-blue-600' : 'text-slate-400'}`} />
              {tool.label}
            </button>
          ))}
        </nav>
      </div>

      {/* Workspace */}
      <div className="flex-1 min-w-0 h-full overflow-y-auto">
        {activeToolConfig ? (
          <ErrorBoundary
            fallback={
              <div className="flex items-center justify-center h-full p-8">
                <EmptyState
                  icon={Wrench}
                  title={t('mining.tool_error_title', 'Tool Error')}
                  description={t('mining.tool_error_desc', 'This tool encountered an error. Please try selecting it again.')}
                  action={
                    <button
                      onClick={() => setActiveTool(activeTool === 'assessment' ? 'whitespace' : 'assessment')}
                      className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 transition-colors"
                    >
                      {t('mining.switch_tool', 'Switch Tool')}
                    </button>
                  }
                />
              </div>
            }
          >
            {activeToolConfig.component}
          </ErrorBoundary>
        ) : (
          <EmptyState
            icon={Search}
            title={t('mining.no_tool_title', 'Select a tool')}
            description={t('mining.no_tool_desc', 'Choose a patent mining tool from the sidebar to get started.')}
          />
        )}
      </div>
    </div>
  );
};

export default PatentMining;
