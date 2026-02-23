import React, { useState } from 'react';
import Card from '../../components/ui/Card';
import { Search, Loader2 } from 'lucide-react';

const NLQueryWidget: React.FC = () => {
  const [query, setQuery] = useState('');
  const [loading, setLoading] = useState(false);
  const [response, setResponse] = useState<string | null>(null);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;

    setLoading(true);
    setResponse(null);

    // Simulate AI delay
    setTimeout(() => {
      setLoading(false);
      // Mock response logic
      if (query.toLowerCase().includes('filed') && query.toLowerCase().includes('us')) {
        setResponse("Based on current portfolio data, 12 patents were filed in the United States in the past 12 months, representing a 23% increase compared to the prior period.");
      } else if (query.toLowerCase().includes('risk') || query.toLowerCase().includes('infringement')) {
        setResponse("There are currently 4 high-risk infringement alerts detected, primarily involving Samsung SDI and LG Chem patents in the blue emitter domain.");
      } else if (query.toLowerCase().includes('expire') || query.toLowerCase().includes('deadline')) {
        setResponse("You have 7 patents with annuities due in the next 30 days. The total estimated cost is $12,500.");
      } else {
        setResponse("I analyzed your portfolio data but couldn't find a specific answer to that. Try asking about filing trends, infringement risks, or upcoming deadlines.");
      }
    }, 1500);
  };

  return (
    <Card className="bg-gradient-to-r from-slate-900 to-slate-800 text-white">
      <div className="flex flex-col md:flex-row items-start md:items-center justify-between gap-6">
        <div className="flex-1 w-full">
          <h3 className="text-lg font-semibold mb-2 flex items-center">
            <span className="bg-blue-500/20 text-blue-400 p-1.5 rounded-lg mr-2">AI</span>
            Ask KeyIP Intelligence
          </h3>
          <form onSubmit={handleSubmit} className="relative">
            <input
              type="text"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Ask about your patent portfolio, e.g., 'How many US patents were filed last year?'"
              className="w-full bg-slate-800/50 border border-slate-600 rounded-lg pl-4 pr-12 py-3 text-white placeholder-slate-400 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <button
              type="submit"
              disabled={loading || !query.trim()}
              className="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 bg-blue-600 rounded-md text-white hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Search className="w-4 h-4" />}
            </button>
          </form>
        </div>

        {response && (
          <div className="flex-1 w-full bg-slate-800/80 rounded-lg p-4 border border-slate-700 animate-in fade-in slide-in-from-top-2 duration-300">
            <p className="text-slate-300 text-sm leading-relaxed">
              <span className="font-semibold text-blue-400 block mb-1">Answer:</span>
              {response}
            </p>
          </div>
        )}
      </div>
    </Card>
  );
};

export default NLQueryWidget;
