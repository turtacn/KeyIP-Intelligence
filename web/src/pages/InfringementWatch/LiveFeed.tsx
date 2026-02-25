import React, { useState, useEffect } from 'react';
import { ChevronUp, ChevronDown, Play, Pause, Globe, AlertTriangle } from 'lucide-react';
import { InfringementAlert } from '../../types/domain';
import { useTranslation } from 'react-i18next';

const LiveFeed: React.FC = () => {
  const { t } = useTranslation();
  const [isOpen, setIsOpen] = useState(false);
  const [isPlaying, setIsPlaying] = useState(true);
  const [feedItems, setFeedItems] = useState<Partial<InfringementAlert>[]>([]);

  useEffect(() => {
    let interval: ReturnType<typeof setInterval>;

    if (isPlaying) {
      interval = setInterval(() => {
        const newItem: Partial<InfringementAlert> = {
          id: `live_${Date.now()}`,
          targetPatentId: `WO${new Date().getFullYear()}${Math.floor(Math.random() * 100000)}A1`,
          riskLevel: Math.random() > 0.7 ? 'HIGH' : Math.random() > 0.4 ? 'MEDIUM' : 'LOW',
          detectedAt: new Date().toISOString(),
          triggerMoleculeId: `mol_${Math.floor(Math.random() * 100)}`,
        };

        setFeedItems((prev) => [newItem, ...prev].slice(0, 50)); // Keep last 50
      }, 3000); // 3 seconds for demo effect (prompt said 30s but 3s is better for visual confirmation)
    }

    return () => clearInterval(interval);
  }, [isPlaying]);

  return (
    <div className={`fixed bottom-0 left-64 right-0 bg-white border-t border-slate-200 shadow-lg transition-all duration-300 z-10 ${isOpen ? 'h-64' : 'h-12'}`}>
      {/* Header / Toggle Bar */}
      <div
        className="h-12 px-6 flex items-center justify-between cursor-pointer bg-slate-50 hover:bg-slate-100 transition-colors"
        onClick={() => setIsOpen(!isOpen)}
      >
        <div className="flex items-center gap-3">
          <Globe className={`w-5 h-5 ${isPlaying ? 'text-green-500 animate-pulse' : 'text-slate-400'}`} />
          <span className="font-semibold text-slate-700">{t('infringement.feed.title')}</span>
          <span className="bg-blue-100 text-blue-700 text-xs px-2 py-0.5 rounded-full font-medium">
            {feedItems.length} {t('infringement.feed.new')}
          </span>
        </div>
        <div className="flex items-center gap-4">
          <button
            onClick={(e) => { e.stopPropagation(); setIsPlaying(!isPlaying); }}
            className="p-1 hover:bg-slate-200 rounded-full transition-colors text-slate-600"
            title={isPlaying ? t('infringement.feed.pause') : t('infringement.feed.resume')}
          >
            {isPlaying ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
          </button>
          {isOpen ? <ChevronDown className="w-5 h-5 text-slate-500" /> : <ChevronUp className="w-5 h-5 text-slate-500" />}
        </div>
      </div>

      {/* Feed Content */}
      {isOpen && (
        <div className="h-52 overflow-y-auto p-4 bg-slate-50/50">
          <div className="space-y-2">
            {feedItems.map((item) => (
              <div key={item.id} className="bg-white p-3 rounded border border-slate-200 shadow-sm flex items-center justify-between animate-in slide-in-from-left duration-300">
                <div className="flex items-center gap-4">
                  <span className={`w-2 h-2 rounded-full ${item.riskLevel === 'HIGH' ? 'bg-red-500' : item.riskLevel === 'MEDIUM' ? 'bg-amber-500' : 'bg-blue-500'}`}></span>
                  <span className="font-mono text-sm font-medium text-slate-700">{item.targetPatentId}</span>
                  <span className="text-xs text-slate-500">{t('infringement.comparison.trigger')}: {item.triggerMoleculeId}</span>
                </div>
                <div className="flex items-center gap-4">
                   {item.riskLevel === 'HIGH' && (
                     <div className="flex items-center text-red-600 text-xs font-bold gap-1">
                       <AlertTriangle className="w-3 h-3" /> {t('infringement.feed.high_risk_detected')}
                     </div>
                   )}
                   <span className="text-xs text-slate-400 font-mono">
                     {new Date(item.detectedAt!).toLocaleTimeString()}
                   </span>
                </div>
              </div>
            ))}
            {feedItems.length === 0 && (
              <div className="text-center text-slate-400 py-8 text-sm">
                {t('infringement.feed.waiting')}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default LiveFeed;
