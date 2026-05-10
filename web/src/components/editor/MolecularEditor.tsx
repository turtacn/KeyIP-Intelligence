import React, { useState } from 'react';
import SMILESEditor from './SMILESEditor';
import FormulaEditor from './FormulaEditor';

// ---------------------------------------------------------------------------
// Tabs
// ---------------------------------------------------------------------------

type EditorTab = 'smiles' | 'formula';

const TABS: { id: EditorTab; label: string; icon: React.ReactNode }[] = [
  {
    id: 'smiles',
    label: 'SMILES Editor',
    icon: (
      <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <circle cx="12" cy="12" r="10" />
        <line x1="2" y1="12" x2="22" y2="12" />
        <path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
      </svg>
    ),
  },
  {
    id: 'formula',
    label: 'Formula Editor',
    icon: (
      <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 2L2 7l10 5 10-5-10-5z" />
        <path d="M2 17l10 5 10-5" />
        <path d="M2 12l10 5 10-5" />
      </svg>
    ),
  },
];

// ---------------------------------------------------------------------------
// Component
// ---------------------------------------------------------------------------

const MolecularEditor: React.FC = () => {
  const [activeTab, setActiveTab] = useState<EditorTab>('smiles');

  return (
    <div className="h-full flex flex-col">
      {/* Tab bar */}
      <div className="flex gap-0.5 mb-4 bg-slate-100 dark:bg-slate-700/50 rounded-lg p-0.5">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            type="button"
            onClick={() => setActiveTab(tab.id)}
            className={`
              flex items-center gap-1.5 flex-1 px-3 py-2 text-xs font-medium rounded-md transition-all
              ${
                activeTab === tab.id
                  ? 'bg-white dark:bg-slate-600 text-blue-600 dark:text-blue-400 shadow-sm'
                  : 'text-slate-500 dark:text-slate-400 hover:text-slate-700 dark:hover:text-slate-300 hover:bg-white/50 dark:hover:bg-slate-600/50'
              }
            `}
          >
            {tab.icon}
            <span>{tab.label}</span>
          </button>
        ))}
      </div>

      {/* Editor content */}
      <div className="flex-1 overflow-y-auto">
        {activeTab === 'smiles' && (
          <div className="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-lg p-4">
            <SMILESEditor />
          </div>
        )}
        {activeTab === 'formula' && (
          <div className="bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-600 rounded-lg p-4">
            <FormulaEditor />
          </div>
        )}
      </div>
    </div>
  );
};

export default MolecularEditor;
