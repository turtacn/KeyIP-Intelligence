import React, { useState } from 'react';
import PatentabilityAssessor from './PatentabilityAssessor';
import WhiteSpaceDiscovery from './WhiteSpaceDiscovery';
import PatentSearch from './PatentSearch';
import PriorArtAnalysis from './PriorArtAnalysis';
import ClaimDraftAssistant from './ClaimDraftAssistant';
import { Search, Map, FileSearch, Scale, PenTool } from 'lucide-react';

const PatentMining: React.FC = () => {
  const [activeTool, setActiveTool] = useState('assessment');

  const tools = [
    { id: 'assessment', label: 'Patentability Assessment', icon: Scale, component: <PatentabilityAssessor /> },
    { id: 'whitespace', label: 'White Space Discovery', icon: Map, component: <WhiteSpaceDiscovery /> },
    { id: 'search', label: 'Patent Search', icon: Search, component: <PatentSearch /> },
    { id: 'priorart', label: 'Prior Art Analysis', icon: FileSearch, component: <PriorArtAnalysis /> },
    { id: 'drafting', label: 'Claim Draft Assistant', icon: PenTool, component: <ClaimDraftAssistant /> },
  ];

  return (
    <div className="flex flex-col lg:flex-row gap-6 h-[calc(100vh-8rem)]">
      {/* Tool Selector */}
      <div className="w-full lg:w-64 flex-shrink-0 bg-white border border-slate-200 rounded-lg shadow-sm overflow-hidden h-fit">
        <div className="p-4 bg-slate-50 border-b border-slate-200 font-semibold text-slate-800">
          Mining Tools
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
        {tools.find(t => t.id === activeTool)?.component}
      </div>
    </div>
  );
};

export default PatentMining;
